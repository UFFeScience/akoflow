package execution

import (
	"context"
	"testing"
	"time"

	domainexecution "github.com/UFFeScience/akoflow/internal/domain/execution"
)

func TestImagePreparationPersistsActiveAndFinalObservations(t *testing.T) {
	ctx := context.Background()
	repository := setup(t)
	start := time.Now().UTC()
	end := start.Add(3 * time.Second)
	entry := domainexecution.ImagePreparationObservation{
		OperationID: "pull", ActivityID: "activity", Attempt: 1,
		Kind: domainexecution.ImagePreparationPull, StartedAt: start, Status: "running",
	}
	write := func(value domainexecution.ImagePreparationObservation) {
		t.Helper()
		if err := repository.RecordImagePreparation(ctx, "run", value); err != nil {
			t.Fatal(err)
		}
	}
	read := func() []domainexecution.ImagePreparationObservation {
		t.Helper()
		items, err := repository.ListImagePreparation(ctx, "run")
		if err != nil {
			t.Fatal(err)
		}
		return items
	}
	write(entry)
	if items := read(); len(items) != 1 || items[0].FinishedAt != nil {
		t.Fatalf("unexpected active observations: %+v", items)
	}
	completed := entry
	completed.Status, completed.FinishedAt = "cancelled", &end
	write(completed)
	write(entry)
	if items := read(); len(items) != 1 || items[0].Status != "cancelled" ||
		items[0].FinishedAt == nil || !items[0].FinishedAt.Equal(end) {
		t.Fatalf("unexpected terminal observations: %+v", items)
	}
	entry.Attempt++
	write(entry)
	if items := read(); len(items) != 2 {
		t.Fatalf("attempt lost: %+v", items)
	}
}
