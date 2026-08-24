package environment

import (
	"context"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type monitorStore struct {
	connection domain.EnvironmentConnection
	checks     []domain.ConnectionCheck
	status     domain.EnvironmentStatus
}

func TestAdaptiveCheckIntervalGrowsWithStableOnlineChecks(t *testing.T) {
	tests := []struct {
		successes int
		want      time.Duration
	}{
		{successes: 0, want: time.Minute},
		{successes: 5, want: 2 * time.Minute},
		{successes: 15, want: 5 * time.Minute},
		{successes: 30, want: 10 * time.Minute},
		{successes: 300, want: 10 * time.Minute},
	}
	for _, test := range tests {
		if got := adaptiveCheckInterval(test.successes); got != test.want {
			t.Fatalf("successes=%d interval=%s want=%s", test.successes, got, test.want)
		}
	}
}

func TestConnectionMonitorSkipsCachedOnlineConnectionUntilDue(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	store := &monitorStore{connection: domain.EnvironmentConnection{ID: "kind", EnvironmentID: "env", Type: domain.ConnectionKubernetes}}
	monitor := NewConnectionMonitor(store, map[domain.ConnectionType]ports.ConnectionProber{
		domain.ConnectionKubernetes: proberStub{health: ports.ConnectionHealth{Healthy: true}},
	})
	monitor.now = func() time.Time { return now }

	monitor.CheckAll(context.Background())
	monitor.CheckAll(context.Background())
	if len(store.checks) != 1 {
		t.Fatalf("checks=%d want=1 before cache expires", len(store.checks))
	}
	now = now.Add(time.Minute)
	monitor.CheckAll(context.Background())
	if len(store.checks) != 2 {
		t.Fatalf("checks=%d want=2 after cache expires", len(store.checks))
	}
}

func TestOfflineResultResetsConnectionSchedule(t *testing.T) {
	monitor := NewConnectionMonitor(&monitorStore{}, nil)
	checkedAt := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	for range 30 {
		monitor.recordResult("cluster", domain.ConnectionOnline, checkedAt)
	}
	monitor.recordResult("cluster", domain.ConnectionOffline, checkedAt)

	schedule := monitor.schedules["cluster"]
	if schedule.consecutiveSuccesses != 0 || !schedule.nextCheckAt.Equal(checkedAt.Add(time.Minute)) {
		t.Fatalf("schedule=%+v", schedule)
	}
}

func (s *monitorStore) Create(context.Context, domain.EnvironmentDefinition) error  { return nil }
func (s *monitorStore) Replace(context.Context, domain.EnvironmentDefinition) error { return nil }
func (s *monitorStore) Delete(context.Context, string) error                        { return nil }
func (s *monitorStore) List(context.Context) ([]domain.EnvironmentDefinition, error) {
	return []domain.EnvironmentDefinition{{Connections: []domain.EnvironmentConnection{s.connection}}}, nil
}
func (s *monitorStore) Find(context.Context, string) (*domain.EnvironmentDefinition, error) {
	return nil, nil
}
func (s *monitorStore) UpdateStatus(_ context.Context, _ string, status domain.EnvironmentStatus) error {
	s.status = status
	return nil
}
func (s *monitorStore) UpsertConnection(context.Context, domain.EnvironmentConnection) error {
	return nil
}
func (s *monitorStore) ListConnections(context.Context, string) ([]domain.EnvironmentConnection, error) {
	return []domain.EnvironmentConnection{s.connection}, nil
}
func (s *monitorStore) FindConnection(context.Context, string) (*domain.EnvironmentConnection, error) {
	return &s.connection, nil
}
func (s *monitorStore) ListAllConnections(context.Context) ([]domain.EnvironmentConnection, error) {
	return []domain.EnvironmentConnection{s.connection}, nil
}
func (s *monitorStore) SaveConnectionCheck(_ context.Context, check domain.ConnectionCheck) error {
	s.checks = append(s.checks, check)
	return nil
}
func (s *monitorStore) ListConnectionChecks(context.Context, string, int) ([]domain.ConnectionCheck, error) {
	return s.checks, nil
}

type proberStub struct{ health ports.ConnectionHealth }

func (p proberStub) Probe(context.Context, domain.EnvironmentConnection) ports.ConnectionHealth {
	return p.health
}

func TestConnectionMonitorPersistsHealthAndEnvironmentStatus(t *testing.T) {
	store := &monitorStore{connection: domain.EnvironmentConnection{ID: "kind", EnvironmentID: "env", Type: domain.ConnectionKubernetes}}
	monitor := NewConnectionMonitor(store, map[domain.ConnectionType]ports.ConnectionProber{
		domain.ConnectionKubernetes: proberStub{health: ports.ConnectionHealth{Healthy: true, Message: "reachable"}},
	})
	check, err := monitor.Check(context.Background(), "kind")
	if err != nil || check.Status != domain.ConnectionOnline || store.status != domain.EnvironmentConnected || len(store.checks) != 1 {
		t.Fatalf("check=%+v status=%q checks=%d err=%v", check, store.status, len(store.checks), err)
	}
}

func TestConnectionMonitorRecordsUnsupportedConnectionOffline(t *testing.T) {
	store := &monitorStore{connection: domain.EnvironmentConnection{ID: "ssh", EnvironmentID: "env", Type: domain.ConnectionSSH}}
	monitor := NewConnectionMonitor(store, nil)
	check, err := monitor.Check(context.Background(), "ssh")
	if err != nil || check.Status != domain.ConnectionOffline || store.status != domain.EnvironmentUnreachable {
		t.Fatalf("check=%+v status=%q err=%v", check, store.status, err)
	}
}
