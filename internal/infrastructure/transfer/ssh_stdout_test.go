package transfer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestSSHStdoutSeparatesDiagnostics(t *testing.T) {
	bin := t.TempDir()
	script := "#!/bin/sh\nprintf '/tmp/akoflow-peer-ABC123\\n'\nprintf 'transport diagnostic\\n' >&2\nexit \"${SSH_TEST_EXIT:-0}\"\n"
	if err := os.WriteFile(filepath.Join(bin, "ssh"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := runSSHStdout(context.Background(), domain.TransferEndpoint{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "/tmp/akoflow-peer-ABC123\n" {
		t.Fatalf("contaminated stdout: %q", output)
	}
	t.Setenv("SSH_TEST_EXIT", "1")
	_, err = runSSHStdout(context.Background(), domain.TransferEndpoint{}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "transport diagnostic") {
		t.Fatalf("missing failure diagnostic: %v", err)
	}
}
