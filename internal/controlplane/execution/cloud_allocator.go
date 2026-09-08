package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/controlplane/eventloop"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
	"github.com/google/uuid"
)

type CloudAllocator interface {
	Allocate(context.Context, string, string, domain.CloudCapacityTarget) (domain.CloudProvisionedInstance, error)
	Release(context.Context, string, map[string]domain.RuntimeAllocation, bool) error
}

type CloudPrewarmer interface {
	Prewarm(context.Context, string, string, domain.CloudCapacityTarget) error
}

// QueuedCloudAllocator is the execution-to-infrastructure boundary. It never
// invokes Terraform itself: it persists an operation, publishes a durable
// message, then waits for the infrastructure worker to settle it.
type QueuedCloudAllocator struct {
	Cloud        ports.CloudConfigurationStore
	Operations   ports.CloudOperationStore
	Queue        ports.QueueStore
	PollInterval time.Duration
}

func (a QueuedCloudAllocator) Prewarm(ctx context.Context, executionRunID, activityID string, target domain.CloudCapacityTarget) error {
	instances, err := a.Cloud.ListProvisionedInstances(ctx, target.EnvironmentID)
	if err != nil {
		return err
	}
	for _, instance := range instances {
		if instance.CapacityTargetID == target.ID && (instance.Status == "ready" || instance.Status == "stopped") {
			return nil
		}
	}
	_, err = a.findOrCreate(ctx, executionRunID, activityID, target, "provision", "")
	return err
}

func (a QueuedCloudAllocator) Allocate(ctx context.Context, executionRunID, activityID string, target domain.CloudCapacityTarget) (domain.CloudProvisionedInstance, error) {
	instances, err := a.Cloud.ListProvisionedInstances(ctx, target.EnvironmentID)
	if err != nil {
		return domain.CloudProvisionedInstance{}, err
	}
	sort.SliceStable(instances, func(i, j int) bool { return instances[i].CreatedAt.Before(instances[j].CreatedAt) })
	var stopped *domain.CloudProvisionedInstance
	for _, instance := range instances {
		if instance.CapacityTargetID != target.ID {
			continue
		}
		if instance.Status == "ready" {
			return instance, nil
		}
		if instance.Status == "stopped" && stopped == nil {
			candidate := instance
			stopped = &candidate
		}
	}
	kind, instanceID := "provision", ""
	if stopped != nil {
		kind, instanceID = "start", stopped.ID
	}
	operation, err := a.findOrCreate(ctx, executionRunID, activityID, target, kind, instanceID)
	if err != nil {
		return domain.CloudProvisionedInstance{}, err
	}
	instance, err := a.wait(ctx, operation)
	if err != nil {
		return domain.CloudProvisionedInstance{}, err
	}
	if kind == "start" {
		validation, validationErr := a.findOrCreate(
			ctx,
			executionRunID,
			activityID,
			target,
			"validate",
			instance.ID,
		)
		if validationErr != nil {
			return domain.CloudProvisionedInstance{}, validationErr
		}
		return a.wait(ctx, validation)
	}
	return instance, nil
}

