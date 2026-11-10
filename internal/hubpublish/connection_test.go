package hubpublish

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testManager(t *testing.T, remote http.HandlerFunc) *Manager {
	t.Helper()
	hub := httptest.NewServer(remote)
	t.Cleanup(hub.Close)
	source := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"name": "Local project"})
	})
	manager, err := New(Options{Directory: t.TempDir(), HubURL: hub.URL, Source: source})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.SaveSettings(context.Background(), true, hub.URL); err != nil {
		t.Fatal(err)
	}
	return manager
}
func TestHubAuthorizationCredentialsRemainPrivateAndSurviveRestart(t *testing.T) {
	secret := strings.Repeat("a", 64)
	device := strings.Repeat("b", 64)
	revoked := false
	manager := testManager(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/oauth/device/authorize":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code": device, "user_code": "ABCD-1234", "expires_in": 600, "interval": 5,
				"verification_uri_complete": "http://" + r.Host + "/app/connect?user_code=ABCD-1234",
			})
		case "/api/v1/oauth/token":
			if err := r.ParseForm(); err != nil || r.Form.Get("device_code") != device {
				t.Error("missing private device code")
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": secret, "token_type": "Bearer", "scope": "publications:write"})
		case "/api/v1/desktop/me":
			if r.Header.Get("Authorization") != "Bearer "+secret {
				t.Error("missing private token")
			}
			_ = json.NewEncoder(w).Encode(Identity{Username: "researcher", ProjectID: "project"})
		case "/api/v1/desktop/session":
			revoked = true
			w.WriteHeader(204)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	})
	status, err := manager.StartAuthorization(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertNoSecrets(t, status, secret, device)
	manager.state.NextPollAt = time.Now().Add(-time.Second)
	status, err = manager.PollAuthorization(context.Background())
	if err != nil || !status.Connected {
		t.Fatalf("connection: %+v %v", status, err)
	}
	assertNoSecrets(t, status, secret, device)
	saved := filepath.Join(manager.options.Directory, "connection.json")
	info, err := os.Stat(saved)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("credential permissions: %v %v", info, err)
	}
	restarted, err := New(manager.options)
	if err != nil {
		t.Fatal(err)
	}
	status, err = restarted.Status(context.Background())
	if err != nil || !status.Connected {
		t.Fatalf("restart: %+v %v", status, err)
	}
	if _, err := restarted.Disconnect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !revoked {
		t.Fatal("disconnect must revoke remote grant")
	}
	data, _ := os.ReadFile(saved)
	if strings.Contains(string(data), secret) || strings.Contains(string(data), device) {
		t.Fatal("disconnect retained secret")
	}
}
func assertNoSecrets(t *testing.T, value any, secrets ...string) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range secrets {
		if strings.Contains(string(data), secret) {
			t.Fatal("secret exposed in public response")
		}
	}
}
func TestHubOriginValidationAndReadOnlyMode(t *testing.T) {
	for _, origin := range []string{"http://public.example", "https://user:pass@example.org", "https://example.org/api", "https://example.org?token=secret", "file:///tmp"} {
		if _, err := validateOrigin(origin); err == nil {
			t.Errorf("accepted %s", origin)
		}
	}
	for _, origin := range []string{"https://hub.example.org", "http://192.168.1.13:3100", "http://127.0.0.1:3100"} {
		if _, err := validateOrigin(origin); err != nil {
			t.Errorf("rejected %s", origin)
		}
	}
	manager := testManager(t, func(w http.ResponseWriter, r *http.Request) { t.Error("read-only mode made remote request") })
	manager.options.ReadOnly = true
	if _, err := manager.StartAuthorization(context.Background()); err == nil {
		t.Fatal("read-only authorized")
	}
	if _, err := manager.SaveSettings(context.Background(), true, manager.state.HubURL); err == nil {
		t.Fatal("read-only changed settings")
	}
}

func TestHubFactoryResetRevokesAndClearsSavedConnection(t *testing.T) {
	revoked := false
	manager := testManager(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" && r.URL.Path == "/api/v1/desktop/session" {
			revoked = true
			w.WriteHeader(204)
			return
		}
		t.Error("unexpected reset request")
	})
	manager.state.AccessToken = strings.Repeat("f", 64)
	if err := manager.persistLocked(); err != nil {
		t.Fatal(err)
	}
	if err := ScheduleReset(manager.options.Directory); err != nil {
		t.Fatal(err)
	}
	restarted, err := New(manager.options)
	if err != nil {
		t.Fatal(err)
	}
	if !revoked || restarted.state.Enabled || restarted.state.AccessToken != "" {
		t.Fatal("factory reset retained authorization")
	}
	if _, err := os.Stat(filepath.Join(manager.options.Directory, ".factory-reset-pending")); !os.IsNotExist(err) {
		t.Fatal("reset marker retained")
	}
}
