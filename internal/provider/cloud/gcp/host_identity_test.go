package gcp

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestVerifyHostIdentityFencesGenerationAndRequiresProviderKey(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	key, _ := ssh.NewPublicKey(pub)
	encoded := strings.Fields(string(ssh.MarshalAuthorizedKey(key)))[1]
	for _, scenario := range []string{"valid", "wrong-generation", "replaced-during-read", "missing-key", "invalid-key", "duplicate-key"} {
		t.Run(scenario, func(t *testing.T) {
			reads := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/token" {
					_, _ = w.Write([]byte(`{"access_token":"token"}`))
					return
				}
				if r.Header.Get("Authorization") != "Bearer token" {
					t.Error("unauthenticated provider read")
				}
				if strings.HasSuffix(r.URL.Path, "getGuestAttributes") {
					items := []map[string]string{{"namespace": "hostkeys", "key": "ssh-ed25519", "value": encoded}}
					if scenario == "missing-key" {
						items = nil
					}
					if scenario == "invalid-key" {
						items[0]["value"] = "invalid"
					}
					if scenario == "duplicate-key" {
						items = append(items, items[0])
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"queryValue": map[string]any{"items": items}})
					return
				}
				reads++
				id := "123"
				if scenario == "wrong-generation" || (scenario == "replaced-during-read" && reads > 1) {
					id = "456"
				}
				_ = json.NewEncoder(w).Encode(map[string]string{"id": id, "name": "worker"})
			}))
			defer server.Close()
			catalog := New(server.Client())
			catalog.computeEndpoint = server.URL
			identity, err := catalog.VerifyHostIdentity(context.Background(), testCredential(t, server.URL+"/token"), "project", "zone", "worker", "123")
			if scenario == "valid" {
				if err != nil || identity.Generation != "123" || identity.Fingerprint != ssh.FingerprintSHA256(key) {
					t.Fatalf("identity=%+v err=%v", identity, err)
				}
			} else if err == nil {
				t.Fatalf("accepted unsafe identity for %s", scenario)
			}
		})
	}
}
