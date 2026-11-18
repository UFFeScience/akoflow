package workflow_engine_api_handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/controlplane/eventloop"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

type recoveryKeyLookup interface {
	FindByIdempotencyKey(context.Context, string) (*domainqueue.Job, error)
}

type recoveryJobPayload struct {
	ports.ExecutionRequest
	Preview executionRecoveryPreview `json:"recoveryPreview"`
}

func (h *Handler) RecoverExecution(w http.ResponseWriter, r *http.Request) {
	if h.readOnly {
		writeError(w, http.StatusForbidden, fmt.Errorf("read-only instances cannot recover executions"))
		return
	}
	store, supportsRecovery := h.executionCommands.(ExecutionRecoveryCommandStore)
	queue, supportsLatest := h.queue.(ExecutionRecoveryQueueStore)
	lookup, supportsLookup := h.queue.(recoveryKeyLookup)
	if !supportsRecovery || !supportsLatest || !supportsLookup {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("execution recovery is unavailable"))
		return
	}
	var input struct {
		Mode string `json:"mode"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.Mode == "" {
		input.Mode = "continue"
	}
	if input.Mode != "continue" && input.Mode != "retry_failures" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("unsupported recovery mode %q", input.Mode))
		return
	}
	runID := r.PathValue("runId")
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(key) > 128 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("recovery submission key must be at most 128 bytes"))
		return
	}
	ctx := r.Context()
	if key != "" {
		key = fmt.Sprintf("execution-recovery-request:%q:%q", runID, key)
		if existingRecovery(w, ctx, lookup, key, input.Mode) {
			return
		}
	}
	latest, err := queue.FindLatestByAggregate(ctx, "execution_run", runID, eventloop.EventExecutionRunRequested)
	if err != nil || latest == nil {
		if err == nil {
			err = fmt.Errorf("the original execution request for %q is unavailable", runID)
		}
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	preview, err := h.executionRecoveryPreview(ctx, runID)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	if !preview.Recoverable {
		if key != "" && existingRecovery(w, ctx, lookup, key, input.Mode) {
			return
		}
		if key == "" && strings.HasPrefix(latest.IdempotencyKey, "execution-recovery-generation:") {
			writeRecoveryResult(w, *latest, input.Mode)
			return
		}
		writeError(w, http.StatusConflict, fmt.Errorf("execution run %q is not recoverable", runID))
		return
	}
	if key == "" {
		key = "execution-recovery-generation:" + latest.ID
	}
	h.submitRecovery(w, ctx, store, *latest, preview, key, input.Mode)
}

func (h *Handler) submitRecovery(w http.ResponseWriter, ctx context.Context, store ExecutionRecoveryCommandStore, latest domainqueue.Job, preview executionRecoveryPreview, key, mode string) {
	var request ports.ExecutionRequest
	if err := json.Unmarshal(latest.Payload, &request); err != nil {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("decode original execution request: %w", err))
		return
	}
	request.Recovery = &ports.ExecutionRecovery{Mode: mode, ReuseActivityIDs: preview.Reusable}
	request.Recovery.RetryActivityIDs = append(append([]string{}, preview.Failed...), preview.Interrupted...)
	payload, err := json.Marshal(recoveryJobPayload{ExecutionRequest: request, Preview: preview})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	job, err := domainqueue.New(domainqueue.CategoryExecution, eventloop.EventExecutionRunRequested, payload, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	job.AggregateType, job.AggregateID, job.IdempotencyKey = "execution_run", preview.RunID, key
	configureExecutionJobRetries(&job, request.Run.Mode)
	stored, err := store.SubmitRecovery(ctx, job, mode)
	if err != nil {
		status := http.StatusUnprocessableEntity
		if errors.Is(err, ports.ErrExecutionSubmissionConflict) {
			status = http.StatusConflict
		}
		writeError(w, status, err)
		return
	}
	writeRecoveryResult(w, stored, mode)
}

func existingRecovery(w http.ResponseWriter, ctx context.Context, lookup recoveryKeyLookup, key, mode string) bool {
	job, err := lookup.FindByIdempotencyKey(ctx, key)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return true
	}
	if job == nil {
		return false
	}
	writeRecoveryResult(w, *job, mode)
	return true
}

func writeRecoveryResult(w http.ResponseWriter, job domainqueue.Job, mode string) {
	var payload recoveryJobPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil || payload.Recovery == nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("stored recovery request is invalid"))
		return
	}
	if payload.Recovery.Mode != mode {
		writeError(w, http.StatusConflict, fmt.Errorf("recovery key is already in use with a different mode"))
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": job, "preview": payload.Preview})
}
