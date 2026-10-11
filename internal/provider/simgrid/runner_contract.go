package simgrid

import (
	"encoding/json"
	"fmt"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/domain/materializationmodel"
	domainworkflow "github.com/UFFeScience/akoflow/internal/domain/workflow"
)

type runnerInput struct {
	MaterializationModelVersion    int                        `json:"materializationModelVersion,omitempty"`
	MaterializationRelayResourceID string                     `json:"materializationRelayResourceId,omitempty"`
	MaterializationModelSource     string                     `json:"materializationModelSource,omitempty"`
	RunID                          string                     `json:"runId"`
	PlanID                         string                     `json:"planId"`
	DeadlineSeconds                float64                    `json:"deadlineSeconds"`
	Budget                         float64                    `json:"budget"`
	Tasks                          []runnerTask               `json:"tasks"`
	Dependencies                   []runnerDependency         `json:"dependencies"`
	ResourceLaneOrders             []runnerLaneOrder          `json:"resourceLaneOrders"`
	Interference                   *domain.InterferenceMatrix `json:"interference,omitempty"`
}

type runnerTask struct {
	ID                        string  `json:"id"`
	ActivityTypeID            string  `json:"activityTypeId"`
	AssignmentID              string  `json:"assignmentId"`
	ResourceID                string  `json:"resourceId"`
	FLOPs                     float64 `json:"flops"`
	OverheadSeconds           float64 `json:"overheadSeconds"`
	PricePerSecond            float64 `json:"pricePerSecond"`
	PreparationSeconds        float64 `json:"preparationSeconds,omitempty"`
	PublicationSeconds        float64 `json:"publicationSeconds,omitempty"`
	PreparationServiceGroupID string  `json:"preparationServiceGroupId,omitempty"`
	PreparationServiceSlots   int     `json:"preparationServiceSlots,omitempty"`
}

type runnerDependency struct {
	ProducerID        string                       `json:"producerId"`
	ConsumerID        string                       `json:"consumerId"`
	Bytes             int64                        `json:"bytes"`
	PricePerByte      float64                      `json:"pricePerByte"`
	Strategy          string                       `json:"strategy,omitempty"`
	LogicalBytes      int64                        `json:"logicalBytes,omitempty"`
	NetworkBytes      int64                        `json:"networkBytes,omitempty"`
	RelayNetworkBytes int64                        `json:"relayNetworkBytes,omitempty"`
	Stages            []materializationmodel.Stage `json:"stages,omitempty"`
}

type runnerLaneOrder struct {
	PredecessorID string `json:"predecessorId"`
	SuccessorID   string `json:"successorId"`
}

