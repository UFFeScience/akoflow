package execution

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type ActivityController interface {
	Start(context.Context, domain.ActivityExecutionContext) (domain.ActivityHandle, error)
	Inspect(context.Context, string, domain.ExecutionMode) (*domain.ActivityHandle, error)
}

type Config struct {
	PollInterval time.Duration
	MaxParallel  int
	Preparer     ports.PreparationCoordinator
}

type Supervisor struct {
	executions ports.ExecutionStore
	activities ActivityController
	simulation ports.PlanExecutor
	config     Config
}

func New(executions ports.ExecutionStore, activities ActivityController, simulation ports.PlanExecutor, config Config) (*Supervisor, error) {
	if executions == nil || activities == nil || simulation == nil {
		return nil, fmt.Errorf("execution repository, activity controller and simulator are required")
	}
	if config.PollInterval <= 0 {
		config.PollInterval = time.Second
	}
	if config.MaxParallel <= 0 {
		config.MaxParallel = 8
	}
	return &Supervisor{executions: executions, activities: activities, simulation: simulation, config: config}, nil
}

func (s *Supervisor) Execute(ctx context.Context, request ports.ExecutionRequest) (trace domain.ExecutionTrace, err error) {
	request.Run.SchedulePlanID = request.Plan.ID
	request.Run.Status = domain.ExecutionRunRunning
	if err := validateRequest(request); err != nil {
		return domain.ExecutionTrace{}, err
	}
	if err := s.executions.CreateRun(ctx, request.Run); err != nil {
		return domain.ExecutionTrace{}, fmt.Errorf("create execution run: %w", err)
	}
	defer func() {
		if err != nil {
			_ = s.executions.FailRun(context.WithoutCancel(ctx), request.Run.ID, err.Error())
		}
	}()
	if request.Run.Mode == domain.ExecutionModeSimulation {
		trace, err = s.simulation.Execute(ctx, request)
	} else {
		trace, err = s.executeActivities(ctx, request)
	}
	if err != nil {
		return domain.ExecutionTrace{}, err
	}
	if request.Run.Mode == domain.ExecutionModeInteractive {
		return trace, nil
	}
	if err = s.executions.CompleteRun(ctx, trace); err != nil {
		return domain.ExecutionTrace{}, fmt.Errorf("complete execution run: %w", err)
	}
	return trace, nil
}

func (s *Supervisor) executeActivities(ctx context.Context, request ports.ExecutionRequest) (domain.ExecutionTrace, error) {
	activities := indexActivities(request.Workflow.Activities)
	resources := indexResources(request.Resources)
	assignments := indexAssignments(request.Plan.Assignments)
	predecessors := make(map[string][]string)
	for _, dependency := range request.Workflow.Dependencies {
		predecessors[dependency.ActivityID] = append(predecessors[dependency.ActivityID], dependency.DependsOnActivityID)
	}
	completed := make(map[string]domain.TaskExecution)
	running := make(map[string]domain.ActivityHandle)
	tasks := make(map[string]domain.TaskExecution)
	transfers := make([]domain.DataTransfer, 0)

	for len(completed) < len(activities) {
		if err := ctx.Err(); err != nil {
			return domain.ExecutionTrace{}, err
		}
		if err := s.inspectRunning(ctx, request.Run.Mode, running, completed, tasks); err != nil {
			return domain.ExecutionTrace{}, err
		}
		if request.Run.Mode == domain.ExecutionModeInteractive && len(running) > 0 {
			return runningTrace(request, tasks, transfers), nil
		}
		ready := readyActivities(activities, predecessors, completed, running, tasks)
		available := s.config.MaxParallel - len(running)
		if available > len(ready) {
			available = len(ready)
		}
		if err := s.startReadyActivities(
			ctx, request, ready[:available], activities, assignments, resources,
			running, completed, tasks, &transfers,
		); err != nil {
			return domain.ExecutionTrace{}, err
		}
		if len(running) == 0 && len(ready) == 0 && len(completed) < len(activities) {
			return domain.ExecutionTrace{}, fmt.Errorf("execution cannot progress: dependency cycle or incomplete plan")
		}
		if len(running) > 0 {
			select {
			case <-ctx.Done():
				return domain.ExecutionTrace{}, ctx.Err()
			case <-time.After(s.config.PollInterval):
			}
		}
	}
	return completedTrace(request, tasks, transfers), nil
}

