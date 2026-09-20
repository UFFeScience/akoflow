package execution

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

const (
	readyActivityConcurrency  = 8
	runningInspectConcurrency = 16
)

type ActivityController interface {
	Start(context.Context, domain.ActivityExecutionContext) (domain.ActivityHandle, error)
	Inspect(context.Context, string, domain.ExecutionMode) (*domain.ActivityHandle, error)
	Stop(context.Context, string, domain.ExecutionMode) error
}

type Config struct {
	PollInterval   time.Duration
	Preparer       ports.PreparationCoordinator
	Data           ports.DataCatalog
	Cloud          ports.CloudProvisioner
	CloudStore     ports.CloudConfigurationStore
	CloudAllocator CloudAllocator
	Workspaces     WorkspaceManager
}

type WorkspaceManager interface {
	Ensure(context.Context, domain.ActivityWorkspace) error
	Inspect(context.Context, domain.ActivityWorkspace) (domain.WorkspaceUsage, error)
	PruneInputs(context.Context, domain.ActivityWorkspace) (domain.WorkspaceReleaseResult, error)
	Release(context.Context, domain.ActivityWorkspace) (domain.WorkspaceReleaseResult, error)
}

type Supervisor struct {
	executions ports.ExecutionStore
	activities ActivityController
	simulation ports.PlanExecutor
	config     Config
}

type startResult struct {
	activityID  string
	assignment  domain.PlanAssignment
	resource    domain.Resource
	allocation  domain.RuntimeAllocation
	preparation *domain.PreparationGate
	handle      domain.ActivityHandle
	transfers   []domain.DataTransfer
	readyAt     float64
	err         error
}

type inspectionResult struct {
	activityID string
	handle     domain.ActivityHandle
	observed   *domain.ActivityHandle
	err        error
}

func New(executions ports.ExecutionStore, activities ActivityController, simulation ports.PlanExecutor, config Config) (*Supervisor, error) {
	if executions == nil || activities == nil || simulation == nil {
		return nil, fmt.Errorf("execution repository, activity controller and simulator are required")
	}
	if config.PollInterval <= 0 {
		config.PollInterval = time.Second
	}
	return &Supervisor{executions: executions, activities: activities, simulation: simulation, config: config}, nil
}

