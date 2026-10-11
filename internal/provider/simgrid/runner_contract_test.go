package simgrid

import (
	"encoding/json"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/domain/materializationmodel"
	domainworkflow "github.com/UFFeScience/akoflow/internal/domain/workflow"
	"github.com/stretchr/testify/require"
)

func materializedRequestFixture() Request {
	request := processRequestFixture()
	request.Workflow.Activities[0].Metadata = map[string]any{"workspaceProjection": domainworkflow.WorkspaceProjection{Contract: "whole-workspace-v1", Source: "baseline", Outputs: []domainworkflow.ProjectedFile{{Path: "data", SizeBytes: 100, ContentID: "output"}}}}
	request.Workflow.Activities[1].Metadata = map[string]any{"workspaceProjection": domainworkflow.WorkspaceProjection{Contract: "whole-workspace-v1", Source: "baseline"}}
	request.Resources = append(request.Resources, domain.Resource{ID: "relay", ComputeSpeedup: 1})
	request.NetworkTopology.Links = []domain.NetworkLink{
		{SourceResourceID: "edge", TargetResourceID: "relay", BandwidthBitsPerSecond: 8000, PricePerByte: .01},
		{SourceResourceID: "relay", TargetResourceID: "cloud", BandwidthBitsPerSecond: 8000, PricePerByte: .02},
	}
	request.NetworkTopology.Metadata = map[string]any{"workspaceMaterializationModel": materializationmodel.Model{
		SchemaVersion: 1, Source: "baseline", RelayResourceID: "relay", Resources: map[string]materializationmodel.Resource{"edge": {Class: "hpc", Domain: "hpc"}, "cloud": {Class: "micro", Domain: "cloud"}, "relay": {Class: "relay", Domain: "relay"}},
		Profiles:      []materializationmodel.Profile{{Strategy: "gateway", SourceClass: "hpc", TargetClass: "micro", Source: "baseline", Samples: 2, ArchiveToWorkspaceRatio: .5, IngressArchiveBytesPerSecond: 50, UploadArchiveBytesPerSecond: 25, IngressIncludesSourcePreparation: true}},
		MergeProfiles: []materializationmodel.MergeProfile{{TargetClass: "micro", Source: "baseline", Samples: 2, CheckSecondsPerInputByte: .01, CopySecondsPerUniqueByte: .02, VerifySecondsPerUniqueByte: .03}},
	}}
	return request
}

func TestRunnerContractMaterializationStagesAndConsumerMerge(t *testing.T) {
	request := materializedRequestFixture()
	payload, err := buildRunnerInput(request, 1e9)
	require.NoError(t, err)
	var input runnerInput
	require.NoError(t, json.Unmarshal(payload, &input))
	require.Equal(t, 1, input.MaterializationModelVersion)
	require.Len(t, input.Dependencies, 1)
	dependency := input.Dependencies[0]
	require.Equal(t, "gateway", dependency.Strategy)
	require.Len(t, dependency.Stages, 2)
	require.EqualValues(t, 50, dependency.Stages[0].Bytes)
	require.EqualValues(t, 100, dependency.NetworkBytes)
	require.EqualValues(t, 100, dependency.RelayNetworkBytes)
	require.InDelta(t, 1.5, float64(dependency.Bytes)*dependency.PricePerByte, 1e-9)
	require.InDelta(t, 6, input.Tasks[1].PreparationSeconds, 1e-9)
	request.Resources = request.Resources[:2]
	_, err = buildRunnerInput(request, 1e9)
	require.ErrorContains(t, err, "absent from platform")
}

func TestRunnerContractKeepsRuntimeAdmissionSeparate(t *testing.T) {
	r := materializedRequestFixture()
	m := r.NetworkTopology.Metadata["workspaceMaterializationModel"].(materializationmodel.Model)
	resource := m.Resources["cloud"]
	resource.ExecutionRuntimeID = "native-cloud"
	m.Resources["cloud"] = resource
	m.AdmissionProfiles = []materializationmodel.AdmissionProfile{{TargetClass: "micro", RuntimeID: "native-cloud", Source: "fixture", Samples: 1, FixedSeconds: 7}}
	m.SchemaVersion = 2
	r.NetworkTopology.Metadata["workspaceMaterializationModel"] = m
	payload, err := buildRunnerInput(r, 1e9)
	require.NoError(t, err)
	var input runnerInput
	require.NoError(t, json.Unmarshal(payload, &input))
	require.Equal(t, 7.0, input.Tasks[1].AdmissionSeconds)
	require.Equal(t, 6.0, input.Tasks[1].PreparationSeconds)
	require.Zero(t, input.Tasks[0].AdmissionSeconds)
}

