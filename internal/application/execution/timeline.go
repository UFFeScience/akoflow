package execution

import (
	"sort"
	"strconv"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

const (
	PhaseResourceQueue         = "resource-queue"
	PhaseProvision             = "infrastructure-provision"
	PhaseStart                 = "infrastructure-start"
	PhaseValidation            = "connection-validation"
	PhaseExecutablePreparation = "executable-preparation"
	PhaseWorkspaceTransfer     = "workspace-transfer"
	PhaseLaunch                = "launch"
	PhaseRuntime               = "runtime"
	PhaseStop                  = "infrastructure-stop"
	PhaseDestroy               = "infrastructure-destroy"
)

// BuildTimeline reconstructs a normalized execution timeline from persisted
// observations. It does not mutate legacy queue or makespan measurements, so
// completed executions gain the corrected semantics without a data migration.
func BuildTimeline(run domain.ExecutionRun, tasks []domain.TaskExecution, transfers []domain.DataTransfer, operations []domain.CloudOperationRun) domain.ExecutionTimeline {
	timeline := domain.ExecutionTimeline{ComputeMakespanSeconds: run.MakespanSeconds, Phases: make([]domain.ExecutionTimelinePhase, 0)}
	if run.StartedAt != nil {
		timeline.StartedAt = unixSeconds(*run.StartedAt)
	}
	if run.FinishedAt != nil {
		timeline.FinishedAt = unixSeconds(*run.FinishedAt)
	}
	if timeline.FinishedAt > timeline.StartedAt {
		timeline.WallSeconds = timeline.FinishedAt - timeline.StartedAt
	}
	for _, operation := range operations {
		if operation.ExecutionRunID != run.ID || operation.StartedAt == nil || operation.FinishedAt == nil {
			continue
		}
		category := infrastructureCategory(operation.Kind)
		if category == "" {
			continue
		}
		timeline.Phases = append(timeline.Phases, phase(operation.ID, category, operation.ActivityID,
			operation.ID, "", operation.Status, "cloud-operation", unixSeconds(*operation.StartedAt), unixSeconds(*operation.FinishedAt)))
	}
	for _, transfer := range transfers {
		if transfer.FinishedAt <= transfer.StartedAt {
			continue
		}
		category := PhaseWorkspaceTransfer
		if transfer.Strategy == domain.TransferSourcePush {
			category = PhaseExecutablePreparation
		}
		timeline.Phases = append(timeline.Phases, phase(transfer.ID, category, transfer.ConsumerActivityID,
			"", transfer.ID, "completed", "transfer-observation", transfer.StartedAt, transfer.FinishedAt))
	}
	for _, task := range tasks {
		appendTaskPhases(&timeline, task)
	}
	sort.SliceStable(timeline.Phases, func(i, j int) bool {
		if timeline.Phases[i].StartedAt == timeline.Phases[j].StartedAt {
			return timeline.Phases[i].ID < timeline.Phases[j].ID
		}
		return timeline.Phases[i].StartedAt < timeline.Phases[j].StartedAt
	})
	for _, item := range timeline.Phases {
		addTimelineTotal(&timeline.Totals, item.Category, item.DurationSeconds)
	}
	timeline.CoveredWallSeconds = coveredSeconds(timeline.Phases, timeline.StartedAt, timeline.FinishedAt)
	timeline.UnclassifiedWallSeconds = maxTimeline(0, timeline.WallSeconds-timeline.CoveredWallSeconds)
	return timeline
}

func appendTaskPhases(timeline *domain.ExecutionTimeline, task domain.TaskExecution) {
	runtimeStart := task.StartedAt
	if task.RuntimeSeconds > 0 && task.FinishedAt >= task.RuntimeSeconds {
		runtimeStart = task.FinishedAt - task.RuntimeSeconds
	}
	if task.FinishedAt > task.StartedAt {
		timeline.Phases = append(timeline.Phases, phase(task.ID+":runtime", PhaseRuntime, task.ActivityID,
			"", "", string(task.Status), "activity-handle", runtimeStart, task.FinishedAt))
	}
	launchStart := timelineNumber(task.Metadata, "preparationFinishedAt")
	if launchStart == 0 {
		launchStart = timelineNumber(task.Metadata, domain.TimingSubmittedAt)
	}
	if launchStart > 0 && runtimeStart > launchStart {
		timeline.Phases = append(timeline.Phases, phase(task.ID+":launch", PhaseLaunch, task.ActivityID,
			"", "", "completed", "task-metadata", launchStart, runtimeStart))
	}
	if task.ReadyAt <= 0 || task.StartedAt <= task.ReadyAt {
		return
	}
	known := make([]timelineInterval, 0)
	for _, item := range timeline.Phases {
		if item.ActivityID == task.ActivityID && item.Category != PhaseRuntime && item.Category != PhaseResourceQueue {
			known = append(known, timelineInterval{start: item.StartedAt, finish: item.FinishedAt})
		}
	}
	for index, gap := range subtractTimeline(task.ReadyAt, task.StartedAt, known) {
		timeline.Phases = append(timeline.Phases, phase(task.ID+":queue:"+strconv.Itoa(index), PhaseResourceQueue,
			task.ActivityID, "", "", "completed", "derived-residual", gap.start, gap.finish))
	}
}

func infrastructureCategory(kind string) string {
	switch kind {
	case "provision":
		return PhaseProvision
	case "start":
		return PhaseStart
	case "validate", "configure":
		return PhaseValidation
	case "stop":
		return PhaseStop
	case "destroy":
		return PhaseDestroy
	default:
		return ""
	}
}

func phase(id, category, activityID, operationID, transferID, status, source string, start, finish float64) domain.ExecutionTimelinePhase {
	return domain.ExecutionTimelinePhase{ID: id, Category: category, ActivityID: activityID,
		OperationID: operationID, TransferID: transferID, Status: status, Source: source,
		StartedAt: start, FinishedAt: finish, DurationSeconds: maxTimeline(0, finish-start)}
}

type timelineInterval struct{ start, finish float64 }

func subtractTimeline(start, finish float64, known []timelineInterval) []timelineInterval {
	known = mergeTimelineIntervals(known, start, finish)
	result, cursor := make([]timelineInterval, 0), start
	for _, item := range known {
		if item.start > cursor {
			result = append(result, timelineInterval{cursor, item.start})
		}
		cursor = maxTimeline(cursor, item.finish)
	}
	if cursor < finish {
		result = append(result, timelineInterval{cursor, finish})
	}
	return result
}

func mergeTimelineIntervals(values []timelineInterval, lower, upper float64) []timelineInterval {
	clipped := make([]timelineInterval, 0, len(values))
	for _, item := range values {
		item.start, item.finish = maxTimeline(lower, item.start), minTimeline(upper, item.finish)
		if item.finish > item.start {
			clipped = append(clipped, item)
		}
	}
	sort.Slice(clipped, func(i, j int) bool { return clipped[i].start < clipped[j].start })
	merged := make([]timelineInterval, 0, len(clipped))
	for _, item := range clipped {
		if len(merged) == 0 || item.start > merged[len(merged)-1].finish {
			merged = append(merged, item)
			continue
		}
		merged[len(merged)-1].finish = maxTimeline(merged[len(merged)-1].finish, item.finish)
	}
	return merged
}

func coveredSeconds(phases []domain.ExecutionTimelinePhase, start, finish float64) float64 {
	values := make([]timelineInterval, 0, len(phases))
	for _, item := range phases {
		values = append(values, timelineInterval{item.StartedAt, item.FinishedAt})
	}
	total := 0.0
	for _, item := range mergeTimelineIntervals(values, start, finish) {
		total += item.finish - item.start
	}
	return total
}

func addTimelineTotal(total *domain.ExecutionTimelineTotals, category string, seconds float64) {
	switch category {
	case PhaseResourceQueue:
		total.ResourceQueueSeconds += seconds
	case PhaseProvision:
		total.ProvisionSeconds += seconds
	case PhaseStart:
		total.StartSeconds += seconds
	case PhaseValidation:
		total.ValidationSeconds += seconds
	case PhaseExecutablePreparation:
		total.ExecutablePreparationSeconds += seconds
	case PhaseWorkspaceTransfer:
		total.WorkspaceTransferSeconds += seconds
	case PhaseLaunch:
		total.LaunchSeconds += seconds
	case PhaseRuntime:
		total.RuntimeSeconds += seconds
	case PhaseStop:
		total.StopSeconds += seconds
	case PhaseDestroy:
		total.DestroySeconds += seconds
	}
}

func timelineNumber(values map[string]any, key string) float64 {
	switch value := values[key].(type) {
	case float64:
		return value
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		return 0
	}
}

func unixSeconds(value time.Time) float64 { return float64(value.UnixNano()) / 1e9 }
func minTimeline(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
}
func maxTimeline(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}
