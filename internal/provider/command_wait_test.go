package provider

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCommandBoundsOrphanedOutputPipeAfterParentExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX child process fixture")
	}
	started := time.Now()
	output, err := (OSCommandExecutor{}).Run(context.Background(), "sh", []string{"-c", "sleep 30 & printf '%s\\n' \"$!\""}, nil)
	// Kill only the exact child created by this fixture, never other processes.
	pid, parseErr := strconv.Atoi(strings.TrimSpace(string(output)))
	if parseErr != nil {
		t.Fatalf("missing fixture child: %q %v", output, parseErr)
	}
	process, findErr := os.FindProcess(pid)
	if findErr == nil {
		defer process.Kill()
	}
	if !errors.Is(err, exec.ErrWaitDelay) {
		t.Fatalf("expected pipe deadline, got %v", err)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatal("orphan pipe held executor")
	}
}
