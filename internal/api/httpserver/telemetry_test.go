package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/infrastructure/hubtelemetry"
)

func TestTelemetryConsentRequiresNativeAuthenticationAndRevokesCollection(t *testing.T) {
	sender := hubtelemetry.New("https://example.test", "fixture")
	handler := telemetryConsent(context.Background(), sender, SecurityOptions{BearerToken: "fixture"}, time.Minute)
	set := func(native bool, body string, want int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPut, "/akoflow-api/telemetry/consent", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer fixture")
		if native {
			req.Header.Set("X-Akoflow-Native-Request", "telemetry-consent")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("got status %d, want %d", w.Code, want)
		}
	}
	set(false, `{"diagnostics":false,"apiAccess":true}`, http.StatusForbidden)
	set(true, `{"apiAccess":true}`, http.StatusBadRequest)
	set(true, `{"diagnostics":false,"apiAccess":true}`, http.StatusNoContent)
	if !sender.Enqueue(hubtelemetry.Event{Stream: hubtelemetry.APIAccess, OccurredAt: time.Now()}) {
		t.Fatal("API access not enabled")
	}
	if sender.Enqueue(hubtelemetry.Event{Stream: hubtelemetry.Diagnostics, OccurredAt: time.Now()}) {
		t.Fatal("diagnostics enabled without opt-in")
	}
	set(true, `{"diagnostics":false,"apiAccess":false}`, http.StatusNoContent)
	if sender.Enqueue(hubtelemetry.Event{Stream: hubtelemetry.APIAccess, OccurredAt: time.Now()}) {
		t.Fatal("API access still enabled after revocation")
	}
}

func TestDesktopTelemetryConsentExpiresWithoutRenewal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sender := hubtelemetry.New("https://example.test", "fixture")
	handler := telemetryConsent(ctx, sender, SecurityOptions{BearerToken: "fixture"}, 20*time.Millisecond)
	req := httptest.NewRequest(http.MethodPut, "/akoflow-api/telemetry/consent", strings.NewReader(`{"diagnostics":true,"apiAccess":true}`))
	req.Header.Set("Authorization", "Bearer fixture")
	req.Header.Set("X-Akoflow-Native-Request", "telemetry-consent")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("got status %d, want 204", w.Code)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		access := sender.Enqueue(hubtelemetry.Event{Stream: hubtelemetry.APIAccess, OccurredAt: time.Now()})
		diagnostic := sender.Enqueue(hubtelemetry.Event{Stream: hubtelemetry.Diagnostics, OccurredAt: time.Now()})
		if !access && !diagnostic {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("Desktop consent remained enabled after the lease expired")
}
