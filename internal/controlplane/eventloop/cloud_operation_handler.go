package eventloop

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

const (
	EventCloudProvisionRequested = "cloud.provision.requested"
	EventCloudConfigureRequested = "cloud.configure.requested"
	EventCloudStartRequested     = "cloud.start.requested"
	EventCloudStopRequested      = "cloud.stop.requested"
	EventCloudDestroyRequested   = "cloud.destroy.requested"
	EventCloudValidateRequested  = "cloud.validate.requested"
)

func CloudOperationEventType(kind string) string {
	return map[string]string{
		"provision": EventCloudProvisionRequested,
		"configure": EventCloudConfigureRequested,
		"start":     EventCloudStartRequested,
		"stop":      EventCloudStopRequested,
		"destroy":   EventCloudDestroyRequested,
		"validate":  EventCloudValidateRequested,
	}[kind]
}

func CloudOperationEventTypes() []string {
	return []string{
		EventCloudProvisionRequested,
		EventCloudConfigureRequested,
		EventCloudStartRequested,
		EventCloudStopRequested,
		EventCloudDestroyRequested,
		EventCloudValidateRequested,
	}
}

type cloudOperationPayload struct {
	OperationID string `json:"operationId"`
}

type CloudOperationHandler struct {
	store       ports.CloudOperationStore
	provisioner ports.CloudProvisioner
}

func NewCloudOperationHandler(store ports.CloudOperationStore, provisioner ports.CloudProvisioner) *CloudOperationHandler {
	return &CloudOperationHandler{store: store, provisioner: provisioner}
}

func (h *CloudOperationHandler) Handle(ctx context.Context, job domainqueue.Job) error {
	if h.store == nil || h.provisioner == nil {
		return fmt.Errorf("cloud operation handler is unavailable")
	}
	var payload cloudOperationPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode cloud operation message: %w", err)
	}
	if payload.OperationID == "" {
		return fmt.Errorf("decode cloud operation message: operation id is required")
	}
	operation, err := h.store.FindCloudOperation(ctx, payload.OperationID)
	if err != nil || operation == nil {
		return fmt.Errorf("cloud operation %q was not found", payload.OperationID)
	}
	if operation.Status == "completed" || operation.Status == "cancelled" {
		return nil
	}
	if operation.Kind == "stop" || operation.Kind == "destroy" {
		if guard, ok := h.store.(ports.CloudLifecycleGuard); ok {
			if guardErr := guard.CheckCloudLifecycle(ctx, operation.InstanceID, operation.ID); guardErr != nil {
				finished := time.Now().UTC()
				operation.Status, operation.Phase, operation.FailureReason, operation.FinishedAt = "failed", "failed", guardErr.Error(), &finished
				_ = h.store.UpdateCloudOperation(ctx, *operation)
				_ = h.store.AppendCloudOperationEvent(ctx, domain.CloudOperationEvent{OperationID: operation.ID, Sequence: 0, Timestamp: finished, Phase: "safety", Level: "error", Event: "lifecycle.blocked", Message: guardErr.Error()})
				return nil
			}
		}
	}
	now := time.Now().UTC()
	operation.Status, operation.StartedAt, operation.FailureReason = "running", &now, ""
	operation.Phase = operationPhase(operation.Kind)
	if err := h.store.UpdateCloudOperation(ctx, *operation); err != nil {
		return err
	}
	// Log events use their physical line number as the durable sequence. The
	// provision log is append-only and is read in full after every attempt, so
	// offsetting those line numbers by the attempt used to duplicate the complete
	// history on every retry. Keep lifecycle markers in a separate high range and
	// let repeated physical lines conflict idempotently in the repository.
	attemptSequence := 900_000_000 + job.Attempts*2
	_ = h.store.AppendCloudOperationEvent(ctx, domain.CloudOperationEvent{OperationID: operation.ID, Sequence: attemptSequence, Timestamp: now, Tool: operationTool(operation.Kind), Phase: operation.Phase, Event: "operation.started", Message: operation.Kind + " started"})
	instance, err := h.execute(ctx, *operation)
	if instance.ID != "" {
		operation.InstanceID = instance.ID
	}
	if operation.InstanceID != "" {
		h.appendProvisionLog(ctx, operation.ID, operation.InstanceID)
	}
	finished := time.Now().UTC()
	operation.FinishedAt = &finished
	if err != nil {
		operation.Status, operation.Phase, operation.FailureReason = "queued", "retrying", err.Error()
		operation.FinishedAt = nil
		if job.Attempts >= job.MaxAttempts {
			operation.Status, operation.Phase, operation.FinishedAt = "failed", "failed", &finished
		}
		_ = h.store.AppendCloudOperationEvent(context.Background(), domain.CloudOperationEvent{
			OperationID:     operation.ID,
			Sequence:        attemptSequence + 1,
			Timestamp:       finished,
			Tool:            operationTool(operation.Kind),
			Phase:           operation.Phase,
			Level:           "error",
			Event:           "operation.failed",
			Message:         err.Error(),
			DurationSeconds: finished.Sub(now).Seconds(),
		})
		_ = h.store.UpdateCloudOperation(context.Background(), *operation)
		return err
	}
	operation.Status, operation.Phase = "completed", "ready"
	_ = h.store.AppendCloudOperationEvent(ctx, domain.CloudOperationEvent{OperationID: operation.ID, Sequence: 1_000_000_000, Timestamp: finished, Tool: operationTool(operation.Kind), Phase: "ready", Event: "operation.completed", Message: operation.Kind + " completed", DurationSeconds: finished.Sub(*operation.StartedAt).Seconds()})
	return h.store.UpdateCloudOperation(ctx, *operation)
}

