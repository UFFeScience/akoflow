package gcp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestDiscoverNormalizesLiveCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/token":
			_, _ = w.Write([]byte(`{"access_token":"token"}`))
		case strings.HasSuffix(r.URL.Path, "/aggregated/machineTypes"):
			_, _ = w.Write([]byte(`{
				"items": {
					"zones/us-central1-a": {"machineTypes": [{
						"name": "c3-standard-8", "guestCpus": 8, "memoryMb": 32768,
						"zone": "zones/us-central1-a", "architecture": "X86_64"
					}]},
					"zones/us-east1-b": {"machineTypes": [{
						"name": "e2-standard-4", "guestCpus": 4, "memoryMb": 16384,
						"zone": "zones/us-east1-b"
					}]}
				}
			}`))
		case strings.HasSuffix(r.URL.Path, "/aggregated/diskTypes"):
			_, _ = w.Write([]byte(`{"items":{"zones/us-central1-a":{"diskTypes":[{"name":"pd-balanced","zone":"zones/us-central1-a"}]}}}`))
		case strings.Contains(r.URL.Path, "/global/images"):
			_, _ = w.Write([]byte(`{"items":[{"name":"ubuntu-2404-v1","selfLink":"projects/ubuntu-os-cloud/global/images/ubuntu-2404-v1","architecture":"X86_64","status":"READY","diskSizeGb":"10"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	credential := testCredential(t, server.URL+"/token")
	catalog := New(server.Client())
	catalog.computeEndpoint = server.URL
	result, err := catalog.Discover(context.Background(), domain.EnvironmentConnection{Configuration: map[string]any{"provider": "gcp", "projectId": "science", "region": "us-central1"}}, credential)
	if err != nil {
		t.Fatal(err)
	}
	if result.Source != "live" || len(result.Machines) != 1 || result.Machines[0].ProviderTypeID != "c3-standard-8" {
		t.Fatalf("unexpected machines %#v", result.Machines)
	}
	if len(result.Disks) != 1 || len(result.Images) != 4 {
		t.Fatalf("unexpected catalog disks=%d images=%d", len(result.Disks), len(result.Images))
	}
}

func testCredential(t *testing.T, tokenURI string) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	encoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	value, _ := json.Marshal(map[string]string{"project_id": "science", "client_email": "worker@example.test", "private_key": string(encoded), "token_uri": tokenURI})
	return value
}
