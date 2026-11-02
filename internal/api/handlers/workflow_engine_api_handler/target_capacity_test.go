package workflow_engine_api_handler

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCloudTargetPreservesExplicitSchedulingCapacity(t *testing.T) {
	target := domain.CloudCapacityTarget{VCPU: 2}
	cap, e := targetSchedulingCapacity(target)
	require.NoError(t, e)
	require.Equal(t, 2., cap)
	target.Configuration = map[string]any{"schedulingCpuCapacity": 1.}
	cap, e = targetSchedulingCapacity(target)
	require.NoError(t, e)
	require.Equal(t, 1., cap)
	require.Equal(t, 2, target.VCPU)
	for _, v := range []any{0., -1., 3., "bad", nil} {
		target.Configuration["schedulingCpuCapacity"] = v
		_, e = targetSchedulingCapacity(target)
		require.Error(t, e)
	}
}

func TestTargetRegistrationPreservesCalibrationOnlyForSameShape(t *testing.T) {
	old := domain.Resource{ID: "r",
		EnvironmentVersionID: "env",
		CPUCores:             2,
		MemoryBytes:          4096,
		ComputeSpeedup:       .25,
		BootOverheadSeconds:  90,
		ContainerOverhead:    5.48,
		Metadata: map[string]any{"providerMachineType": "e2-medium",
			"calibrationSource": "observed",
			"priceSource":       "old"}}
	target := domain.Resource{ID: "r", EnvironmentVersionID: "env", CPUCores: 2, MemoryBytes: 4096, ComputeSpeedup: 1, Metadata: map[string]any{"providerMachineType": "e2-medium", "priceSource": "current"}}
	preserveTargetCalibration(&target, []domain.Resource{old})
	require.Equal(t, 5.48, target.ContainerOverhead)
	require.Equal(t, 90., target.BootOverheadSeconds)
	require.Equal(t, .25, target.ComputeSpeedup)
	require.Equal(t, "observed", target.Metadata["calibrationSource"])
	require.Equal(t, "current", target.Metadata["priceSource"])
	target.MemoryBytes = 8192
	target.ContainerOverhead = 0
	preserveTargetCalibration(&target, []domain.Resource{old})
	require.Zero(t, target.ContainerOverhead)
}
