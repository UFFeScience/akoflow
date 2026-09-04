package cloud

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestManagerStoresAndResolvesCredential(t *testing.T) {
	manager := New(t.TempDir())
	reference, err := manager.Save("gcp-test", "gcp", json.RawMessage(`{"project_id":"science"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(reference, "cloud-file:") {
		t.Fatalf("unexpected reference %q", reference)
	}
	value, err := manager.Resolve(reference)
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != `{"project_id":"science"}` {
		t.Fatalf("unexpected credential %s", value)
	}
	path := strings.TrimPrefix(reference, "cloud-file:")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("credential mode is %o", info.Mode().Perm())
	}
}

func TestManagerRejectsUnsafeValues(t *testing.T) {
	manager := New(t.TempDir())
	for _, test := range []struct{ id, provider, payload string }{{"../escape", "gcp", "{}"}, {"valid", "other", "{}"}, {"valid", "gcp", "{"}} {
		if _, err := manager.Save(test.id, test.provider, json.RawMessage(test.payload)); err == nil {
			t.Fatalf("expected rejection for %#v", test)
		}
	}
	if _, err := manager.Resolve("cloud-file:/tmp/outside.json"); err == nil {
		t.Fatal("expected outside reference rejection")
	}
}
