package workflow_engine_api_handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
)

type interruptedInstanceArchive struct {
	ports.InstanceArchive
	started bool
}

func (archive interruptedInstanceArchive) Export(_ context.Context, output io.Writer, _ bool) error {
	if archive.started {
		_, _ = io.WriteString(output, "unfinished archive")
	}
	return errors.New("archive fixture failed")
}

func TestInstanceArchiveExportFailureBeforeBody(t *testing.T) {
	handler := &Handler{instanceArchive: interruptedInstanceArchive{}}
	response := httptest.NewRecorder()
	handler.ExportArchiveInstance(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusUnprocessableEntity || response.Header().Get("Content-Disposition") != "" {
		t.Fatalf("expected an API error, got status %d and headers %v", response.Code, response.Header())
	}
}

func TestInstanceArchiveExportAbortsAfterBodyStarts(t *testing.T) {
	handler := &Handler{instanceArchive: interruptedInstanceArchive{started: true}}
	response := httptest.NewRecorder()
	defer func() {
		if value := recover(); value != http.ErrAbortHandler {
			t.Fatalf("expected the server to abort the response, got %v", value)
		}
		if response.Body.String() != "unfinished archive" {
			t.Fatalf("unexpected body %q", response.Body.String())
		}
	}()
	handler.ExportArchiveInstance(response, httptest.NewRequest(http.MethodGet, "/", nil))
}
