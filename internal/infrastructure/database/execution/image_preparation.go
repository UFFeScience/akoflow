package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	domainexecution "github.com/UFFeScience/akoflow/internal/domain/execution"
)

const imagePreparationEvent = "execution.image-preparation.observed"

type imagePreparationRecord struct {
	ID            string                                      `json:"id"`
	Type          string                                      `json:"eventType"`
	AggregateType string                                      `json:"aggregateType"`
	AggregateID   string                                      `json:"aggregateId"`
	Payload       domainexecution.ImagePreparationObservation `json:"payload"`
	OccurredAt    time.Time                                   `json:"occurredAt"`
}

func (r *Repository) RecordImagePreparation(ctx context.Context, runID string, observation domainexecution.ImagePreparationObservation) error {
	if runID == "" || observation.OperationID == "" || observation.ActivityID == "" ||
		observation.Attempt < 1 || observation.StartedAt.IsZero() {
		return fmt.Errorf("image preparation requires run, operation, activity, attempt and start")
	}
	id := fmt.Sprintf("image-preparation:%s:%s:%s:%d", runID, observation.OperationID, observation.ActivityID, observation.Attempt)
	record := imagePreparationRecord{
		ID: id, Type: imagePreparationEvent, AggregateType: "execution_run",
		AggregateID: runID, Payload: observation, OccurredAt: observation.StartedAt,
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO domain_events (id,event_type,aggregate_type,aggregate_id,payload,occurred_at)
		VALUES (?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET payload=excluded.payload
		WHERE domain_events.event_type=excluded.event_type
		  AND json_extract(domain_events.payload,'$.payload.finishedAt') IS NULL`,
		id, record.Type, record.AggregateType, runID, string(payload), record.OccurredAt)
	return err
}

func (r *Repository) ListImagePreparation(ctx context.Context, runID string) ([]domainexecution.ImagePreparationObservation, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT payload FROM domain_events
		WHERE aggregate_type='execution_run' AND aggregate_id=? AND event_type=?
		ORDER BY occurred_at, id`, runID, imagePreparationEvent)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var observations []domainexecution.ImagePreparationObservation
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var record imagePreparationRecord
		if err := json.Unmarshal(payload, &record); err != nil {
			return nil, err
		}
		observations = append(observations, record.Payload)
	}
	return observations, rows.Err()
}
