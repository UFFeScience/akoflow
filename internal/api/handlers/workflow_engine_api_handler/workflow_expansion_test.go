package workflow_engine_api_handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/UFFeScience/akoflow/internal/controlplane/eventloop"
)

func TestRequestWorkflowExpansionPublishesIdempotentCommand(t *testing.T) {
	events := &eventPublisherStub{}
	handler := newTestHandler()
	handler.events = events
	body := []byte(`{
		"sourceActivityId":"root", "sourceEventId":"event-1", "sequence":1,
		"activities":[{"key":"child","activity":{"name":"child","kind":"task",
		"capabilities":["real"],"command":{"entrypoint":"true"},"resources":{},"policy":{}}}]
	}`)
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.SetPathValue("versionId", "version-1")
	recorder := httptest.NewRecorder()
	handler.RequestWorkflowExpansion(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("unexpected response %d: %s", recorder.Code, recorder.Body.String())
	}
	if len(events.jobs) != 1 || events.jobs[0].Type != eventloop.EventWorkflowExpansionRequested || events.jobs[0].IdempotencyKey != "workflow-expansion:version-1:event-1" {
		t.Fatalf("unexpected queued command: %+v", events.jobs)
	}
}
