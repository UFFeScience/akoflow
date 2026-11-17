package preflight

import (
	"context"
	"strings"
	"testing"
)

func TestEvaluateAggregatesIndependentFailuresAndSanitizesEvidence(t *testing.T) {
	calls := 0
	probes := []Probe{
		{Code: "credentials", Run: func(context.Context) (string, string, string) {
			calls++
			return "blocked", "token=unique-secret", "Correct credentials."
		}},
		{Code: "storage", Run: func(context.Context) (string, string, string) {
			calls++
			return "unverified", "No measured capacity.", "Check storage."
		}},
	}
	report := Evaluate(context.Background(), "report", "hash", "real", probes)
	if report.Ready || report.Warnings != 1 || calls != 2 || len(report.Checks) != 2 || strings.Contains(report.Checks[0].Message, "unique-secret") {
		t.Fatalf("invalid report: %+v", report)
	}
	if !report.ExpiresAt.After(report.CreatedAt) || report.Checks[0].ObservedAt.IsZero() {
		t.Fatal("missing observation validity")
	}
}
func TestCancelledPreflightCannotApproveOrRunProbes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report := Evaluate(ctx, "report", "hash", "real", []Probe{{Run: func(context.Context) (string, string, string) {
		t.Fatal("cancelled probe executed")
		return "passed", "", ""
	}}})
	if report.Ready || report.Checks[0].Status != "blocked" {
		t.Fatalf("cancelled report approved: %+v", report)
	}
}
func TestProbeReceivesBoundedDeadline(t *testing.T) {
	report := Evaluate(context.Background(), "report", "hash", "real", []Probe{{Run: func(ctx context.Context) (string, string, string) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("unbounded probe")
		}
		return "passed", "Ready", ""
	}}})
	if !report.Ready {
		t.Fatal(report)
	}
}
