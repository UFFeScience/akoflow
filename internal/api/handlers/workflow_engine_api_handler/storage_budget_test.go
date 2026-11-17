package workflow_engine_api_handler

import (
	"context"
	"encoding/json"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExecutionPreparationRejectsExhaustedQuota(t *testing.T) {
	h := &Handler{}
	var request ports.ExecutionRequest
	require.NoError(t, json.Unmarshal([]byte(`{
		"run":{"id":"run","mode":"real"},
 "runtimes":[{"id":"runtime","mode":"execution","driver":"local"}],
 "runtimeBindings":[{"resourceId":"node","runtimeId":"runtime","enabled":true}],
 "workflow":{"id":"workflow","activities":[{"id":"a","metadata":{"workspaceProjection":{"contract":"whole-workspace-v1","source":"measured","inputs":[{"path":"input.fits","contentId":"hash","sizeBytes":100}]}}}]},
		"resources":[{"id":"node","metadata":{"workspaceStorageBudget":{"storageId":"home-quota","availableBytes":0,"observedAt":"2026-10-11T00:00:00Z","source":"quota-api"}}}],
		"plan":{"id":"plan","assignments":[{"activityId":"a","resourceId":"node"}]}
	}`), &request))
	require.ErrorContains(t, h.PrepareExecution(context.Background(), &request), "quota")
}