func (a QueuedCloudAllocator) wait(ctx context.Context, operation domain.CloudOperationRun) (domain.CloudProvisionedInstance, error) {
	interval := a.PollInterval
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		stored, err := a.Operations.FindCloudOperation(ctx, operation.ID)
		if err != nil {
			return domain.CloudProvisionedInstance{}, err
		}
		if stored == nil {
			return domain.CloudProvisionedInstance{}, fmt.Errorf("cloud allocation operation %q disappeared", operation.ID)
		}
		switch stored.Status {
		case "completed":
			instance, err := a.Cloud.FindProvisionedInstance(ctx, stored.InstanceID)
			if err != nil || instance == nil || instance.Status != "ready" {
				return domain.CloudProvisionedInstance{}, fmt.Errorf("cloud allocation %q completed without a ready instance: %w", stored.ID, err)
			}
			return *instance, nil
		case "failed", "cancelled":
			return domain.CloudProvisionedInstance{}, fmt.Errorf("cloud allocation %q %s: %s", stored.ID, stored.Status, stored.FailureReason)
		}
		select {
		case <-ctx.Done():
			return domain.CloudProvisionedInstance{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (a QueuedCloudAllocator) findOrCreate(ctx context.Context, executionRunID, activityID string, target domain.CloudCapacityTarget, kind, instanceID string) (domain.CloudOperationRun, error) {
	operations, err := a.Operations.ListCloudOperations(ctx)
	if err != nil {
		return domain.CloudOperationRun{}, err
	}
	for _, operation := range operations {
		if operation.Kind == kind && operation.ExecutionRunID == executionRunID && operation.ActivityID == activityID && operation.CapacityTargetID == target.ID {
			return operation, nil
		}
	}
	if kind == "provision" && instanceID == "" {
		instanceID = "cloud-instance-" + uuid.NewString()
	}
	operation := domain.CloudOperationRun{ID: "cloud-run-" + uuid.NewString(), Kind: kind, Status: "queued", Phase: "queued", EnvironmentID: target.EnvironmentID, CapacityTargetID: target.ID, InstanceID: instanceID, ExecutionRunID: executionRunID, ActivityID: activityID, Request: domain.CloudProvisionRequest{CapacityTargetID: target.ID, InstanceID: instanceID}, CreatedAt: time.Now().UTC()}
	if err := a.Operations.CreateCloudOperation(ctx, operation); err != nil {
		return domain.CloudOperationRun{}, err
	}
	payload, _ := json.Marshal(map[string]string{"operationId": operation.ID})
	job, err := domainqueue.New(domainqueue.CategoryInfrastructure, eventloop.CloudOperationEventType(kind), payload, time.Now().UTC())
	if err == nil {
		job.AggregateType, job.AggregateID = "cloud_operation", operation.ID
		job.IdempotencyKey = "cloud-allocation:" + executionRunID + ":" + activityID + ":" + target.ID + ":" + kind
		job.Priority, job.MaxAttempts = 100, 3
		_, err = a.Queue.Publish(ctx, job)
	}
	if err != nil {
		now := time.Now().UTC()
		operation.Status, operation.Phase, operation.FailureReason, operation.FinishedAt = "failed", "failed", err.Error(), &now
		_ = a.Operations.UpdateCloudOperation(ctx, operation)
		return operation, err
	}
	return operation, nil
}

func (a QueuedCloudAllocator) Release(ctx context.Context, executionRunID string, allocations map[string]domain.RuntimeAllocation, failed bool) error {
	seen := map[string]bool{}
	for activityID, allocation := range allocations {
		if allocation.CloudInstanceID == "" || seen[allocation.CloudInstanceID] {
			continue
		}
		seen[allocation.CloudInstanceID] = true
		instance, err := a.Cloud.FindProvisionedInstance(ctx, allocation.CloudInstanceID)
		if err != nil {
			return err
		}
		if instance == nil {
			return fmt.Errorf("cloud release instance %q was not found", allocation.CloudInstanceID)
		}
		target, err := a.Cloud.FindCapacityTarget(ctx, instance.CapacityTargetID)
		if err != nil {
			return err
		}
		if target == nil {
			return fmt.Errorf("cloud release capacity target %q was not found", instance.CapacityTargetID)
		}
		kind := ""
		if failed {
			switch target.LifecyclePolicy {
			case "destroy-on-failure":
				kind = "destroy"
			case "stop-on-failure":
				kind = "stop"
			}
		} else {
			switch target.LifecyclePolicy {
			case "destroy-after-run", "destroy-after-execution":
				kind = "destroy"
			case "stop-when-idle":
				kind = "stop"
			}
		}
		if kind == "" {
			continue
		}
		operation := domain.CloudOperationRun{ID: "cloud-run-" + uuid.NewString(), Kind: kind, Status: "queued", Phase: "queued", EnvironmentID: instance.EnvironmentID, CapacityTargetID: instance.CapacityTargetID, InstanceID: instance.ID, ExecutionRunID: executionRunID, ActivityID: activityID, CreatedAt: time.Now().UTC()}
		if err := a.Operations.CreateCloudOperation(ctx, operation); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]string{"operationId": operation.ID})
		job, err := domainqueue.New(domainqueue.CategoryInfrastructure, eventloop.CloudOperationEventType(kind), payload, time.Now().UTC())
		if err != nil {
			return err
		}
		job.AggregateType, job.AggregateID, job.IdempotencyKey, job.Priority = "cloud_operation", operation.ID, "cloud-release:"+executionRunID+":"+instance.ID+":"+kind, 100
		if _, err := a.Queue.Publish(ctx, job); err != nil {
			return err
		}
	}
	return nil
}
