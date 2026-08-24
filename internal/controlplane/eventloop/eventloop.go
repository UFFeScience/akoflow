package eventloop

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

type Config struct {
	Owner             string
	Categories        []string
	Concurrency       int
	LeaseDuration     time.Duration
	PollInterval      time.Duration
	RetryBaseInterval time.Duration
}

func DefaultConfig(owner string) Config {
	return Config{
		Owner: owner, Concurrency: 8, LeaseDuration: 30 * time.Second,
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
	if config.Owner == "" || config.Concurrency < 1 || config.LeaseDuration <= 0 || config.PollInterval <= 0 {
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
	var running sync.WaitGroup

	defer func() {
		running.Wait()
	}()

	l.drain(ctx, semaphore, &running)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			_, _ = l.queue.ReleaseExpired(ctx, time.Now().UTC())
			l.drain(ctx, semaphore, &running)
		case <-l.wakeUp:
			l.drain(ctx, semaphore, &running)
		}
	}
}

func (l *Loop) drain(ctx context.Context, semaphore chan struct{}, running *sync.WaitGroup) {
	available := cap(semaphore) - len(semaphore)
	if available <= 0 {
		return
	}
	jobs, err := l.queue.Lease(ctx, l.config.Owner, l.config.Categories, available, l.config.LeaseDuration)
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

func (l *Loop) handle(ctx context.Context, job domainqueue.Job) {
	jobCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	leaseDone := make(chan struct{})
	go l.keepLeaseAlive(jobCtx, job, cancel, leaseDone)
	err := l.dispatcher.Dispatch(jobCtx, job)
	cancel()
	<-leaseDone
	now := time.Now().UTC()
	if err == nil {
		_ = l.queue.Complete(ctx, job.ID, l.config.Owner, now)
		return
	}
	backoff := l.config.RetryBaseInterval * time.Duration(1<<min(job.Attempts-1, 8))
	_ = l.queue.Retry(ctx, job.ID, l.config.Owner, err, now.Add(backoff))
}

func (l *Loop) keepLeaseAlive(ctx context.Context, job domainqueue.Job, cancel context.CancelFunc, done chan<- struct{}) {
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
		case now := <-ticker.C:
			if err := l.queue.RenewLease(ctx, job.ID, l.config.Owner, now.Add(l.config.LeaseDuration)); err != nil {
				cancel()
				return
			}
		}
	}
}
