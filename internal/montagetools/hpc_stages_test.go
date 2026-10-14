package montagetools

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPositiveStageLineIsVolumeBasedNotMakespanMultiplier(t *testing.T) {
	a, b := positiveLine([]float64{0, 1e9, 2e9}, []float64{5, 15, 25})
	require.InDelta(t, 5, a, 1e-9)
	require.InDelta(t, 1e-8, b, 1e-15)
	a, b = positiveLine([]float64{0, 1e9, 2e9}, []float64{10, 4, 1})
	require.GreaterOrEqual(t, a, 0.0)
	require.GreaterOrEqual(t, b, 0.0)
}
