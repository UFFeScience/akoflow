package kubernetes

import (
	"context"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestConnectionProberUsesConfiguredNamespace(t *testing.T) {
	health := NewConnectionProber(&apiFake{}, "default").Probe(
		context.Background(),
		domain.EnvironmentConnection{Configuration: map[string]any{"namespace": "akoflow"}},
	)
	if !health.Healthy || !strings.Contains(health.Message, "akoflow") {
		t.Fatalf("health=%+v", health)
	}
}

func TestConnectionProberReportsMissingClient(t *testing.T) {
	health := NewConnectionProber(nil, "akoflow").Probe(context.Background(), domain.EnvironmentConnection{})
	if health.Healthy || !strings.Contains(health.Message, "not configured") {
		t.Fatalf("health=%+v", health)
	}
}
