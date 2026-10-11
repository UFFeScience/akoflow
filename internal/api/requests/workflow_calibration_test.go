package requests

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestWorkflowCalibrationFamiliesAndProjectionSurvivePortableRoundTrip(t *testing.T) {
	request := Workflow{Name: "Measured Montage", Spec: WorkflowSpec{Namespace: "experiment", Activities: []WorkflowActivity{
		{Name: "project", ActivityType: "mproject", Simulation: &domain.ActivitySimulation{DurationSeconds: 100}, Metadata: map[string]any{"workspaceProjection": map[string]any{"contract": "whole-workspace-v1"}, "workspaceProjectionVolumes": "stale", "workspaceSeedPath": "untrusted"}, WorkspaceSeedPath: "verified"},
		{Name: "diff", ActivityType: "mdifffit", Simulation: &domain.ActivitySimulation{DurationSeconds: 10}, DependsOn: []string{"project"}},
	}}}
	definition, err := request.Domain()
	require.NoError(t, err)
	require.Equal(t, "measured-montage-mproject", definition.Version.Activities[0].ActivityTypeID)
	require.Equal(t, "measured-montage-mdifffit", definition.Version.Activities[1].ActivityTypeID)
	require.Contains(t, definition.Version.Activities[0].Metadata, "workspaceProjection")
	require.NotContains(t, definition.Version.Activities[0].Metadata, "workspaceProjectionVolumes")
	require.Equal(t, "verified", definition.Version.Activities[0].Metadata["workspaceSeedPath"])
	reimported, err := FromDomain(definition).Domain()
	require.NoError(t, err)
	require.Equal(t, definition.Version.Activities[0].ActivityTypeID, reimported.Version.Activities[0].ActivityTypeID)
	require.Equal(t, definition.Version.Activities[0].Metadata["workspaceProjection"], reimported.Version.Activities[0].Metadata["workspaceProjection"])
	require.Len(t, reimported.Version.Dependencies, 1)
}
