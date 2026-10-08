package execution

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type independentStartController struct {
	activityControllerFake
	slowEntered chan struct{}
	releaseSlow chan struct{}
	successor   chan struct{}
	once        sync.Once
	lateHandle  bool
	failFast    bool
}

func (c *independentStartController) Start(ctx context.Context, execution domain.ActivityExecutionContext) (domain.ActivityHandle, error) {
	if c.failFast && execution.Activity.ID == "a" {
		select {
		case <-c.slowEntered:
			return domain.ActivityHandle{}, errors.New("start rejected")
		case <-ctx.Done():
			return domain.ActivityHandle{}, ctx.Err()
		}
	}
	if execution.Activity.ID == "slow" {
		close(c.slowEntered)
		select {
		case <-c.releaseSlow:
		case <-ctx.Done():
			if !c.lateHandle {
				return domain.ActivityHandle{}, ctx.Err()
			}
		}
	}
	if execution.Activity.ID == "b" {
		c.once.Do(func() { close(c.successor) })
	}
	return c.activityControllerFake.Start(ctx, execution)
}

func TestStartFailureStillOwnsAndStopsConcurrentLateHandle(t *testing.T) {
	request := requestFixture(domain.ExecutionModeReal)
	request.Workflow.Dependencies = nil
	request.Workflow.Activities[1].ID = "slow"
	request.Plan.Assignments[1].ActivityID = "slow"
	c := &independentStartController{slowEntered: make(chan struct{}), releaseSlow: make(chan struct{}), successor: make(chan struct{}), lateHandle: true, failFast: true}
	s, _ := New(&executionStoreFake{}, c, &planExecutorFake{}, Config{PollInterval: time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := s.Execute(ctx, request); err == nil || err.Error() != "start rejected" {
		t.Fatalf("err=%v", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.stopped) != 1 || c.stopped[0] != "h-slow" {
		t.Fatalf("concurrent handle not stopped: %v", c.stopped)
	}
}

func TestIndependentStartPublishesSuccessorWhilePeerStartIsBlocked(t *testing.T) {
	request := requestFixture(domain.ExecutionModeReal)
	slow := request.Workflow.Activities[0]
	slow.ID = "slow"
	request.Workflow.Activities = append(request.Workflow.Activities, slow)
	request.Plan.Assignments = append(request.Plan.Assignments, domain.PlanAssignment{ID: "ps", ActivityID: "slow", ResourceID: "r"})
	c := &independentStartController{slowEntered: make(chan struct{}), releaseSlow: make(chan struct{}), successor: make(chan struct{})}
	s, _ := New(&executionStoreFake{}, c, &planExecutorFake{}, Config{PollInterval: time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := s.Execute(ctx, request); done <- err }()
	select {
	case <-c.slowEntered:
	case <-ctx.Done():
		t.Fatal("slow start never entered")
	}
	select {
	case <-c.successor:
	case <-ctx.Done():
		t.Fatal("independent successor waited for blocked peer")
	}
	close(c.releaseSlow)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	counts := map[string]int{}
	for _, id := range c.started {
		counts[id]++
	}
	for _, id := range []string{"a", "b", "slow"} {
		if counts[id] != 1 {
			t.Fatalf("starts=%v", counts)
		}
	}
}

func TestCancellationJoinsAndStopsLateSuccessfulStart(t *testing.T) {
	request := requestFixture(domain.ExecutionModeReal)
	request.Workflow.Activities = request.Workflow.Activities[:1]
	request.Workflow.Activities[0].ID = "slow"
	request.Workflow.Dependencies = nil
	request.Plan.Assignments = []domain.PlanAssignment{{ID: "ps", ActivityID: "slow", ResourceID: "r"}}
	c := &independentStartController{slowEntered: make(chan struct{}), releaseSlow: make(chan struct{}), successor: make(chan struct{}), lateHandle: true}
	s, _ := New(&executionStoreFake{}, c, &planExecutorFake{}, Config{PollInterval: time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := s.Execute(ctx, request); done <- err }()
	select {
	case <-c.slowEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("slow start never entered")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancel did not join worker")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.stopped) != 1 || c.stopped[0] != "h-slow" {
		t.Fatalf("late handle not stopped: %v", c.stopped)
	}
}
