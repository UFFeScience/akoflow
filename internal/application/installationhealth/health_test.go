package installationhealth

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestInstallationHealthBoundsAnUnavailableProbeIndependently(t *testing.T) {
	blocked := make(chan struct{})
	defer close(blocked)
	var expensiveAttempts atomic.Int32
	stale := time.Now().Add(-time.Hour)
	service := New([]Probe{
		{ID: "docker", Name: "Docker", Expensive: true, Check: func(context.Context) Observation {
			expensiveAttempts.Add(1)
			<-blocked
			return Observation{Status: "healthy"}
		}},
		{ID: "database", Name: "Database", Check: func(context.Context) Observation {
			return Observation{Status: "healthy", Detail: "Database read succeeded."}
		}},
		{ID: "storage", Name: "Storage", Check: func(context.Context) Observation {
			return Observation{Status: "degraded", Detail: "Storage write failed."}
		}},
	}, func(context.Context) ([]Observation, error) {
		return []Observation{{ID: "old", Status: "healthy", ObservedAt: &stale}}, nil
	}, 30*time.Millisecond, 15*time.Minute)
	initial := service.Snapshot(context.Background(), false)
	if initial.Components[1].Status != "unknown" || expensiveAttempts.Load() != 0 ||
		initial.Connections[0].Status != "stale" {
		t.Fatalf("unexpected initial report: %+v", initial)
	}
	started := time.Now()
	refreshed := service.Snapshot(context.Background(), true)
	if time.Since(started) > 500*time.Millisecond {
		t.Fatal("an unavailable dependency delayed the entire report")
	}
	if refreshed.Components[1].Status != "unavailable" || refreshed.Components[1].ObservedAt == nil ||
		refreshed.Components[2].Status != "healthy" || refreshed.Components[3].Status != "degraded" ||
		len(refreshed.Connections) != 1 || refreshed.Connections[0].Status != "stale" {
		t.Fatalf("one probe's failure changed other observations: %+v", refreshed)
	}
	_ = service.Snapshot(context.Background(), true)
	if expensiveAttempts.Load() != 1 {
		t.Fatal("refresh duplicated a blocked probe")
	}
}

func TestInstallationHealthHidesConnectionErrors(t *testing.T) {
	service := New(nil, func(context.Context) ([]Observation, error) {
		return nil, errors.New("sensitive connection URL")
	}, time.Second, time.Minute)
	report := service.Snapshot(context.Background(), true)
	if report.ConnectionNote != "Connection observations are unavailable. Refresh to try again." ||
		len(report.Components) != 1 || report.Components[0].Status != "healthy" {
		t.Fatalf("unexpected report: %+v", report)
	}
}
