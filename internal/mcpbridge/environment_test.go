package mcpbridge

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestEnvironmentRegistrationValidatesConnectionsBeforePersistence(t *testing.T) {
	for _, test := range []struct {
		name                       string
		healthy, secret, simulated bool
		checks, registrations      int64
	}{
		{"connected", true, false, false, 1, 1},
		{"unhealthy", false, false, false, 1, 0},
		{"embedded-secret", true, true, false, 0, 0},
		{"simulated", false, false, true, 0, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			var checks, registrations atomic.Int64
			bridge := testBridge(t, Write, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/akoflow-api/connection-tests/" {
					checks.Add(1)
					var connection map[string]any
					if json.NewDecoder(r.Body).Decode(&connection) != nil || connection["credentialRef"] != "ssh:reference" {
						t.Error("connection definition changed")
					}
					json.NewEncoder(w).Encode(map[string]any{"healthy": test.healthy})
					return
				}
				if !test.simulated && checks.Load() != 1 {
					t.Error("environment persisted before validation")
				}
				registrations.Add(1)
				w.WriteHeader(201)
				w.Write([]byte(`{"environment":{"id":"env-one"}}`))
			})
			body := map[string]any{"environment": map[string]any{"id": "env-one"}}
			if !test.simulated {
				connection := map[string]any{"type": "ssh", "credentialRef": "ssh:reference"}
				if test.secret {
					connection["configuration"] = map[string]any{"token": "do-not-persist"}
				}
				body["connections"] = []any{connection}
			}
			session := testSession(t, bridge)
			result := callTool(t, session, "create_environment", map[string]any{"body": body, "requestId": test.name})
			if result.IsError != (test.registrations == 0) {
				t.Fatalf("unexpected result: %#v", result)
			}
			if checks.Load() != test.checks || registrations.Load() != test.registrations {
				t.Fatalf("checks=%d registrations=%d", checks.Load(), registrations.Load())
			}
		})
	}
}
func TestUploadedEnvironmentCannotBypassConnectionValidation(t *testing.T) {
	var checks, registrations atomic.Int64
	bridge := testBridge(t, Write, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/connection-tests/") {
			checks.Add(1)
			w.Write([]byte(`{"healthy":false,"message":"offline"}`))
			return
		}
		registrations.Add(1)
		w.WriteHeader(201)
	})
	payload := "environment:\n  id: offline\nconnections:\n  - type: ssh\n    credentialRef: ssh:reference\n"
	id, _, err := bridge.transfers.save(strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	session := testSession(t, bridge)
	result := callTool(t, session, "create_environment", map[string]any{
		"uploadId": id, "mediaType": "application/yaml", "requestId": "uploaded-environment",
	})
	if !result.IsError || checks.Load() != 1 || registrations.Load() != 0 {
		t.Fatal("uploaded environment bypassed connection validation")
	}
}

func TestEnvironmentValidationUsesDaemonCaseInsensitiveFieldRules(t *testing.T) {
	var checks, registrations atomic.Int64
	bridge := testBridge(t, Write, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/connection-tests/") {
			checks.Add(1)
			w.Write([]byte(`{"healthy":false}`))
			return
		}
		registrations.Add(1)
		w.WriteHeader(201)
	})
	session := testSession(t, bridge)
	result := callTool(t, session, "create_environment", map[string]any{
		"body":      map[string]any{"Connections": []any{map[string]any{"Type": "ssh", "CredentialRef": "ssh:reference"}}},
		"requestId": "case-insensitive-connections",
	})
	if !result.IsError || checks.Load() != 1 || registrations.Load() != 0 {
		t.Fatal("case aliases bypassed connection validation")
	}
}
