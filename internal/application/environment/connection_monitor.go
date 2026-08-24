package environment

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
	"github.com/UFFeScience/akoflow/internal/provider"
)

type ConnectionMonitor struct {
	store   connectionCatalog
	probers map[domain.ConnectionType]ports.ConnectionProber

	scheduleMu sync.Mutex
	schedules  map[string]connectionSchedule
	now        func() time.Time
	audit      ports.AuditStore
}

type connectionSchedule struct {
	consecutiveSuccesses int
	nextCheckAt          time.Time
}

const maximumStableCheckInterval = 10 * time.Minute

type connectionCatalog interface {
	ports.EnvironmentCatalog
	ports.ConnectionStore
}

var _ ports.ConnectionHealthMonitor = (*ConnectionMonitor)(nil)

func NewConnectionMonitor(
	store connectionCatalog,
	probers map[domain.ConnectionType]ports.ConnectionProber,
	audits ...ports.AuditStore,
) *ConnectionMonitor {
	monitor := &ConnectionMonitor{store: store, probers: probers, schedules: map[string]connectionSchedule{}, now: time.Now}
	if len(audits) > 0 {
		monitor.audit = audits[0]
	}
	return monitor
}

func (m *ConnectionMonitor) Check(ctx context.Context, connectionID string) (domain.ConnectionCheck, error) {
	connection, err := m.store.FindConnection(ctx, connectionID)
	if err != nil {
		return domain.ConnectionCheck{}, err
	}
	if connection == nil {
		return domain.ConnectionCheck{}, fmt.Errorf("environment connection %q was not found", connectionID)
	}
	started := m.now()
	health := ports.ConnectionHealth{Message: "no connection prober is configured"}
	if prober := m.probers[connection.Type]; prober != nil {
		health = prober.Probe(ctx, *connection)
	}
	checkedAt := m.now().UTC()
	status := domain.ConnectionOffline
	environmentStatus := domain.EnvironmentUnreachable
	if health.Healthy {
		status = domain.ConnectionOnline
		environmentStatus = domain.EnvironmentConnected
	}
	check := domain.ConnectionCheck{
		ID: fmt.Sprintf("connection-check-%d", checkedAt.UnixNano()), ConnectionID: connection.ID,
		Status: status, Message: health.Message,
		LatencyMS: float64(m.now().Sub(started).Microseconds()) / 1000, CheckedAt: checkedAt,
		Metadata: map[string]any{"endpoint": connection.Endpoint, "connectionType": connection.Type},
	}
	if err := m.store.SaveConnectionCheck(ctx, check); err != nil {
		return domain.ConnectionCheck{}, err
	}
	if err := m.store.UpdateStatus(ctx, connection.EnvironmentID, environmentStatus); err != nil {
		return domain.ConnectionCheck{}, err
	}
	m.recordResult(connection.ID, status, checkedAt)
	if m.audit != nil {
		outcome := domainaudit.OutcomeFailed
		if health.Healthy {
			outcome = domainaudit.OutcomeSucceeded
		}
		_ = m.audit.RecordAuditEvent(ctx, domainaudit.Event{ID: provider.NewID("audit"), EventType: "connection.health.checked",
			EnvironmentID: connection.EnvironmentID, ConnectionID: connection.ID, Outcome: outcome, Summary: health.Message,
			Metadata: map[string]any{"latencyMs": check.LatencyMS, "status": status}, OccurredAt: checkedAt})
	}
	return check, nil
}

func (m *ConnectionMonitor) History(
	ctx context.Context,
	connectionID string,
	limit int,
) ([]domain.ConnectionCheck, error) {
	return m.store.ListConnectionChecks(ctx, connectionID, limit)
}

func (m *ConnectionMonitor) CheckAll(ctx context.Context) {
	connections, err := m.store.ListAllConnections(ctx)
	if err != nil {
		return
	}
	for _, connection := range connections {
		if ctx.Err() != nil {
			return
		}
		if !m.isDue(connection.ID, m.now()) {
			continue
		}
		_, _ = m.Check(ctx, connection.ID)
	}
}

func (m *ConnectionMonitor) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	m.CheckAll(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.CheckAll(ctx)
		}
	}
}

func (m *ConnectionMonitor) isDue(connectionID string, now time.Time) bool {
	m.scheduleMu.Lock()
	defer m.scheduleMu.Unlock()
	schedule, found := m.schedules[connectionID]
	return !found || !now.Before(schedule.nextCheckAt)
}

func (m *ConnectionMonitor) recordResult(connectionID string, status domain.ConnectionStatus, checkedAt time.Time) {
	m.scheduleMu.Lock()
	defer m.scheduleMu.Unlock()
	schedule := m.schedules[connectionID]
	if status == domain.ConnectionOnline {
		schedule.consecutiveSuccesses++
	} else {
		schedule.consecutiveSuccesses = 0
	}
	schedule.nextCheckAt = checkedAt.Add(adaptiveCheckInterval(schedule.consecutiveSuccesses))
	m.schedules[connectionID] = schedule
}

func adaptiveCheckInterval(consecutiveSuccesses int) time.Duration {
	switch {
	case consecutiveSuccesses >= 30:
		return maximumStableCheckInterval
	case consecutiveSuccesses >= 15:
		return 5 * time.Minute
	case consecutiveSuccesses >= 5:
		return 2 * time.Minute
	default:
		return time.Minute
	}
}
