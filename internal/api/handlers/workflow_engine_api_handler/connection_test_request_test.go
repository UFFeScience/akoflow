package workflow_engine_api_handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/provider/local"
)

func TestLocalConnectionDocumentationRequest(t *testing.T) {
	var received domain.EnvironmentConnection
	handler := &Handler{connectionTest: func(ctx context.Context, connection domain.EnvironmentConnection) ports.ConnectionHealth {
		received = connection
		return local.NewConnectionProber().Probe(ctx, connection)
	}}
	request := httptest.NewRequest(http.MethodPost, "/connection-tests/", strings.NewReader(`{"type":"local"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.TestEnvironmentConnection(response, request)
	var result struct {
		Healthy bool   `json:"healthy"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != http.StatusOK || received.Type != domain.ConnectionLocal || !result.Healthy || result.Message == "" {
		t.Fatalf("local connection request: status=%d received=%#v result=%#v error=%v", response.Code, received, result, err)
	}
}