func (s *Supervisor) Execute(ctx context.Context, request ports.ExecutionRequest) (trace domain.ExecutionTrace, err error) {
	if request.RuntimeAllocations == nil {
		request.RuntimeAllocations = make(map[string]domain.RuntimeAllocation)
	}
	request.Run.SchedulePlanID = request.Plan.ID
	request.Run.Status = domain.ExecutionRunRunning
	if err := validateRequest(request); err != nil {
		return domain.ExecutionTrace{}, err
	}
	if err := s.executions.CreateRun(ctx, request.Run); err != nil {
		return domain.ExecutionTrace{}, fmt.Errorf("create execution run: %w", err)
	}
	if request.Recovery != nil {
		if recoverErr := s.RecoverWorkspaces(ctx); recoverErr != nil {
			return domain.ExecutionTrace{}, fmt.Errorf("reconcile workspaces before recovery: %w", recoverErr)
		}
	}
	defer func() {
		if request.Run.Mode != domain.ExecutionModeSimulation && request.Run.Mode != domain.ExecutionModeInteractive {
			// Lifecycle cleanup belongs to the infrastructure result, not the
			// scientific result. Release records its own failed instance status;
			// a teardown failure must not turn completed computation into failure.
			retained, releaseErr := s.releaseCloud(context.WithoutCancel(ctx), request, err != nil)
			if err != nil && retained > 0 {
				err = fmt.Errorf("%w; retained %d cloud instance(s) with ephemeral outputs for recovery", err, retained)
			}
			if releaseErr != nil && err != nil {
				err = fmt.Errorf("%w; cloud cleanup: %v", err, releaseErr)
			}
		}
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

func (s *Supervisor) releaseCloud(ctx context.Context, request ports.ExecutionRequest, failed bool) (int, error) {
	if s.config.CloudAllocator != nil {
		allocations := request.RuntimeAllocations
		retained := 0
		if failed && s.config.Data != nil {
			locations, err := s.config.Data.ListLocations(ctx, request.Run.ID)
			if err != nil {
				return 0, fmt.Errorf("inspect ephemeral outputs before cloud cleanup: %w", err)
			}
			allocations, retained = retainCloudSourcesWithEphemeralOutputs(allocations, locations)
		}
		if !failed {
			if stopper, ok := s.config.CloudAllocator.(CloudStopper); ok {
				selected := plannedStopResources(request.Plan, indexResources(request.Resources))
				toStop := make(map[string]domain.RuntimeAllocation)
				toRelease := make(map[string]domain.RuntimeAllocation)
				assignments := indexAssignments(request.Plan.Assignments)
				stopInstances := make(map[string]bool)
				for activityID, allocation := range allocations {
					if selected[assignments[activityID].ResourceID] {
						stopInstances[allocation.CloudInstanceID] = true
					}
				}
				for activityID, allocation := range allocations {
					if stopInstances[allocation.CloudInstanceID] {
						toStop[activityID] = allocation
					} else {
						toRelease[activityID] = allocation
					}
				}
				if err := stopper.Stop(ctx, request.Run.ID, toStop); err != nil {
					return retained, err
				}
				allocations = toRelease
			}
		}
		return retained, s.config.CloudAllocator.Release(ctx, request.Run.ID, allocations, failed)
	}
	if s.config.Cloud == nil {
		return 0, nil
	}
	resourceIDs := make([]string, 0, len(request.Plan.Assignments))
	for _, assignment := range request.Plan.Assignments {
		if runtimeDriver(request, assignment.ActivityID) == domain.RuntimeDriverCloud {
			resourceIDs = append(resourceIDs, assignment.ResourceID)
		}
	}
	if err := s.config.Cloud.Release(ctx, resourceIDs); err != nil {
		return 0, fmt.Errorf("release cloud capacity: %w", err)
	}
	return 0, nil
}

func retainCloudSourcesWithEphemeralOutputs(allocations map[string]domain.RuntimeAllocation, locations []domain.DataLocation) (map[string]domain.RuntimeAllocation, int) {
	protectedResources := make(map[string]bool)
	for _, location := range locations {
		if location.Status == domain.DataLocationEphemeral {
			protectedResources[location.ResourceID] = true
		}
	}
	protectedInstances := make(map[string]bool)
	for _, allocation := range allocations {
		if allocation.CloudInstanceID != "" && protectedResources[allocation.ResourceID] {
			protectedInstances[allocation.CloudInstanceID] = true
		}
	}
	remaining := make(map[string]domain.RuntimeAllocation, len(allocations))
	for activityID, allocation := range allocations {
		if !protectedInstances[allocation.CloudInstanceID] {
			remaining[activityID] = allocation
		}
	}
	return remaining, len(protectedInstances)
}

func (s *Supervisor) executeActivities(ctx context.Context, request ports.ExecutionRequest) (domain.ExecutionTrace, error) {
	if request.RuntimeAllocations == nil {
		request.RuntimeAllocations = make(map[string]domain.RuntimeAllocation)
	}
	activities := indexActivities(request.Workflow.Activities)
	resources := indexResources(request.Resources)
	assignments := indexAssignments(request.Plan.Assignments)
	if err := s.initializeWorkspaces(ctx, request, activities, assignments, resources); err != nil {
		return domain.ExecutionTrace{}, fmt.Errorf("initialize activity workspaces: %w", err)
	}
	predecessors := make(map[string][]string)
	for activityID := range activities {
		predecessors[activityID] = workspaceProducers(request.Workflow, activityID)
	}
	completed := make(map[string]domain.TaskExecution)
	running := make(map[string]domain.ActivityHandle)
	tasks := make(map[string]domain.TaskExecution)
	attempts := make(map[string]int)
	if request.Recovery != nil {
		reader, ok := s.executions.(interface {
			ListTasks(context.Context, string) ([]domain.TaskExecution, error)
		})
		if !ok {
			return domain.ExecutionTrace{}, fmt.Errorf("execution repository does not support recovery")
		}
		previous, loadErr := reader.ListTasks(ctx, request.Run.ID)
		if loadErr != nil {
			return domain.ExecutionTrace{}, fmt.Errorf("load prior activity attempts: %w", loadErr)
		}
		reusable := make(map[string]bool, len(request.Recovery.ReuseActivityIDs))
		for _, activityID := range request.Recovery.ReuseActivityIDs {
			reusable[activityID] = true
		}
		latest := make(map[string]domain.TaskExecution)
		for _, task := range previous {
			if task.Attempt > attempts[task.ActivityID] {
				attempts[task.ActivityID] = task.Attempt
				latest[task.ActivityID] = task
			}
		}
		for _, task := range latest {
			if task.Status == domain.TaskCompleted && reusable[task.ActivityID] {
				tasks[task.ActivityID], completed[task.ActivityID] = task, task
			}
		}
	}
	transfers := make([]domain.DataTransfer, 0)
	type allocationResult struct {
		activityID string
		allocation domain.RuntimeAllocation
		err        error
	}
	allocationResults := make(chan allocationResult, len(activities))
	pendingAllocations := make(map[string]bool)
	readyAllocations := make(map[string]domain.RuntimeAllocation)
	stoppingInstances := make(map[string]bool)

	for len(completed) < len(activities) {
		if err := ctx.Err(); err != nil {
			return domain.ExecutionTrace{}, err
		}
		if err := s.inspectRunning(ctx, request.Run.Mode, request.Workflow, request.Run.ID, running, completed, tasks); err != nil {
			return domain.ExecutionTrace{}, err
		}
		if request.Run.Mode == domain.ExecutionModeInteractive && len(running) > 0 {
			return runningTrace(request, tasks, transfers), nil
		}
		ready := readyActivities(activities, predecessors, completed, running, tasks)
		admitted := plannedReadyActivities(
			ready, assignments, pendingAllocations,
		)
		for _, activityID := range ready {
			queueReason := "awaiting assigned runtime"
			if queued, exists := tasks[activityID]; exists {
				if queued.Metadata["queueReason"] != queueReason {
					if queued.Metadata == nil {
						queued.Metadata = make(map[string]any)
					}
					queued.Metadata["queueReason"] = queueReason
					if err := s.executions.SaveTask(ctx, queued); err != nil {
						return domain.ExecutionTrace{}, fmt.Errorf("update queue for activity %q: %w", activityID, err)
					}
					tasks[activityID] = queued
				}
				continue
			}
			now := unixNow()
			assignment := assignments[activityID]
			attempt := attempts[activityID] + 1
			queued := domain.TaskExecution{
				ID: taskExecutionID(request.Run.ID, activityID, attempt), ExecutionRunID: request.Run.ID,
				PlanAssignmentID: assignment.ID, ActivityID: activityID,
				PlannedResourceID: assignment.ResourceID, Attempt: attempt,
				Status: domain.TaskQueued, ReadyAt: now, QueuedAt: now,
				Metadata: map[string]any{"queueReason": queueReason},
			}
			if err := s.executions.SaveTask(ctx, queued); err != nil {
				return domain.ExecutionTrace{}, fmt.Errorf("queue activity %q: %w", activityID, err)
			}
			tasks[activityID] = queued
		}
		candidates := make([]string, 0)
		immediate := make([]string, 0)
		for _, activityID := range admitted {
			if runtimeDriver(request, activityID) != domain.RuntimeDriverCloud {
				immediate = append(immediate, activityID)
				continue
			}
			if _, allocated := readyAllocations[activityID]; allocated {
				immediate = append(immediate, activityID)
				continue
			}
			candidates = append(candidates, activityID)
		}
		if err := s.prewarmReadyCloud(ctx, request, candidates, assignments, resources); err != nil {
			return domain.ExecutionTrace{}, fmt.Errorf("prewarm cloud capacity: %w", err)
		}
		for _, activityID := range candidates {
			assignment := assignments[activityID]
			resource := resources[assignment.ResourceID]
			pendingAllocations[activityID] = true
			go func(activityID string, assignment domain.PlanAssignment, resource domain.Resource) {
				allocation, err := s.ensureRuntimeAllocation(ctx, request, activityID, assignment, resource)
				allocationResults <- allocationResult{activityID: activityID, allocation: allocation, err: err}
			}(activityID, assignment, resource)
		}
		if err := s.startReadyActivities(
			ctx, request, immediate, activities, assignments, resources,
			running, completed, tasks, &transfers, readyAllocations,
		); err != nil {
			return domain.ExecutionTrace{}, err
		}
		for _, activityID := range immediate {
			delete(readyAllocations, activityID)
		}
		s.stopIdleCloud(ctx, request, completed, tasks, stoppingInstances)
		if len(running) == 0 && len(pendingAllocations) == 0 && len(immediate) == 0 && len(completed) < len(activities) {
			return domain.ExecutionTrace{}, fmt.Errorf("execution cannot progress: dependency cycle or incomplete plan")
		}
		if len(running) > 0 || len(pendingAllocations) > 0 {
			select {
			case <-ctx.Done():
				return domain.ExecutionTrace{}, ctx.Err()
			case result := <-allocationResults:
				delete(pendingAllocations, result.activityID)
				if result.err != nil {
					failure := fmt.Errorf("allocate runtime for activity %q: %w", result.activityID, result.err)
					assignment := assignments[result.activityID]
					resource := resources[assignment.ResourceID]
					_ = s.recordStartFailure(ctx, request.Run.ID, result.activityID, assignment, resource, selectRuntime(request, assignment), tasks[result.activityID].Attempt, failure)
					return domain.ExecutionTrace{}, failure
				}
				readyAllocations[result.activityID] = result.allocation
			case <-time.After(s.config.PollInterval):
			}
		}
	}
	return completedTrace(request, tasks, transfers), nil
}

// Submit every ready cloud target before Allocate waits for the first VM.
// This preserves on-demand provisioning while allowing independent Terraform
// operations to run concurrently in the infrastructure queue.
func (s *Supervisor) prewarmReadyCloud(
	ctx context.Context,
	request ports.ExecutionRequest,
	ready []string,
	assignments map[string]domain.PlanAssignment,
	resources map[string]domain.Resource,
) error {
	prewarmer, ok := s.config.CloudAllocator.(CloudPrewarmer)
	if !ok || s.config.CloudStore == nil {
		return nil
	}
	type pendingTarget struct {
		activityID string
		target     domain.CloudCapacityTarget
	}
	pending := make([]pendingTarget, 0)
	seen := make(map[string]bool)
	for _, activityID := range ready {
		if runtimeDriver(request, activityID) != domain.RuntimeDriverCloud {
			continue
		}
		assignment := assignments[activityID]
		resource, exists := resources[assignment.ResourceID]
		if !exists {
			return fmt.Errorf("resource %q not found", assignment.ResourceID)
		}
		targetID := resource.ID
		if value, _ := resource.Metadata["capacityTargetId"].(string); strings.TrimSpace(value) != "" {
			targetID = strings.TrimSpace(value)
		}
		if seen[targetID] {
			continue
		}
		target, err := s.config.CloudStore.FindCapacityTarget(ctx, targetID)
		if err != nil {
			return fmt.Errorf("load cloud capacity target %q: %w", targetID, err)
		}
		if target == nil {
			return fmt.Errorf("cloud capacity target %q not found", targetID)
		}
		pending = append(pending, pendingTarget{activityID: activityID, target: *target})
		seen[targetID] = true
	}
	for _, item := range pending {
		if err := prewarmer.Prewarm(ctx, request.Run.ID, item.activityID, item.target); err != nil {
			return fmt.Errorf("target %q: %w", item.target.ID, err)
		}
	}
	return nil
}

func (s *Supervisor) inspectRunning(
	ctx context.Context,
	mode domain.ExecutionMode,
	workflow domain.WorkflowVersion,
	runID string,
	running map[string]domain.ActivityHandle,
	completed map[string]domain.TaskExecution,
	tasks map[string]domain.TaskExecution,
) error {
	results := s.inspectActivities(ctx, mode, running)
	var firstErr error
	for _, result := range results {
		firstErr = s.applyInspectionResult(ctx, workflow, runID, result, running, completed, tasks, firstErr)
	}
	if firstErr != nil {
		s.cancelRunningActivities(ctx, mode, running, tasks, firstErr)
	}
	return firstErr
}

func (s *Supervisor) inspectActivities(
	ctx context.Context,
	mode domain.ExecutionMode,
	running map[string]domain.ActivityHandle,
) []inspectionResult {
	results := make(chan inspectionResult, len(running))
	semaphore := make(chan struct{}, runningInspectConcurrency)
	var group sync.WaitGroup
	for activityID, handle := range running {
		group.Add(1)
		go func(activityID string, handle domain.ActivityHandle) {
			defer group.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			observed, err := s.activities.Inspect(ctx, handle.ID, mode)
			results <- inspectionResult{activityID: activityID, handle: handle, observed: observed, err: err}
		}(activityID, handle)
	}
	group.Wait()
	close(results)
	collected := make([]inspectionResult, 0, len(running))
	for result := range results {
		collected = append(collected, result)
	}
	return collected
}

func (s *Supervisor) applyInspectionResult(
	ctx context.Context,
	workflow domain.WorkflowVersion,
	runID string,
	result inspectionResult,
	running map[string]domain.ActivityHandle,
	completed map[string]domain.TaskExecution,
	tasks map[string]domain.TaskExecution,
	firstErr error,
) error {
	activityID, observed := result.activityID, result.observed
	if result.err != nil {
		if firstErr == nil {
			return fmt.Errorf("inspect activity %q: %w", activityID, result.err)
		}
		return firstErr
	}
	if observed == nil {
		if firstErr == nil {
			return fmt.Errorf("inspect activity %q: runtime returned no handle for %q", activityID, result.handle.ID)
		}
		return firstErr
	}
	task := tasks[activityID]
	switch observed.Status {
	case domain.HandleCompleted:
		if err := s.validateExpectedOutputs(ctx, workflow, runID, activityID); err != nil {
			task.Status, task.FailureReason = domain.TaskFailed, err.Error()
			_ = s.executions.SaveTask(ctx, task)
			if firstErr == nil {
				firstErr = fmt.Errorf("activity %q outputs: %w", activityID, err)
			}
			return firstErr
		}
		completeTask(&task, *observed)
		if err := s.sealWorkspace(ctx, runID, activityID, observed.Artifacts); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("seal workspace for activity %q: %w", activityID, err)
			}
			return firstErr
		}
		if err := s.executions.SaveTask(ctx, task); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return firstErr
		}
		tasks[activityID], completed[activityID] = task, task
		delete(running, activityID)
	case domain.HandleFailed, domain.HandleStopped:
		task.Status, task.FailureReason = domain.TaskFailed, observed.Failure
		task.FinishedAt = observed.FinishedAt
		applyHandleTiming(&task, *observed)
		task.RuntimeSeconds = maxFloat(0, observed.FinishedAt-executionStartedAt(*observed))
		_ = s.executions.SaveTask(ctx, task)
		tasks[activityID] = task
		delete(running, activityID)
		if firstErr == nil {
			firstErr = fmt.Errorf("activity %q failed: %s", activityID, observed.Failure)
		}
	}
	return firstErr
}

