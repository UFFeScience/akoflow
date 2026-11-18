package local

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type nativeLaunchRecorderFunc func() error

func (f nativeLaunchRecorderFunc) RecordNativeLaunch(context.Context, string, string, int, string, string) error {
	return f()
}

func TestNativeLaunchRequiresDurableIdentityBeforeCommand(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("native process checkpoint requires procfs")
	}
	file := filepath.Join(t.TempDir(), "effect")
	adapter := New()
	execution := domain.ActivityExecutionContext{
		SharedLocalReservation: true,
		Activity: domain.Activity{Command: domain.ActivityCommand{
			Entrypoint: "/bin/sh", Arguments: []string{"-c", `printf started > "$1"`, "fixture", file},
		}},
	}
	for _, checkpointErr := range []error{errors.New("storage unavailable"), nil} {
		adapter.WithNativeLaunchRecorder(nativeLaunchRecorderFunc(func() error {
			if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("command ran before recording process identity: %v", err)
			}
			return checkpointErr
		}))
		command, gate, err := adapter.nativeCommand(execution)
		if err != nil {
			t.Fatal(err)
		}
		err = adapter.startNativeCommand(context.Background(), execution, command, gate)
		if !errors.Is(err, checkpointErr) {
			t.Fatalf("launch error: %v, expected %v", err, checkpointErr)
		}
		if checkpointErr == nil {
			if err := command.Wait(); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal(err)
	}
}
