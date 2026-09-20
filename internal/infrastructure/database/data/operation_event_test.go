package data

import (
	"context"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestWorkflowOperationEventsArePersistedInOrder(t *testing.T) {
	repository := testRepository(t)
	ctx := context.Background()
	for sequence, phase := range []string{"queued", "started", "progress", "completed"} {
		event := domain.WorkflowOperationEvent{
			ID: "event-" + phase, ExecutionRunID: "run", ActivityID: "activity",
			OperationID: "operation-transfer", TransferRunID: "transfer",
			Sequence: int64(sequence + 1), Level: "info", Category: "transfer",
			Phase: phase, Message: phase, ProgressBytes: int64(sequence * 10), TotalBytes: 30,
			Metadata: map[string]any{"safe": true}, OccurredAt: time.Unix(int64(sequence+1), 0).UTC(),
		}
		if err := repository.AppendWorkflowOperationEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	events, err := repository.ListWorkflowOperationEvents(ctx, "run")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 || events[0].Phase != "queued" || events[3].Phase != "completed" || events[2].ProgressBytes != 20 {
		t.Fatalf("events=%+v", events)
	}
}
