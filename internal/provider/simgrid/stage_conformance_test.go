package simgrid

import (
	"context"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/domain/materializationmodel"
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

func admissionRequest() Request {
	r := materializedRequestFixture()
	m := r.NetworkTopology.Metadata["workspaceMaterializationModel"].(materializationmodel.Model)
	resource := m.Resources["cloud"]
	resource.ExecutionRuntimeID = "cloud-native"
	m.Resources["cloud"] = resource
	m.AdmissionProfiles = []materializationmodel.AdmissionProfile{{TargetClass: "micro", RuntimeID: "cloud-native", Source: "fixture", Samples: 1, FixedSeconds: 7}}
	m.SchemaVersion = 2
	r.NetworkTopology.Metadata["workspaceMaterializationModel"] = m
	return r
}

func TestAdmissionRequiresNativeCapabilityBeforeExecution(t *testing.T) {
	r := admissionRequest()
	fake := &runnerCommandFake{capabilities: []byte(`{"workspaceMaterializationModelVersion":1,"workspaceMaterializationModelMaxVersion":2,"bulkServicePoolModelVersion":1}`)}
	executor, err := NewProcessExecutor(fake, ProcessConfig{BinaryPath: "old-runner", Workspace: t.TempDir()})
	require.NoError(t, err)
	_, err = executor.Execute(context.Background(), r)
	require.ErrorContains(t, err, "runtime admission model")
	require.Equal(t, []string{"--capabilities"}, fake.args)
}

func TestAdmissionTraceRejectsIgnoredMissingAndDuplicateStages(t *testing.T) {
	r := admissionRequest()
	m, err := materializationmodel.Decode(r.NetworkTopology.Metadata)
	require.NoError(t, err)
	for _, v := range []any{nil, 0.0, math.NaN(), math.Inf(1)} {
		require.Error(t, validateAdmissionTrace(r, m, domain.ExecutionTrace{Tasks: []domain.TaskExecution{{ActivityID: "b", Metadata: map[string]any{"externalAdmissionSeconds": v}}}}))
	}
	valid := domain.TaskExecution{ActivityID: "b", Metadata: map[string]any{"externalAdmissionSeconds": 7.0}}
	require.NoError(t, validateAdmissionTrace(r, m, domain.ExecutionTrace{Tasks: []domain.TaskExecution{valid}}))
	require.Error(t, validateAdmissionTrace(r, m, domain.ExecutionTrace{}))
	require.Error(t, validateAdmissionTrace(r, m, domain.ExecutionTrace{Tasks: []domain.TaskExecution{valid, valid}}))
	require.NoError(t, validateAdmissionTrace(r, nil, domain.ExecutionTrace{}))
}
