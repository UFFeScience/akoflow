package queue

import (
	"testing"
	"time"
)

func TestNewJobBuildsValidPendingJob(t *testing.T) {
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	job, err := New(CategoryExecution, "activity.submission.requested", []byte(`{"activityId":1}`), now)
	if err != nil {
		t.Fatal(err)
	}
	if job.ID == "" || job.Status != StatusPending || job.MaxAttempts != 5 || !job.AvailableAt.Equal(now) {
		t.Fatalf("unexpected job: %+v", job)
	}
	if err := job.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestJobValidationRejectsIncompleteJobs(t *testing.T) {
	if _, err := New("", "event", nil, time.Time{}); err == nil {
		t.Fatal("missing category must fail")
	}
	if err := (Job{ID: "id", Category: "queue", Type: "event"}).Validate(); err == nil {
		t.Fatal("non-positive max attempts must fail")
	}
}
