package simgrid

import (
	"context"
	"github.com/UFFeScience/akoflow/internal/domain/materializationmodel"
	"strings"
	"testing"
)

func TestPreparationLaneRequiresUpdatedRunner(t *testing.T) {
	r := materializedRequestFixture()
	r.Plan.Assignments[1].ResourceID = r.Plan.Assignments[0].ResourceID
	r.Plan.Assignments[1].CoreID = r.Plan.Assignments[0].CoreID
	r.Plan.Assignments[1].OrderOnResource = 1
	m, err := materializationmodel.Decode(r.NetworkTopology.Metadata)
	if err != nil {
		t.Fatal(err)
	}
	fake := &runnerCommandFake{capabilities: []byte(`{"workspaceMaterializationModelVersion":1}`)}
	e, err := NewProcessExecutor(fake, ProcessConfig{BinaryPath: "old", Workspace: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	err = e.checkPreparationCapabilities(context.Background(), r, m)
	if err == nil || !strings.Contains(err.Error(), "preparation lane barrier") {
		t.Fatalf("old runner silently accepted: %v", err)
	}
	fake.capabilities = []byte(`{"workspaceMaterializationModelVersion":1,"preparationLaneBarrierVersion":1}`)
	if err = e.checkPreparationCapabilities(context.Background(), r, m); err != nil {
		t.Fatal(err)
	}
}