func (s *Supervisor) inspectRunning(
	ctx context.Context,
	mode domain.ExecutionMode,
	running map[string]domain.ActivityHandle,
	completed map[string]domain.TaskExecution,
	tasks map[string]domain.TaskExecution,
) error {
	for activityID, handle := range running {
		observed, err := s.activities.Inspect(ctx, handle.ID, mode)
		if err != nil {
			return fmt.Errorf("inspect activity %q: %w", activityID, err)
		}
		task := tasks[activityID]
		switch observed.Status {
		case domain.HandleCompleted:
			completeTask(&task, *observed)
			if err := s.executions.SaveTask(ctx, task); err != nil {
				return err
			}
			tasks[activityID], completed[activityID] = task, task
			delete(running, activityID)
		case domain.HandleFailed, domain.HandleStopped:
			task.Status, task.FailureReason = domain.TaskFailed, observed.Failure
			_ = s.executions.SaveTask(ctx, task)
			return fmt.Errorf("activity %q failed: %s", activityID, observed.Failure)
		}
	}
	return nil
}

func (s *Supervisor) startReadyActivities(
	ctx context.Context,
	request ports.ExecutionRequest,
	ready []string,
	activities map[string]domain.Activity,
	assignments map[string]domain.PlanAssignment,
	resources map[string]domain.Resource,
	running map[string]domain.ActivityHandle,
	completed map[string]domain.TaskExecution,
	tasks map[string]domain.TaskExecution,
	transfers *[]domain.DataTransfer,
) error {
	for _, activityID := range ready {
		readyAt := unixNow()
		activity, assignment := activities[activityID], assignments[activityID]
		resource, ok := resources[assignment.ResourceID]
		if !ok {
			failure := fmt.Errorf("resource %q not found", assignment.ResourceID)
			_ = s.recordStartFailure(
				ctx,
				request.Run.ID,
				activityID,
				assignment,
				domain.Resource{ID: assignment.ResourceID},
				selectRuntime(request, assignment),
				failure,
			)
			return fmt.Errorf("resource %q not found", assignment.ResourceID)
		}
		var preparation *domain.PreparationGate
		if requirement, required := request.PreparationRequirementsByActivity[activityID]; required {
			var prepareErr error
			preparation, prepareErr = s.prepareActivity(ctx, request.Run.ID, activityID, requirement)
			if prepareErr != nil {
				failure := fmt.Errorf("prepare activity %q: %w", activityID, prepareErr)
				_ = s.recordStartFailure(ctx, request.Run.ID, activityID, assignment, resource, selectRuntime(request, assignment), failure)
				return failure
			}
			*transfers = append(*transfers, transferObservations(request.Run.ID, activityID, resource.ID, preparation.TransferRuns)...)
		} else if activity.Command.Executable != nil && activity.Command.Executable.Source.Type == domain.ExecutableSourceType("build") {
			// Authored executable references are location-independent contracts.
			// Running them without a generated preparation requirement would let a
			// provider fall back to a caller supplied path/image and bypass the
			// materialization gate.
			failure := fmt.Errorf("activity %q has an executable reference but no preparation requirement", activityID)
			_ = s.recordStartFailure(ctx, request.Run.ID, activityID, assignment, resource, selectRuntime(request, assignment), failure)
			return failure
		}
		handle, err := s.activities.Start(ctx, domain.ActivityExecutionContext{
			Run: request.Run, Workflow: request.Workflow, Activity: activity,
			Assignment: assignment, Resource: resource,
			RuntimeID: selectRuntime(request, assignment), Preparation: preparation,
		})
		if err != nil {
			_ = s.recordStartFailure(
				ctx,
				request.Run.ID,
				activityID,
				assignment,
				resource,
				selectRuntime(request, assignment),
				err,
			)
			return fmt.Errorf("start activity %q: %w", activityID, err)
		}
		task := newRunningTask(request.Run.ID, activityID, assignment, resource, handle, readyAt, preparation)
		tasks[activityID], running[activityID] = task, handle
		if handle.Status == domain.HandleCompleted {
			completeTask(&task, handle)
			tasks[activityID], completed[activityID] = task, task
			delete(running, activityID)
		}
		if err := s.executions.SaveTask(ctx, task); err != nil {
			return err
		}
	}
	return nil
}

func (s *Supervisor) prepareActivity(
	ctx context.Context,
	runID, activityID string,
	requirement domain.PreparationRequirement,
) (*domain.PreparationGate, error) {
	if s.config.Preparer == nil {
		return nil, fmt.Errorf("activity %q requires materialization but no preparation coordinator is configured", activityID)
	}
	if requirement.Artifact != nil {
		requirement.Artifact.RunID = runID
		requirement.Artifact.ActivityID = activityID
	}
	return s.config.Preparer.Prepare(ctx, activityID, requirement)
}

