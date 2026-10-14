package execution

import (
	"context"
	"fmt"
	"hash/fnv"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type Controller struct {
	runtimes ports.RuntimeResolver
	handles  ports.ActivityExecutionStore
	data     ports.DataCatalog
	// Fixed stripes serialize lifecycle operations on the same handle without
	// retaining an unbounded lock entry for every scientific activity.
	handleLocks [256]sync.Mutex
}

func (s *Controller) lockHandle(id string) func() {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	lock := &s.handleLocks[h.Sum32()%uint32(len(s.handleLocks))]
	lock.Lock()
	return lock.Unlock
}

func New(runtimes ports.RuntimeResolver, handles ports.ActivityExecutionStore, catalogs ...ports.DataCatalog) *Controller {
	controller := &Controller{runtimes: runtimes, handles: handles}
	if len(catalogs) > 0 {
		controller.data = catalogs[0]
	}
	return controller
}

func (s *Controller) Start(ctx context.Context, execution domain.ActivityExecutionContext) (domain.ActivityHandle, error) {
	if err := execution.Activity.Validate(); err != nil {
		return domain.ActivityHandle{}, err
	}
	// This is intentionally enforced at the final execution boundary as well as
	// by the supervisor. A caller cannot bypass a failed materialization by
	// constructing an ActivityExecutionContext directly.
	if err := prepareActivityCommand(&execution); err != nil {
		return domain.ActivityHandle{}, err
	}
	capability, err := capabilityFor(execution.Run.Mode)
	if err != nil {
		return domain.ActivityHandle{}, err
	}
	if !execution.Activity.Supports(capability) {
		return domain.ActivityHandle{}, fmt.Errorf("activity %q does not support %q execution", execution.Activity.ID, execution.Run.Mode)
	}
	handleID := execution.HandleID()
	unlock := s.lockHandle(handleID)
	defer unlock()
	existing, err := s.handles.Find(ctx, handleID)
	if err != nil {
		return domain.ActivityHandle{}, fmt.Errorf("find activity handle: %w", err)
	}
	if existing != nil {
		if existing.Metadata[ports.RuntimeReconciliationPending] == true {
			return *existing, ports.NewRuntimeReconciliationError(fmt.Errorf("runtime cleanup is not confirmed"), existing.Metadata)
		}
		return *existing, nil
	}
	if execution.RuntimeID == "" {
		return domain.ActivityHandle{}, fmt.Errorf("activity %q has no selected runtime", execution.Activity.ID)
	}
	runtime, err := s.runtimes.Resolve(execution.Run.Mode, execution.RuntimeID)
	if err != nil {
		return domain.ActivityHandle{}, err
	}
	handle, err := runtime.Start(ctx, execution)
	if err != nil {
		return s.persistUncertainStart(ctx, execution, err)
	}
	if handle.ActivityID != execution.Activity.ID || handle.RunID != execution.Run.ID {
		return domain.ActivityHandle{}, fmt.Errorf("runtime returned an invalid activity handle")
	}
	handle.ID = handleID
	handle.Log = appendLog(handle.Log, "info", "Activity started on runtime "+handle.RuntimeID+".")
	if handle.Status == domain.HandleCompleted {
		handle.Log = appendLog(handle.Log, "success", "Activity completed successfully.")
	}
	if handle.Status == domain.HandleFailed {
		handle.Log = appendLog(handle.Log, "error", failureMessage(handle.Failure))
	}
	if len(handle.Metrics) > 0 {
		if metrics, ok := s.handles.(interface {
			SaveActivityMetrics(context.Context, []domain.ActivityMetricSample) error
		}); ok {
			if metricErr := metrics.SaveActivityMetrics(ctx, handle.Metrics); metricErr != nil {
				if handle.Metadata == nil {
					handle.Metadata = make(map[string]any)
				}
				handle.Metadata["metricCollectionError"] = metricErr.Error()
			}
		}
	}
	if err := s.handles.Save(ctx, handle); err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if stopErr := runtime.Stop(cleanupCtx, handle); stopErr != nil {
			return domain.ActivityHandle{}, fmt.Errorf("persist activity handle: %w; stop submitted runtime %q: %v", err, handle.ExternalID, stopErr)
		}
		return domain.ActivityHandle{}, fmt.Errorf("persist activity handle: %w", err)
	}
	return handle, nil
}

