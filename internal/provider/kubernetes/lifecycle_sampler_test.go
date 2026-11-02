package kubernetes

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLifecycleSamplerStopsWithoutWaitingForSamplingInterval(t *testing.T) {
	for _, exitCode := range []int{0, 17} {
		t.Run(fmt.Sprint(exitCode), func(t *testing.T) {
			metrics := t.TempDir()
			for name, value := range map[string]string{
				"cpu.stat":       "usage_usec 1000000\n",
				"memory.current": "1024\n",
				"io.stat":        "8:0 rbytes=12 wbytes=34\n",
			} {
				if err := os.WriteFile(filepath.Join(metrics, name), []byte(value), 0600); err != nil {
					t.Fatal(err)
				}
			}
			script := strings.Replace(renderShellLifecycle(),
				`metric_root="/sys/fs/cgroup${metric_cgroup}"`,
				`metric_root="$AKOFLOW_TEST_METRIC_ROOT"`, 1)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, "/bin/sh", "-c", script, "akoflow-entrypoint",
				"/bin/sh", "-c", fmt.Sprintf("printf result > result.txt; exit %d", exitCode))
			command.WaitDelay = 250 * time.Millisecond
			command.Dir = t.TempDir()
			command.Env = append(os.Environ(),
				"AKOFLOW_RUN_ID=run", "AKOFLOW_ACTIVITY_ID=activity", "AKOFLOW_OBSERVATION_ROOT=.",
				"AKOFLOW_METRIC_INTERVAL_SECONDS=60", "AKOFLOW_TEST_METRIC_ROOT="+metrics)
			started := time.Now()
			output, err := command.CombinedOutput()
			if ctx.Err() != nil || time.Since(started) >= 3*time.Second {
				t.Fatalf("sampler delayed command completion: %v: %s", err, output)
			}
			if exitCode == 0 && err != nil {
				t.Fatalf("activity left open pipes or failed: %v: %s", err, output)
			}
			if exitCode != 0 {
				exitError, ok := err.(*exec.ExitError)
				if !ok || exitError.ExitCode() != exitCode {
					t.Fatalf("unexpected process error: %v: %s", err, output)
				}
			}
			if command.ProcessState == nil || command.ProcessState.ExitCode() != exitCode {
				t.Fatalf("activity exit code was changed: %v: %s", err, output)
			}
			if strings.Count(string(output), "AKOFLOW_ACTIVITY_METRIC=") < 2 {
				t.Fatalf("initial or final metrics missing: %s", output)
			}
			if !strings.Contains(string(output), manifestLogPrefix) {
				t.Fatalf("final manifest missing: %s", output)
			}
		})
	}
}
