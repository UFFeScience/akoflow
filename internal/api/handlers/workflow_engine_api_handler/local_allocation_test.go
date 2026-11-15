package workflow_engine_api_handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestLocalRegistrationValidatesActualDeviceCapacity(t *testing.T) {
	capacity, _ := domain.NewLocalCapacity(8, 16<<30, "docker")
	handler := &Handler{environments: environmentRepositoryStub{}, connectionTest: func(context.Context, domain.EnvironmentConnection) ports.ConnectionHealth {
		return ports.ConnectionHealth{Healthy: true, Capacity: capacity}
	}}
	for _, cpu := range []float64{0, 1, 8, 9, 1.5} {
		definition := domain.EnvironmentDefinition{
			Connections: []domain.EnvironmentConnection{{Type: domain.ConnectionLocal}},
			Resources:   []domain.Resource{{Type: domain.ResourceLocalMachine, CPUCores: 99, CPUCapacity: cpu, MemoryBytes: 1 << 30, Metadata: map[string]any{"localAllocation": true}}},
		}
		raw, _ := json.Marshal(definition)
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(raw))
		request.Header.Set("Content-Type", "application/json")
		handler.CreateEnvironment(response, request)
		valid := cpu == 1 || cpu == 8
		if valid && response.Code != http.StatusCreated {
			t.Fatalf("valid allocation rejected: %s", response.Body.String())
		}
		if !valid && response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("invalid CPU accepted: %v", cpu)
		}
		if valid {
			var saved domain.EnvironmentDefinition
			json.Unmarshal(response.Body.Bytes(), &saved)
			if saved.Resources[0].CPUCores != int(cpu) {
				t.Fatal("forged CPU core count was trusted")
			}
		}
	}
	definition := domain.EnvironmentDefinition{
		Connections: []domain.EnvironmentConnection{{Type: domain.ConnectionLocal}},
		Resources:   []domain.Resource{{Type: domain.ResourceLocalMachine, CPUCapacity: 2, MemoryBytes: 17 << 30, Metadata: map[string]any{"localAllocation": true}}},
	}
	if err := handler.validateLocalAllocation(context.Background(), &definition); err == nil {
		t.Fatal("excess memory accepted")
	}
}
