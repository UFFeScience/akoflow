package slurm

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func uncertainSubmission(jobName string, cause error) error {
	return ports.NewRuntimeReconciliationError(cause, map[string]any{
		"slurmSubmissionName": jobName, "executionTarget": string(domain.ExecutionTargetBatch),
	})
}

func (a *Adapter) matchingSubmissionJobs(ctx context.Context, jobName string) ([]string, error) {
	output, err := a.executor.Run(ctx, "squeue", []string{"--me", "--noheader", "--format=%i|%j"}, nil)
	if err != nil {
		return nil, err
	}
	var jobs []string
	for _, line := range strings.Split(string(output), "\n") {
		id, name, ok := strings.Cut(strings.TrimSpace(line), "|")
		if !ok || name != jobName {
			continue
		}
		numericID, parseErr := strconv.ParseUint(id, 10, 64)
		if parseErr != nil || numericID == 0 {
			return nil, fmt.Errorf("invalid exact-attempt Slurm job ID")
		}
		jobs = append(jobs, id)
	}
	return jobs, nil
}

func (a *Adapter) stopUncertainSubmission(ctx context.Context, handle domain.ActivityHandle) error {
	jobName, _ := handle.Metadata["slurmSubmissionName"].(string)
	if handle.ID == "" || jobName != fmt.Sprintf("akoflow-%x", sha256.Sum256([]byte(handle.ID))) {
		return fmt.Errorf("uncertain Slurm submission identity does not match exact attempt")
	}
	if a.executor == nil {
		return fmt.Errorf("slurm command executor is required")
	}
	cleanup, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	jobs, err := a.matchingSubmissionJobs(cleanup, jobName)
	if err != nil {
		return fmt.Errorf("inspect uncertain submission: %w", err)
	}
	if len(jobs) > 0 {
		if _, err := a.executor.Run(cleanup, "scancel", jobs, nil); err != nil {
			return fmt.Errorf("cancel uncertain submission: %w", err)
		}
	}
	for len(jobs) > 0 {
		select {
		case <-cleanup.Done():
			return fmt.Errorf("exact-attempt cancellation not confirmed: %w", cleanup.Err())
		case <-time.After(100 * time.Millisecond):
		}
		jobs, err = a.matchingSubmissionJobs(cleanup, jobName)
		if err != nil {
			return fmt.Errorf("verify uncertain submission cancellation: %w", err)
		}
	}
	return nil
}