func buildRunnerInput(request ports.ExecutionRequest, referenceFLOPS float64) ([]byte, error) {
	projected, err := domainworkflow.ProjectWorkspaceDependenciesForResourceTypes(request.Workflow, assignedResourceTypes(request))
	if err != nil {
		return nil, fmt.Errorf("project workspace dependencies: %w", err)
	}
	request.Workflow = projected
	preparationModel, err := materializationmodel.Decode(request.NetworkTopology.Metadata)
	if err != nil {
		return nil, err
	}
	model := indexRequest(request)
	input := runnerInput{
		RunID: request.Run.ID, PlanID: request.Plan.ID,
		DeadlineSeconds: request.Plan.DeadlineSeconds, Budget: request.Plan.Budget,
	}
	if preparationModel != nil {
		input.MaterializationModelVersion = preparationModel.SchemaVersion
		input.MaterializationRelayResourceID = preparationModel.RelayResourceID
		input.MaterializationModelSource = preparationModel.Source
	}
	if raw := request.Plan.Metadata["interferenceMatrix"]; raw != nil {
		encoded, err := json.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("encode plan interference matrix: %w", err)
		}
		var matrix domain.InterferenceMatrix
		if err := json.Unmarshal(encoded, &matrix); err != nil {
			return nil, fmt.Errorf("decode plan interference matrix: %w", err)
		}
		input.Interference = &matrix
	}
	for _, activity := range request.Workflow.Activities {
		assignment, exists := model.assignments[activity.ID]
		if !exists {
			return nil, fmt.Errorf("activity %q has no SimGrid assignment", activity.ID)
		}
		resource, exists := model.resources[assignment.ResourceID]
		if !exists {
			return nil, fmt.Errorf("assignment for %q references missing resource %q", activity.ID, assignment.ResourceID)
		}
		flops := activityFLOPS(activity)
		overhead := resolveAssignmentOverhead(assignment, resource)
		if flops <= 0 {
			runtimeSeconds := resolveRuntime(activity, assignment, resource, model.profiles)
			flops = runtimeSeconds * resourceFLOPS(resource, referenceFLOPS)
		}
		preparationSeconds := 0.0
		publicationSeconds := 0.0
		preparationServiceGroupID, preparationServiceSlots := "", 0
		if preparationModel != nil {
			service := preparationModel.Resources[resource.ID]
			preparationServiceGroupID, preparationServiceSlots = service.BulkServiceGroupID, service.BulkServiceSlots
			encoded, err := json.Marshal(activity.Metadata["workspaceProjectionVolumes"])
			if err != nil {
				return nil, err
			}
			var volume domainworkflow.WorkspaceProjectionVolumes
			if activity.Metadata["workspaceProjectionVolumes"] == nil {
				return nil, fmt.Errorf("activity %q lacks projected materialization volumes", activity.ID)
			}
			if err = json.Unmarshal(encoded, &volume); err != nil {
				return nil, err
			}
			preparationSeconds, err = preparationModel.PreparationSeconds(resource.ID, volume.InputTotalBytes, volume.InputUniqueBytes)
			if err != nil {
				return nil, fmt.Errorf("consumer %q merge: %w", activity.ID, err)
			}
			publicationSeconds, err = preparationModel.PublicationSeconds(resource.ID, volume.TerminalObservationBytes, volume.PublicationCleanupBytes)
			if err != nil {
				return nil, err
			}
		}
		input.Tasks = append(input.Tasks, runnerTask{
			ID: activity.ID, ActivityTypeID: activity.ActivityTypeID,
			AssignmentID: assignment.ID, ResourceID: resource.ID,
			FLOPs: flops, OverheadSeconds: overhead,
			PricePerSecond:            resource.PricePerSecond,
			PreparationSeconds:        preparationSeconds,
			PublicationSeconds:        publicationSeconds,
			PreparationServiceGroupID: preparationServiceGroupID,
			PreparationServiceSlots:   preparationServiceSlots,
		})
	}
	dataBytes := make(map[string]int64)
	for _, dependency := range request.Workflow.DataDependencies {
		key := dependency.ProducerActivityID + "\x00" + dependency.ConsumerActivityID
		dataBytes[key] += dependency.SizeBytes
	}
	for _, dependency := range request.Workflow.Dependencies {
		producer := model.assignments[dependency.DependsOnActivityID]
		consumer := model.assignments[dependency.ActivityID]
		bytes := dataBytes[dependency.DependsOnActivityID+"\x00"+dependency.ActivityID]
		entry := runnerDependency{
			ProducerID: dependency.DependsOnActivityID, ConsumerID: dependency.ActivityID,
			Bytes: bytes, PricePerByte: transferPrice(request.NetworkTopology.Links, producer.ResourceID, consumer.ResourceID),
		}
		if preparationModel != nil {
			route, err := preparationModel.Route(producer.ResourceID, consumer.ResourceID, bytes)
			if err != nil {
				return nil, fmt.Errorf("dependency %s -> %s: %w", entry.ProducerID, entry.ConsumerID, err)
			}
			entry.Strategy, entry.LogicalBytes, entry.NetworkBytes, entry.RelayNetworkBytes, entry.Stages = route.Strategy, route.LogicalBytes, route.NetworkBytes, route.RelayNetworkBytes, route.Stages
			transferCost := 0.0
			for _, stage := range route.Stages {
				if _, exists := model.resources[stage.SourceResourceID]; !exists {
					return nil, fmt.Errorf("materialization stage source %q absent from platform", stage.SourceResourceID)
				}
				if _, exists := model.resources[stage.TargetResourceID]; !exists {
					return nil, fmt.Errorf("materialization stage target %q absent from platform", stage.TargetResourceID)
				}
				if stage.Bytes == 0 {
					continue
				}
				_, cost, exists := resolveNetworkPath(request.NetworkTopology.Links, stage.SourceResourceID, stage.TargetResourceID, stage.Bytes)
				if !exists {
					return nil, fmt.Errorf("no materialization network path %q -> %q", stage.SourceResourceID, stage.TargetResourceID)
				}
				transferCost += cost
			}
			if bytes > 0 {
				entry.PricePerByte = transferCost / float64(bytes)
			}
		}
		input.Dependencies = append(input.Dependencies, entry)
	}
	for successor, predecessor := range model.lanePrevious {
		input.ResourceLaneOrders = append(input.ResourceLaneOrders, runnerLaneOrder{
			PredecessorID: predecessor, SuccessorID: successor,
		})
	}
	return json.MarshalIndent(input, "", "  ")
}

func activityFLOPS(activity domain.Activity) float64 {
	if activity.Simulation != nil {
		if activity.Simulation.FLOPs > 0 {
			return activity.Simulation.FLOPs
		}
		if value, ok := numberMetadata(activity.Simulation.Parameters, "flops"); ok && value > 0 {
			return value
		}
	}
	if value, ok := numberMetadata(activity.Metadata, "flops"); ok && value > 0 {
		return value
	}
	return 0
}

func resourceFLOPS(resource domain.Resource, reference float64) float64 {
	if value, ok := numberMetadata(resource.Metadata, "flopsPerSecond"); ok && value > 0 {
		return value
	}
	if resource.ComputeSpeedup > 0 {
		return reference * resource.ComputeSpeedup
	}
	return reference
}

func transferPrice(links []domain.NetworkLink, source, target string) float64 {
	if source == target {
		return 0
	}
	_, cost, ok := resolveNetworkPath(links, source, target, 1)
	if !ok {
		return 0
	}
	return cost
}
