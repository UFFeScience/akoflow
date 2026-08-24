package eventloop

import (
	"context"
	"database/sql"
	"sync/atomic"
	"testing"
	"time"

	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database/queue"
	_ "github.com/mattn/go-sqlite3"
)

func TestLoopDispatchesDurableJob(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := database.Bootstrap(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	repository, err := queue.New(db)
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := NewDispatcher()
	var calls atomic.Int32
	if err := dispatcher.Register("test", HandlerFunc(func(context.Context, domainqueue.Job) error {
		calls.Add(1)
		return nil
	})); err != nil {
		t.Fatal(err)
	}
	config := DefaultConfig("test-worker")
	config.PollInterval = 5 * time.Millisecond
	loop, err := New(repository, dispatcher, config)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- loop.Run(ctx) }()
	job, _ := domainqueue.New(domainqueue.CategoryExecution, "test", nil, time.Now())
	if _, err := repository.Publish(ctx, job); err != nil {
		t.Fatal(err)
	}
	loop.Notify()
	deadline := time.After(time.Second)
	for calls.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("event was not dispatched")
		case <-time.After(5 * time.Millisecond):
		}
	}
	for {
		stored, findErr := repository.FindByID(ctx, job.ID)
		if findErr != nil {
			t.Fatal(findErr)
		}
		if stored != nil && stored.Status == domainqueue.StatusCompleted {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("job did not complete: %+v", stored)
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestDispatcherRejectsDuplicatesAndUnknownEvents(t *testing.T) {
	dispatcher := NewDispatcher()
	handler := HandlerFunc(func(context.Context, domainqueue.Job) error { return nil })
	if err := dispatcher.Register("event", handler); err != nil {
		t.Fatal(err)
	}
	if err := dispatcher.Register("event", handler); err == nil {
		t.Fatal("duplicate must fail")
	}
	if err := dispatcher.Dispatch(context.Background(), domainqueue.Job{Type: "unknown"}); err == nil {
		t.Fatal("unknown event must fail")
	}
}

func TestLoopRenewsLeaseWhileHandlerIsRunning(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := database.Bootstrap(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	repository, err := queue.New(db)
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := NewDispatcher()
	started := make(chan struct{})
	release := make(chan struct{})
	if err := dispatcher.Register("slow", HandlerFunc(func(context.Context, domainqueue.Job) error {
		close(started)
		<-release
		return nil
	})); err != nil {
		t.Fatal(err)
	}
	config := DefaultConfig("lease-worker")
	config.PollInterval = 5 * time.Millisecond
	config.LeaseDuration = 60 * time.Millisecond
	loop, err := New(repository, dispatcher, config)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- loop.Run(ctx) }()
	job, _ := domainqueue.New(domainqueue.CategoryExecution, "slow", nil, time.Now())
	if _, err := repository.Publish(ctx, job); err != nil {
		t.Fatal(err)
	}
	loop.Notify()
	<-started
	time.Sleep(90 * time.Millisecond)
	stored, err := repository.FindByID(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != domainqueue.StatusLeased || stored.LeaseExpiresAt == nil || !stored.LeaseExpiresAt.After(time.Now()) {
		t.Fatalf("lease was not renewed: %+v", stored)
	}
	close(release)
	time.Sleep(20 * time.Millisecond)
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
