package execution

import (
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	domainexecution "github.com/UFFeScience/akoflow/internal/domain/execution"
)

func TestTimelineCountsParallelPreparationAndDeduplicatesSharedWork(t *testing.T) {
	start, end := time.Unix(100, 0), time.Unix(130, 0)
	run := domain.ExecutionRun{StartedAt: &start, FinishedAt: &end, MakespanSeconds: 4}
	pull := domainexecution.ImagePreparationObservation{
		OperationID: "pull", ActivityID: "first", Attempt: 1,
		Kind: domainexecution.ImagePreparationPull, Status: "completed", StartedAt: start, FinishedAt: &end,
	}
	shared := pull
	shared.ActivityID = "second"
	shared.Shared = true
	build := pull
	build.OperationID, build.Kind = "build", domainexecution.ImagePreparationBuild
	tasks := []domain.TaskExecution{
		{ID: "first", ActivityID: "first", ReadyAt: 100, StartedAt: 130, Metadata: map[string]any{domain.TimingSubmittedAt: float64(100)}},
		{ID: "second", ActivityID: "second", ReadyAt: 100, StartedAt: 130, Metadata: map[string]any{domain.TimingSubmittedAt: float64(100)}},
	}
	timeline := BuildTimelineWithImagePreparation(run, tasks, nil, nil, []domainexecution.ImagePreparationObservation{pull, shared, build}, end)
	assertTimelineSeconds(t, "pull work", timeline.Totals.ImagePullSeconds, 30)
	assertTimelineSeconds(t, "build work", timeline.Totals.ImageBuildSeconds, 30)
	assertTimelineSeconds(t, "covered wall", timeline.CoveredWallSeconds, 30)
	assertTimelineSeconds(t, "launch", timeline.Totals.LaunchSeconds, 0)
	assertTimelineSeconds(t, "queue", timeline.Totals.ResourceQueueSeconds, 0)
	assertTimelineSeconds(t, "compute", timeline.ComputeMakespanSeconds, 4)
}

func TestFailedPreparationBeforeLaunchDoesNotProduceRuntime(t *testing.T) {
	start, end := time.Unix(100, 0), time.Unix(115, 0)
	run := domain.ExecutionRun{StartedAt: &start, FinishedAt: &end}
	tasks := []domain.TaskExecution{{ID: "task", ActivityID: "activity", ReadyAt: 100, FinishedAt: 115}}
	observations := []domainexecution.ImagePreparationObservation{{
		OperationID: "failed-pull", ActivityID: "activity", Attempt: 1,
		Kind: domainexecution.ImagePreparationPull, Status: "failed", StartedAt: start, FinishedAt: &end,
	}}
	timeline := BuildTimelineWithImagePreparation(run, tasks, nil, nil, observations, end)
	assertTimelineSeconds(t, "runtime", timeline.Totals.RuntimeSeconds, 0)
	assertTimelineSeconds(t, "pull", timeline.Totals.ImagePullSeconds, 15)
}
