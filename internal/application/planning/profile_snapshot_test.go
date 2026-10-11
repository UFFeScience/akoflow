package planning

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestIndependentProfileSnapshotNeedsWorkRatherThanLegacySimulationDuration(t *testing.T) {
	workflow := domain.WorkflowVersion{Activities: []domain.Activity{{ID: "a", ActivityTypeID: "family", Metadata: map[string]any{"computeWorkUnits": 6.0}}}}
	profile := domain.ActivityResourceProfile{ID: "measured", ActivityTypeID: "family", ResourceID: "hpc", Source: "input-hash", ModelVersion: "compute-work-v1", RuntimeSeconds: 50, SampleSize: 4}
	decoded, err := calibratedProfileSnapshot([]domain.ActivityResourceProfile{profile}, workflow, []domain.Resource{{ID: "hpc"}})
	require.NoError(t, err)
	require.Equal(t, 300.0, decoded[0].RuntimeFor(workflow.Activities[0]))
	delete(workflow.Activities[0].Metadata, "computeWorkUnits")
	_, err = calibratedProfileSnapshot([]domain.ActivityResourceProfile{profile}, workflow, []domain.Resource{{ID: "hpc"}})
	require.Error(t, err)
}

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
