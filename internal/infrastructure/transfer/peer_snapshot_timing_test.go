package transfer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestPeerProtocolCommandTimeExcludesAdmissionWait(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "ssh"), []byte("#!/bin/sh\nsleep 0.05\nprintf protocol\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	var wait, execution float64
	output, err := runSSHProtocolMeasured(context.Background(), domain.TransferEndpoint{}, nil, &wait, &execution)
	if err != nil || string(output) != "protocol" || execution < .04 || wait >= execution {
		t.Fatalf("separate queue/command evidence: %q %g %g %v", output, wait, execution, err)
	}
}
