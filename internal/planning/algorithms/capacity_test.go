package algorithms

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	"testing"
)

func TestPlanCoresRespectExplicitSingleMicroLane(t *testing.T) {
	got := cores(domain.Resource{ID: "micro", CPUCores: 2, CPUCapacity: 1})
	if len(got) != 1 || got[0] != "micro-core-1" {
		t.Fatalf("lanes=%v", got)
	}
}
