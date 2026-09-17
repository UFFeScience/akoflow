package slurm

import "testing"

func TestParseMetricSamplesIgnoresIncompleteRows(t *testing.T) {
	samples := parseMetricSamples("100.5\t2.25\t4096\t10\t20\ninvalid\n101.5\t3.25\t8192\t30\t40\n", "run", "activity")
	if len(samples) != 2 || samples[1].MemoryBytes != 8192 || samples[1].WriteBytes != 40 || samples[0].Scope != "slurm-job" {
		t.Fatalf("unexpected samples: %+v", samples)
	}
}
