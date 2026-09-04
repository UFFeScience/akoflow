package ansible

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateKeyPathRejectsExternalSchemes(t *testing.T) {
	if _, err := privateKeyPath("vault:key"); err == nil {
		t.Fatal("expected unsupported credential reference")
	}
}

func TestPrivateKeyPathReturnsAbsolutePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	resolved, err := privateKeyPath("file:" + path)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(resolved) {
		t.Fatalf("expected absolute path, got %q", resolved)
	}
}
