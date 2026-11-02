package workflow_engine_api_handler

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func preservationFixture(t *testing.T, name, content string, kind byte) []byte {
	t.Helper()
	var output bytes.Buffer
	gz := gzip.NewWriter(&output)
	archive := tar.NewWriter(gz)
	if err := archive.WriteHeader(&tar.Header{Name: "runs/run/producer/", Typeflag: tar.TypeDir, Mode: 0700}); err != nil {
		t.Fatal(err)
	}
	header := &tar.Header{Name: name, Typeflag: kind, Mode: 0600, Size: int64(len(content))}
	if kind != tar.TypeReg {
		header.Size = 0
		header.Linkname = "outside"
	}
	if err := archive.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if kind == tar.TypeReg {
		if _, err := archive.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestPreservationValidatesManifestAndDurableCopy(t *testing.T) {
	data := preservationFixture(t, "runs/run/producer/output.txt", "science", tar.TypeReg)
	digest := sha256.Sum256([]byte("science"))
	workspace := domain.ActivityWorkspace{ActivityID: "producer", Manifest: domain.WorkspaceManifest{Final: []domain.WorkspaceEntry{{Path: "output.txt", SizeBytes: 7, Digest: "sha256:" + hex.EncodeToString(digest[:])}}}}
	archiveDigest := sha256.Sum256(data)
	file, err := preserveArchive(bytes.NewReader(data), t.TempDir(), hex.EncodeToString(archiveDigest[:]), "run", []domain.ActivityWorkspace{workspace})
	if err != nil {
		t.Fatal(err)
	}
	preserved, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(preserved, data) {
		t.Fatalf("durable archive changed: %v", err)
	}
	workspace.Manifest.Final[0].Digest = "sha256:wrong"
	if err := validatePreservedArchive(bytes.NewReader(data), "run", []domain.ActivityWorkspace{workspace}); err == nil {
		t.Fatal("changed manifest accepted")
	}
	if _, err := preserveArchive(bytes.NewReader(data), t.TempDir(), "wrong", "run", nil); err == nil {
		t.Fatal("wrong archive hash accepted")
	}
}

func TestPreservationRejectsLinksTraversalMissingWorkspaceAndCorruption(t *testing.T) {
	for _, fixture := range []struct {
		name string
		kind byte
	}{{"runs/run/producer/link", tar.TypeSymlink}, {"runs/run/producer/link", tar.TypeLink}, {"runs/run/../escape", tar.TypeReg}, {"runs/another/file", tar.TypeReg}} {
		data := preservationFixture(t, fixture.name, "science", fixture.kind)
		if err := validatePreservedArchive(bytes.NewReader(data), "run", nil); err == nil {
			t.Fatalf("unsafe archive accepted: %+v", fixture)
		}
	}
	data := preservationFixture(t, "runs/run/producer/output.txt", "science", tar.TypeReg)
	if err := validatePreservedArchive(bytes.NewReader(data), "run", []domain.ActivityWorkspace{{ActivityID: "missing"}}); err == nil {
		t.Fatal("missing workspace accepted")
	}
	data[len(data)-5] ^= 0xff
	if err := validatePreservedArchive(bytes.NewReader(data), "run", nil); err == nil {
		t.Fatal("corrupt gzip accepted")
	}
}
