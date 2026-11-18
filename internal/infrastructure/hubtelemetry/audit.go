package hubtelemetry

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"regexp"
	"runtime"
	"strings"

	"github.com/google/uuid"

	"github.com/UFFeScience/akoflow/internal/domain/audit"
)

var accessRoute = regexp.MustCompile(`^(/[a-z][a-z0-9_-]*|/\{[a-zA-Z][a-zA-Z0-9_]*\})+/?$`)

func (s *Sender) RecordAccess(access audit.Event, release string) {
	if access.EventType != "api.access" || access.Outcome == audit.OutcomeStarted ||
		access.Status < 100 || access.Status > 599 || access.DurationMS < 0 || access.DurationMS > 86400000 {
		return
	}
	pattern := strings.Fields(access.Route)
	if len(pattern) == 0 || !strings.HasPrefix(pattern[len(pattern)-1], "/") {
		return
	}
	route := pattern[len(pattern)-1]
	if route == "/akoflow-api/telemetry/consent" || len(route) > 160 || !accessRoute.MatchString(route) {
		return
	}
	switch access.Method {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS":
	default:
		return
	}
	eventID, err := uuid.Parse(access.ID)
	if err != nil {
		return
	}
	event := Event{
		ID: eventID, OccurredAt: access.OccurredAt, Stream: APIAccess, Component: "daemon",
		Release: release, Platform: runtime.GOOS, Architecture: runtime.GOARCH,
		Severity: "info", Code: "api.access",
		Fields: map[string]any{
			"route": route, "method": access.Method, "status": access.Status, "durationMs": access.DurationMS,
		},
	}
	if correlation, err := uuid.Parse(access.CorrelationID); err == nil {
		event.CorrelationID = &correlation
	}
	s.Enqueue(event)
	if access.Status == http.StatusInternalServerError {
		s.recordInternalFailure(access, release, "api.internal_error", access.Method+":"+route)
	}
}

func (s *Sender) recordInternalFailure(entry audit.Event, release, code, fingerprintSource string) {
	fingerprint := sha256.Sum256([]byte("daemon:" + code + ":" + fingerprintSource))
	event := Event{
		ID: uuid.New(), OccurredAt: entry.OccurredAt, Stream: Diagnostics, Component: "daemon",
		Release: release, Platform: runtime.GOOS, Architecture: runtime.GOARCH,
		Severity: "error", Code: code, Fingerprint: hex.EncodeToString(fingerprint[:]),
	}
	if correlation, err := uuid.Parse(entry.CorrelationID); err == nil {
		event.CorrelationID = &correlation
	}
	s.Enqueue(event)
}