func (s *Supervisor) cancelRunningActivities(
	ctx context.Context,
	mode domain.ExecutionMode,
	running map[string]domain.ActivityHandle,
	tasks map[string]domain.TaskExecution,
	cause error,
) {
	type stopResult struct {
		activityID string
		err        error
	}
	results := make(chan stopResult, len(running))
	var group sync.WaitGroup
	semaphore := make(chan struct{}, runningInspectConcurrency)
	for activityID, handle := range running {
		group.Add(1)
		go func(activityID, handleID string) {
			defer group.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			results <- stopResult{activityID: activityID, err: s.activities.Stop(ctx, handleID, mode)}
		}(activityID, handle.ID)
	}
	group.Wait()
	close(results)
	for result := range results {
		task := tasks[result.activityID]
		task.FinishedAt = unixNow()
		if result.err == nil {
			task.Status = domain.TaskCancelled
			task.FailureReason = "cancelled after execution failure: " + cause.Error()
			delete(running, result.activityID)
		} else {
			task.Status = domain.TaskFailed
			task.FailureReason = "failed to cancel after execution failure: " + result.err.Error()
		}
		tasks[result.activityID] = task
		_ = s.executions.SaveTask(context.WithoutCancel(ctx), task)
	}
}

func (s *Supervisor) validateExpectedOutputs(ctx context.Context, workflow domain.WorkflowVersion, runID, activityID string) error {
	if s.config.Data == nil {
		return nil
	}
	activity := indexActivities(workflow.Activities)[activityID]
	raw := activity.Metadata["expectedOutputs"]
	values := make([]string, 0)
	switch typed := raw.(type) {
	case []string:
		values = typed
	case []any:
		for _, value := range typed {
			if name, ok := value.(string); ok {
				values = append(values, name)
			}
		}
	}
	if len(values) == 0 {
		return nil
	}
	instances, err := s.config.Data.ListInstances(ctx, runID)
	if err != nil {
		return fmt.Errorf("load observed outputs: %w", err)
	}
	return validateExpectedOutputInstances(activityID, values, instances)
}

