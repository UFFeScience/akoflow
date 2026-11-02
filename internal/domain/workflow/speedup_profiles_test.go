package workflow

import (
	"github.com/UFFeScience/akoflow/internal/domain/resource"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSpeedupProfilesPreserveMeasuredAndScaleOnlyMissing(t *testing.T) {
	a := []Activity{{ActivityTypeID: "family"}, {ActivityTypeID: "family"}}
	r := []resource.Resource{{ID: "micro", ComputeSpeedup: .25, Schedulable: true}, {ID: "hpc", ComputeSpeedup: 1, Schedulable: true}, {ID: "relay", Schedulable: false}}
	p := []ActivityResourceProfile{{ID: "measured", ActivityTypeID: "family", ResourceID: "micro", RuntimeSeconds: 8, ModelVersion: "compute-work-v1", SampleSize: 3}}
	out, err := CompleteSpeedupProfiles(a, r, p)
	require.NoError(t, err)
	require.Len(t, out, 2)
	require.Equal(t, p[0], out[0])
	require.Equal(t, 2.0, out[1].RuntimeSeconds)
	require.Zero(t, out[1].SampleSize)
	require.Equal(t, true, out[1].Metadata["estimated"])
	require.Equal(t, 6.0, out[1].RuntimeFor(Activity{Metadata: map[string]any{"computeWorkUnits": 3.0}}))
	_, err = CompleteSpeedupProfiles(a, r, nil)
	require.Error(t, err)
}
