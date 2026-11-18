package execution

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
	database "github.com/UFFeScience/akoflow/internal/infrastructure/database"
	dbqueue "github.com/UFFeScience/akoflow/internal/infrastructure/database/queue"
	"github.com/mattn/go-sqlite3"
)

func submissionJob(t *testing.T, runID, key string) domainqueue.Job {
	t.Helper()
	job, err := domainqueue.New(domainqueue.CategoryExecution, "execution.run.requested", []byte(`{"plan":"plan"}`), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	job.IdempotencyKey, job.AggregateType, job.AggregateID = key, "execution_run", runID
	return job
}

func submissionRun(id string) domain.ExecutionRun {
	return domain.ExecutionRun{ID: id, SchedulePlanID: "plan", Mode: domain.ExecutionModeReal, Status: domain.ExecutionRunCreated}
}

func TestExecutionSubmissionConcurrentRetriesReturnOriginalJob(t *testing.T) {
	repository := setup(t)
	ctx := context.Background()
	run := submissionRun("run")
	var workers sync.WaitGroup
	jobs := make(chan domainqueue.Job, 8)
	for range 8 {
		job := submissionJob(t, run.ID, "same-key")
		workers.Go(func() {
			stored, err := repository.SubmitRun(ctx, run, job)
			if err != nil {
				t.Error(err)
			}
			jobs <- stored
		})
	}
	workers.Wait()
	close(jobs)
	var first string
	for job := range jobs {
		if first == "" {
			first = job.ID
		}
		if job.ID == "" || job.ID != first {
			t.Fatalf("expected original queue ID %q, got %q", first, job.ID)
		}
	}
	changed := submissionJob(t, run.ID, "same-key")
	changed.Payload = []byte(`{"plan":"different"}`)
	if _, err := repository.SubmitRun(ctx, run, changed); !errors.Is(err, ports.ErrExecutionSubmissionConflict) {
		t.Fatalf("expected payload conflict, got %v", err)
	}
	second := submissionRun("different-run")
	if _, err := repository.SubmitRun(ctx, second, submissionJob(t, second.ID, "new-key")); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := repository.db.QueryRow("SELECT count(*) FROM queue_jobs WHERE aggregate_type='execution_run'").Scan(&count); err != nil || count != 2 {
		t.Fatalf("expected one queue job per intentional execution, count=%d err=%v", count, err)
	}
}

func TestExecutionSubmissionRollsBackAndRemainsDispatchableAfterRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "execution.db")
	open := func() *sql.DB {
		db, err := sql.Open("sqlite3", path+"?_foreign_keys=on")
		if err != nil {
			t.Fatal(err)
		}
		return db
	}
	db := open()
	defer func() { db.Close() }()
	if err := database.Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	repository := New(db)
	seedExecutionParents(t, repository)
	run := submissionRun("run")
	job := submissionJob(t, run.ID, "retry-key")
	child := exec.Command(os.Args[0], "-test.run=^TestExecutionSubmissionCrashProcess$")
	child.Env = append(os.Environ(), "AKOFLOW_TEST_CRASH_DATABASE="+path)
	if output, err := child.CombinedOutput(); err == nil || child.ProcessState.ExitCode() != 73 {
		t.Fatalf("expected subprocess to exit during submission: %v, %s", err, output)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM queue_jobs").Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial queue intent survived process exit: count=%d err=%v", count, err)
	}
	if stored, err := repository.FindRun(ctx, run.ID); err != nil || stored != nil {
		t.Fatalf("partial run survived process exit: run=%+v err=%v", stored, err)
	}
	original, err := repository.SubmitRun(ctx, run, job)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db = open()
	repository = New(db)
	retried, err := repository.SubmitRun(ctx, run, submissionJob(t, run.ID, "retry-key"))
	if err != nil || retried.ID != original.ID {
		t.Fatalf("lost-response retry: job=%+v err=%v", retried, err)
	}
	queue, err := dbqueue.New(db)
	if err != nil {
		t.Fatal(err)
	}
	leased, err := queue.Lease(ctx, "worker", []string{"execution"}, 10, time.Minute)
	if err != nil || len(leased) != 1 || leased[0].ID != original.ID {
		t.Fatalf("durable dispatch: jobs=%+v err=%v", leased, err)
	}
}

func TestExecutionSubmissionCrashProcess(t *testing.T) {
	path := os.Getenv("AKOFLOW_TEST_CRASH_DATABASE")
	if path == "" {
		return
	}
	sql.Register("sqlite3-execution-crash", &sqlite3.SQLiteDriver{
		ConnectHook: func(connection *sqlite3.SQLiteConn) error {
			return connection.RegisterFunc("interrupt_submission", func() int {
				os.Exit(73)
				return 0
			}, false)
		},
	})
	db, err := sql.Open("sqlite3-execution-crash", path+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TEMP TRIGGER interrupt_execution_insert BEFORE INSERT ON execution_runs
		BEGIN SELECT interrupt_submission(); END`); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	run := submissionRun("run")
	_, err = New(db).SubmitRun(ctx, run, submissionJob(t, run.ID, "retry-key"))
	t.Fatalf("submission returned instead of exiting the process: %v", err)
}
