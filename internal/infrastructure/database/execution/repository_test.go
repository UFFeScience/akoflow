package execution

import (
	"context"
	"database/sql"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	database "github.com/UFFeScience/akoflow/internal/infrastructure/database"
	_ "github.com/mattn/go-sqlite3"
)

func setup(t *testing.T) *Repository {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if err := database.Bootstrap(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	repository := &Repository{db: db}
	seedExecutionParents(t, repository)
	return repository
}

func seedExecutionParents(t *testing.T, repository *Repository) {
	t.Helper()
	statements := []string{
		`INSERT INTO environments(id, name) VALUES ('environment', 'test')`,
		`INSERT INTO environment_versions(id, environment_id, version, status, network_model, interference_model, cost_model, configuration_hash) VALUES ('env', 'environment', 1, 'published', '{}', '{}', '{}', 'hash')`,
		`INSERT INTO workflow_definitions(id, external_id, name) VALUES ('workflow', 'workflow', 'test')`,
		`INSERT INTO workflow_versions(id, workflow_id, version, definition_hash) VALUES ('workflow-version', 'workflow', 1, 'hash')`,
		`INSERT INTO activity_types(id, name) VALUES ('type', 'task')`,
		`INSERT INTO activity_definitions(
			id, workflow_version_id, activity_type_id, external_id, name, kind,
			capabilities, command_spec, resource_requirements, policy
		) VALUES ('activity', 'workflow-version', 'type', 'activity', 'activity', 'task', '{}', '{}', '{}', '{}')`,
		`INSERT INTO resources(id, environment_version_id, type, name, provider_id) VALUES ('resource', 'env', 'local_machine', 'local', 'resource')`,
		`INSERT INTO execution_scopes(id, name) VALUES ('scope', 'test')`,
		`INSERT INTO execution_scope_environments(execution_scope_id, environment_version_id) VALUES ('scope', 'env')`,
		`INSERT INTO schedule_plans(id, workflow_version_id, execution_scope_id, source, algorithm) VALUES ('plan', 'workflow-version', 'scope', 'plugin', 'test')`,
		`INSERT INTO schedule_plan_assignments(id, schedule_plan_id, activity_id, resource_id, order_on_resource) VALUES ('assignment', 'plan', 'activity', 'resource', 1)`,
	}
	for _, statement := range statements {
		if _, err := repository.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRepositoryOwnsCompleteExecutionAggregate(t *testing.T) {
	repository := setup(t)
	ctx := context.Background()
	run := domain.ExecutionRun{ID: "run", SchedulePlanID: "plan", Mode: domain.ExecutionModeReal, Status: domain.ExecutionRunRunning}
	if err := repository.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	got, err := repository.FindRun(ctx, "run")
	if err != nil || got == nil || got.Mode != domain.ExecutionModeReal {
		t.Fatalf("run=%+v err=%v", got, err)
	}
	handle := domain.ActivityHandle{
		ID: "handle", RunID: "run", ActivityID: "activity", ResourceID: "resource",
		RuntimeID: "local", Status: domain.HandleRunning,
		Artifacts: &domain.ArtifactManifest{SchemaVersion: 1, Files: []domain.ArtifactObservation{{
			Path: "result.csv", Change: domain.ArtifactCreated, SizeBytes: 42,
		}}},
		Metadata: map[string]any{"pid": 1},
	}
	if err := repository.Save(ctx, handle); err != nil {
		t.Fatal(err)
	}
	if stored, err := repository.Find(ctx, "handle"); err != nil || stored == nil ||
		stored.Metadata["pid"].(float64) != 1 || stored.Artifacts.Files[0].SizeBytes != 42 {
		t.Fatalf("handle=%+v err=%v", stored, err)
	}
	if handles, err := repository.ListHandles(ctx, "run"); err != nil ||
		len(handles) != 1 || handles[0].Artifacts.Files[0].Path != "result.csv" {
		t.Fatalf("handles=%+v err=%v", handles, err)
	}
	task := domain.TaskExecution{ID: "task", ExecutionRunID: "run", PlanAssignmentID: "assignment", ActivityID: "activity", PlannedResourceID: "resource", Attempt: 1, Status: domain.TaskRunning}
	if err := repository.SaveTask(ctx, task); err != nil {
		t.Fatal(err)
	}
	task.Status, task.FinishedAt = domain.TaskCompleted, 4
	if err := repository.SaveTask(ctx, task); err != nil {
		t.Fatal(err)
	}
	trace := domain.ExecutionTrace{
		RunID:    "run",
		Executed: domain.ExecutionMetrics{MakespanSeconds: 4, Cost: 2},
		Transfers: []domain.DataTransfer{{
			ID: "transfer", ExecutionRunID: "run", ProducerActivityID: "activity",
			ConsumerActivityID: "activity", SourceResourceID: "resource",
			TargetResourceID: "resource", Bytes: 1024, StartedAt: 1,
			FinishedAt: 2, DurationSeconds: 1, Cost: 0.25,
		}},
	}
	if err := repository.CompleteRun(ctx, trace); err != nil {
		t.Fatal(err)
	}
	transfers, err := repository.ListTransfers(ctx, "run")
	if err != nil || len(transfers) != 1 || transfers[0].Bytes != 1024 || transfers[0].DurationSeconds != 1 {
		t.Fatalf("transfers=%+v err=%v", transfers, err)
	}
	var domainEvents, lifecycleEvents, outboxDeliveries int
	if err := repository.db.QueryRow(`SELECT COUNT(*) FROM domain_events`).Scan(&domainEvents); err != nil {
		t.Fatal(err)
	}
	if err := repository.db.QueryRow(`SELECT COUNT(*) FROM activity_lifecycle_events`).Scan(&lifecycleEvents); err != nil {
		t.Fatal(err)
	}
	if err := repository.db.QueryRow(`SELECT COUNT(*) FROM queue_jobs WHERE category='monitoring'`).Scan(&outboxDeliveries); err != nil {
		t.Fatal(err)
	}
	if domainEvents != 4 || lifecycleEvents != 2 || outboxDeliveries != domainEvents {
		t.Fatalf("events=%d lifecycle=%d outbox=%d", domainEvents, lifecycleEvents, outboxDeliveries)
	}
	if err := repository.FailRun(ctx, "missing", "failure"); err == nil {
		t.Fatal("missing run must fail")
	}
}

func TestRepositoryReturnsNilForMissingRecords(t *testing.T) {
	repository := setup(t)
	ctx := context.Background()
	if run, err := repository.FindRun(ctx, "missing"); err != nil || run != nil {
		t.Fatal("missing run must be nil")
	}
	if handle, err := repository.Find(ctx, "missing"); err != nil || handle != nil {
		t.Fatal("missing handle must be nil")
	}
}

func TestListRunsPageCombinesWorkflowInteractiveAndStandaloneRuns(t *testing.T) {
	repository := setup(t)
	ctx := context.Background()
	run := domain.ExecutionRun{ID: "workflow-run", SchedulePlanID: "plan",
		Mode: domain.ExecutionModeReal, Status: domain.ExecutionRunRunning}
	if err := repository.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`INSERT INTO console_sessions(id,resource_id,runtime_id,connection_id,status,created_at,connected_at)
			VALUES ('session','resource','ssh','connection','connected','2099-08-23T12:01:00Z','2099-08-23T12:01:01Z')`,
		`INSERT INTO console_commands(id,resource_id,runtime_id,connection_id,command_text,timeout_seconds,
			status,created_at,started_at) VALUES
			('command','resource','ssh','connection','hostname',30,'running','2099-08-23T12:02:00Z','2099-08-23T12:02:00Z')`,
	}
	for _, statement := range statements {
		if _, err := repository.db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	first, err := repository.ListRunsPage(ctx, 1, 2, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if first.Total != 3 || len(first.Items) != 2 || !first.HasNext ||
		first.Items[0].Kind != domain.ExecutionRunStandalone ||
		first.Items[1].Kind != domain.ExecutionRunInteractive {
		t.Fatalf("page=%+v", first)
	}
	interactive, err := repository.FindRun(ctx, "session")
	if err != nil || interactive == nil || interactive.Status != domain.ExecutionRunRunning ||
		interactive.ResourceID != "resource" {
		t.Fatalf("interactive=%+v err=%v", interactive, err)
	}
	filtered, err := repository.ListRunsPage(ctx, 1, 20, "interactive", "", "running")
	if err != nil || filtered.Total != 1 || filtered.Items[0].ID != "session" {
		t.Fatalf("filtered=%+v err=%v", filtered, err)
	}
}
