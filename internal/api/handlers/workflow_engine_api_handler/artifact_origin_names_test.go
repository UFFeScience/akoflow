package workflow_engine_api_handler

import (
	"context"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/api/requests"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestPortableArtifactNamesSurviveExportAndImport(t *testing.T) {
	defaultImage := "registry.example/workflow@sha256:" + strings.Repeat("a", 64)
	overrideImage := "registry.example/stage@sha256:" + strings.Repeat("b", 64)
	command := domain.ActivityCommand{Entrypoint: "true"}
	authored := requests.Workflow{
		Name: "Reference workflow",
		Spec: requests.WorkflowSpec{
			Namespace: "science", Image: defaultImage,
			Activities: []requests.WorkflowActivity{
				{Name: "Prepare", Command: command},
				{Name: "Analyze", Command: command, Image: overrideImage},
			},
		},
	}
	definition, err := authored.Domain()
	require.NoError(t, err)
	portable := requests.PortableWorkflow(definition)
	require.Equal(t, defaultImage, portable.Spec.Image)
	require.Empty(t, requests.FromDomain(definition).Spec.Image)
	imported, err := portable.Domain()
	require.NoError(t, err)
	handler := Handler{workflows: &workflowRepositoryStub{definition: &imported}}
	assignment := domain.PlanAssignment{ResourceID: "resource"}
	resource := domain.Resource{ID: "resource", Architecture: "amd64"}
	request := ports.ExecutionRequest{
		Workflow: imported.Version,
		Runtimes: []domain.EnvironmentRuntime{{
			ID: "local", Driver: domain.RuntimeDriverLocal, Mode: domain.RuntimeModeExecution,
		}},
		RuntimeBindings: []domain.ResourceRuntimeBinding{{
			RuntimeID: "local", ResourceID: resource.ID, Enabled: true,
		}},
	}
	for i, want := range []struct{ label, image string }{
		{"Reference workflow", defaultImage},
		{"Analyze", overrideImage},
	} {
		prepared, err := handler.preparePortableOrigin(context.Background(), &request, i, assignment, resource)
		require.NoError(t, err)
		require.True(t, prepared)
		activity := request.Workflow.Activities[i]
		require.Nil(t, activity.Command.Executable.Source.Origin)
		require.Equal(t, want.label, request.PreparationRequirementsByActivity[activity.ID].ReproductionName)
		require.Equal(t, want.image, request.PreparationRequirementsByActivity[activity.ID].Reproduction.Image)
	}
}