func TestRunnerContractIncludesFixedMergeCommandServiceOnce(t *testing.T) {
	request := materializedRequestFixture()
	model := request.NetworkTopology.Metadata["workspaceMaterializationModel"].(materializationmodel.Model)
	model.MergeProfiles[0].CommandOverheadSeconds = 2.5
	request.NetworkTopology.Metadata["workspaceMaterializationModel"] = model
	payload, err := buildRunnerInput(request, 1e9)
	require.NoError(t, err)
	var input runnerInput
	require.NoError(t, json.Unmarshal(payload, &input))
	require.Zero(t, input.Tasks[0].PreparationSeconds)
	require.InDelta(t, 8.5, input.Tasks[1].PreparationSeconds, 1e-9)
	require.EqualValues(t, 100, input.Dependencies[0].NetworkBytes)
}

func TestRunnerContractIncludesInputObservationOnce(t *testing.T) {
	request := materializedRequestFixture()
	projection := request.Workflow.Activities[1].Metadata["workspaceProjection"].(domainworkflow.WorkspaceProjection)
	terminalBytes := int64(300)
	projection.TerminalObservationBytes = &terminalBytes
	cleanupBytes := int64(100)
	projection.PublicationCleanupBytes = &cleanupBytes
	request.Workflow.Activities[1].Metadata["workspaceProjection"] = projection
	model := request.NetworkTopology.Metadata["workspaceMaterializationModel"].(materializationmodel.Model)
	model.InputObservationProfiles = []materializationmodel.InputObservationProfile{{TargetClass: "micro", Source: "queue-free-fixture", Samples: 1, SecondsPerUniqueByte: .1}}
	model.PublicationProfiles = []materializationmodel.PublicationProfile{{TargetClass: "micro", Source: "queue-free-fixture", Samples: 1, ScanSecondsPerWorkspaceByte: .02, CleanupSecondsPerInputByte: .01}}
	request.NetworkTopology.Metadata["workspaceMaterializationModel"] = model
	payload, err := buildRunnerInput(request, 1e9)
	require.NoError(t, err)
	var input runnerInput
	require.NoError(t, json.Unmarshal(payload, &input))
	require.Zero(t, input.Tasks[0].PreparationSeconds)
	require.InDelta(t, 16, input.Tasks[1].PreparationSeconds, 1e-9)
	require.InDelta(t, 7, input.Tasks[1].PublicationSeconds, 1e-9)
	require.EqualValues(t, 100, input.Dependencies[0].NetworkBytes)
}

func TestRunnerInputUsesWholeProducerWorkspaceNotDeclaredEdge(t *testing.T) {
	request := ports.ExecutionRequest{
		Run:       domain.ExecutionRun{ID: "run"},
		Plan:      domain.SchedulePlan{Assignments: []domain.PlanAssignment{{ActivityID: "a", ResourceID: "source"}, {ActivityID: "b", ResourceID: "target"}}},
		Resources: []domain.Resource{{ID: "source", ComputeSpeedup: 1}, {ID: "target", ComputeSpeedup: 1}},
		Workflow: domain.WorkflowVersion{
			Activities: []domain.Activity{
				{ID: "a", Metadata: map[string]any{"workspaceProjection": domainworkflow.WorkspaceProjection{Contract: "whole-workspace-v1", Source: "baseline", Inputs: []domainworkflow.ProjectedFile{{Path: "in.fits", SizeBytes: 1000, ContentID: "in"}}, Outputs: []domainworkflow.ProjectedFile{{Path: "out.fits", SizeBytes: 2000, ContentID: "estimated-output"}}}}},
				{ID: "b", Metadata: map[string]any{"workspaceProjection": domainworkflow.WorkspaceProjection{Contract: "whole-workspace-v1", Source: "baseline"}}},
			},
			Dependencies:     []domain.ActivityDependency{{ActivityID: "b", DependsOnActivityID: "a"}},
			DataDependencies: []domain.ActivityDataDependency{{ProducerActivityID: "a", ConsumerActivityID: "b", SizeBytes: 12}},
		},
	}
	payload, err := buildRunnerInput(request, 1e9)
	require.NoError(t, err)
	var input runnerInput
	require.NoError(t, json.Unmarshal(payload, &input))
	require.Len(t, input.Dependencies, 1)
	require.EqualValues(t, 3000, input.Dependencies[0].Bytes)
	require.EqualValues(t, 12, request.Workflow.DataDependencies[0].SizeBytes)
}

