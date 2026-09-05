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
	now := time.Now().UTC()
	operation.Status, operation.StartedAt, operation.FailureReason = "running", &now, ""
	if err := h.store.UpdateCloudOperation(ctx, *operation); err != nil {
		return err
	}
	var instance domain.CloudProvisionedInstance
	switch operation.Kind {
	case "provision":
		instance, err = h.provisioner.Provision(ctx, operation.EnvironmentID, operation.Request)
	case "configure":
		instance, err = h.provisioner.Configure(ctx, operation.InstanceID)
	case "destroy":
		instance, err = h.provisioner.Destroy(ctx, operation.InstanceID)
	default:
		err = fmt.Errorf("unsupported cloud operation %q", operation.Kind)
	}
	if instance.ID != "" {
		operation.InstanceID = instance.ID
	}
	finished := time.Now().UTC()
	operation.FinishedAt = &finished
	if err != nil {
		operation.Status, operation.FailureReason = "failed", err.Error()
		_ = h.store.UpdateCloudOperation(context.Background(), *operation)
		// The operation itself records a terminal failure. Do not retry Terraform
		// implicitly because provisioning is not safe to duplicate.
		return nil
	}
	operation.Status = "completed"
	return h.store.UpdateCloudOperation(ctx, *operation)
}
