package simgrid

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	"strings"
	"testing"
)

func TestPlatformRespectsExplicitSingleMicroLane(t *testing.T) {
	xml, err := buildPlatformXML([]domain.Resource{{ID: "micro", CPUCores: 2, CPUCapacity: 1, ComputeSpeedup: 0.25}}, domain.NetworkTopology{}, 1e9)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(xml), `core="1"`) || strings.Contains(string(xml), `core="2"`) {
		t.Fatalf("platform=%s", xml)
	}
}