func TestRunnerInputPrefersActivityRuntimeAndSeparatesOverhead(t *testing.T) {
	request := ports.ExecutionRequest{
		Run: domain.ExecutionRun{ID: "run"},
		Plan: domain.SchedulePlan{ID: "plan", Assignments: []domain.PlanAssignment{{
			ID: "assignment", ActivityID: "activity", ResourceID: "resource",
			Metadata: map[string]any{"bootOverheadSeconds": 12.0, "containerOverheadSeconds": 3.0},
		}}},
		Workflow: domain.WorkflowVersion{Activities: []domain.Activity{{
			ID: "activity", ActivityTypeID: "shared-type",
			Metadata: map[string]any{"baseRuntimeSeconds": 1_000.0},
		}}},
		Resources: []domain.Resource{{ID: "resource", ComputeSpeedup: 10}},
		ActivityProfiles: []domain.ActivityResourceProfile{{
			ActivityTypeID: "shared-type", ResourceID: "resource", RuntimeSeconds: 0.178,
		}},
	}

	payload, err := buildRunnerInput(request, 1e9)
	require.NoError(t, err)
	var input runnerInput
	require.NoError(t, json.Unmarshal(payload, &input))
	require.Len(t, input.Tasks, 1)
	require.InDelta(t, 1e12, input.Tasks[0].FLOPs, 1)
	require.InDelta(t, 15, input.Tasks[0].OverheadSeconds, 1e-9)
}

func TestRunnerInputUsesFrozenPlannedRuntimeForProfileSimulation(t *testing.T) {
	request := ports.ExecutionRequest{
		Run: domain.ExecutionRun{ID: "run"},
		Plan: domain.SchedulePlan{ID: "plan", Assignments: []domain.PlanAssignment{{
			ID: "assignment", ActivityID: "activity", ResourceID: "resource",
			PredictedRuntimeSeconds: 2,
		}}},
		Workflow: domain.WorkflowVersion{Activities: []domain.Activity{{
			ID: "activity", Simulation: &domain.ActivitySimulation{DurationSeconds: 10},
		}}},
		Resources: []domain.Resource{{ID: "resource", ComputeSpeedup: 5}},
	}

	payload, err := buildRunnerInput(request, 1e9)
	require.NoError(t, err)
	var input runnerInput
	require.NoError(t, json.Unmarshal(payload, &input))
	require.Len(t, input.Tasks, 1)
	require.InDelta(t, 10e9, input.Tasks[0].FLOPs, 1)
}

func TestRunnerInputUsesCleanRuntimeBeforeApplyingExecutionInterference(t *testing.T) {
	request := ports.ExecutionRequest{
		Run: domain.ExecutionRun{ID: "run"},
		Plan: domain.SchedulePlan{ID: "plan", Assignments: []domain.PlanAssignment{{
			ID: "assignment", ActivityID: "activity", ResourceID: "resource",
			PredictedRuntimeSeconds: 6,
			Metadata:                map[string]any{"executionBaseRuntimeSeconds": 2.0},
		}}},
		Workflow: domain.WorkflowVersion{Activities: []domain.Activity{{
			ID: "activity", Simulation: &domain.ActivitySimulation{DurationSeconds: 10},
		}}},
		Resources: []domain.Resource{{ID: "resource", ComputeSpeedup: 5}},
	}

	payload, err := buildRunnerInput(request, 1e9)
	require.NoError(t, err)
	var input runnerInput
	require.NoError(t, json.Unmarshal(payload, &input))
	require.Len(t, input.Tasks, 1)
	require.InDelta(t, 10e9, input.Tasks[0].FLOPs, 1)
}

