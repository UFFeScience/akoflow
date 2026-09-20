package slurm

import (
	"strconv"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// metricSamplerScript runs in the same Slurm job cgroup as the activity. It
// writes outside the artifact workspace and never changes activity exit status.
const metricSamplerScript = `
metric_path="${sentinel}.metrics.tsv"
metric_cgroup=$(awk -F: '$1=="0" {print $3; exit}' /proc/self/cgroup 2>/dev/null || true)
metric_root="/sys/fs/cgroup${metric_cgroup}"
metric_pid=""
metric_epoch_anchor=$(date +%s)
metric_uptime_anchor=$(awk '{print $1}' /proc/uptime)
metric_interval=${AKOFLOW_METRIC_INTERVAL_SECONDS:-10}
case "$metric_interval" in ''|*[!0-9]*) metric_interval=10 ;; esac
if [ "$metric_interval" -lt 5 ] || [ "$metric_interval" -gt 60 ]; then metric_interval=10; fi
sample_metrics() {
  metric_uptime_now=$(awk '{print $1}' /proc/uptime)
  observed_at=$(awk -v epoch="$metric_epoch_anchor" -v anchor="$metric_uptime_anchor" -v current="$metric_uptime_now" 'BEGIN {printf "%.3f", epoch+current-anchor}')
  cpu_seconds=$(awk '$1=="usage_usec" {printf "%.6f", $2/1000000}' "$metric_root/cpu.stat" 2>/dev/null)
  memory_bytes=$(cat "$metric_root/memory.current" 2>/dev/null)
  disk_bytes=$(awk '{for (i=2;i<=NF;i++) {if ($i ~ /^rbytes=/) {split($i,a,"="); r+=a[2]} if ($i ~ /^wbytes=/) {split($i,a,"="); w+=a[2]}}} END {printf "%.0f %.0f",r,w}' "$metric_root/io.stat" 2>/dev/null)
  set -- $disk_bytes
  if [ -n "$cpu_seconds" ] && [ -n "$memory_bytes" ]; then
    printf '%s\t%s\t%s\t%s\t%s\n' "$observed_at" "$cpu_seconds" "$memory_bytes" "${1:-0}" "${2:-0}" >> "$metric_path"
  fi
}
if [ -n "$metric_cgroup" ] && [ "$metric_cgroup" != "/" ] && [ -f "$metric_root/cpu.stat" ] && [ -f "$metric_root/memory.current" ]; then
  sample_metrics
  (
    while :; do
      sleep "$metric_interval"
      sample_metrics
    done
  ) >/dev/null 2>&1 &
  metric_pid=$!
fi
`

func applyMetricSamples(handle *domain.ActivityHandle, payload string) {
	previous, _ := handle.Metadata["metricsSampleCount"].(float64)
	if value, ok := handle.Metadata["metricsSampleCount"].(int); ok {
		previous = float64(value)
	}
	samples := parseMetricSamples(payload, handle.RunID, handle.ActivityID)
	if int(previous) >= len(samples) {
		return
	}
	samples = samples[int(previous):]
	handle.Metrics = samples
	handle.Metadata["metricsSampleCount"] = int(previous) + len(samples)
}

func parseMetricSamples(payload, runID, activityID string) []domain.ActivityMetricSample {
	result := make([]domain.ActivityMetricSample, 0)
	for _, line := range strings.Split(payload, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 5 {
			continue
		}
		observed, e1 := strconv.ParseFloat(fields[0], 64)
		cpu, e2 := strconv.ParseFloat(fields[1], 64)
		memory, e3 := strconv.ParseInt(fields[2], 10, 64)
		read, e4 := strconv.ParseInt(fields[3], 10, 64)
		written, e5 := strconv.ParseInt(fields[4], 10, 64)
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil || observed <= 0 || cpu < 0 || memory < 0 || read < 0 || written < 0 {
			continue
		}
		result = append(result, domain.ActivityMetricSample{RunID: runID, ActivityID: activityID,
			Attempt: 1, ObservedAt: observed, CPUSeconds: cpu, MemoryBytes: memory,
			ReadBytes: read, WriteBytes: written, Source: "cgroup-v2", Scope: "slurm-job"})
	}
	return result
}