func transferObservations(
	runID, activityID, resourceID string,
	observations []domain.DataTransferRun,
) []domain.DataTransfer {
	transfers := make([]domain.DataTransfer, 0, len(observations))
	for _, observation := range observations {
		// Artifact stores are not execution resources in the current transfer
		// schema. Attribute ingress to the selected resource until the schema
		// includes a storage endpoint dimension.
		transfers = append(transfers, domain.DataTransfer{
			ID:                 runID + ":" + activityID + ":" + observation.ID,
			ExecutionRunID:     runID,
			ConsumerActivityID: activityID,
			SourceResourceID:   resourceID,
			TargetResourceID:   resourceID,
			Bytes:              observation.TransferredBytes,
			StartedAt:          observation.StartedAt,
			FinishedAt:         observation.FinishedAt,
			DurationSeconds:    maxFloat(0, observation.FinishedAt-observation.StartedAt),
		})
	}
	return transfers
}

func (s *Supervisor) recordStartFailure(
	ctx context.Context,
	runID, activityID string,
	assignment domain.PlanAssignment,
	resource domain.Resource,
	runtimeID string,
	err error,
) error {
	now := float64(time.Now().UnixNano()) / float64(time.Second)
	message := "start activity: " + err.Error()
	handle := domain.ActivityHandle{
		ID:         runID + ":" + activityID,
		RunID:      runID,
		ActivityID: activityID,
		ResourceID: resource.ID,
		RuntimeID:  runtimeID,
		Status:     domain.HandleFailed,
		StartedAt:  now,
		FinishedAt: now,
		Failure:    message, Log: "[AKOFLOW ERROR] " + message + "\n",
	}
	if saveErr := s.executions.Save(ctx, handle); saveErr != nil {
		return saveErr
	}
	task := newRunningTask(runID, activityID, assignment, resource, handle, now, nil)
	task.Status, task.FinishedAt, task.FailureReason = domain.TaskFailed, now, message
	return s.executions.SaveTask(ctx, task)
}

// selectRuntime honors the explicit runtime selected while planning when it is
// still enabled for the assigned resource and compatible with the run mode.
// Plans created before runtime selection keep the deterministic first binding.
func selectRuntime(request ports.ExecutionRequest, assignment domain.PlanAssignment) string {
	runtimeMode := domain.RuntimeModeExecution
	if request.Run.Mode == domain.ExecutionModeSimulation {
		runtimeMode = domain.RuntimeModeSimulation
	}
	runtimes := make(map[string]domain.EnvironmentRuntime, len(request.Runtimes))
	for _, runtime := range request.Runtimes {
		runtimes[runtime.ID] = runtime
	}
	selected, _ := assignment.Metadata["runtimeId"].(string)
	for _, binding := range request.RuntimeBindings {
		if binding.ResourceID != assignment.ResourceID || !binding.Enabled {
			continue
		}
		if runtime, found := runtimes[binding.RuntimeID]; found && runtime.Mode == runtimeMode {
			if selected != "" && runtime.ID != selected {
				continue
			}
			return runtime.ID
		}
	}
	return ""
}

func newRunningTask(
	runID string,
	activityID string,
	assignment domain.PlanAssignment,
	resource domain.Resource,
	handle domain.ActivityHandle,
	readyAt float64,
	preparation *domain.PreparationGate,
) domain.TaskExecution {
	task := domain.TaskExecution{
		ID: runID + ":" + activityID, ExecutionRunID: runID,
		PlanAssignmentID: assignment.ID, ActivityID: activityID,
		PlannedResourceID: assignment.ResourceID, AllocatedResourceID: resource.ID,
		Attempt: 1, Status: domain.TaskRunning, ReadyAt: readyAt, DataReadyAt: unixNow(),
		QueuedAt: handle.StartedAt, StartedAt: handle.StartedAt,
	}
	if preparation != nil {
		for _, transfer := range preparation.TransferRuns {
			task.TransferSeconds += maxFloat(0, transfer.FinishedAt-transfer.StartedAt)
			task.TransferBytes += transfer.TransferredBytes
		}
	}
	return task
}

func completeTask(task *domain.TaskExecution, handle domain.ActivityHandle) {
	task.Status, task.FinishedAt = domain.TaskCompleted, handle.FinishedAt
	applyHandleTiming(task, handle)
	task.RuntimeSeconds = maxFloat(0, handle.FinishedAt-executionStartedAt(handle))
}

