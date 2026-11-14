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
)

func TestProjectCannotChooseAnotherHub(t *testing.T) {
	manager := testManager(t, func(http.ResponseWriter, *http.Request) { t.Fatal("unexpected network request") })
	before := manager.state.HubURL
	_, err := manager.SaveSettings(context.Background(), true, "http://192.168.1.99:3100")
	if err == nil || manager.state.HubURL != before {
		t.Fatal("project replaced the installation Hub")
	}
	status, err := manager.SaveSettings(context.Background(), true, "")
	if err != nil || !status.Configured || status.HubURL != before {
		t.Fatalf("managed configuration: %+v %v", status, err)
	}
}

func TestConfigurationChangeNeverSendsSavedCredentialsToNewHub(t *testing.T) {
	directory := t.TempDir()
	secret := strings.Repeat("a", 64)
	saved := savedState{Enabled: true, HubURL: "http://192.168.1.99:3100", AccessToken: secret, DeviceCode: strings.Repeat("b", 64), DefaultPublicationID: "old-publication"}
	data, _ := json.Marshal(saved)
	if err := os.WriteFile(filepath.Join(directory, "connection.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	remote := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("old credential was sent to new Hub") }))
	defer remote.Close()
	manager, err := New(Options{Directory: directory, HubURL: remote.URL, BrowserURL: "http://localhost:3100", Source: http.NotFoundHandler()})
	if err != nil {
		t.Fatal(err)
	}
	status, err := manager.Status(context.Background())
	if err != nil || status.Connected || status.Enabled || !status.Configured || status.BrowserURL != "http://localhost:3100" {
		t.Fatalf("configuration change: %+v %v", status, err)
	}
	data, _ = os.ReadFile(filepath.Join(directory, "connection.json"))
	if strings.Contains(string(data), secret) || strings.Contains(string(data), "old-publication") {
		t.Fatal("credentials from a different Hub were retained")
	}
	if manager.browserAddress(remote.URL+"/app/connect?user_code=ABCD-1234") != "http://localhost:3100/app/connect?user_code=ABCD-1234" {
		t.Fatal("browser route did not preserve the authorization path and human code")
	}
	if manager.browserAddress("https://untrusted.example/app/connect") != "" {
		t.Fatal("unrelated browser authorization origin accepted")
	}
}

func TestMissingInstallationHubCannotBeSetByProject(t *testing.T) {
	manager, err := New(Options{Directory: t.TempDir(), Source: http.NotFoundHandler()})
	if err != nil {
		t.Fatal(err)
	}
	status, err := manager.Status(context.Background())
	if err != nil || status.Configured {
		t.Fatalf("missing configuration: %+v %v", status, err)
	}
	if _, err := manager.SaveSettings(context.Background(), true, "https://other.example"); err == nil {
		t.Fatal("project configured a missing installation Hub")
	}
	if _, err := manager.StartAuthorization(context.Background()); err == nil {
		t.Fatal("authorization started without a configured service")
	}
}

func TestSignInUsesManagedHubWithoutSeparateEnableStep(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/oauth/device/authorize" {
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code": strings.Repeat("b", 64), "user_code": "ABCD-1234", "expires_in": 600, "interval": 5,
			"verification_uri_complete": "http://" + r.Host + "/app/connect?user_code=ABCD-1234",
		})
	}))
	defer remote.Close()
	source := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "local-project", "name": "Local research"})
	})
	manager, err := New(Options{Directory: t.TempDir(), HubURL: remote.URL, BrowserURL: "http://localhost:3100", Source: source})
	if err != nil {
		t.Fatal(err)
	}
	status, err := manager.StartAuthorization(context.Background())
	if err != nil || !status.Enabled || status.UserCode != "ABCD-1234" || status.VerificationURL != "http://localhost:3100/app/connect?user_code=ABCD-1234" {
		t.Fatalf("one-step sign-in: %+v %v", status, err)
	}
}
