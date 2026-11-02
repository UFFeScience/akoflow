package main

import (
	"crypto/sha512"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestManifestDetectsCollisionCorruptionAndWrongVersion(t *testing.T) {
	for _, scenario := range []string{"valid", "overwritten", "size", "version", "duplicate", "missing", "empty", "legacy-hash", "legacy-path"} {
		t.Run(scenario, func(t *testing.T) {
			directory := t.TempDir()
			payload := []byte("installer")
			digest := sha512.Sum512(payload)
			size := int64(len(payload))
			entry := releaseFile{
				URL:    "Desktop-1.0.10-setup.exe",
				SHA512: base64.StdEncoding.EncodeToString(digest[:]),
				Size:   &size,
			}
			manifest := releaseManifest{Version: "1.0.10", Files: []releaseFile{entry}}
			switch scenario {
			case "overwritten":
				payload = []byte("portable!")
			case "size":
				size++
			case "version":
				manifest.Version = "1.0.9"
			case "duplicate":
				manifest.Files = append(manifest.Files, entry)
			case "empty":
				manifest.Files = nil
			case "legacy-hash":
				manifest.Path, manifest.SHA512 = entry.URL, "different"
			case "legacy-path":
				manifest.Path, manifest.SHA512 = "different.exe", entry.SHA512
			}
			if scenario != "missing" {
				if err := os.WriteFile(filepath.Join(directory, entry.URL), payload, 0600); err != nil {
					t.Fatal(err)
				}
			}
			encoded, err := yaml.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "latest.yml"), encoded, 0600); err != nil {
				t.Fatal(err)
			}
			err = run([]string{"--dir", directory, "--version", "v1.0.10"}, io.Discard)
			if (err == nil) != (scenario == "valid") {
				t.Fatalf("scenario=%s err=%v", scenario, err)
			}
		})
	}
}

func TestArtifactPathsAndChecksumsFailClosed(t *testing.T) {
	for _, name := range []string{"../outside.exe", "nested/file.exe", `nested\file.exe`, ".", ""} {
		if verifyFile(t.TempDir(), releaseFile{URL: name}) == nil {
			t.Fatalf("unsafe path accepted: %q", name)
		}
	}
	if run([]string{"--dir", t.TempDir(), "--version", "1.0.10"}, io.Discard) == nil {
		t.Fatal("missing manifests accepted")
	}
}
