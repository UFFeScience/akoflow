package hubtelemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Stream string

const (
	Diagnostics        Stream = "diagnostics"
	APIAccess          Stream = "api_access"
	DesktopInteraction Stream = "desktop_interaction"
	queueLimit                = 256
	batchLimit                = 20
)

type Event struct {
	ID            uuid.UUID      `json:"id"`
	OccurredAt    time.Time      `json:"occurredAt"`
	Stream        Stream         `json:"stream"`
	Component     string         `json:"component"`
	Release       string         `json:"release"`
	Severity      string         `json:"severity"`
	Code          string         `json:"code"`
	Platform      string         `json:"platform,omitempty"`
	Architecture  string         `json:"architecture,omitempty"`
	Operation     string         `json:"operation,omitempty"`
	Screen        string         `json:"screen,omitempty"`
	CorrelationID *uuid.UUID     `json:"correlationId,omitempty"`
	Fingerprint   string         `json:"fingerprint,omitempty"`
	Fields        map[string]any `json:"fields,omitempty"`
}

type streamQueue struct {
	enabled   bool
	enabledAt time.Time
	pending   [][]byte
	cancel    context.CancelFunc
}

type Sender struct {
	mu       sync.Mutex
	streams  map[Stream]*streamQueue
	endpoint string
	token    string
	client   *http.Client
}

func New(endpoint, token string) *Sender {
	return &Sender{
		endpoint: endpoint,
		token:    token,
		streams:  map[Stream]*streamQueue{Diagnostics: {}, APIAccess: {}, DesktopInteraction: {}},
		client: &http.Client{
			Timeout:       3 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (s *Sender) SetToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = token
}

func (s *Sender) SetConsent(stream Stream, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	queue := s.streams[stream]
	if queue == nil || queue.enabled == enabled {
		return
	}
	queue.enabled = enabled
	if enabled {
		queue.enabledAt = time.Now()
		return
	}
	queue.pending = nil
	if queue.cancel != nil {
		queue.cancel()
		queue.cancel = nil
	}
}

func (s *Sender) Enqueue(event Event) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	queue := s.streams[event.Stream]
	if queue == nil || !queue.enabled || event.OccurredAt.Before(queue.enabledAt) ||
		len(queue.pending) >= queueLimit || s.token == "" {
		return false
	}
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	payload, err := json.Marshal(event)
	if err != nil || len(payload) > 4096 {
		return false
	}
	queue.pending = append(queue.pending, payload)
	return true
}

func (s *Sender) Run(ctx context.Context) {
	target, err := url.Parse(s.endpoint)
	if err != nil || target.Scheme != "https" || target.Host == "" || target.User != nil {
		return
	}
	for _, stream := range []Stream{Diagnostics, APIAccess, DesktopInteraction} {
		go s.runStream(ctx, stream)
	}
	<-ctx.Done()
}

func (s *Sender) runStream(ctx context.Context, stream Stream) {
	delay := time.Second
	for ctx.Err() == nil {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if s.flush(ctx, stream) {
			delay = time.Second
		} else {
			delay = min(delay*2, time.Minute)
		}
	}
}

func (s *Sender) flush(ctx context.Context, stream Stream) bool {
	s.mu.Lock()
	queue := s.streams[stream]
	if !queue.enabled || len(queue.pending) == 0 {
		s.mu.Unlock()
		return true
	}
	count := min(len(queue.pending), batchLimit)
	batch := bytes.Join(queue.pending[:count], []byte(","))
	enabledAt := queue.enabledAt
	requestCtx, cancel := context.WithCancel(ctx)
	queue.cancel = cancel
	token := s.token
	s.mu.Unlock()

	discard := s.submit(requestCtx, batch, token)
	cancel()

	s.mu.Lock()
	defer s.mu.Unlock()
	if !queue.enabled || queue.enabledAt != enabledAt {
		return true
	}
	queue.cancel = nil
	if discard {
		clear(queue.pending[:count])
		queue.pending = queue.pending[count:]
	}
	return discard
}

func (s *Sender) submit(ctx context.Context, batch []byte, token string) bool {
	body := append([]byte(`{"schemaVersion":1,"events":[`), batch...)
	body = append(body, ']', '}')
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusBadRequest ||
		resp.StatusCode == http.StatusRequestEntityTooLarge
}
