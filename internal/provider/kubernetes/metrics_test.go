package kubernetes

import (
	"strings"
	"testing"
)

func TestExtractActivityMetricsKeepsApplicationOutput(t *testing.T) {
	log, samples := extractActivityMetrics("hello\nAKOFLOW_ACTIVITY_METRIC=100.5\t2.25\t4096\t10\t20\nbye\n", "run", "task")
	if strings.Contains(log, activityMetricPrefix) || !strings.Contains(log, "hello\nbye") {
		t.Fatalf("visible log=%q", log)
	}
	if len(samples) != 1 || samples[0].CPUSeconds != 2.25 || samples[0].Scope != "container" {
		t.Fatalf("samples=%+v", samples)
	}
}
