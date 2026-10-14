package simgrid

import (
	"fmt"
	"math"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

// BuildOfflineCase uses the exact production contract without submitting a run,
// contacting infrastructure, or reading the execution database. Assignments are
// retained, while transfer stages use the shared materialization model.
func BuildOfflineCase(request ports.ExecutionRequest, referenceFLOPS float64) (platform, input []byte, err error) {
	if request.Run.Mode != domain.ExecutionModeSimulation {
		return nil, nil, fmt.Errorf("offline case requires simulation mode")
	}
	if referenceFLOPS <= 0 || math.IsNaN(referenceFLOPS) || math.IsInf(referenceFLOPS, 0) {
		return nil, nil, fmt.Errorf("positive finite reference FLOPS required")
	}
	platform, err = buildPlatformXML(request.Resources, request.NetworkTopology, referenceFLOPS)
	if err != nil {
		return nil, nil, err
	}
	input, err = buildRunnerInput(request, referenceFLOPS)
	return platform, input, err
}
