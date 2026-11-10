package hubpublish

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestHubConfirmationRequiresReviewedVisibilityAndExactSnapshot(t *testing.T) {
	visibility := "public"
	confirmations := 0
	reviewed := "exact immutable reviewed bytes"
	manager := testManager(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/desktop/me":
			_ = json.NewEncoder(w).Encode(Identity{Username: "researcher"})
		case "/api/v1/desktop/publications/publication":
			_ = json.NewEncoder(w).Encode(publicationInfo{Publication: Publication{
				ID: "publication", Visibility: visibility, Owner: "researcher", Slug: "experiment",
			}})
		case "/api/v1/desktop/publications/publication/import/confirm":
			var input importInput
			if json.NewDecoder(r.Body).Decode(&input) != nil || input.BundleFile != reviewed || input.BaseVersion != 3 {
				t.Error("reviewed bytes changed")
			}
			confirmations++
			_ = json.NewEncoder(w).Encode(map[string]any{"experimentId": "capture", "version": map[string]int{"number": 4}, "resultCount": 1})
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	})
	manager.state.AccessToken = strings.Repeat("c", 64)
	manager.previews["review"] = preparedPublication{
		Input:       importInput{BundleFile: reviewed, BaseVersion: 3},
		Publication: Publication{ID: "publication", Visibility: "public"},
		TokenHash:   digest([]byte(manager.state.AccessToken)), ExpiresAt: time.Now().Add(time.Minute),
	}
	ctx := context.Background()
	if _, err := manager.Confirm(ctx, "review", false); err == nil {
		t.Fatal("public capture without acknowledgement")
	}
	visibility = "private"
	if _, err := manager.Confirm(ctx, "review", true); err == nil {
		t.Fatal("changed visibility accepted")
	}
	visibility = "public"
	receipt, err := manager.Confirm(ctx, "review", true)
	if err != nil || receipt.Version != 4 || !strings.HasSuffix(receipt.URL, "/u/researcher/experiment") {
		t.Fatalf("confirmation: %+v %v", receipt, err)
	}
	manager.state.AccessToken = strings.Repeat("d", 64)
	if _, err := manager.Confirm(ctx, "review", true); err == nil {
		t.Fatal("review reused by another identity")
	}
	if confirmations != 1 {
		t.Fatalf("unexpected uploads: %d", confirmations)
	}
}
func TestHubRedactionPreservesScientificNumbersAndCredentialReferences(t *testing.T) {
	raw := map[string]any{
		"password": "private-observation-secret", "cores": float64(8),
		"log":        "password=hidden Bearer abcdefghijklmnop private-observation-secret",
		"credential": "credential://cloud/key", "uri": "https://storage.example/a?X-Goog-Signature=secret&part=1",
	}
	cleaned, paths, err := redactSnapshot(raw)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(cleaned)
	for _, secret := range []string{"private-observation-secret", "abcdefghijklmnop", "=hidden", "Signature=secret"} {
		if strings.Contains(string(data), secret) {
			t.Fatal("unredacted secret")
		}
	}
	snapshot := cleaned.(map[string]any)
	if snapshot["cores"] != float64(8) || snapshot["credential"] != "credential://cloud/key" || len(paths) < 3 {
		t.Fatalf("scientific data changed: %+v", snapshot)
	}
}
