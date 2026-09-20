package transfer

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type operationEventStore interface {
	AppendWorkflowOperationEvent(context.Context, domain.WorkflowOperationEvent) error
}

var operationEventSequence atomic.Int64

func (m Materializer) operationEvent(ctx context.Context, plan domain.DataTransferPlan, phase, level, message string, progress, total int64, metadata map[string]any) {
	store, ok := m.Progress.(operationEventStore)
	if !ok || plan.ExecutionRunID == "" {
		return
	}
	now := time.Now().UTC()
	sequence := operationEventSequence.Add(1)
	event := domain.WorkflowOperationEvent{
		ID:             fmt.Sprintf("operation-event-%d-%d", now.UnixNano(), sequence),
		ExecutionRunID: plan.ExecutionRunID, ActivityID: plan.ConsumerActivityID,
		OperationID: "operation-" + plan.ID, TransferRunID: plan.ID,
		CommandID: "command-" + plan.ID, Sequence: sequence, Level: level,
		Category: "transfer", Phase: phase, Message: message,
		ProgressBytes: progress, TotalBytes: total, Metadata: metadata, OccurredAt: now,
	}
	if started, ok := metadataFloat(metadata, "startedAt"); ok && progress > 0 {
		elapsed := now.Sub(time.Unix(0, int64(started*float64(time.Second)))).Seconds()
		if elapsed > 0 {
			event.ThroughputBPS = float64(progress) / elapsed
		}
	}
	// Observability must never change the scientific result.
	_ = store.AppendWorkflowOperationEvent(context.WithoutCancel(ctx), event)
}

func metadataFloat(metadata map[string]any, key string) (float64, bool) {
	if metadata == nil {
		return 0, false
	}
	value, ok := metadata[key].(float64)
	return value, ok
}

type transferProgressReader struct {
	reader       io.Reader
	transferred  int64
	total        int64
	lastBytes    int64
	lastReported time.Time
	report       func(int64)
}

func (r *transferProgressReader) Read(buffer []byte) (int, error) {
	count, err := r.reader.Read(buffer)
	r.transferred += int64(count)
	now := time.Now()
	threshold := max(int64(64<<20), r.total/20)
	if r.report != nil && (r.transferred-r.lastBytes >= threshold || now.Sub(r.lastReported) >= 5*time.Second || err == io.EOF) {
		r.lastBytes, r.lastReported = r.transferred, now
		r.report(r.transferred)
	}
	return count, err
}
