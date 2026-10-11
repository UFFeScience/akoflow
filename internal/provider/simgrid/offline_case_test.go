package simgrid

import (
	"math"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestOfflineCaseMatchesProductionContract(t *testing.T) {
	request := materializedRequestFixture()
	request.Run.Mode = domain.ExecutionModeSimulation
	platform, input, err := BuildOfflineCase(request, 1e9)
	require.NoError(t, err)
	wantPlatform, err := buildPlatformXML(request.Resources, request.NetworkTopology, 1e9)
	require.NoError(t, err)
	wantInput, err := buildRunnerInput(request, 1e9)
	require.NoError(t, err)
	require.Equal(t, wantPlatform, platform)
	require.JSONEq(t, string(wantInput), string(input))
}

func TestOfflineCaseRejectsRealAndInvalidReference(t *testing.T) {
	request := processRequestFixture()
	request.Run.Mode = domain.ExecutionModeReal
	_, _, err := BuildOfflineCase(request, 1e9)
	require.ErrorContains(t, err, "simulation mode")
	request.Run.Mode = domain.ExecutionModeSimulation
	for _, invalid := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		_, _, err = BuildOfflineCase(request, invalid)
		require.ErrorContains(t, err, "reference FLOPS")
	}
}
