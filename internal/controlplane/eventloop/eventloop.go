package eventloop

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

type Config struct {
	Owner                     string
	Categories                []string
	Concurrency               int
	InfrastructureConcurrency int
	LeaseDuration             time.Duration
	PollInterval              time.Duration
	RetryBaseInterval         time.Duration
}

func DefaultConfig(owner string) Config {
	return Config{
		Owner: owner, Concurrency: 8, InfrastructureConcurrency: 4, LeaseDuration: 30 * time.Second,
		PollInterval: time.Second, RetryBaseInterval: 2 * time.Second,
	}
}

type Loop struct {
	queue      ports.QueueStore
	dispatcher *Dispatcher
	config     Config
	wakeUp     chan struct{}
}

func New(repository ports.QueueStore, dispatcher *Dispatcher, config Config) (*Loop, error) {
	if repository == nil || dispatcher == nil {
		return nil, fmt.Errorf("event loop requires queue repository and dispatcher")
	}
	if config.Owner == "" || config.Concurrency < 1 || config.InfrastructureConcurrency < 1 || config.LeaseDuration <= 0 || config.PollInterval <= 0 {
		return nil, fmt.Errorf("invalid event loop configuration")
	}
	if config.RetryBaseInterval <= 0 {
		config.RetryBaseInterval = time.Second
	}
	return &Loop{queue: repository, dispatcher: dispatcher, config: config, wakeUp: make(chan struct{}, 1)}, nil
}

func (l *Loop) Notify() {
	select {
	case l.wakeUp <- struct{}{}:
	default:
	}
}

func (l *Loop) Run(ctx context.Context) error {
	ticker := time.NewTicker(l.config.PollInterval)
	defer ticker.Stop()
	semaphore := make(chan struct{}, l.config.Concurrency)
	infrastructure := make(chan struct{}, l.config.InfrastructureConcurrency)
	infrastructureCategories, regularCategories := splitCategories(l.config.Categories)
	var running sync.WaitGroup

	defer func() {
		running.Wait()
	}()

	l.drain(ctx, infrastructure, &running, infrastructureCategories)
	l.drain(ctx, semaphore, &running, regularCategories)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			_, _ = l.queue.ReleaseExpired(ctx, time.Now().UTC())
			l.drain(ctx, infrastructure, &running, infrastructureCategories)
			l.drain(ctx, semaphore, &running, regularCategories)
		case <-l.wakeUp:
			l.drain(ctx, infrastructure, &running, infrastructureCategories)
			l.drain(ctx, semaphore, &running, regularCategories)
		}
	}
}

func (l *Loop) drain(ctx context.Context, semaphore chan struct{}, running *sync.WaitGroup, categories []string) {
	if len(categories) == 0 {
		return
	}
	available := cap(semaphore) - len(semaphore)
	if available <= 0 {
		return
	}
	jobs, err := l.queue.Lease(ctx, l.config.Owner, categories, available, l.config.LeaseDuration)
	if err != nil {
		return
	}
	for _, leased := range jobs {
		job := leased
		semaphore <- struct{}{}
		running.Add(1)
		go func() {
			defer func() {
				<-semaphore
				running.Done()
				l.Notify()
			}()
			l.handle(ctx, job)
		}()
	}
}

func nonInfrastructureCategories() []string {
	return []string{domainqueue.CategoryEnvironment, domainqueue.CategoryOrchestration, domainqueue.CategoryExecution, domainqueue.CategoryMonitoring, domainqueue.CategoryTransfer, domainqueue.CategoryMaintenance, domainqueue.CategoryPlanning}
}

func splitCategories(configured []string) ([]string, []string) {
	if len(configured) == 0 {
		return []string{domainqueue.CategoryInfrastructure}, nonInfrastructureCategories()
	}
	infrastructure, regular := []string{}, []string{}
	for _, category := range configured {
		if category == domainqueue.CategoryInfrastructure {
			infrastructure = append(infrastructure, category)
		} else {
			regular = append(regular, category)
		}
	}
	return infrastructure, regular
}

func (l *Loop) handle(ctx context.Context, job domainqueue.Job) {
	jobCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	leaseDone := make(chan struct{})
	go l.keepLeaseAlive(jobCtx, job, cancel, leaseDone)
	err := l.dispatcher.Dispatch(jobCtx, job)
	if errors.Is(err, context.Canceled) && context.Cause(jobCtx) != nil {
		err = fmt.Errorf("queue handler cancelled: %w", context.Cause(jobCtx))
	}
	cancel(nil)
	<-leaseDone
	now := time.Now().UTC()
	if err == nil {
		_ = l.queue.Complete(ctx, job.ID, l.config.Owner, now)
		return
	}
	backoff := l.config.RetryBaseInterval * time.Duration(1<<min(job.Attempts-1, 8))
	_ = l.queue.Retry(ctx, job.ID, l.config.Owner, err, now.Add(backoff))
}

func (l *Loop) keepLeaseAlive(ctx context.Context, job domainqueue.Job, cancel context.CancelCauseFunc, done chan<- struct{}) {
	defer close(done)
	interval := l.config.LeaseDuration / 3
	if interval < 10*time.Millisecond {
		interval = 10 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// A buffered tick can predate a stalled worker; never renew from that stale time.
			if err := l.queue.RenewLease(ctx, job.ID, l.config.Owner, time.Now().UTC().Add(l.config.LeaseDuration)); err != nil {
				cancel(fmt.Errorf("renew lease for queue job %q: %w", job.ID, err))
				return
			}
		}
	}
}
