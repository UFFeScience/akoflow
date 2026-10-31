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
