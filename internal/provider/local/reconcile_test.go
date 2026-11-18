package local

import (
	"context"
	"errors"
	"testing"
)

type reservationInspectResult struct {
	output string
	err    error
}

func (r reservationInspectResult) Run(context.Context, string, []string, []byte) ([]byte, error) {
	return []byte(r.output), r.err
}

func TestContainerReservationReleasesOnlyWithTerminalEvidence(t *testing.T) {
	name := ExecutionContainerName("run", "task", 2)
	unavailable := errors.New("docker command failed")
	for _, test := range []struct {
		output   string
		err      error
		released bool
	}{
		{"running", nil, false},
		{"created", nil, false},
		{"exited", nil, true},
		{"Error response from daemon: No such container: " + name, unavailable, true},
		{"Cannot connect to the Docker daemon", unavailable, false},
		{"Error: No such container: different", unavailable, false},
	} {
		got := ContainerReservationReleased(context.Background(), reservationInspectResult{test.output, test.err}, name)
		if got != test.released {
			t.Errorf("%q: got %v, want %v", test.output, got, test.released)
		}
	}
}
