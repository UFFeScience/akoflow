package main

import "testing"

func TestMatrixRetryRequiresExplicitSingleNonsharedEntry(t *testing.T) {
	for _, tc := range []struct {
		entry   string
		attempt int
		valid   bool
	}{
		{"", 1, true}, {"", 2, false}, {"moheft-time/medium", 1, false},
		{"moheft-time/medium", 2, true}, {"moheft-cost/micro", 3, true},
		{"moheft-time/hpc", 2, false}, {"moheft-time/typo", 2, false},
	} {
		if err := validateMatrixRetry(tc.entry, tc.attempt); (err == nil) != tc.valid {
			t.Fatalf("entry=%q attempt=%d valid=%v: %v", tc.entry, tc.attempt, tc.valid, err)
		}
	}
}
