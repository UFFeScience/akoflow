package recurring

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	applicationexecution "github.com/UFFeScience/akoflow/internal/application/execution"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainexecution "github.com/UFFeScience/akoflow/internal/domain/execution"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
	"github.com/google/uuid"
)

var ErrConfiguration = errors.New("invalid execution schedule")
var ErrNotFound = errors.New("execution schedule not found")

type Store interface {
	SaveSchedule(context.Context, domainexecution.Schedule) error
	ListSchedules(context.Context) ([]domainexecution.Schedule, error)
	FindSchedule(context.Context, string) (*domainexecution.Schedule, error)
	DueSchedules(context.Context, time.Time) ([]domainexecution.Schedule, error)
	SetSchedulePaused(context.Context, string, bool, time.Time) error
	RecordOccurrence(context.Context, domainexecution.Schedule, time.Time, domainexecution.ScheduleOutcome, domain.ExecutionRun, domainqueue.Job) error
}

type Coordinator struct {
	Store   Store
	Request func(context.Context, domainexecution.Schedule) (ports.ExecutionRequest, error)
}

func (c Coordinator) Create(ctx context.Context, schedule domainexecution.Schedule, now time.Time) (domainexecution.Schedule, error) {
	schedule.Name = strings.TrimSpace(schedule.Name)
	if schedule.Name == "" || len(schedule.Name) > 120 || schedule.PlanID == "" || schedule.WorkflowVersionID == "" {
		return schedule, fmt.Errorf("%w: name, saved plan and workflow version are required", ErrConfiguration)
	}
	if (schedule.MissedPolicy != "skip" && schedule.MissedPolicy != "latest") ||
		(schedule.OverlapPolicy != "skip" && schedule.OverlapPolicy != "allow") {
		return schedule, fmt.Errorf("%w: choose how to handle missed and overlapping runs", ErrConfiguration)
	}
	rule, err := scheduleCalendar(schedule)
	if err != nil {
		return schedule, fmt.Errorf("%w: %v", ErrConfiguration, err)
	}
	schedule.ID = uuid.NewString()
	schedule.Paused = false
	schedule.LastOccurrence, schedule.LastLaunch = nil, nil
	schedule.NextAt = rule.adjacent(now, 1)
	if schedule.NextAt.IsZero() {
		return schedule, fmt.Errorf("%w: no upcoming occurrence", ErrConfiguration)
	}
	if _, err := c.Request(ctx, schedule); err != nil {
		return schedule, err
	}
	return schedule, c.Store.SaveSchedule(ctx, schedule)
}

func (c Coordinator) SetPaused(ctx context.Context, id string, paused bool, now time.Time) (*domainexecution.Schedule, error) {
	schedule, err := c.Store.FindSchedule(ctx, id)
	if err != nil {
		return nil, err
	}
	if schedule == nil {
		return nil, ErrNotFound
	}
	if schedule.Paused == paused {
		return schedule, nil
	}
	next := schedule.NextAt
	if !paused {
		rule, err := scheduleCalendar(*schedule)
		if err != nil {
			return nil, err
		}
		next = rule.adjacent(now, 1)
	}
	if err := c.Store.SetSchedulePaused(ctx, id, paused, next); err != nil {
		return nil, err
	}
	return c.Store.FindSchedule(ctx, id)
}

func (c Coordinator) Dispatch(ctx context.Context, now time.Time) error {
	due, err := c.Store.DueSchedules(ctx, now)
	if err != nil {
		return err
	}
	var failures error
	for _, schedule := range due {
		if err := c.dispatchOne(ctx, schedule, now); err != nil {
			failures = errors.Join(failures, fmt.Errorf("schedule %s: %w", schedule.ID, err))
		}
	}
	return failures
}

func (c Coordinator) dispatchOne(ctx context.Context, schedule domainexecution.Schedule, now time.Time) error {
	rule, err := scheduleCalendar(schedule)
	if err != nil {
		return err
	}
	latest, next := rule.adjacent(now, -1), rule.adjacent(now, 1)
	if latest.IsZero() || next.IsZero() {
		return fmt.Errorf("could not resolve the current schedule occurrence")
	}
	if latest.Before(schedule.NextAt) {
		latest = schedule.NextAt
	}
	outcome := domainexecution.ScheduleOutcome{ScheduledAt: latest, HandledAt: now, Outcome: "launched"}
	var run domain.ExecutionRun
	var job domainqueue.Job
	if now.Sub(latest) >= time.Minute && schedule.MissedPolicy == "skip" {
		outcome.Outcome = "missed"
	} else {
		request, err := c.Request(ctx, schedule)
		if err != nil && !errors.Is(err, ErrConfiguration) {
			return err
		}
		if err != nil {
			outcome.Outcome, outcome.Detail = "blocked", err.Error()
		} else {
			run = request.Run
			run.ID = "scheduled-" + schedule.ID + "-" + latest.UTC().Format("20060102T150405Z")
			request.Run = run
			job, err = applicationexecution.NewExecutionJob(request, "scheduled-occurrence:"+run.ID, now)
			if err != nil {
				return err
			}
		}
	}
	return c.Store.RecordOccurrence(ctx, schedule, next, outcome, run, job)
}
