package algorithms

import (
	"math"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/domain/materializationmodel"
	domainworkflow "github.com/UFFeScience/akoflow/internal/domain/workflow"
)

// Same causal fixture as the native mixed-origin test: roots finish at 1/5,
// peer ends at 6, relay legs at 7/10, merge ends at 12, consumer at 13.
func TestPRISMMaterializationMatchesNativeMixedOriginBarrier(t *testing.T) {
	model := materializationmodel.Model{SchemaVersion: 1, Source: "measured-fixture", RelayResourceID: "relay",
		Resources: map[string]materializationmodel.Resource{
			"peer": {Class: "micro", Domain: "cloud"}, "target": {Class: "micro", Domain: "cloud"},
			"hpc": {Class: "hpc", Domain: "hpc"}, "relay": {Class: "relay", Domain: "relay"},
		},
		Profiles: []materializationmodel.Profile{
			{Strategy: "direct-runtime", SourceClass: "micro", TargetClass: "micro", Source: "fixture", Samples: 1, ArchiveToWorkspaceRatio: 1, UploadArchiveBytesPerSecond: 100},
			{Strategy: "gateway", SourceClass: "hpc", TargetClass: "micro", Source: "fixture", Samples: 1, ArchiveToWorkspaceRatio: 1, IngressArchiveBytesPerSecond: 100, UploadArchiveBytesPerSecond: 100, IngressIncludesSourcePreparation: true, ValidationSecondsPerByte: .01, ReceiverSecondsPerByte: .02},
		}, MergeProfiles: []materializationmodel.MergeProfile{{TargetClass: "micro", Source: "fixture", Samples: 1, CheckSecondsPerInputByte: .01}},
	}
	activities := []domain.Activity{planningActivity("a", 1), planningActivity("c", 5), planningActivity("b", 1)}
	for index := range activities {
		projection := domainworkflow.WorkspaceProjection{Contract: "whole-workspace-v1", Source: "fixture"}
		if index < 2 {
			projection.Outputs = []domainworkflow.ProjectedFile{{Path: activities[index].ID, SizeBytes: 100, ContentID: activities[index].ID}}
		}
		activities[index].Metadata = map[string]any{"workspaceProjection": projection}
	}
	request := domain.PlanningRequest{
		Workflow:        domain.WorkflowVersion{Activities: activities, Dependencies: []domain.ActivityDependency{{ActivityID: "b", DependsOnActivityID: "a"}, {ActivityID: "b", DependsOnActivityID: "c"}}},
		ExecutionScope:  domain.ExecutionScope{EnvironmentVersionIDs: []string{"environment"}},
		Resources:       []domain.Resource{planningResource("peer", 1, 1, 0), planningResource("hpc", 1, 1, 0), planningResource("target", 1, 1, 0), planningResource("relay", 1, 1, 0)},
		NetworkTopology: domain.NetworkTopology{Metadata: map[string]any{"workspaceMaterializationModel": model}},
	}
	for index, pair := range [][2]string{{"peer", "target"}, {"hpc", "relay"}, {"relay", "target"}} {
		request.NetworkTopology.Links = append(request.NetworkTopology.Links, domain.NetworkLink{ID: string(rune('a' + index)), SourceResourceID: pair[0], TargetResourceID: pair[1], BandwidthBitsPerSecond: 8e9, SharingPolicy: "SHARED", PricePerByte: .001})
	}
	search, err := newCompactPRISMContext(request, nil)
	if err != nil {
		t.Fatal(err)
	}
	assignments := []domain.PlanAssignment{{ActivityID: "a", ResourceID: "peer", CoreID: "peer-core-1"}, {ActivityID: "c", ResourceID: "hpc", CoreID: "hpc-core-1"}, {ActivityID: "b", ResourceID: "target", CoreID: "target-core-1"}}
	evaluated, err := evaluateCompleteCompactPRISMState(search, compactPRISMStateFromEvaluation(compactPRISMState{}, assignments, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(evaluated.makespan-13) > 1e-8 {
		t.Fatalf("native parity expected 13 seconds, got %g", evaluated.makespan)
	}
	actual := compactPRISMAssignments(search, evaluated)
	for _, assignment := range actual {
		if assignment.ActivityID == "b" && (math.Abs(assignment.PredictedStartAt-12) > 1e-8 || assignment.Metadata["workspacePreparationSeconds"] != float64(2)) {
			t.Fatalf("incorrect consumer barrier: %+v", assignment)
		}
	}
	model.PublicationProfiles = []materializationmodel.PublicationProfile{{TargetClass: "micro", Source: "queue-free-fixture", Samples: 1, ScanSecondsPerWorkspaceByte: .01}}
	request.NetworkTopology.Metadata["workspaceMaterializationModel"] = model
	publicationSearch, err := newCompactPRISMContext(request, nil)
	if err != nil {
		t.Fatal(err)
	}
	published, err := evaluateCompleteCompactPRISMState(publicationSearch, compactPRISMStateFromEvaluation(compactPRISMState{}, assignments, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(published.makespan-15) > 1e-8 {
		t.Fatalf("publication barrier expected 15, got %g", published.makespan)
	}
	model.PublicationProfiles = nil
	model.InputObservationProfiles = []materializationmodel.InputObservationProfile{{TargetClass: "micro", Source: "queue-free-fixture", Samples: 1, SecondsPerUniqueByte: .1}}
	request.NetworkTopology.Metadata["workspaceMaterializationModel"] = model
	observedSearch, err := newCompactPRISMContext(request, nil)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := evaluateCompleteCompactPRISMState(observedSearch, compactPRISMStateFromEvaluation(compactPRISMState{}, assignments, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(observed.makespan-33) > 1e-8 {
		t.Fatalf("input union observation must add 20 seconds once, got %g", observed.makespan)
	}
	model.InputObservationProfiles = nil
	// Independently measured 2s source preparation is charged before ingress,
	// not as extra network bytes or a second observed queue.
	model.Profiles[1].IngressIncludesSourcePreparation = false
	model.Profiles[1].SeparatedIngressServiceSource = "queue-free-fixture"
	model.Profiles[1].SnapshotSecondsPerByte = .02
	request.NetworkTopology.Metadata["workspaceMaterializationModel"] = model
	separated, err := newCompactPRISMContext(request, nil)
	if err != nil {
		t.Fatal(err)
	}
	withPreparation, err := evaluateCompleteCompactPRISMState(separated, compactPRISMStateFromEvaluation(compactPRISMState{}, assignments, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(withPreparation.makespan-15) > 1e-8 {
		t.Fatalf("separated native parity expected 15, got %g", withPreparation.makespan)
	}
	model.Profiles[1].IngressIncludesSourcePreparation = true
	model.Profiles[1].SeparatedIngressServiceSource = ""
	model.Profiles[1].SnapshotSecondsPerByte = 0
	// Same native fixture with a single shared transport slot: peer (1s),
	// relay ingress (2s), egress (3s), then one merge (2s). Roots end at 5.
	for id, resource := range model.Resources {
		if resource.Domain == "relay" {
			continue
		}
		resource.BulkServiceGroupID = "control-transport"
		resource.BulkServiceSlots = 1
		resource.BulkServiceSource = "fixture-provider-config"
		model.Resources[id] = resource
	}
	request.NetworkTopology.Metadata["workspaceMaterializationModel"] = model
	search, err = newCompactPRISMContext(request, nil)
	if err != nil {
		t.Fatal(err)
	}
	evaluated, err = evaluateCompleteCompactPRISMState(search, compactPRISMStateFromEvaluation(compactPRISMState{}, assignments, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(evaluated.makespan-14) > 1e-8 {
		t.Fatalf("shared service pool native parity expected 14 seconds, got %g", evaluated.makespan)
	}
}

func TestMaterializationRateLimitAndReceiverAreSeparateEvents(t *testing.T) {
	flow := prismEvaluationFlow{active: true, remaining: 100, rateLimit: 25}
	rates, at := prismEvaluationFlowRates([]prismEvaluationFlow{flow}, 0)
	if rates[0] != 25 || at != 4 {
		t.Fatalf("effective cap ignored: %v %g", rates, at)
	}
	flow.postStage, flow.payloadAt, flow.remaining = true, 7, 0
	rates, at = prismEvaluationFlowRates([]prismEvaluationFlow{flow}, 4)
	if rates[0] != 0 || at != 7 {
		t.Fatalf("receiver work consumed bandwidth: %v %g", rates, at)
	}
}

func TestPRISMSixSourceFanInUsesTwoBulkServiceWaves(t *testing.T) {
	model := materializationmodel.Model{SchemaVersion: 1, Source: "service-fixture", RelayResourceID: "relay",
		Resources: map[string]materializationmodel.Resource{
			"hpc":    {Class: "hpc", Domain: "hpc", BulkServiceGroupID: "hpc-session", BulkServiceSlots: 3, BulkServiceSource: "fixture"},
			"target": {Class: "micro", Domain: "cloud", BulkServiceGroupID: "cloud-session", BulkServiceSlots: 3, BulkServiceSource: "fixture"},
			"relay":  {Class: "relay", Domain: "relay"},
		},
		Profiles: []materializationmodel.Profile{{Strategy: "gateway", SourceClass: "hpc", TargetClass: "micro", Source: "fixture", Samples: 1,
			ArchiveToWorkspaceRatio: 1, IngressArchiveBytesPerSecond: 10, UploadArchiveBytesPerSecond: 100, IngressIncludesSourcePreparation: true}},
		MergeProfiles: []materializationmodel.MergeProfile{{TargetClass: "micro", Source: "fixture", Samples: 1, CheckSecondsPerInputByte: 1.0 / 300}},
	}
	request := domain.PlanningRequest{ExecutionScope: domain.ExecutionScope{EnvironmentVersionIDs: []string{"environment"}},
		Resources: []domain.Resource{planningResource("hpc", 1, 1, 0), planningResource("target", 1, 1, 0), planningResource("relay", 1, 1, 0)},
		NetworkTopology: domain.NetworkTopology{Metadata: map[string]any{"workspaceMaterializationModel": model}, Links: []domain.NetworkLink{
			{ID: "ingress", SourceResourceID: "hpc", TargetResourceID: "relay", BandwidthBitsPerSecond: 8e9, SharingPolicy: "SHARED"},
			{ID: "egress", SourceResourceID: "relay", TargetResourceID: "target", BandwidthBitsPerSecond: 8e9, SharingPolicy: "SHARED"},
		}},
	}
	assignments := []domain.PlanAssignment{}
	for i := 0; i < 6; i++ {
		id := string(rune('a' + i))
		activity := planningActivity(id, 1)
		activity.Metadata = map[string]any{"workspaceProjection": domainworkflow.WorkspaceProjection{Contract: "whole-workspace-v1", Source: "fixture",
			Outputs: []domainworkflow.ProjectedFile{{Path: id, SizeBytes: 100, ContentID: id}}}}
		request.Workflow.Activities = append(request.Workflow.Activities, activity)
		request.Workflow.Dependencies = append(request.Workflow.Dependencies, domain.ActivityDependency{ActivityID: "consumer", DependsOnActivityID: id})
		assignments = append(assignments, domain.PlanAssignment{ActivityID: id, ResourceID: "hpc", CoreID: "hpc-core-1"})
	}
	consumer := planningActivity("consumer", 1)
	consumer.Metadata = map[string]any{"workspaceProjection": domainworkflow.WorkspaceProjection{Contract: "whole-workspace-v1", Source: "fixture"}}
	request.Workflow.Activities = append(request.Workflow.Activities, consumer)
	assignments = append(assignments, domain.PlanAssignment{ActivityID: "consumer", ResourceID: "target", CoreID: "target-core-1"})
	search, err := newCompactPRISMContext(request, nil)
	if err != nil {
		t.Fatal(err)
	}
	evaluated, err := evaluateCompleteCompactPRISMState(search, compactPRISMStateFromEvaluation(compactPRISMState{}, assignments, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(evaluated.makespan-30) > 1e-8 {
		t.Fatalf("expected six serial roots plus two waves, merge and compute: 30s; got %g", evaluated.makespan)
	}
}
