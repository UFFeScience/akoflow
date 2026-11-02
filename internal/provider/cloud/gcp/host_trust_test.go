package gcp

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestEnrollmentPreservesGenerationsAndRejectsChangedKeys(t *testing.T) {
	makeIdentity := func(generation string) HostIdentity {
		pub, _, _ := ed25519.GenerateKey(rand.Reader)
		key, _ := ssh.NewPublicKey(pub)
		return HostIdentity{Generation: generation, PublicKey: string(ssh.MarshalAuthorizedKey(key)), Fingerprint: ssh.FingerprintSHA256(key)}
	}
	root := t.TempDir()
	first := makeIdentity("123")
	alias, path, err := first.Enroll(root, "worker")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := first.Enroll(root, "worker"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := makeIdentity("123").Enroll(root, "worker"); err == nil {
		t.Fatal("overwrote generation trust")
	}
	nextAlias, nextPath, err := makeIdentity("456").Enroll(root, "worker")
	if err != nil || nextAlias == alias || nextPath == path {
		t.Fatalf("replacement enrollment: %s %s %v", nextAlias, nextPath, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("previous trust record lost")
	}
}
