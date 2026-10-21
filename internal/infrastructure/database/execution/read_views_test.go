package execution

import (
	"context"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestScopedActivityReadsKeepOutputOutOfMonitor(t *testing.T) {
	repo := setup(t)
	ctx := context.Background()
	require.NoError(t, repo.CreateRun(ctx, domain.ExecutionRun{ID: "run", SchedulePlanID: "plan", Mode: domain.ExecutionModeReal, Status: domain.ExecutionRunRunning}))
	_, err := repo.db.Exec(`INSERT INTO activity_definitions(id, workflow_version_id, activity_type_id, external_id, name, kind, capabilities, command_spec, resource_requirements, policy)
 VALUES ('second','workflow-version','type','second','second','task','{}','{}','{}','{}')`)
	require.NoError(t, err)
	for _, id := range []string{"activity", "second"} {
		require.NoError(t, repo.SaveTask(ctx, domain.TaskExecution{ID: id, ExecutionRunID: "run", PlanAssignmentID: "assignment", PlannedResourceID: "resource", ActivityID: id, Attempt: 1, Status: domain.TaskRunning}))
		require.NoError(t, repo.Save(ctx, domain.ActivityHandle{ID: id, RunID: "run", ResourceID: "resource", RuntimeID: "local", ActivityID: id, Status: domain.HandleRunning,
			Log: "large output", Artifacts: &domain.ArtifactManifest{SchemaVersion: 1}, Metadata: map[string]any{"pid": 42}}))
	}
	handles, err := repo.ListHandlesView(ctx, "run", "", false)
	require.NoError(t, err)
	require.Len(t, handles, 2)
	require.Empty(t, handles[0].Log)
	require.Nil(t, handles[0].Artifacts)
	require.Equal(t, float64(42), handles[0].Metadata["pid"])
	handles, err = repo.ListHandlesView(ctx, "run", "second", true)
	require.NoError(t, err)
	require.Len(t, handles, 1)
	require.Equal(t, "large output", handles[0].Log)
	require.NotNil(t, handles[0].Artifacts)
	logs, err := repo.ListProcessHandles(ctx, "run")
	require.NoError(t, err)
	require.Len(t, logs, 2)
	require.Equal(t, "large output", logs[0].Log)
	require.Nil(t, logs[0].Artifacts)
	tasks, err := repo.ListTasksForActivity(ctx, "run", "second")
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Equal(t, "second", tasks[0].ActivityID)
}

func TestEventPagesPreserveAuditOrderWithoutDuplicates(t *testing.T) {
	repo := setup(t)
	ctx := context.Background()
	require.NoError(t, repo.CreateRun(ctx, domain.ExecutionRun{ID: "run", SchedulePlanID: "plan", Mode: domain.ExecutionModeReal, Status: domain.ExecutionRunRunning}))
	require.NoError(t, repo.SaveTask(ctx, domain.TaskExecution{ID: "task", ExecutionRunID: "run", PlanAssignmentID: "assignment", PlannedResourceID: "resource", ActivityID: "activity", Attempt: 1, Status: domain.TaskRunning}))
	all, err := repo.ListEvents(ctx, "run")
	require.NoError(t, err)
	require.Len(t, all, 2)
	first, more, err := repo.ListEventsPage(ctx, "run", 1, 1)
	require.NoError(t, err)
	require.True(t, more)
	require.Equal(t, all[:1], first)
	second, more, err := repo.ListEventsPage(ctx, "run", 2, 1)
	require.NoError(t, err)
	require.False(t, more)
	require.Equal(t, all[1:], second)
}
