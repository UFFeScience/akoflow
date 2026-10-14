package transfer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestPeerSnapshotSourceServiceTiming(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	output, err := exec.Command(python, "-c", peerSnapshotTimingScript, "sleep 0.05; printf 'archive-metadata\\n'").CombinedOutput()
	if err != nil {
		t.Fatalf("source timing: %s: %v", output, err)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) != 2 || lines[0] != "archive-metadata" {
		t.Fatalf("protocol changed: %q", output)
	}
	timing, err := parsePeerSnapshotServiceTiming(lines[1])
	if err != nil || timing.WallSeconds < .04 || timing.CPUSeconds >= timing.WallSeconds {
		t.Fatalf("sleep must not become CPU work: %+v %v", timing, err)
	}
	if _, err := exec.Command(python, "-c", peerSnapshotTimingScript, "exit 7").Output(); err == nil {
		t.Fatal("failed build must not become a valid measurement")
	}
}

func TestPeerSnapshotServiceTimingRejectsIncompleteEvidence(t *testing.T) {
	for _, input := range []string{`{}`, `{"wallSeconds":1}`, `{"wallSeconds":-1,"cpuSeconds":0}`, `{"wallSeconds":1,"cpuSeconds":-1}`, `{"wallSeconds":1e999,"cpuSeconds":0}`} {
		if _, err := parsePeerSnapshotServiceTiming(input); err == nil {
			t.Fatalf("accepted invalid evidence %s", input)
		}
	}
	if timing, err := parsePeerSnapshotServiceTiming(`{"wallSeconds":0,"cpuSeconds":0}`); err != nil || timing == nil {
		t.Fatalf("zero measurement must differ from absent legacy evidence: %v", err)
	}
}
