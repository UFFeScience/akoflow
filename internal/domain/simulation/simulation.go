package simulation

import (
	"time"

	"github.com/UFFeScience/akoflow/internal/domain/resource"
)

type Engine struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Driver  string `json:"driver"`
	Enabled bool   `json:"enabled"`
}

// Model is the immutable representation used by a simulation engine. Its
// network is independent from the operational network of the source
// environment, even when it was initially cloned from it.
type Model struct {
	ID                string                   `json:"id"`
	Name              string                   `json:"name"`
	BaseEnvironmentID string                   `json:"baseEnvironmentVersionId,omitempty"`
	EngineID          string                   `json:"engineId"`
	Resources         []resource.Resource      `json:"resources"`
	NetworkTopology   resource.NetworkTopology `json:"networkTopology"`
	InterferenceModel map[string]any           `json:"interferenceModel,omitempty"`
	CostModel         map[string]any           `json:"costModel,omitempty"`
	DataScale         float64                  `json:"dataScale"`
	CreatedAt         time.Time                `json:"createdAt"`
}

// Scenario is an immutable experiment configuration. It references an
// environment version/snapshot but is not itself an executable environment.
type Scenario struct {
	ID                string         `json:"id"`
	Name              string         `json:"name"`
	SimulationModelID string         `json:"simulationModelId"`
	EngineID          string         `json:"engineId"`
	Seed              int64          `json:"seed"`
	InterferenceModel map[string]any `json:"interferenceModel,omitempty"`
	CostModel         map[string]any `json:"costModel,omitempty"`
	DataScale         float64        `json:"dataScale"`
	CreatedAt         time.Time      `json:"createdAt"`
}

type Run struct {
	ID             string    `json:"id"`
	ScenarioID     string    `json:"scenarioId"`
	ExecutionRunID string    `json:"executionRunId"`
	CreatedAt      time.Time `json:"createdAt"`
}