func (s *Controller) Inspect(ctx context.Context, handleID string, mode domain.ExecutionMode) (*domain.ActivityHandle, error) {
	unlock := s.lockHandle(handleID)
	defer unlock()
	handle, err := s.handles.Find(ctx, handleID)
	if err != nil || handle == nil {
		return handle, err
	}
	if handle.Status == domain.HandleStopped || handle.Status == domain.HandleCompleted || handle.Status == domain.HandleFailed {
		return handle, nil
	}
	runtime, err := s.runtimes.Resolve(mode, handle.RuntimeID)
	if err != nil {
		return s.persistInspectionFailure(ctx, handle, err)
	}
	updated, err := runtime.Inspect(ctx, *handle)
	if err != nil {
		return s.persistInspectionFailure(ctx, handle, err)
	}
	updated.Log = mergeLogs(handle.Log, updated.Log)
	s.saveInspectionMetrics(ctx, handle, &updated)
	if updated.Status != handle.Status {
		switch updated.Status {
		case domain.HandleCompleted:
			updated.Log = appendLog(updated.Log, "success", "Activity completed successfully.")
		case domain.HandleFailed, domain.HandleStopped:
			updated.Log = appendLog(updated.Log, "error", failureMessage(updated.Failure))
		}
	}
	if s.data != nil && updated.Artifacts != nil &&
		(updated.Status == domain.HandleCompleted || updated.Status == domain.HandleFailed) {
		if catalogErr := s.data.CatalogArtifacts(ctx, updated); catalogErr != nil {
			if updated.Metadata == nil {
				updated.Metadata = make(map[string]any)
			}
			updated.Metadata["artifactCatalogError"] = catalogErr.Error()
			if updated.Status == domain.HandleCompleted {
				updated.Status = domain.HandleFailed
				updated.FinishedAt = float64(time.Now().UnixNano()) / float64(time.Second)
				updated.Failure = "catalog activity outputs: " + catalogErr.Error()
				updated.Log = appendLog(updated.Log, "error", updated.Failure)
			}
		}
	}
	if err := s.handles.Save(ctx, updated); err != nil {
		return nil, err
	}
	if updated.Status == domain.HandleCompleted || updated.Status == domain.HandleFailed || updated.Status == domain.HandleStopped {
		if releaser, ok := runtime.(interface {
			Release(context.Context, domain.ActivityHandle) error
		}); ok {
			if releaseErr := releaser.Release(ctx, updated); releaseErr != nil {
				if updated.Metadata == nil {
					updated.Metadata = make(map[string]any)
				}
				updated.Metadata["runtimeCleanupError"] = releaseErr.Error()
				if saveErr := s.handles.Save(ctx, updated); saveErr != nil {
					return nil, saveErr
				}
			}
		}
	}
	return &updated, nil
}

func prepareActivityCommand(execution *domain.ActivityExecutionContext) error {
	if execution.Preparation == nil {
		return nil
	}
	if err := execution.Preparation.Ready(); err != nil {
		return fmt.Errorf("activity preparation is not ready: %w", err)
	}
	if executable := execution.Preparation.Executable; executable != nil {
		execution.Activity.Command.ResolvedExecutable = &domain.ResolvedExecutable{
			VariantID: executable.VariantID, Digest: executable.Digest,
			LocalPath: executable.DestinationPath, EnvironmentID: executable.EnvironmentID,
			ResourceID: executable.ResourceID, MaterializationID: executable.ID,
			MaterializationDone: executable.Committed(),
		}
	}
	if workspace := execution.Preparation.Workspace; workspace != nil && execution.Activity.Command.WorkingDirectory == "" {
		if destination, err := url.Parse(workspace.Destination.URI); err == nil && destination.Path != "" {
			execution.Activity.Command.WorkingDirectory = destination.Path
		}
	}
	return nil
}

func (s *Controller) saveInspectionMetrics(ctx context.Context, previous *domain.ActivityHandle, updated *domain.ActivityHandle) {
	metrics, ok := s.handles.(interface {
		SaveActivityMetrics(context.Context, []domain.ActivityMetricSample) error
	})
	if !ok || len(updated.Metrics) == 0 {
		return
	}
	if err := metrics.SaveActivityMetrics(ctx, updated.Metrics); err != nil {
		if updated.Metadata == nil {
			updated.Metadata = make(map[string]any)
		}
		// Preserve the cursor so the next inspection retries the same rows.
		if cursor, exists := previous.Metadata["metricsSampleCount"]; exists {
			updated.Metadata["metricsSampleCount"] = cursor
		} else {
			delete(updated.Metadata, "metricsSampleCount")
		}
		updated.Metadata["metricCollectionError"] = err.Error()
	} else {
		delete(updated.Metadata, "metricCollectionError")
	}
}

