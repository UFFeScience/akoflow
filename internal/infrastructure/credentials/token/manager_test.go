package token

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagerSavesTrimmedTokenWithRestrictedPermissions(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "credentials")
	reference, err := New(directory).Save(" cluster-1 ", " secret-token \n")
	if err != nil {
		t.Fatal(err)
	}
	path := strings.TrimPrefix(reference, "file:")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "secret-token\n" || info.Mode().Perm() != 0o600 {
		t.Fatalf("content=%q permissions=%o", content, info.Mode().Perm())
	}
	if directoryInfo, err := os.Stat(directory); err != nil || directoryInfo.Mode().Perm() != 0o700 {
		t.Fatalf("directory permissions=%v err=%v", directoryInfo.Mode().Perm(), err)
	}
}

func TestManagerRejectsInvalidIDAndEmptyToken(t *testing.T) {
	manager := New(t.TempDir())
	for _, id := range []string{"", "UPPERCASE", "starts_underscore", strings.Repeat("a", 64)} {
		if _, err := manager.Save(id, "token"); err == nil {
			t.Fatalf("invalid id accepted: %q", id)
		}
	}
	if _, err := manager.Save("cluster", " \n"); err == nil {
		t.Fatal("empty token must fail")
	}
}

func TestManagerReportsCredentialDirectoryFailure(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(file, []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(filepath.Join(file, "credentials")).Save("cluster", "token"); err == nil || !strings.Contains(err.Error(), "create credential directory") {
		t.Fatalf("directory error=%v", err)
	}
}
