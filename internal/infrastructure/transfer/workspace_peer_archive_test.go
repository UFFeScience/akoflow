package transfer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPeerProbeDiagnosticsDoNotPublishRawSSHText(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{context.DeadlineExceeded, "probe_timeout"},
		{fmt.Errorf("stream directly between cloud instances: worker@private.example: Permission denied (publickey)."), "authentication_rejected"},
		{fmt.Errorf("install temporary destination credential: Permission denied secret-path"), "destination_credential_installation_failed"},
		{fmt.Errorf("Host key verification failed for private.example"), "host_key_rejected"},
		{fmt.Errorf("Load key secret-path: invalid format"), "identity_format_rejected"},
		{fmt.Errorf("unexpected SSH text containing credential-data"), "ssh_probe_failed"},
	} {
		if got := peerProbeFailureCategory(tc.err); got != tc.want {
			t.Fatalf("diagnostic category %q want %q", got, tc.want)
		}
	}
}

func TestPeerArchiveRejectsDamagedGzipBeforePublishing(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "file", Mode: 0600, Size: 1, Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"truncated", "bad-crc"} {
		t.Run(kind, func(t *testing.T) {
			data := append([]byte(nil), buffer.Bytes()...)
			if kind == "truncated" {
				data = data[:len(data)-4]
			} else {
				data[len(data)-8] ^= 1
			}
			root := t.TempDir()
			archive := filepath.Join(root, "peer-workspace.tar.gz")
			if err := os.WriteFile(archive, data, 0600); err != nil {
				t.Fatal(err)
			}
			digest := fmt.Sprintf("%x", sha256.Sum256(data))
			if output, err := exec.Command(python, "-c", peerArchiveVerifyScript, archive, root, digest).CombinedOutput(); err == nil {
				t.Fatalf("damaged gzip accepted: %s", output)
			}
			if _, err := os.Stat(filepath.Join(root, "contents")); !os.IsNotExist(err) {
				t.Fatal("published damaged archive")
			}
		})
	}
}

func TestPeerArchiveVerifiesBeforeExtraction(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal("Python receiver runtime is required for this integration test")
	}
	for _, tc := range []struct {
		name      string
		headers   []*tar.Header
		wrongHash bool
		wantError bool
	}{
		{"regular", []*tar.Header{{Name: "file", Mode: 0600, Size: 1, Typeflag: tar.TypeReg}}, false, false},
		{"traversal", []*tar.Header{{Name: "../escape", Size: 1, Typeflag: tar.TypeReg}}, false, true},
		{"absolute", []*tar.Header{{Name: "/escape", Size: 1, Typeflag: tar.TypeReg}}, false, true},
		{"symlink", []*tar.Header{{Name: "link", Linkname: "../escape", Typeflag: tar.TypeSymlink}}, false, true},
		{"hardlink", []*tar.Header{{Name: "link", Linkname: "file", Typeflag: tar.TypeLink}}, false, true},
		{"duplicate", []*tar.Header{{Name: "file", Size: 1, Typeflag: tar.TypeReg}, {Name: "file", Size: 1, Typeflag: tar.TypeReg}}, false, true},
		{"parent file", []*tar.Header{{Name: "parent/child", Size: 1, Typeflag: tar.TypeReg}, {Name: "parent", Size: 1, Typeflag: tar.TypeReg}}, false, true},
		{"digest mismatch", []*tar.Header{{Name: "file", Size: 1, Typeflag: tar.TypeReg}}, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			archive := filepath.Join(root, "peer-workspace.tar.gz")
			f, err := os.Create(archive)
			if err != nil {
				t.Fatal(err)
			}
			gz := gzip.NewWriter(f)
			writer := tar.NewWriter(gz)
			for _, header := range tc.headers {
				if err := writer.WriteHeader(header); err != nil {
					t.Fatal(err)
				}
				if header.Size > 0 {
					if _, err := writer.Write([]byte("x")); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			if err := gz.Close(); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(archive)
			if err != nil {
				t.Fatal(err)
			}
			digest := fmt.Sprintf("%x", sha256.Sum256(data))
			if tc.wrongHash {
				digest = fmt.Sprintf("%064d", 0)
			}
			if err := os.WriteFile(filepath.Join(root, "peer-probe"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			output, err := exec.Command(python, "-c", peerArchiveVerifyScript, archive, root, digest).CombinedOutput()
			if tc.wantError {
				if err == nil {
					t.Fatal("unsafe archive accepted")
				}
				if _, err := os.Stat(filepath.Join(root, "contents")); !os.IsNotExist(err) {
					t.Fatal("published extraction before full validation")
				}
				return
			}
			if err != nil {
				t.Fatalf("receiver failed: %v: %s", err, output)
			}
			content, err := os.ReadFile(filepath.Join(root, "contents", "file"))
			if err != nil || string(content) != "x" {
				t.Fatalf("wrong extracted content: %q %v", content, err)
			}
			for _, name := range []string{archive, filepath.Join(root, "peer-probe")} {
				if _, err := os.Stat(name); !os.IsNotExist(err) {
					t.Fatal("transport files must not become workspace inputs")
				}
			}
		})
	}
}