func applyHandleTiming(task *domain.TaskExecution, handle domain.ActivityHandle) {
	if submittedAt, ok := metadataFloat(handle.Metadata, domain.TimingSubmittedAt); ok {
		task.QueuedAt = submittedAt
		task.QueueSeconds = maxFloat(0, handle.StartedAt-submittedAt)
	}
	if handle.StartedAt > 0 {
		task.StartedAt = handle.StartedAt
	}
	if containerStartedAt, ok := metadataFloat(handle.Metadata, domain.TimingContainerStartedAt); ok {
		task.OverheadSeconds = maxFloat(0, containerStartedAt-handle.StartedAt)
	}
}

func executionStartedAt(handle domain.ActivityHandle) float64 {
	if containerStartedAt, ok := metadataFloat(handle.Metadata, domain.TimingContainerStartedAt); ok && containerStartedAt > handle.StartedAt {
		return containerStartedAt
	}
	return handle.StartedAt
}

func metadataFloat(metadata map[string]any, key string) (float64, bool) {
	value, ok := metadata[key]
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int:
		return float64(typed), true
	default:
		return 0, false
	}
}

func unixNow() float64 { return float64(time.Now().UnixNano()) / float64(time.Second) }

func validateRequest(request ports.ExecutionRequest) error {
	if request.Run.ID == "" || request.Plan.ID == "" || request.Workflow.ID == "" {
		return fmt.Errorf("run, plan and workflow identifiers are required")
	}
	if request.Run.Mode != domain.ExecutionModeReal && request.Run.Mode != domain.ExecutionModeSimulation && request.Run.Mode != domain.ExecutionModeInteractive {
		return fmt.Errorf("unsupported execution mode %q", request.Run.Mode)
	}
	if len(request.Plan.Assignments) != len(request.Workflow.Activities) {
		return fmt.Errorf("plan must assign every activity exactly once")
	}
	for _, assignment := range request.Plan.Assignments {
		if selectRuntime(request, assignment) == "" {
			return fmt.Errorf("resource %q has no enabled runtime for %q mode",
				assignment.ResourceID, request.Run.Mode)
		}
	}
	return nil
}

func indexActivities(values []domain.Activity) map[string]domain.Activity {
	result := make(map[string]domain.Activity, len(values))
	for _, value := range values {
		result[value.ID] = value
	}
	return result
}
func indexResources(values []domain.Resource) map[string]domain.Resource {
	result := make(map[string]domain.Resource, len(values))
	for _, value := range values {
		result[value.ID] = value
	}
	return result
}
func indexAssignments(values []domain.PlanAssignment) map[string]domain.PlanAssignment {
	result := make(map[string]domain.PlanAssignment, len(values))
	for _, value := range values {
		result[value.ActivityID] = value
	}
	return result
}

func readyActivities(activities map[string]domain.Activity, predecessors map[string][]string, completed map[string]domain.TaskExecution, running map[string]domain.ActivityHandle, tasks map[string]domain.TaskExecution) []string {
	ready := make([]string, 0)
	for id := range activities {
		if _, ok := completed[id]; ok {
			continue
		}
		if _, ok := running[id]; ok {
			continue
		}
		if _, attempted := tasks[id]; attempted {
			continue
		}
		all := true
		for _, predecessor := range predecessors[id] {
			if _, ok := completed[predecessor]; !ok {
				all = false
				break
			}
		}
		if all {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)
	return ready
}

func completedTrace(request ports.ExecutionRequest, tasks map[string]domain.TaskExecution, transfers []domain.DataTransfer) domain.ExecutionTrace {
	trace := runningTrace(request, tasks, transfers)
	trace.Executed.Feasible = true
	firstStart := 0.0
	lastFinish := 0.0
	for _, task := range trace.Tasks {
		if firstStart == 0 || task.StartedAt < firstStart {
			firstStart = task.StartedAt
		}
		lastFinish = maxFloat(lastFinish, task.FinishedAt)
		trace.Executed.ComputeSeconds += task.RuntimeSeconds
		trace.Executed.TransferSeconds += task.TransferSeconds
		trace.Executed.QueueSeconds += task.QueueSeconds
		trace.Executed.Cost += task.Cost
	}
	trace.Executed.MakespanSeconds = maxFloat(0, lastFinish-firstStart)
	return trace
}
func runningTrace(request ports.ExecutionRequest, tasks map[string]domain.TaskExecution, transfers []domain.DataTransfer) domain.ExecutionTrace {
	result := make([]domain.TaskExecution, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, task)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ActivityID < result[j].ActivityID })
	return domain.ExecutionTrace{RunID: request.Run.ID, PlanID: request.Plan.ID, Mode: request.Run.Mode, Predicted: request.Plan.Predicted, Tasks: result, Transfers: transfers}
}
func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