func validateExpectedOutputInstances(activityID string, values []string, instances []domain.DataObjectInstance) error {
	observed := make(map[string]bool)
	for _, instance := range instances {
		if instance.ProducerActivityID == activityID && instance.Checksum != "" {
			observed[instance.RelativePath] = true
		}
	}
	for _, expected := range values {
		if !observed[expected] {
			return fmt.Errorf("required output %q was not observed with a checksum", expected)
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
	readyAllocations map[string]domain.RuntimeAllocation,
) error {
	results := make([]startResult, len(ready))
	semaphore := make(chan struct{}, readyActivityConcurrency)
	var group sync.WaitGroup
	for _, activityID := range ready {
		task, exists := tasks[activityID]
		if !exists {
			continue
		}
		task.Status = domain.TaskPreparing
		if task.Metadata == nil {
			task.Metadata = make(map[string]any)
		}
		task.Metadata["queueReason"] = "synchronizing predecessor workspaces and preparing runtime"
		if err := s.executions.SaveTask(ctx, task); err != nil {
			return fmt.Errorf("mark activity %q preparing: %w", activityID, err)
		}
		tasks[activityID] = task
	}
	for index, activityID := range ready {
		group.Add(1)
		go func(index int, activityID string) {
			defer group.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			results[index] = s.startReadyActivity(ctx, request, activityID, activities, assignments, resources, readyAllocations)
		}(index, activityID)
	}
	group.Wait()
	for _, result := range results {
		activityID := result.activityID
		if result.err != nil {
			attempt := tasks[activityID].Attempt
			_ = s.recordStartFailure(ctx, request.Run.ID, activityID, result.assignment, result.resource, selectRuntime(request, result.assignment), attempt, result.err)
			return result.err
		}
		request.RuntimeAllocations[activityID] = result.allocation
		*transfers = append(*transfers, result.transfers...)
		queued := tasks[activityID]
		task := newRunningTask(request.Run.ID, activityID, result.assignment, result.resource, result.allocation, result.handle, queued.Attempt, result.readyAt, result.preparation)
		if queued.Attempt > 0 {
			task.ReadyAt = queued.ReadyAt
			task.QueuedAt = queued.QueuedAt
			task.QueueSeconds = maxFloat(0, task.StartedAt-queued.QueuedAt)
		}
		tasks[activityID], running[activityID] = task, result.handle
		if result.handle.Status == domain.HandleCompleted {
			completeTask(&task, result.handle)
			if err := s.sealWorkspace(ctx, request.Run.ID, activityID, result.handle.Artifacts); err != nil {
				return fmt.Errorf("seal workspace for activity %q: %w", activityID, err)
			}
			tasks[activityID], completed[activityID] = task, task
			delete(running, activityID)
		}
		if err := s.executions.SaveTask(ctx, task); err != nil {
			return err
		}
	}
	return nil
}

func (s *Supervisor) startReadyActivity(
	ctx context.Context,
	request ports.ExecutionRequest,
	activityID string,
	activities map[string]domain.Activity,
	assignments map[string]domain.PlanAssignment,
	resources map[string]domain.Resource,
	readyAllocations map[string]domain.RuntimeAllocation,
) startResult {
	result := startResult{activityID: activityID, readyAt: unixNow()}
	activity, assignment := activities[activityID], assignments[activityID]
	result.assignment = assignment
	resource, ok := resources[assignment.ResourceID]
	if !ok {
		result.resource = domain.Resource{ID: assignment.ResourceID}
		result.err = fmt.Errorf("resource %q not found", assignment.ResourceID)
		return result
	}
	result.resource = resource
	allocation, allocated := readyAllocations[activityID]
	if !allocated {
		var err error
		allocation, err = s.ensureRuntimeAllocation(ctx, request, activityID, assignment, resource)
		if err != nil {
			result.err = fmt.Errorf("allocate runtime for activity %q: %w", activityID, err)
			return result
		}
	}
	result.allocation = allocation
	request.RuntimeAllocations = cloneRuntimeAllocations(request.RuntimeAllocations)
	request.RuntimeAllocations[activityID] = allocation
	request.PreparationRequirementsByActivity = clonePreparationRequirements(request.PreparationRequirementsByActivity)
	if err := s.addWorkspacePreparation(ctx, &request, activityID, resource, workspaceProducers(request.Workflow, activityID)); err != nil {
		result.err = fmt.Errorf("prepare workspace for activity %q: %w", activityID, err)
		return result
	}
	if requirement, required := request.PreparationRequirementsByActivity[activityID]; required {
		preparation, err := s.prepareActivity(ctx, request.Run.ID, activityID, requirement)
		if err != nil {
			result.err = fmt.Errorf("prepare activity %q: %w", activityID, err)
			return result
		}
		result.preparation = preparation
		result.transfers = transferObservations(request.Run.ID, activityID, resource.ID, workspaceProducers(request.Workflow, activityID), requirement, preparation.TransferRuns, request.NetworkTopology)
		if err := s.releaseProducerWorkspaceLeases(ctx, request.Run.ID, activityID); err != nil {
			result.err = fmt.Errorf("release workspace leases for activity %q: %w", activityID, err)
			return result
		}
	} else if activity.Command.Executable != nil && activity.Command.Executable.Source.Type == domain.ExecutableSourceType("build") {
		result.err = fmt.Errorf("activity %q has an executable reference but no preparation requirement", activityID)
		return result
	}
	if err := s.activateWorkspace(ctx, request.Run.ID, activityID); err != nil {
		result.err = fmt.Errorf("activate workspace for activity %q: %w", activityID, err)
		return result
	}
	handle, err := s.activities.Start(ctx, domain.ActivityExecutionContext{
		Run: request.Run, Workflow: request.Workflow, Activity: activity, Assignment: assignment,
		Resource: resource, RuntimeID: selectRuntime(request, assignment), Allocation: allocation, Preparation: result.preparation,
	})
	if err != nil {
		result.err = err
		return result
	}
	result.handle = handle
	return result
}

func cloneRuntimeAllocations(source map[string]domain.RuntimeAllocation) map[string]domain.RuntimeAllocation {
	result := make(map[string]domain.RuntimeAllocation, len(source)+1)
	for key, value := range source {
		result[key] = value
	}
	return result
}

func clonePreparationRequirements(source map[string]domain.PreparationRequirement) map[string]domain.PreparationRequirement {
	result := make(map[string]domain.PreparationRequirement, len(source)+1)
	for key, value := range source {
		result[key] = value
	}
	return result
}

func (s *Supervisor) ensureRuntimeAllocation(
	ctx context.Context,
	request ports.ExecutionRequest,
	activityID string,
	assignment domain.PlanAssignment,
	resource domain.Resource,
) (domain.RuntimeAllocation, error) {
	allocation := domain.RuntimeAllocation{
		ResourceID:   resource.ID,
		RuntimeID:    selectRuntime(request, assignment),
		ConnectionID: runtimeConnectionID(request, activityID),
	}
	if runtimeDriver(request, activityID) != domain.RuntimeDriverCloud {
		return allocation, nil
	}
	if s.config.Cloud == nil || s.config.CloudStore == nil {
		return allocation, fmt.Errorf("cloud allocation service is unavailable")
	}
	targetID := resource.ID
	if value, _ := resource.Metadata["capacityTargetId"].(string); strings.TrimSpace(value) != "" {
		targetID = strings.TrimSpace(value)
	}
	target, err := s.config.CloudStore.FindCapacityTarget(ctx, targetID)
	if err != nil || target == nil {
		return allocation, fmt.Errorf("load cloud capacity target %q: %w", targetID, err)
	}
	instances, err := s.config.CloudStore.ListProvisionedInstances(ctx, target.EnvironmentID)
	if err != nil {
		return allocation, err
	}
	sort.SliceStable(instances, func(left, right int) bool {
		return instances[left].CreatedAt.Before(instances[right].CreatedAt)
	})
	for _, instance := range instances {
		if instance.CapacityTargetID == target.ID && instance.Status == "ready" {
			allocation.CloudInstanceID = instance.ID
			return allocation, nil
		}
	}
	var instance domain.CloudProvisionedInstance
	if s.config.CloudAllocator != nil {
		instance, err = s.config.CloudAllocator.Allocate(ctx, request.Run.ID, activityID, *target)
	} else {
		instance, err = s.config.Cloud.Provision(ctx, target.EnvironmentID, domain.CloudProvisionRequest{CapacityTargetID: target.ID})
	}
	if err != nil {
		return allocation, err
	}
	if instance.ID == "" || instance.Status != "ready" {
		return allocation, fmt.Errorf("cloud capacity target %q did not produce a ready instance", target.ID)
	}
	allocation.CloudInstanceID = instance.ID
	return allocation, nil
}

func workspaceProducers(workflow domain.WorkflowVersion, activityID string) []string {
	producers := make([]string, 0)
	seen := make(map[string]bool)
	for _, dependency := range workflow.DataDependencies {
		if dependency.ConsumerActivityID == activityID && !seen[dependency.ProducerActivityID] {
			producers = append(producers, dependency.ProducerActivityID)
			seen[dependency.ProducerActivityID] = true
		}
	}
	// A control edge carries the predecessor workspace by default. Explicit data
	// dependencies remain useful for naming/filtering but are not required.
	for _, dependency := range workflow.Dependencies {
		if dependency.ActivityID == activityID && !seen[dependency.DependsOnActivityID] {
			producers = append(producers, dependency.DependsOnActivityID)
			seen[dependency.DependsOnActivityID] = true
		}
	}
	return producers
}

func (s *Supervisor) addWorkspacePreparation(
	ctx context.Context,
	request *ports.ExecutionRequest,
	activityID string,
	resource domain.Resource,
	producerIDs []string,
) error {
	driver := runtimeDriver(*request, activityID)
	if s.config.Data == nil || (len(producerIDs) == 0 && driver != domain.RuntimeDriverKubernetes && driver != domain.RuntimeDriverCloud && driver != domain.RuntimeDriverLocal) {
		return nil
	}
	destination, err := workspaceDestination(*request, activityID, resource, 0)
	if err != nil {
		return err
	}
	requirement := request.PreparationRequirementsByActivity[activityID]
	requirement.Workspace = &domain.WorkspaceMaterialization{
		ID: "workspace-" + request.Run.ID + "-" + activityID, RevisionID: request.Run.ID,
		Destination: destination, Status: domain.MaterializationPlanned,
	}
	requirement.WorkspaceTransfer = nil
	requirement.WorkspaceTransfers = nil
	for index, producerID := range producerIDs {
		if err := s.checkEphemeralCloudSource(ctx, *request, producerID); err != nil {
			return err
		}
		source, sourceErr := workspaceSourceForActivity(*request, producerID)
		if sourceErr != nil {
			return sourceErr
		}
		requirement.WorkspaceTransfers = append(requirement.WorkspaceTransfers, domain.DataTransferPlan{
			ID:                 fmt.Sprintf("transfer-workspace-%s-%s-%d", request.Run.ID, activityID, index),
			ExecutionRunID:     request.Run.ID,
			ProducerActivityID: producerID,
			ConsumerActivityID: activityID,
			Source:             source, Destination: destination, SyncWorkspace: true,
		})
	}
	if request.PreparationRequirementsByActivity == nil {
		request.PreparationRequirementsByActivity = make(map[string]domain.PreparationRequirement)
	}
	request.PreparationRequirementsByActivity[activityID] = requirement
	return nil
}

func (s *Supervisor) checkEphemeralCloudSource(ctx context.Context, request ports.ExecutionRequest, producerID string) error {
	instanceID := request.RuntimeAllocations[producerID].CloudInstanceID
	if instanceID == "" || s.config.CloudStore == nil {
		return nil
	}
	instance, err := s.config.CloudStore.FindProvisionedInstance(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("check source instance %q for producer %q: %w", instanceID, producerID, err)
	}
	if instance != nil && instance.Status == "ready" {
		return nil
	}
	status := "not found"
	if instance != nil {
		status = instance.Status
	}
	return fmt.Errorf("source instance %q for producer %q is unavailable (%s); ephemeral outputs cannot be transferred", instanceID, producerID, status)
}

func workspaceSourceForActivity(request ports.ExecutionRequest, activityID string) (domain.TransferLocation, error) {
	assignment, ok := indexAssignments(request.Plan.Assignments)[activityID]
	if !ok {
		return domain.TransferLocation{}, fmt.Errorf("producer %q has no resource assignment", activityID)
	}
	resource, ok := indexResources(request.Resources)[assignment.ResourceID]
	if !ok {
		return domain.TransferLocation{}, fmt.Errorf("producer %q resource %q was not found", activityID, assignment.ResourceID)
	}
	connectionID := runtimeConnectionID(request, activityID)
	allocation := request.RuntimeAllocations[activityID]
	if runtimeDriver(request, activityID) == domain.RuntimeDriverLocal {
		return transferLocation(localWorkspaceURI(request.Run.ID, activityID), resource, allocation), nil
	}
	if connectionID == "" {
		return domain.TransferLocation{}, fmt.Errorf("producer %q has no runtime connection", activityID)
	}
	switch runtimeDriver(request, activityID) {
	case domain.RuntimeDriverKubernetes:
		u := &url.URL{Scheme: "kubernetes", Host: connectionID, Path: "/tmp/akoflow/workspace"}
		query := u.Query()
		query.Set("namespace", runtimeNamespace(request, activityID))
		query.Set("claim", workspaceClaimName(request.Run.ID, activityID))
		if nodeName := kubernetesNodeName(resource); nodeName != "" {
			query.Set("nodeName", nodeName)
		}
		u.RawQuery = query.Encode()
		return transferLocation(u.String(), resource, allocation), nil
	case domain.RuntimeDriverSlurm:
		home := environmentHome(request.Resources, resource.EnvironmentVersionID)
		if home == "" {
			return domain.TransferLocation{}, fmt.Errorf("producer %q HPC environment has no discovered home directory", activityID)
		}
		u := &url.URL{Scheme: "file", Path: path.Join(home, "akoflow-workspaces", request.Run.ID, activityID)}
		query := u.Query()
		query.Set("connectionId", connectionID)
		u.RawQuery = query.Encode()
		return transferLocation(u.String(), resource, allocation), nil
	case domain.RuntimeDriverCloud:
		u := &url.URL{
			Scheme: "file",
			Path:   path.Join("/akoflow/workspace/runs", request.Run.ID, activityID),
		}
		query := u.Query()
		query.Set("connectionId", connectionID)
		u.RawQuery = query.Encode()
		return transferLocation(u.String(), resource, allocation), nil
	default:
		return domain.TransferLocation{}, fmt.Errorf("runtime for producer %q does not support workspace transfer", activityID)
	}
}

func workspaceDestination(request ports.ExecutionRequest, activityID string, resource domain.Resource, totalBytes int64) (domain.TransferLocation, error) {
	connectionID := runtimeConnectionID(request, activityID)
	allocation := request.RuntimeAllocations[activityID]
	if runtimeDriver(request, activityID) == domain.RuntimeDriverLocal {
		return transferLocation(localWorkspaceURI(request.Run.ID, activityID), resource, allocation), nil
	}
	if connectionID == "" {
		return domain.TransferLocation{}, fmt.Errorf("runtime for activity %q has no connection", activityID)
	}
	switch runtimeDriver(request, activityID) {
	case domain.RuntimeDriverKubernetes:
		u := &url.URL{Scheme: "kubernetes", Host: connectionID, Path: "/tmp/akoflow/workspace"}
		query := u.Query()
		query.Set("namespace", runtimeNamespace(request, activityID))
		query.Set("claim", workspaceClaimName(request.Run.ID, activityID))
		if nodeName := kubernetesNodeName(resource); nodeName != "" {
			query.Set("nodeName", nodeName)
		}
		query.Set("createClaim", "true")
		query.Set("claimBytes", fmt.Sprint(totalBytes*2))
		query.Set("runId", request.Run.ID)
		query.Set("activityId", activityID)
		u.RawQuery = query.Encode()
		return transferLocation(u.String(), resource, allocation), nil
	case domain.RuntimeDriverSlurm:
		home := environmentHome(request.Resources, resource.EnvironmentVersionID)
		if home == "" {
			return domain.TransferLocation{}, fmt.Errorf("HPC environment has no discovered home directory")
		}
		u := &url.URL{Scheme: "file", Path: path.Join(home, "akoflow-workspaces", request.Run.ID, activityID)}
		query := u.Query()
		query.Set("connectionId", connectionID)
		u.RawQuery = query.Encode()
		return transferLocation(u.String(), resource, allocation), nil
	case domain.RuntimeDriverCloud:
		u := &url.URL{
			Scheme: "file",
			Path:   path.Join("/akoflow/workspace/runs", request.Run.ID, activityID),
		}
		query := u.Query()
		query.Set("connectionId", connectionID)
		u.RawQuery = query.Encode()
		return transferLocation(u.String(), resource, allocation), nil
	default:
		return domain.TransferLocation{}, fmt.Errorf("runtime for activity %q does not support workspace transfer", activityID)
	}
}

func localWorkspaceURI(runID, activityID string) string {
	root := os.Getenv("AKOFLOW_LOCAL_WORKSPACE_ROOT")
	if root == "" {
		root = filepath.Join(os.TempDir(), "akoflow", "workspace")
	}
	return (&url.URL{
		Scheme: "file",
		Path:   filepath.Join(root, "runs", runID, activityID),
	}).String()
}

func kubernetesNodeName(resource domain.Resource) string {
	if resource.Type != domain.ResourceKubernetesMachine {
		return ""
	}
	if nodeName, _ := resource.Metadata["observedHostname"].(string); nodeName != "" {
		return nodeName
	}
	return resource.ProviderID
}

func transferLocation(uri string, resource domain.Resource, allocation domain.RuntimeAllocation) domain.TransferLocation {
	return domain.TransferLocation{
		URI: uri, ResourceID: resource.ID, EnvironmentID: resource.EnvironmentVersionID,
		RuntimeID: allocation.RuntimeID, ConnectionID: allocation.ConnectionID,
		CloudInstanceID: allocation.CloudInstanceID,
	}
}

func environmentHome(resources []domain.Resource, environmentVersionID string) string {
	for _, resource := range resources {
		if resource.EnvironmentVersionID == environmentVersionID {
			if home, _ := resource.Metadata["homeDirectory"].(string); home != "" {
				return home
			}
		}
	}
	return ""
}

func resourceConnectionID(resources []domain.Resource, resourceID string) (string, bool) {
	for _, resource := range resources {
		if resource.ID == resourceID {
			value, ok := resource.Metadata["connectionId"].(string)
			return value, ok && value != ""
		}
	}
	return "", false
}

func runtimeDriver(request ports.ExecutionRequest, activityID string) domain.RuntimeDriver {
	runtimeID := resolvedRuntimeID(request, activityID)
	for _, runtime := range request.Runtimes {
		if runtime.ID == runtimeID {
			return runtime.Driver
		}
	}
	return ""
}

func runtimeConnectionID(request ports.ExecutionRequest, activityID string) string {
	runtimeID := resolvedRuntimeID(request, activityID)
	for _, runtime := range request.Runtimes {
		if runtime.ID == runtimeID {
			value, _ := runtime.Configuration["connectionId"].(string)
			return value
		}
	}
	return ""
}

func runtimeNamespace(request ports.ExecutionRequest, activityID string) string {
	runtimeID := resolvedRuntimeID(request, activityID)
	for _, runtime := range request.Runtimes {
		if runtime.ID == runtimeID {
			if value, _ := runtime.Configuration["namespace"].(string); value != "" {
				return value
			}
		}
	}
	return "default"
}

func resolvedRuntimeID(request ports.ExecutionRequest, activityID string) string {
	for _, assignment := range request.Plan.Assignments {
		if assignment.ActivityID == activityID {
			return selectRuntime(request, assignment)
		}
	}
	return ""
}

func workspaceClaimName(runID, activityID string) string {
	value := strings.ToLower("akoflow-" + runID + "-" + activityID + "-workspace")
	value = strings.Map(func(character rune) rune {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-' {
			return character
		}
		return '-'
	}, value)
	value = strings.Trim(value, "-")
	if len(value) > 63 {
		digest := sha256.Sum256([]byte(value))
		value = strings.Trim(value[:54], "-") + fmt.Sprintf("-%x", digest[:4])
	}
	return value
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
	runID, activityID, targetResourceID string,
	producerIDs []string,
	requirement domain.PreparationRequirement,
	observations []domain.DataTransferRun,
	topology domain.NetworkTopology,
) []domain.DataTransfer {
	transfers := make([]domain.DataTransfer, 0, len(observations))
	for _, observation := range observations {
		sourceResourceID, producerActivityID := targetResourceID, ""
		plans := append([]domain.DataTransferPlan(nil), requirement.WorkspaceTransfers...)
		if requirement.WorkspaceTransfer != nil {
			plans = append(plans, *requirement.WorkspaceTransfer)
		}
		for _, plan := range plans {
			if plan.ID == observation.PlanID {
				sourceResourceID = plan.Source.ResourceID
				if plan.ProducerActivityID != "" {
					producerActivityID = plan.ProducerActivityID
				} else if len(producerIDs) == 1 {
					producerActivityID = producerIDs[0]
				}
				break
			}
		}
		// Artifact stores are not execution resources in the current transfer
		// schema. Attribute ingress to the selected resource until the schema
		// includes a storage endpoint dimension.
		bytes := observation.NetworkBytes
		if bytes <= 0 {
			bytes = observation.TransferredBytes
		}
		transfers = append(transfers, domain.DataTransfer{
			ID:                 runID + ":" + activityID + ":" + observation.ID,
			ExecutionRunID:     runID,
			ProducerActivityID: producerActivityID,
			ConsumerActivityID: activityID,
			SourceResourceID:   sourceResourceID,
			TargetResourceID:   targetResourceID,
			Bytes:              observation.TransferredBytes,
			StartedAt:          observation.StartedAt,
			FinishedAt:         observation.FinishedAt,
			DurationSeconds:    maxFloat(0, observation.FinishedAt-observation.StartedAt),
			Cost:               transferPrice(topology, sourceResourceID, targetResourceID) * float64(bytes),
			Strategy:           observation.Strategy,
			Route:              observation.Route,
			LogicalBytes:       observation.LogicalBytes,
			NetworkBytes:       observation.NetworkBytes,
			FilesTransferred:   observation.FilesTransferred,
		})
	}
	return transfers
}

func transferPrice(topology domain.NetworkTopology, sourceResourceID, targetResourceID string) float64 {
	for _, link := range topology.Links {
		direct := link.SourceResourceID == sourceResourceID && link.TargetResourceID == targetResourceID
		reverse := link.Bidirectional && link.SourceResourceID == targetResourceID && link.TargetResourceID == sourceResourceID
		if direct || reverse {
			return link.PricePerByte
		}
	}
	return 0
}

func (s *Supervisor) recordStartFailure(
	ctx context.Context,
	runID, activityID string,
	assignment domain.PlanAssignment,
	resource domain.Resource,
	runtimeID string,
	attempt int,
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
	task := newRunningTask(runID, activityID, assignment, resource, domain.RuntimeAllocation{ResourceID: resource.ID, RuntimeID: runtimeID}, handle, attempt, now, nil)
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
	allocation domain.RuntimeAllocation,
	handle domain.ActivityHandle,
	attempt int,
	readyAt float64,
	preparation *domain.PreparationGate,
) domain.TaskExecution {
	dataReadyAt := handle.StartedAt
	task := domain.TaskExecution{
		ID: taskExecutionID(runID, activityID, attempt), ExecutionRunID: runID,
		PlanAssignmentID: assignment.ID, ActivityID: activityID,
		PlannedResourceID: assignment.ResourceID, AllocatedResourceID: resource.ID,
		RuntimeID: allocation.RuntimeID, ConnectionID: allocation.ConnectionID,
		CloudInstanceID: allocation.CloudInstanceID,
		Attempt:         attempt, Status: domain.TaskRunning, ReadyAt: readyAt, DataReadyAt: dataReadyAt,
		QueuedAt: handle.StartedAt, StartedAt: handle.StartedAt,
		Metadata: map[string]any{"pricePerSecond": resource.PricePerSecond},
	}
	if preparation != nil {
		var preparationStartedAt, preparationFinishedAt float64
		for _, transfer := range preparation.TransferRuns {
			task.TransferSeconds += maxFloat(0, transfer.FinishedAt-transfer.StartedAt)
			task.TransferBytes += transfer.TransferredBytes
			if transfer.StartedAt > 0 && (preparationStartedAt == 0 || transfer.StartedAt < preparationStartedAt) {
				preparationStartedAt = transfer.StartedAt
			}
			if transfer.FinishedAt > preparationFinishedAt {
				preparationFinishedAt = transfer.FinishedAt
			}
		}
		if preparationFinishedAt > 0 {
			task.DataReadyAt = preparationFinishedAt
			task.Metadata["preparationStartedAt"] = preparationStartedAt
			task.Metadata["preparationFinishedAt"] = preparationFinishedAt
			task.Metadata["transferElapsedSeconds"] = maxFloat(0, preparationFinishedAt-preparationStartedAt)
			task.Metadata["transferWorkSeconds"] = task.TransferSeconds
			task.Metadata["readyWaitSeconds"] = maxFloat(0, preparationStartedAt-readyAt)
			task.Metadata["launchWaitSeconds"] = maxFloat(0, handle.StartedAt-preparationFinishedAt)
		}
	}
	if preparation == nil {
		task.Metadata["readyWaitSeconds"] = maxFloat(0, handle.StartedAt-readyAt)
		task.Metadata["launchWaitSeconds"] = 0.0
	}
	return task
}

func taskExecutionID(runID, activityID string, attempt int) string {
	if attempt <= 1 {
		return runID + ":" + activityID
	}
	return fmt.Sprintf("%s:%s:attempt-%d", runID, activityID, attempt)
}

func completeTask(task *domain.TaskExecution, handle domain.ActivityHandle) {
	task.Status, task.FinishedAt = domain.TaskCompleted, handle.FinishedAt
	applyHandleTiming(task, handle)
	task.RuntimeSeconds = maxFloat(0, handle.FinishedAt-executionStartedAt(handle))
	if pricePerSecond, ok := metadataFloat(task.Metadata, "pricePerSecond"); ok {
		task.Cost = task.RuntimeSeconds * pricePerSecond
	}
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
		if task, attempted := tasks[id]; attempted && task.Status != domain.TaskQueued && task.Status != domain.TaskReady {
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
	for _, transfer := range trace.Transfers {
		trace.Executed.Cost += transfer.Cost
	}
	trace.Executed.Cost += observedCloudIdleAndDiskCost(trace.Tasks, request.Resources)
	trace.Executed.MakespanSeconds = maxFloat(0, lastFinish-firstStart)
	return trace
}

func observedCloudIdleAndDiskCost(tasks []domain.TaskExecution, resources []domain.Resource) float64 {
	resourcesByID := indexResources(resources)
	type instanceWindow struct {
		first, last, taskCost float64
		resource              domain.Resource
	}
	windows := map[string]instanceWindow{}
	for _, task := range tasks {
		if task.CloudInstanceID == "" || task.FinishedAt <= task.StartedAt {
			continue
		}
		window := windows[task.CloudInstanceID]
		if window.first == 0 || task.StartedAt < window.first {
			window.first = task.StartedAt
		}
		if task.FinishedAt > window.last {
			window.last = task.FinishedAt
		}
		window.taskCost += task.Cost
		window.resource = resourcesByID[task.AllocatedResourceID]
		if window.resource.ID == "" {
			window.resource = resourcesByID[task.PlannedResourceID]
		}
		windows[task.CloudInstanceID] = window
	}
	total := 0.0
	for _, window := range windows {
		seconds := maxFloat(0, window.last-window.first)
		compute := seconds * window.resource.PricePerSecond
		idle := maxFloat(0, compute-window.taskCost)
		diskGiB := float64(window.resource.StorageBytes) / float64(1<<30)
		disk := diskGiB * metadataFloatValue(window.resource.Metadata, "diskPricePerGiBMonth") * seconds / (730 * 3600)
		total += idle + disk
	}
	return total
}

func metadataFloatValue(values map[string]any, key string) float64 {
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
