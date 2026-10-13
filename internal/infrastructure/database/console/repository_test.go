package console

import (
	"context"
	"database/sql"
	"testing"
	"time"

	domainconsole "github.com/UFFeScience/akoflow/internal/domain/console"
	database "github.com/UFFeScience/akoflow/internal/infrastructure/database"
	_ "github.com/mattn/go-sqlite3"
)

func testRepository(t *testing.T) *Repository {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err = database.Bootstrap(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{`INSERT INTO environments(id,name) VALUES('environment','Environment')`, `INSERT INTO environment_versions(id,environment_id,version,status,network_model,interference_model,cost_model,configuration_hash) VALUES('version','environment',1,'published','{}','{}','{}','hash')`, `INSERT INTO resources(id,environment_version_id,type,name,provider_id,schedulable) VALUES('resource','version','hpc_machine','Node','node',1)`} {
		if _, err = db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	return New(db)
}

func TestConsoleCommandLifecycleAndListing(t *testing.T) {
	repository, ctx := testRepository(t), context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	command := domainconsole.Command{ID: "command", ResourceID: "resource", RuntimeID: "runtime", ConnectionID: "connection", ActorID: "user", Command: "hostname", WorkingDirectory: "/work", Environment: map[string]string{"A": "B"}, CPUCores: 2, MemoryBytes: 1024, TimeoutSeconds: 30, Status: domainconsole.CommandRunning, CreatedAt: now, StartedAt: now}
	if err := repository.SaveConsoleCommand(ctx, command); err != nil {
		t.Fatal(err)
	}
	exit := 0
	finished := now.Add(time.Second)
	command.Status, command.Stdout, command.Stderr, command.ExitCode, command.ExternalID, command.FinishedAt = domainconsole.CommandCompleted, "node\n", "warning", &exit, "job", &finished
	if err := repository.SaveConsoleCommand(ctx, command); err != nil {
		t.Fatal(err)
	}
	found, err := repository.FindConsoleCommand(ctx, "command")
	if err != nil || found == nil || found.Status != domainconsole.CommandCompleted || found.Environment["A"] != "B" || found.ExitCode == nil || *found.ExitCode != 0 || found.FinishedAt == nil {
		t.Fatalf("FindConsoleCommand()=%#v,%v", found, err)
	}
	values, err := repository.ListConsoleCommands(ctx, 0)
	if err != nil || len(values) != 1 || values[0].ID != "command" {
		t.Fatalf("ListConsoleCommands()=%#v,%v", values, err)
	}
	values, err = repository.ListConsoleCommands(ctx, 501)
	if err != nil || len(values) != 1 {
		t.Fatalf("bounded list=%#v,%v", values, err)
	}
	missing, err := repository.FindConsoleCommand(ctx, "missing")
	if err != nil || missing != nil {
		t.Fatalf("missing=%#v,%v", missing, err)
	}
}

func TestConsoleSessionAndTranscriptLifecycle(t *testing.T) {
	repository, ctx := testRepository(t), context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	session := domainconsole.Session{ID: "session", ResourceID: "resource", RuntimeID: "runtime", ConnectionID: "connection", ActorID: "user", Status: domainconsole.SessionStarting, CreatedAt: now}
	if err := repository.SaveConsoleSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	connected := now.Add(time.Second)
	session.Status, session.ConnectedAt = domainconsole.SessionConnected, &connected
	if err := repository.SaveConsoleSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	if err := repository.AppendConsoleSessionLog(ctx, session.ID, "input", []byte("hostname\n"), now); err != nil {
		t.Fatal(err)
	}
	if err := repository.AppendConsoleSessionLog(ctx, session.ID, "output", []byte("node\n"), now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	transcript, err := repository.ReadConsoleSessionLog(ctx, session.ID)
	if err != nil || string(transcript) != "hostname\nnode\n" {
		t.Fatalf("transcript=%q,%v", transcript, err)
	}
	empty, err := repository.ReadConsoleSessionLog(ctx, "missing")
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty=%q,%v", empty, err)
	}
}

func TestConsoleRepositoryReportsConstraintFailures(t *testing.T) {
	repository, ctx := testRepository(t), context.Background()
	now := time.Now().UTC()
	if err := repository.SaveConsoleCommand(ctx, domainconsole.Command{ID: "bad", ResourceID: "missing", TimeoutSeconds: 1, Status: domainconsole.CommandRunning, CreatedAt: now, StartedAt: now}); err == nil {
		t.Fatal("expected foreign key error")
	}
	if err := repository.SaveConsoleSession(ctx, domainconsole.Session{ID: "bad", ResourceID: "missing", Status: domainconsole.SessionStarting, CreatedAt: now}); err == nil {
		t.Fatal("expected session foreign key error")
	}
	if err := repository.AppendConsoleSessionLog(ctx, "session", "invalid", []byte("x"), now); err == nil {
		t.Fatal("expected direction constraint")
	}
}