func (h *CloudOperationHandler) execute(ctx context.Context, operation domain.CloudOperationRun) (domain.CloudProvisionedInstance, error) {
	switch operation.Kind {
	case "provision":
		return h.provisioner.Provision(ctx, operation.EnvironmentID, operation.Request)
	case "configure":
		return h.provisioner.Configure(ctx, operation.InstanceID)
	case "destroy":
		return h.provisioner.Destroy(ctx, operation.InstanceID)
	case "start", "stop", "validate":
		return h.executeLifecycle(ctx, operation)
	default:
		return domain.CloudProvisionedInstance{}, fmt.Errorf("unsupported cloud operation %q", operation.Kind)
	}
}

func (h *CloudOperationHandler) executeLifecycle(ctx context.Context, operation domain.CloudOperationRun) (domain.CloudProvisionedInstance, error) {
	lifecycle, ok := h.provisioner.(ports.CloudLifecycleProvisioner)
	if !ok {
		return domain.CloudProvisionedInstance{}, fmt.Errorf("cloud lifecycle operation %q is unavailable", operation.Kind)
	}
	switch operation.Kind {
	case "start":
		return lifecycle.Start(ctx, operation.InstanceID)
	case "stop":
		return lifecycle.Stop(ctx, operation.InstanceID)
	default:
		return lifecycle.Validate(ctx, operation.InstanceID)
	}
}

func (h *CloudOperationHandler) appendProvisionLog(ctx context.Context, operationID, instanceID string) {
	raw, err := h.provisioner.Log(ctx, instanceID)
	if err != nil {
		return
	}
	for _, event := range ParseCloudOperationLog(operationID, raw) {
		_ = h.store.AppendCloudOperationEvent(ctx, event)
	}
}

func operationPhase(kind string) string {
	if kind == "configure" {
		return "ansible"
	}
	if kind == "validate" {
		return "validation"
	}
	return "terraform"
}

func operationTool(kind string) string {
	if kind == "configure" || kind == "validate" {
		return "ansible"
	}
	return "terraform"
}