func TestRunnerInputCarriesPlanInterferenceMatrix(t *testing.T) {
	request := ports.ExecutionRequest{
		Run: domain.ExecutionRun{ID: "run"},
		Plan: domain.SchedulePlan{
			ID: "plan",
			Assignments: []domain.PlanAssignment{{
				ID: "assignment", ActivityID: "activity", ResourceID: "resource",
			}},
			Metadata: map[string]any{"interferenceMatrix": map[string]any{
				"schemaVersion": "1", "model": "pairwise-cpu-priority",
				"aggregation": "minimum", "entries": []any{},
			}},
		},
		Workflow:  domain.WorkflowVersion{Activities: []domain.Activity{{ID: "activity"}}},
		Resources: []domain.Resource{{ID: "resource"}},
	}

	payload, err := buildRunnerInput(request, 1e9)
	require.NoError(t, err)
	var input runnerInput
	require.NoError(t, json.Unmarshal(payload, &input))
	require.NotNil(t, input.Interference)
	require.Equal(t, "pairwise-cpu-priority", input.Interference.Model)
}

func TestRunnerInputCarriesPairwiseSlowdownMatrix(t *testing.T) {
	request := ports.ExecutionRequest{
		Run: domain.ExecutionRun{ID: "run"},
		Plan: domain.SchedulePlan{
			ID: "plan",
			Assignments: []domain.PlanAssignment{
				{ID: "assignment-a", ActivityID: "a", ResourceID: "resource"},
				{ID: "assignment-b", ActivityID: "b", ResourceID: "resource"},
			},
			Metadata: map[string]any{"interferenceMatrix": map[string]any{
				"schemaVersion": "2", "model": "pairwise-slowdown",
				"aggregation": "maximum", "entries": []any{map[string]any{
					"affectedActivityId": "a", "interferingActivityId": "b", "slowdownFactor": 1.5,
				}},
			}},
		},
		Workflow:  domain.WorkflowVersion{Activities: []domain.Activity{{ID: "a"}, {ID: "b"}}},
		Resources: []domain.Resource{{ID: "resource"}},
	}

	payload, err := buildRunnerInput(request, 1e9)
	require.NoError(t, err)
	var input runnerInput
	require.NoError(t, json.Unmarshal(payload, &input))
	require.NotNil(t, input.Interference)
	require.Equal(t, "pairwise-slowdown", input.Interference.Model)
	require.InDelta(t, 1.5, input.Interference.Entries[0].SlowdownFactor, 1e-9)
}

func TestRunnerInputCarriesAdditivePairwiseSlowdownAggregation(t *testing.T) {
	request := ports.ExecutionRequest{
		Run: domain.ExecutionRun{ID: "run"},
		Plan: domain.SchedulePlan{
			ID: "plan",
			Assignments: []domain.PlanAssignment{
				{ID: "assignment-a", ActivityID: "a", ResourceID: "resource"},
				{ID: "assignment-b", ActivityID: "b", ResourceID: "resource"},
				{ID: "assignment-c", ActivityID: "c", ResourceID: "resource"},
			},
			Metadata: map[string]any{"interferenceMatrix": map[string]any{
				"schemaVersion": "4", "model": "pairwise-slowdown",
				"aggregation": "additive-excess", "entries": []any{},
			}},
		},
		Workflow:  domain.WorkflowVersion{Activities: []domain.Activity{{ID: "a"}, {ID: "b"}, {ID: "c"}}},
		Resources: []domain.Resource{{ID: "resource"}},
	}

	payload, err := buildRunnerInput(request, 1e9)
	require.NoError(t, err)
	var input runnerInput
	require.NoError(t, json.Unmarshal(payload, &input))
	require.NotNil(t, input.Interference)
	require.Equal(t, "additive-excess", input.Interference.Aggregation)
}

func TestRunnerInputCarriesActivityTypeForFamilySlowdown(t *testing.T) {
	request := ports.ExecutionRequest{
		Run: domain.ExecutionRun{ID: "run"},
		Plan: domain.SchedulePlan{
			ID: "plan",
			Assignments: []domain.PlanAssignment{{
				ID: "assignment", ActivityID: "activity", ResourceID: "resource",
			}},
		},
		Workflow: domain.WorkflowVersion{Activities: []domain.Activity{{
			ID: "activity", ActivityTypeID: "activity-family",
		}}},
		Resources: []domain.Resource{{ID: "resource"}},
	}

	payload, err := buildRunnerInput(request, 1e9)
	require.NoError(t, err)
	var input runnerInput
	require.NoError(t, json.Unmarshal(payload, &input))
	require.Equal(t, "activity-family", input.Tasks[0].ActivityTypeID)
}