func (s *Controller) persistInspectionFailure(ctx context.Context, handle *domain.ActivityHandle, err error) (*domain.ActivityHandle, error) {
	if ctx.Err() != nil {
		// Observation cancellation is not evidence that computation failed.
		return handle, ctx.Err()
	}
	handle.Status = domain.HandleFailed
	handle.FinishedAt = float64(time.Now().UnixNano()) / float64(time.Second)
	handle.Failure = "inspect activity: " + err.Error()
	handle.Log = appendLog(handle.Log, "error", handle.Failure)
	if saveErr := s.handles.Save(ctx, *handle); saveErr != nil {
		return nil, fmt.Errorf("inspect activity: %w; save failed handle: %v", err, saveErr)
	}
	return handle, nil
}

func appendLog(log, level, message string) string {
	if message == "" {
		return log
	}
	entry := fmt.Sprintf("[AKOFLOW %s] %s", strings.ToUpper(level), message)
	if strings.Contains(log, entry) {
		return log
	}
	if log == "" {
		return entry + "\n"
	}
	return strings.TrimRight(log, "\n") + "\n" + entry + "\n"
}

func mergeLogs(previous, observed string) string {
	if observed == "" || observed == previous {
		return previous
	}
	runtimeLog, lifecycleLog := splitLifecycleLog(previous)
	if runtimeLog == "" || strings.HasPrefix(observed, runtimeLog) {
		return strings.TrimRight(observed, "\n") + lifecycleLog
	}
	if strings.HasPrefix(runtimeLog, observed) {
		return previous
	}
	return strings.TrimRight(observed, "\n") + lifecycleLog
}

func splitLifecycleLog(log string) (string, string) {
	lines := strings.Split(strings.TrimRight(log, "\n"), "\n")
	firstLifecycle := len(lines)
	for index, line := range lines {
		if strings.HasPrefix(line, "[AKOFLOW ") {
			firstLifecycle = index
			break
		}
	}
	if firstLifecycle == len(lines) {
		return strings.TrimRight(log, "\n"), ""
	}
	runtimeLog := strings.Join(lines[:firstLifecycle], "\n")
	if runtimeLog != "" {
		runtimeLog += "\n"
	}
	return runtimeLog, "\n" + strings.Join(lines[firstLifecycle:], "\n") + "\n"
}

func failureMessage(failure string) string {
	if failure != "" {
		return failure
	}
	return "Activity stopped without a reported failure reason."
}

func (s *Controller) Stop(ctx context.Context, handleID string, mode domain.ExecutionMode) error {
	unlock := s.lockHandle(handleID)
	defer unlock()
	handle, err := s.handles.Find(ctx, handleID)
	if err != nil {
		return err
	}
	if handle == nil {
		return fmt.Errorf("activity handle %q not found", handleID)
	}
	if handle.Status == domain.HandleStopped || handle.Status == domain.HandleCompleted {
		return nil
	}
	runtime, err := s.runtimes.Resolve(mode, handle.RuntimeID)
	if err != nil {
		return err
	}
	if err := runtime.Stop(ctx, *handle); err != nil {
		return err
	}
	stopped := *handle
	stopped.Status = domain.HandleStopped
	stopped.Metadata = make(map[string]any, len(handle.Metadata))
	for key, value := range handle.Metadata {
		if key != ports.RuntimeReconciliationPending {
			stopped.Metadata[key] = value
		}
	}
	return s.handles.Save(ctx, stopped)
}

func capabilityFor(mode domain.ExecutionMode) (domain.ActivityCapability, error) {
	switch mode {
	case domain.ExecutionModeReal:
		return domain.ActivityCapabilityReal, nil
	case domain.ExecutionModeSimulation:
		return domain.ActivityCapabilitySimulation, nil
	case domain.ExecutionModeInteractive:
		return domain.ActivityCapabilityInteractive, nil
	default:
		return "", fmt.Errorf("unsupported execution mode %q", mode)
	}
}
