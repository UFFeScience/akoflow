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

const EventCloudOperationRequested = "cloud.operation.requested"

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
	_ = h.store.AppendCloudOperationEvent(ctx, domain.CloudOperationEvent{OperationID: operation.ID, Sequence: 0, Timestamp: now, Tool: operationTool(operation.Kind), Phase: operation.Phase, Event: "operation.started", Message: operation.Kind + " started"})
	var instance domain.CloudProvisionedInstance
	switch operation.Kind {
	case "provision":
		instance, err = h.provisioner.Provision(ctx, operation.EnvironmentID, operation.Request)
	case "configure":
		instance, err = h.provisioner.Configure(ctx, operation.InstanceID)
	case "destroy":
		instance, err = h.provisioner.Destroy(ctx, operation.InstanceID)
	case "start", "stop", "validate":
		lifecycle, ok := h.provisioner.(ports.CloudLifecycleProvisioner)
		if !ok {
			err = fmt.Errorf("cloud lifecycle operation %q is unavailable", operation.Kind)
			break
		}
		switch operation.Kind {
		case "start":
			instance, err = lifecycle.Start(ctx, operation.InstanceID)
		case "stop":
			instance, err = lifecycle.Stop(ctx, operation.InstanceID)
		case "validate":
			instance, err = lifecycle.Validate(ctx, operation.InstanceID)
		}
	default:
		err = fmt.Errorf("unsupported cloud operation %q", operation.Kind)
	}
	if instance.ID != "" {
		operation.InstanceID = instance.ID
	}
	if operation.InstanceID != "" {
		if raw, logErr := h.provisioner.Log(ctx, operation.InstanceID); logErr == nil {
			for _, event := range ParseCloudOperationLog(operation.ID, raw) {
				_ = h.store.AppendCloudOperationEvent(ctx, event)
			}
		}
	}
	finished := time.Now().UTC()
	operation.FinishedAt = &finished
	if err != nil {
		operation.Status, operation.Phase, operation.FailureReason = "failed", "failed", err.Error()
		_ = h.store.AppendCloudOperationEvent(context.Background(), domain.CloudOperationEvent{OperationID: operation.ID, Sequence: 1_000_000_000, Timestamp: finished, Tool: operationTool(operation.Kind), Phase: "failed", Level: "error", Event: "operation.failed", Message: err.Error()})
		_ = h.store.UpdateCloudOperation(context.Background(), *operation)
		// The operation itself records a terminal failure. Do not retry Terraform
		// implicitly because provisioning is not safe to duplicate.
		return nil
	}
	operation.Status, operation.Phase = "completed", "ready"
	_ = h.store.AppendCloudOperationEvent(ctx, domain.CloudOperationEvent{OperationID: operation.ID, Sequence: 1_000_000_000, Timestamp: finished, Tool: operationTool(operation.Kind), Phase: "ready", Event: "operation.completed", Message: operation.Kind + " completed", DurationSeconds: finished.Sub(*operation.StartedAt).Seconds()})
	return h.store.UpdateCloudOperation(ctx, *operation)
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
