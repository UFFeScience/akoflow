package execution

import (
	"context"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain/preflight"
)

func TestPreflightPersistsWithoutCreatingExecutionOrChangingSchema(t *testing.T) {
	repository := setup(t)
	report := preflight.Report{ID: "preflight-test", ContextHash: "hash", Ready: true, CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Minute)}
	if err := repository.SavePreflight(context.Background(), report); err != nil {
		t.Fatal(err)
	}
	stored, err := repository.FindPreflight(context.Background(), report.ID)
	if err != nil || stored == nil || stored.ContextHash != report.ContextHash || !stored.Ready {
		t.Fatalf("lost report: %+v %v", stored, err)
	}
	missing, err := repository.FindPreflight(context.Background(), "unknown")
	if err != nil || missing != nil {
		t.Fatalf("unknown report: %+v %v", missing, err)
	}
	var count int
	if err := repository.db.QueryRow("SELECT count(*) FROM execution_runs").Scan(&count); err != nil || count != 0 {
		t.Fatalf("validation created work: %d %v", count, err)
	}
}
