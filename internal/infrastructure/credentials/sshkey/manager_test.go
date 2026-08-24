package sshkey

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateAndListExposeOnlyPublicMetadata(t *testing.T) {
	directory := t.TempDir()
	manager := New(directory)
	created, err := manager.Generate("plafrim-engine", "akoflow-engine@plafrim")
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	if created.PublicKey == "" || created.Fingerprint == "" || created.CredentialRef == "" {
		t.Fatalf("incomplete public metadata: %#v", created)
	}
	if info, err := os.Stat(filepath.Join(directory, "plafrim-engine")); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("private key permissions = %v, %v", info, err)
	}
	keys, err := manager.List()
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	if len(keys) != 1 || keys[0] != created {
		t.Fatalf("listed keys = %#v, want %#v", keys, created)
	}
}

func TestGenerateRejectsInvalidAndDuplicateIDs(t *testing.T) {
	manager := New(t.TempDir())
	if _, err := manager.Generate("../outside", ""); err == nil {
		t.Fatal("invalid id must be rejected")
	}
	if _, err := manager.Generate("engine", ""); err != nil {
		t.Fatalf("first generation: %v", err)
	}
	if _, err := manager.Generate("engine", ""); err == nil {
		t.Fatal("duplicate id must be rejected")
	}
}

func TestListEmptyDirectory(t *testing.T) {
	keys, err := New(filepath.Join(t.TempDir(), "not-created")).List()
	if err != nil || len(keys) != 0 {
		t.Fatalf("list empty directory = %#v, %v", keys, err)
	}
}
