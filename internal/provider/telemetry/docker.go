package telemetry

import (
	"context"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

const dockerSampleInterval = 10 * time.Second

// DockerRunner is satisfied by both local and SSH-backed command executors.
type DockerRunner func(context.Context, string, []string, []byte) ([]byte, error)

// ObserveDocker adds a best-effort container observation to a handle. Docker's
// CPU percentage is integrated over time; unlike cgroup CPU time it is an
// approximation, and the source label makes that distinction visible.
func ObserveDocker(ctx context.Context, handle *domain.ActivityHandle, containerID string, run DockerRunner) {
	if containerID == "" || handle.RunID == "" || handle.ActivityID == "" || run == nil {
		return
	}
	if handle.Metadata == nil {
		handle.Metadata = make(map[string]any)
	}
	now := float64(time.Now().UnixNano()) / float64(time.Second)
	previousAt := number(handle.Metadata["dockerMetricObservedAt"])
	if previousAt > 0 && now-previousAt < dockerSampleInterval.Seconds() {
		return
	}
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	output, err := run(bounded, "docker", []string{"stats", "--no-stream", "--format", "{{json .}}", containerID}, nil)
	if err != nil {
		return
	}
	observed, ok := parseDockerStats(output)
	if !ok {
		return
	}
	cpuSeconds := number(handle.Metadata["dockerMetricCPUSeconds"])
	if previousAt > 0 {
		cpuSeconds += observed.cpuCores * math.Max(0, now-previousAt)
	}
	handle.Metrics = []domain.ActivityMetricSample{{RunID: handle.RunID, ActivityID: handle.ActivityID,
		Attempt: 1, ObservedAt: now, CPUSeconds: cpuSeconds, MemoryBytes: observed.memoryBytes,
		ReadBytes: observed.readBytes, WriteBytes: observed.writeBytes,
		Source: "docker-stats-estimated-cpu", Scope: "container"}}
	handle.Metadata["dockerMetricObservedAt"] = now
	handle.Metadata["dockerMetricCPUSeconds"] = cpuSeconds
}

type dockerStats struct {
	cpuCores    float64
	memoryBytes int64
	readBytes   int64
	writeBytes  int64
}

func parseDockerStats(output []byte) (dockerStats, bool) {
	var payload struct {
		CPUPercent string `json:"CPUPerc"`
		Memory     string `json:"MemUsage"`
		BlockIO    string `json:"BlockIO"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(output))), &payload); err != nil {
		return dockerStats{}, false
	}
	cpu, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(payload.CPUPercent), "%"), 64)
	if err != nil || math.IsNaN(cpu) || math.IsInf(cpu, 0) || cpu < 0 {
		return dockerStats{}, false
	}
	memory, ok := parseBytes(strings.Split(payload.Memory, "/")[0])
	if !ok {
		return dockerStats{}, false
	}
	parts := strings.Split(payload.BlockIO, "/")
	if len(parts) != 2 {
		return dockerStats{}, false
	}
	read, okRead := parseBytes(parts[0])
	written, okWrite := parseBytes(parts[1])
	if !okRead || !okWrite {
		return dockerStats{}, false
	}
	return dockerStats{cpuCores: cpu / 100, memoryBytes: memory, readBytes: read, writeBytes: written}, true
}

func parseBytes(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	position := 0
	for position < len(value) && (value[position] >= '0' && value[position] <= '9' || value[position] == '.') {
		position++
	}
	if position == 0 {
		return 0, false
	}
	number, err := strconv.ParseFloat(value[:position], 64)
	if err != nil || number < 0 || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, false
	}
	unit := strings.ToLower(strings.TrimSpace(value[position:]))
	factor := map[string]float64{"": 1, "b": 1, "kb": 1e3, "mb": 1e6, "gb": 1e9,
		"tb": 1e12, "kib": 1 << 10, "mib": 1 << 20, "gib": 1 << 30, "tib": 1 << 40}[unit]
	if factor == 0 || number*factor > math.MaxInt64 {
		return 0, false
	}
	return int64(number * factor), true
}

func number(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	default:
		return 0
	}
}
