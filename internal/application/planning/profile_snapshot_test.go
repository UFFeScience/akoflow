package planning

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestProfileSnapshotRejectsStaleResourcesDuplicatePairsAndLegacyUnits(t *testing.T) {
	workflow := domain.WorkflowVersion{Activities: []domain.Activity{{ID: "a", ActivityTypeID: "family", Simulation: &domain.ActivitySimulation{DurationSeconds: 10}}}}
	resources := []domain.Resource{{ID: "micro"}}
	profile := domain.ActivityResourceProfile{ID: "measured", ActivityTypeID: "family", ResourceID: "micro", Source: "baseline-hash", ModelVersion: "reference-work-v1", RuntimeSeconds: 2, SampleSize: 5}
	decoded, err := calibratedProfileSnapshot([]domain.ActivityResourceProfile{profile}, workflow, resources)
	require.NoError(t, err)
	require.Equal(t, float64(20), decoded[0].RuntimeFor(workflow.Activities[0]))
	_, err = calibratedProfileSnapshot([]domain.ActivityResourceProfile{profile, profile}, workflow, resources)
	require.Error(t, err)
	_, err = calibratedProfileSnapshot([]domain.ActivityResourceProfile{profile}, workflow, nil)
	require.Error(t, err)
	profile.ModelVersion = "legacy-absolute"
	_, err = calibratedProfileSnapshot([]domain.ActivityResourceProfile{profile}, workflow, resources)
	require.Error(t, err)
}
