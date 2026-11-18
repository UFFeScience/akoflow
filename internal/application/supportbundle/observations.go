package supportbundle

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/installationhealth"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
)

type System struct {
	Version      string `json:"version"`
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
}

type Configuration struct {
	ReadOnly             bool              `json:"readOnly"`
	ConsoleEnabled       bool              `json:"consoleEnabled"`
	BuildContextMaxBytes int64             `json:"buildContextMaxBytes"`
	ComponentStatus      map[string]string `json:"componentStatus"`
	ConnectionStatus     map[string]int    `json:"connectionStatus"`
}

type Operation struct {
	OccurredAt time.Time `json:"occurredAt"`
	Type       string    `json:"type"`
	Outcome    string    `json:"outcome"`
	Status     int       `json:"status,omitempty"`
	DurationMS int64     `json:"durationMs,omitempty"`
}

type Failure struct {
	Source string `json:"source"`
	Code   string `json:"code"`
}

type Observations struct {
	System        *System        `json:"system,omitempty"`
	Configuration *Configuration `json:"configuration,omitempty"`
	Operations    *[]Operation   `json:"operations,omitempty"`
	Failures      []Failure      `json:"failures"`
}

type Collector struct {
	Version              string
	ReadOnly             bool
	ConsoleEnabled       bool
	BuildContextMaxBytes int64
	Health               *installationhealth.Service
	Audit                ports.AuditStore
}

func (c Collector) Collect(ctx context.Context, options map[string]bool, from, to time.Time) Observations {
	result := Observations{Failures: []Failure{}}
	if options["system"] {
		result.System = &System{Version: c.Version, OS: runtime.GOOS, Architecture: runtime.GOARCH}
	}
	if options["configuration"] {
		configuration, partial := c.configuration(ctx)
		result.Configuration = &configuration
		if partial {
			result.Failures = append(result.Failures, Failure{Source: "connection-health", Code: "unavailable"})
		}
	}
	if options["operations"] {
		operations, err := c.operations(ctx, from, to)
		if err != nil {
			result.Failures = append(result.Failures, Failure{Source: "daemon-operations", Code: "unavailable"})
		} else {
			result.Operations = &operations
		}
	}
	return result
}

var validStatus = map[string]bool{"healthy": true, "degraded": true, "unavailable": true, "unknown": true, "stale": true}
var coreComponents = map[string]bool{"daemon": true, "database": true, "storage": true, "docker": true, "buildkit": true}

func (c Collector) configuration(ctx context.Context) (Configuration, bool) {
	result := Configuration{ReadOnly: c.ReadOnly, ConsoleEnabled: c.ConsoleEnabled,
		BuildContextMaxBytes: c.BuildContextMaxBytes, ComponentStatus: map[string]string{}, ConnectionStatus: map[string]int{}}
	if c.Health == nil {
		return result, true
	}
	health := c.Health.Snapshot(ctx, false)
	for _, component := range health.Components {
		if coreComponents[component.ID] && validStatus[component.Status] {
			result.ComponentStatus[component.ID] = component.Status
		}
	}
	for _, connection := range health.Connections {
		if validStatus[connection.Status] {
			result.ConnectionStatus[connection.Status]++
		}
	}
	return result, health.ConnectionNote != ""
}

var operationTypes = map[string]bool{
	"api.access": true, "connection.health.checked": true, "resource.discovery.started": true,
	"resource.discovery.failed": true, "resource.discovery.completed": true,
	"execution.run.created": true, "execution.run.started": true, "execution.run.completed": true,
	"execution.run.failed": true, "execution.run.cancelled": true,
	"queue.job.pending": true, "queue.job.leased": true, "queue.job.failed": true,
	"queue.job.completed": true, "queue.job.cancelled": true,
}

func (c Collector) operations(ctx context.Context, from, to time.Time) ([]Operation, error) {
	if c.Audit == nil {
		return nil, fmt.Errorf("audit store unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	events, err := c.Audit.ListAuditEvents(ctx, domainaudit.Filter{
		From: from.Format(time.RFC3339Nano), To: to.Format(time.RFC3339Nano), Limit: 200,
	})
	if err != nil {
		return nil, err
	}
	result := make([]Operation, 0, min(len(events), 200))
	for _, event := range events {
		if len(result) == 200 {
			break
		}
		if !operationTypes[event.EventType] || event.OccurredAt.Before(from) || event.OccurredAt.After(to) {
			continue
		}
		switch event.Outcome {
		case domainaudit.OutcomeStarted, domainaudit.OutcomeSucceeded, domainaudit.OutcomeFailed:
		default:
			continue
		}
		item := Operation{OccurredAt: event.OccurredAt.UTC(), Type: event.EventType,
			Outcome: string(event.Outcome), DurationMS: max(0, event.DurationMS)}
		if event.Status >= 100 && event.Status < 600 {
			item.Status = event.Status
		}
		result = append(result, item)
	}
	return result, nil
}
