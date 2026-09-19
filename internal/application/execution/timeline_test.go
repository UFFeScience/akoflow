package execution

import (
	"math"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestBuildTimelineSeparatesQueueInfrastructurePreparationLaunchAndRuntime(t *testing.T) {
	started, finished := time.Unix(100, 0).UTC(), time.Unix(150, 0).UTC()
	provisionStarted, provisionFinished := time.Unix(105, 0).UTC(), time.Unix(125, 0).UTC()
	run := domain.ExecutionRun{ID: "run", StartedAt: &started, FinishedAt: &finished, MakespanSeconds: 42}
	tasks := []domain.TaskExecution{{
		ID: "task", ExecutionRunID: "run", ActivityID: "activity", Status: domain.TaskCompleted,
		ReadyAt: 100, StartedAt: 132, FinishedAt: 142, RuntimeSeconds: 9,
		Metadata: map[string]any{"preparationFinishedAt": float64(130)},
	}}
	transfers := []domain.DataTransfer{{
		ID: "executable", ExecutionRunID: "run", ConsumerActivityID: "activity",
		Strategy: domain.TransferSourcePush, StartedAt: 125, FinishedAt: 130,
	}}
	operations := []domain.CloudOperationRun{{
		ID: "provision", Kind: "provision", Status: "completed", ExecutionRunID: "run",
		ActivityID: "activity", StartedAt: &provisionStarted, FinishedAt: &provisionFinished,
	}}

	timeline := BuildTimeline(run, tasks, transfers, operations)
	assertTimelineSeconds(t, "wall", timeline.WallSeconds, 50)
	assertTimelineSeconds(t, "queue", timeline.Totals.ResourceQueueSeconds, 5)
	assertTimelineSeconds(t, "provision", timeline.Totals.ProvisionSeconds, 20)
	assertTimelineSeconds(t, "executable", timeline.Totals.ExecutablePreparationSeconds, 5)
	assertTimelineSeconds(t, "launch", timeline.Totals.LaunchSeconds, 3)
	assertTimelineSeconds(t, "runtime", timeline.Totals.RuntimeSeconds, 9)
	assertTimelineSeconds(t, "covered", timeline.CoveredWallSeconds, 42)
	assertTimelineSeconds(t, "unclassified", timeline.UnclassifiedWallSeconds, 8)
	if len(timeline.Phases) != 5 {
		t.Fatalf("phases=%+v", timeline.Phases)
	}
}

func TestBuildTimelineDoesNotDoubleCountOverlappingWallCoverage(t *testing.T) {
	started, finished := time.Unix(10, 0).UTC(), time.Unix(30, 0).UTC()
	run := domain.ExecutionRun{ID: "run", StartedAt: &started, FinishedAt: &finished}
	tasks := []domain.TaskExecution{
		{ID: "one", ActivityID: "one", Status: domain.TaskCompleted, StartedAt: 12, FinishedAt: 22},
		{ID: "two", ActivityID: "two", Status: domain.TaskCompleted, StartedAt: 18, FinishedAt: 28},
	}
	timeline := BuildTimeline(run, tasks, nil, nil)
	assertTimelineSeconds(t, "runtime work", timeline.Totals.RuntimeSeconds, 20)
	assertTimelineSeconds(t, "covered wall", timeline.CoveredWallSeconds, 16)
	assertTimelineSeconds(t, "unclassified wall", timeline.UnclassifiedWallSeconds, 4)
}

func assertTimelineSeconds(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.000001 {
		t.Fatalf("%s=%f want %f", name, got, want)
	}
}
