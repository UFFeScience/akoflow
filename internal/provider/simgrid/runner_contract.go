package simgrid

import (
	"encoding/json"
	"fmt"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type runnerInput struct {
	RunID              string             `json:"runId"`
	PlanID             string             `json:"planId"`
	DeadlineSeconds    float64            `json:"deadlineSeconds"`
	Budget             float64            `json:"budget"`
	Tasks              []runnerTask       `json:"tasks"`
	Dependencies       []runnerDependency `json:"dependencies"`
	ResourceLaneOrders []runnerLaneOrder  `json:"resourceLaneOrders"`
}

type runnerTask struct {
	ID              string  `json:"id"`
	AssignmentID    string  `json:"assignmentId"`
	ResourceID      string  `json:"resourceId"`
	FLOPs           float64 `json:"flops"`
	OverheadSeconds float64 `json:"overheadSeconds"`
	PricePerSecond  float64 `json:"pricePerSecond"`
}

type runnerDependency struct {
	ProducerID   string  `json:"producerId"`
	ConsumerID   string  `json:"consumerId"`
	Bytes        int64   `json:"bytes"`
	PricePerByte float64 `json:"pricePerByte"`
}

type runnerLaneOrder struct {
	PredecessorID string `json:"predecessorId"`
	SuccessorID   string `json:"successorId"`
}

func buildRunnerInput(request ports.ExecutionRequest, referenceFLOPS float64) ([]byte, error) {
	model := indexRequest(request)
	input := runnerInput{
		RunID: request.Run.ID, PlanID: request.Plan.ID,
		DeadlineSeconds: request.Plan.DeadlineSeconds, Budget: request.Plan.Budget,
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
			runtimeSeconds := resolveRuntime(activity, resource, model.profiles)
			flops = runtimeSeconds * resourceFLOPS(resource, referenceFLOPS)
		}
		input.Tasks = append(input.Tasks, runnerTask{
			ID: activity.ID, AssignmentID: assignment.ID, ResourceID: resource.ID,
			FLOPs: flops, OverheadSeconds: overhead,
			PricePerSecond: resource.PricePerSecond,
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
		input.Dependencies = append(input.Dependencies, runnerDependency{
			ProducerID: dependency.DependsOnActivityID, ConsumerID: dependency.ActivityID,
			Bytes: bytes, PricePerByte: transferPrice(request.NetworkTopology.Links, producer.ResourceID, consumer.ResourceID),
		})
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
