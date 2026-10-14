package resource

import "testing"

func TestSchedulingCPUCoresRespectsExplicitCapacity(t *testing.T) {
	for _, test := range []struct {
		cores    int
		capacity float64
		want     int
	}{
		{2, 1, 1}, {8, 3.5, 3}, {2, 4, 2}, {2, 0, 2}, {0, 0, 1}, {2, 0.25, 1},
	} {
		r := Resource{CPUCores: test.cores, CPUCapacity: test.capacity}
		if got := r.SchedulingCPUCores(); got != test.want {
			t.Fatalf("resource=%+v got=%d want=%d", r, got, test.want)
		}
	}
}
