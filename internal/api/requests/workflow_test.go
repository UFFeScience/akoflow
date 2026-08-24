package requests

import (
	"encoding/json"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestStructuredExecutableWorkflowConvertsToDomain(t *testing.T) {
	definition, err := (Workflow{Name: "portable-flow", Spec: WorkflowSpec{
		Namespace: "science",
		Activities: []WorkflowActivity{{
			Name: "compute",
			Command: domain.ActivityCommand{
				Entrypoint: "sh",
				Arguments:  []string{"-c", "echo ready"},
				Executable: &domain.ExecutableReference{
					Source: domain.ExecutableSource{
						Type:           domain.ExecutableSourceRemoteFile,
						Path:           "/apps/tool.sif",
						EnvironmentRef: "plafrim",
					},
					Delivery: domain.ExecutableDelivery{Strategy: domain.DeliveryUseInPlace},
				},
			},
		}},
	}}).Domain()
	require.NoError(t, err)
	activity := definition.Version.Activities[0]
	require.Equal(t, "/apps/tool.sif", activity.Command.Executable.Source.Path)
	require.Equal(t, domain.DeliveryUseInPlace, activity.Command.Executable.Delivery.Strategy)
}

func TestWorkflowExportYAMLRoundTripsThroughImportContract(t *testing.T) {
	original, err := (Workflow{Name: "portable-flow", Spec: WorkflowSpec{
		Namespace: "science",
		Activities: []WorkflowActivity{
			{Name: "prepare", CPULimit: "250m", MemoryLimit: "32Mi", Command: domain.ActivityCommand{
				Entrypoint: "sh", Arguments: []string{"-c", "prepare"},
				Executable: &domain.ExecutableReference{Source: domain.ExecutableSource{Type: domain.ExecutableSourceOCI, Reference: "docker.io/library/alpine:3.20"}, Delivery: domain.ExecutableDelivery{Strategy: domain.DeliveryManaged}},
			}},
			{Name: "compute", DependsOn: []string{"prepare"}, Command: domain.ActivityCommand{
				Entrypoint: "sh", Arguments: []string{"-c", "compute"},
				Executable: &domain.ExecutableReference{Source: domain.ExecutableSource{Type: domain.ExecutableSourceRemoteFile, Path: "/apps/tool.sif", EnvironmentRef: "hpc"}, Delivery: domain.ExecutableDelivery{Strategy: domain.DeliveryUseInPlace}},
			}},
		},
	}}).Domain()
	require.NoError(t, err)
	payload, err := FromDomain(original).YAML()
	require.NoError(t, err)
	var document any
	require.NoError(t, yaml.Unmarshal(payload, &document))
	normalized, err := json.Marshal(document)
	require.NoError(t, err)
	var imported Workflow
	require.NoError(t, json.Unmarshal(normalized, &imported))
	rebuilt, err := imported.Domain()
	require.NoError(t, err)
	require.Equal(t, original.Name, rebuilt.Name)
	require.Equal(t, original.Namespace, rebuilt.Namespace)
	require.Len(t, rebuilt.Version.Activities, 2)
	require.Equal(t, domain.DeliveryManaged, rebuilt.Version.Activities[0].Command.Executable.Delivery.Strategy)
	require.Equal(t, "/apps/tool.sif", rebuilt.Version.Activities[1].Command.Executable.Source.Path)
	require.Equal(t, "prepare", rebuilt.Version.Dependencies[0].DependsOnActivityID)
}

func TestLegacyShapedWorkflowConvertsToNormalizedDomain(t *testing.T) {
	definition, err := (Workflow{Name: "science-flow", Spec: WorkflowSpec{
		Namespace: "science", Image: "busybox:1.36",
		Activities: []WorkflowActivity{
			{Name: "prepare", Run: "prepare", CPULimit: "100m", MemoryLimit: "16Mi"},
			{Name: "process", Run: "process", DependsOn: []string{"prepare"}},
		},
	}}).Domain()
	require.NoError(t, err)
	require.Equal(t, "science-flow-v1", definition.Version.ID)
	require.Equal(t, 0.1, definition.Version.Activities[0].Resources.CPU)
	require.Equal(t, int64(16*1024*1024), definition.Version.Activities[0].Resources.MemoryBytes)
	require.Equal(t, "process", definition.Version.Dependencies[0].ActivityID)
	require.Equal(t, "prepare", definition.Version.Dependencies[0].DependsOnActivityID)
}

func TestWorkflowRejectsUnknownDependency(t *testing.T) {
	_, err := (Workflow{Name: "flow", Spec: WorkflowSpec{
		Namespace: "science", Image: "busybox",
		Activities: []WorkflowActivity{{Name: "process", Run: "run", DependsOn: []string{"missing"}}},
	}}).Domain()
	require.ErrorContains(t, err, "unknown activity")
}
