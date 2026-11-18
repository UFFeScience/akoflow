package hubtelemetry

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestSenderRetriesSameEventsAndPurgesOnOptOut(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	hub := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		bodies = append(bodies, string(body))
		mu.Unlock()
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer hub.Close()
	sender := New(hub.URL, "fixture")
	sender.client = hub.Client()
	before := Event{Stream: APIAccess, OccurredAt: time.Now()}
	if sender.Enqueue(before) {
		t.Fatal("collected with default consent")
	}
	sender.SetConsent(APIAccess, true)
	if sender.Enqueue(before) {
		t.Fatal("collected historical event")
	}
	for i := range queueLimit + 1 {
		if accepted := sender.Enqueue(Event{Stream: APIAccess, OccurredAt: time.Now()}); accepted != (i < queueLimit) {
			t.Fatalf("incorrect queue result at index %d", i)
		}
	}
	for range 2 {
		if sender.flush(context.Background(), APIAccess) {
			t.Fatal("unavailable Hub did not trigger retry")
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 2 || bodies[0] != bodies[1] {
		t.Fatal("retry changed event payload")
	}
	sender.SetConsent(APIAccess, false)
	sender.SetConsent(APIAccess, true)
	if len(sender.streams[APIAccess].pending) != 0 {
		t.Fatal("events survived opt-out")
	}
	if sender.Enqueue(Event{Stream: Diagnostics, OccurredAt: time.Now()}) {
		t.Fatal("API consent also enabled diagnostics")
	}
}
