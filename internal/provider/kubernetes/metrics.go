package kubernetes

import (
	"strconv"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
)

const activityMetricPrefix = "AKOFLOW_ACTIVITY_METRIC="

func extractActivityMetrics(log, runID, activityID string) (string, []domain.ActivityMetricSample) {
	lines := strings.Split(log, "\n")
	visible := make([]string, 0, len(lines))
	samples := make([]domain.ActivityMetricSample, 0)
	for _, line := range lines {
		if !strings.HasPrefix(line, activityMetricPrefix) {
			visible = append(visible, line)
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(line, activityMetricPrefix))
		if len(fields) != 5 {
			continue
		}
		at, e1 := strconv.ParseFloat(fields[0], 64)
		cpu, e2 := strconv.ParseFloat(fields[1], 64)
		memory, e3 := strconv.ParseInt(fields[2], 10, 64)
		read, e4 := strconv.ParseInt(fields[3], 10, 64)
		written, e5 := strconv.ParseInt(fields[4], 10, 64)
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil || at <= 0 || cpu < 0 || memory < 0 || read < 0 || written < 0 {
			continue
		}
		samples = append(samples, domain.ActivityMetricSample{RunID: runID, ActivityID: activityID,
			Attempt: 1, ObservedAt: at, CPUSeconds: cpu, MemoryBytes: memory,
			ReadBytes: read, WriteBytes: written, Source: "cgroup-v2", Scope: "container"})
	}
	return strings.Join(visible, "\n"), samples
}
