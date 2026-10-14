package slurm

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type unavailableSubmissionExecutor struct{}

func (unavailableSubmissionExecutor) Run(context.Context, string, []string, []byte) ([]byte, error) {
	return nil, fmt.Errorf("transport unavailable after submission")
}

type reconciliationExecutor struct {
	name                string
	queries             int
	cancelled           []string
	verificationFailure bool
}

func (e *reconciliationExecutor) Run(_ context.Context, command string, args []string, _ []byte) ([]byte, error) {
	if command == "scancel" {
		e.cancelled = append(e.cancelled, args...)
		return nil, nil
	}
	e.queries++
	if e.queries > 1 {
		if e.verificationFailure {
			return nil, fmt.Errorf("queue verification unavailable")
		}
		return []byte("456|unrelated-attempt\n"), nil
	}
	return []byte("123|" + e.name + "\n456|unrelated-attempt\n"), nil
}

func TestUncertainSubmissionStopRequiresExactTerminalProof(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			id := "run:activity:attempt-2"
			name := fmt.Sprintf("akoflow-%x", sha256.Sum256([]byte(id)))
			executor := &reconciliationExecutor{name: name, verificationFailure: failure}
			a := New(executor, "routage")
			handle := domain.ActivityHandle{ID: id, Status: domain.HandleStarting, Metadata: map[string]any{
				ports.RuntimeReconciliationPending: true, "slurmSubmissionName": name,
			}}
			err := a.Stop(context.Background(), handle)
			if (err != nil) != failure {
				t.Fatalf("terminal proof mismatch: %v", err)
			}
			if !reflect.DeepEqual(executor.cancelled, []string{"123"}) {
				t.Fatalf("wrong jobs cancelled: %v", executor.cancelled)
			}
		})
	}
}

func TestUncertainSubmissionRejectsMismatchedIdentity(t *testing.T) {
	executor := &reconciliationExecutor{}
	a := New(executor, "routage")
	err := a.Stop(context.Background(), domain.ActivityHandle{ID: "attempt", Metadata: map[string]any{
		ports.RuntimeReconciliationPending: true, "slurmSubmissionName": "another-attempt",
	}})
	if err == nil || executor.queries != 0 || len(executor.cancelled) != 0 {
		t.Fatal("identity mismatch must fail before external commands")
	}
}

func TestLostSubmissionExportsReconciliationEvidenceWhenQueueUnavailable(t *testing.T) {
	a := NewWithConfig(unavailableSubmissionExecutor{}, Config{ScriptDirectory: t.TempDir(), SubmitFromStdin: true})
	execution := domain.ActivityExecutionContext{
		Run: domain.ExecutionRun{ID: "run"}, Activity: domain.Activity{ID: "activity", Command: domain.ActivityCommand{Entrypoint: "true"}}, Attempt: 2,
	}
	_, err := a.Start(context.Background(), execution)
	var pending interface{ RuntimeReconciliationMetadata() map[string]any }
	if !errors.As(err, &pending) {
		t.Fatalf("lost submission must retain durable reconciliation evidence: %v", err)
	}
	if pending.RuntimeReconciliationMetadata()["slurmSubmissionName"] == nil {
		t.Fatal("exact attempt identity missing")
	}
}
