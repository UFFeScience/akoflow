package installationhealth

import (
	"context"
	"sync"
	"time"
)

type Observation struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Status        string     `json:"status"`
	ObservedAt    *time.Time `json:"observedAt,omitempty"`
	Detail        string     `json:"detail,omitempty"`
	NextAction    string     `json:"nextAction,omitempty"`
	EnvironmentID string     `json:"environmentId,omitempty"`
}

type Report struct {
	Components     []Observation `json:"components"`
	Connections    []Observation `json:"connections"`
	ConnectionNote string        `json:"connectionNote,omitempty"`
}

type Probe struct {
	ID, Name, Action string
	Expensive        bool
	Check            func(context.Context) Observation
}

type CheckConnections func(context.Context) ([]Observation, error)

type Service struct {
	probes      []Probe
	connections CheckConnections
	timeout     time.Duration
	staleAfter  time.Duration
	mu          sync.Mutex
	last        map[string]Observation
	running     map[string]chan struct{}
}

func New(probes []Probe, connections CheckConnections, timeout, staleAfter time.Duration) *Service {
	return &Service{
		probes: probes, connections: connections, timeout: timeout, staleAfter: staleAfter,
		last: map[string]Observation{}, running: map[string]chan struct{}{},
	}
}

func (s *Service) Snapshot(ctx context.Context, refresh bool) Report {
	now := time.Now().UTC()
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	report := Report{
		Components:  []Observation{{ID: "daemon", Name: "AkôFlow daemon", Status: "healthy", ObservedAt: &now, Detail: "The daemon answered the health request."}},
		Connections: []Observation{},
	}
	pending := make(map[string]chan struct{}, len(s.probes))
	for _, probe := range s.probes {
		pending[probe.ID] = s.scheduleProbe(ctx, probe, refresh, now)
	}
	type connectionResult struct {
		items []Observation
		err   error
	}
	results := make(chan connectionResult, 1)
	if s.connections != nil {
		go func() {
			items, err := s.connections(ctx)
			results <- connectionResult{items, err}
		}()
	} else {
		results <- connectionResult{}
	}
	for _, probe := range s.probes {
		report.Components = append(report.Components, s.observe(ctx, probe, pending[probe.ID], now))
	}
	applyConnections := func(result connectionResult) {
		if result.err != nil {
			report.ConnectionNote = "Connection observations are unavailable. Refresh to try again."
		}
		for _, observation := range result.items {
			report.Connections = append(report.Connections, markStale(observation, now, s.staleAfter))
		}
	}
	select {
	case result := <-results:
		applyConnections(result)
	case <-ctx.Done():
		select {
		case result := <-results:
			applyConnections(result)
		default:
			report.ConnectionNote = "Connection observations could not be loaded in time. Refresh to try again."
		}
	}
	return report
}

func (s *Service) scheduleProbe(ctx context.Context, probe Probe, refresh bool, now time.Time) chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing := s.running[probe.ID]; existing != nil {
		return existing
	}
	prior := s.last[probe.ID]
	if !refresh && (probe.Expensive || prior.ObservedAt != nil && now.Sub(*prior.ObservedAt) < time.Minute) {
		return nil
	}
	done := make(chan struct{})
	s.running[probe.ID] = done
	go func() {
		result := probe.Check(ctx)
		captured := time.Now().UTC()
		result.ID, result.Name, result.ObservedAt = probe.ID, probe.Name, &captured
		if result.Status != "healthy" && result.NextAction == "" {
			result.NextAction = probe.Action
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if ctx.Err() == nil {
			s.last[probe.ID] = result
		} else {
			s.last[probe.ID] = timeoutObservation(probe)
		}
		delete(s.running, probe.ID)
		close(done)
	}()
	return done
}

func (s *Service) observe(ctx context.Context, probe Probe, done chan struct{}, now time.Time) Observation {
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			select {
			case <-done:
			default:
				return timeoutObservation(probe)
			}
		}
	}
	s.mu.Lock()
	result := s.last[probe.ID]
	s.mu.Unlock()
	if result.ObservedAt == nil {
		return Observation{ID: probe.ID, Name: probe.Name, Status: "unknown",
			Detail: "This installation has no observation yet.", NextAction: "Refresh checks to collect a new observation."}
	}
	return markStale(result, now, s.staleAfter)
}

func timeoutObservation(probe Probe) Observation {
	at := time.Now().UTC()
	return Observation{ID: probe.ID, Name: probe.Name, Status: "unavailable", ObservedAt: &at,
		Detail: "The check timed out.", NextAction: probe.Action}
}

func markStale(observation Observation, now time.Time, expiry time.Duration) Observation {
	if observation.ObservedAt != nil && observation.ObservedAt.After(now.Add(30*time.Second)) {
		observation.Status = "unknown"
		observation.Detail = "The last observation has an invalid time."
		observation.NextAction = "Check the installation clock and collect a new observation."
	} else if observation.ObservedAt != nil && now.Sub(*observation.ObservedAt) > expiry {
		observation.Status = "stale"
		observation.Detail = "The last observation is outdated."
		if observation.EnvironmentID != "" {
			observation.NextAction = "Open the environment to run a new connection check."
		} else if observation.NextAction == "" {
			observation.NextAction = "Refresh checks to collect a new observation."
		}
	}
	return observation
}
