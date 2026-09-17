package telemetry

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestParseDockerStats(t *testing.T) {
	stats, ok := parseDockerStats([]byte(`{"CPUPerc":"125.50%","MemUsage":"1.5GiB / 4GiB","BlockIO":"1.2MB / 512KiB"}`))
	if !ok || math.Abs(stats.cpuCores-1.255) > .0001 || stats.memoryBytes != 1610612736 || stats.readBytes != 1200000 || stats.writeBytes != 524288 {
		t.Fatalf("stats=%+v ok=%v", stats, ok)
	}
	if _, ok := parseDockerStats([]byte(`{"CPUPerc":"bad","MemUsage":"1MiB / 2MiB","BlockIO":"0B / 0B"}`)); ok {
		t.Fatal("invalid CPU percentage was accepted")
	}
}

func TestObserveDockerIsBoundedAndOptional(t *testing.T) {
	calls := 0
	run := func(_ context.Context, _ string, _ []string, _ []byte) ([]byte, error) {
		calls++
		return []byte(`{"CPUPerc":"50%","MemUsage":"100MiB / 1GiB","BlockIO":"20MiB / 30MiB"}`), nil
	}
	handle := domain.ActivityHandle{RunID: "run", ActivityID: "task", Metadata: map[string]any{
		"dockerMetricObservedAt": float64(time.Now().Add(-20 * time.Second).Unix()),
		"dockerMetricCPUSeconds": float64(3),
	}}
	ObserveDocker(context.Background(), &handle, "container", run)
	if calls != 1 || len(handle.Metrics) != 1 || handle.Metrics[0].CPUSeconds < 12 || handle.Metrics[0].Source != "docker-stats-estimated-cpu" {
		t.Fatalf("calls=%d metrics=%+v", calls, handle.Metrics)
	}
	ObserveDocker(context.Background(), &handle, "container", run)
	if calls != 1 {
		t.Fatalf("Docker was sampled more than once within interval: %d", calls)
	}
}
