package execution

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
	database "github.com/UFFeScience/akoflow/internal/infrastructure/database"
	dbqueue "github.com/UFFeScience/akoflow/internal/infrastructure/database/queue"
)

func TestRecoverySubmissionPreservesFirstIntentAcrossConcurrentRetries(t *testing.T) {
	ctx := context.Background()
	repository := setup(t)
	run := submissionRun("run")
	if err := repository.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	if err := repository.FailRun(ctx, run.ID, "fixture"); err != nil {
		t.Fatal(err)
	}
	intent := func(mode, reuse string) domainqueue.Job {
		job := submissionJob(t, run.ID, "recovery-key")
		job.Payload, _ = json.Marshal(ports.ExecutionRequest{Recovery: &ports.ExecutionRecovery{
			Mode: mode, ReuseActivityIDs: []string{reuse},
		}})
		return job
	}
	initial := intent("continue", "completed-activity")
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			stored, err := repository.SubmitRecovery(ctx, initial, "continue")
			if err != nil || stored.ID != initial.ID {
				t.Errorf("recovery %s: %v", stored.ID, err)
			}
		})
	}
	workers.Wait()
	stored, err := repository.SubmitRecovery(ctx, intent("continue", "different-stale-preview"), "continue")
	if err != nil || string(stored.Payload) != string(initial.Payload) {
		t.Fatalf("lost-response retry replaced the original reuse set: %v", err)
	}
	if _, err := repository.SubmitRecovery(ctx, intent("retry_failures", ""), "retry_failures"); !errors.Is(err, ports.ErrExecutionSubmissionConflict) {
		t.Fatalf("changed request did not conflict: %v", err)
	}
	var count int
	if err := repository.db.QueryRow("SELECT count(*) FROM queue_jobs WHERE idempotency_key='recovery-key'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("recovery queue contains %d jobs: %v", count, err)
	}
}

func TestRecoverySubmissionSurvivesProcessExitBeforeDispatch(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "recovery.db")
	db := openRecoveryFixture(t, path)
	if err := database.Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	repository := New(db)
	seedExecutionParents(t, repository)
	if err := repository.CreateRun(ctx, submissionRun("run")); err != nil {
		t.Fatal(err)
	}
	if err := repository.FailRun(ctx, "run", "fixture"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	child := exec.Command(os.Args[0], "-test.run=^TestRecoverySubmissionExitProcess$")
	child.Env = append(os.Environ(), "AKOFLOW_TEST_RECOVERY_DATABASE="+path)
	if output, err := child.CombinedOutput(); err == nil || child.ProcessState.ExitCode() != 74 {
		t.Fatalf("expected process exit after commit and before dispatch: %v, %s", err, output)
	}
	reopened := openRecoveryFixture(t, path)
	defer reopened.Close()
	reopenedRepository := New(reopened)
	run, err := reopenedRepository.FindRun(ctx, "run")
	if err != nil || run == nil || run.Status != "created" {
		t.Fatalf("reopened recovery state: %+v, %v", run, err)
	}
	queue, err := dbqueue.New(reopened)
	if err != nil {
		t.Fatal(err)
	}
	original, err := queue.FindByIdempotencyKey(ctx, "durable-recovery")
	if err != nil || original == nil {
		t.Fatalf("reopened recovery intent: %+v, %v", original, err)
	}
	retry, err := reopenedRepository.SubmitRecovery(ctx, recoveryRestartJob(t), "continue")
	if err != nil || retry.ID != original.ID {
		t.Fatalf("lost-response retry: %+v, %v", retry, err)
	}
	leased, err := queue.Lease(ctx, "worker", []string{"execution"}, 10, time.Minute)
	if err != nil || len(leased) != 1 || leased[0].ID != original.ID {
		t.Fatalf("recovery dispatch after restart: %+v, %v", leased, err)
	}
	var recovered ports.ExecutionRequest
	if err := json.Unmarshal(leased[0].Payload, &recovered); err != nil || recovered.Recovery == nil ||
		len(recovered.Recovery.ReuseActivityIDs) != 1 || recovered.Recovery.ReuseActivityIDs[0] != "finished" {
		t.Fatalf("recovery dispatch lost the validated reuse set: %+v, %v", recovered.Recovery, err)
	}
}

func TestRecoverySubmissionExitProcess(t *testing.T) {
	path := os.Getenv("AKOFLOW_TEST_RECOVERY_DATABASE")
	if path == "" {
		return
	}
	db := openRecoveryFixture(t, path)
	if _, err := New(db).SubmitRecovery(context.Background(), recoveryRestartJob(t), "continue"); err != nil {
		t.Fatal(err)
	}
	os.Exit(74)
}

func openRecoveryFixture(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func recoveryRestartJob(t *testing.T) domainqueue.Job {
	t.Helper()
	job := submissionJob(t, "run", "durable-recovery")
	job.Payload = []byte(`{"recovery":{"mode":"continue","reuseActivityIds":["finished"]}}`)
	return job
}
