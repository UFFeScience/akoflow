package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const managedByAkoflowSelector = "app.kubernetes.io/managed-by=akoflow"

type CleanupResult struct {
	JobsDeleted int
	PodsDeleted int
}

type HistoryCleaner struct {
	api       API
	namespace string
	retention time.Duration
	now       func() time.Time
}

func NewHistoryCleaner(api API, namespace string, retention time.Duration) (*HistoryCleaner, error) {
	if api == nil {
		return nil, fmt.Errorf("Kubernetes API client is required")
	}
	if namespace == "" {
		namespace = "default"
	}
	if retention < 0 {
		return nil, fmt.Errorf("Kubernetes history retention cannot be negative")
	}
	return &HistoryCleaner{api: api, namespace: namespace, retention: retention, now: time.Now}, nil
}

func (c *HistoryCleaner) Run(ctx context.Context, interval time.Duration, report func(CleanupResult, error)) error {
	if interval <= 0 {
		return fmt.Errorf("Kubernetes cleanup interval must be positive")
	}
	c.runOnce(ctx, report)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			c.runOnce(ctx, report)
		}
	}
}

func (c *HistoryCleaner) Cleanup(ctx context.Context) (CleanupResult, error) {
	payload, err := c.api.List(ctx, c.namespace, "jobs", managedByAkoflowSelector)
	if err != nil {
		return CleanupResult{}, fmt.Errorf("list completed Akoflow jobs: %w", err)
	}
	var list kubernetesJobList
	if err := json.Unmarshal(payload, &list); err != nil {
		return CleanupResult{}, fmt.Errorf("decode Akoflow jobs: %w", err)
	}
	cutoff := c.now().Add(-c.retention)
	result := CleanupResult{}
	var cleanupErrors []error
	for _, job := range list.Items {
		if !job.expired(cutoff) {
			continue
		}
		podsDeleted, err := c.deleteJobHistory(ctx, job.Metadata.Name)
		result.PodsDeleted += podsDeleted
		if err != nil {
			cleanupErrors = append(cleanupErrors, err)
			continue
		}
		result.JobsDeleted++
	}
	return result, errors.Join(cleanupErrors...)
}

func (c *HistoryCleaner) runOnce(ctx context.Context, report func(CleanupResult, error)) {
	result, err := c.Cleanup(ctx)
	if report != nil {
		report(result, err)
	}
}

func (c *HistoryCleaner) deleteJobHistory(ctx context.Context, jobName string) (int, error) {
	pods, err := c.jobPods(ctx, jobName)
	if err != nil {
		return 0, err
	}
	deleted := 0
	var cleanupErrors []error
	for _, pod := range pods {
		if err := ignoreNotFound(c.api.Delete(ctx, c.namespace, "pods", pod)); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("delete pod %q: %w", pod, err))
		} else {
			deleted++
		}
	}
	if err := ignoreNotFound(c.api.Delete(ctx, c.namespace, "services", jobName)); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("delete service %q: %w", jobName, err))
	}
	if err := ignoreNotFound(c.api.Delete(ctx, c.namespace, "jobs", jobName)); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("delete job %q: %w", jobName, err))
	}
	return deleted, errors.Join(cleanupErrors...)
}

func (c *HistoryCleaner) jobPods(ctx context.Context, jobName string) ([]string, error) {
	payload, err := c.api.List(ctx, c.namespace, "pods", "job-name="+jobName)
	if err != nil {
		return nil, fmt.Errorf("list pods for job %q: %w", jobName, err)
	}
	var list struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := json.Unmarshal(payload, &list); err != nil {
		return nil, fmt.Errorf("decode pods for job %q: %w", jobName, err)
	}
	result := make([]string, 0, len(list.Items))
	for _, pod := range list.Items {
		if pod.Metadata.Name != "" {
			result = append(result, pod.Metadata.Name)
		}
	}
	return result, nil
}

type kubernetesJobList struct {
	Items []kubernetesJob `json:"items"`
}

type kubernetesJob struct {
	Metadata struct {
		Name              string    `json:"name"`
		CreationTimestamp time.Time `json:"creationTimestamp"`
	} `json:"metadata"`
	Status struct {
		Active         int        `json:"active"`
		Succeeded      int        `json:"succeeded"`
		Failed         int        `json:"failed"`
		CompletionTime *time.Time `json:"completionTime"`
		Conditions     []struct {
			Type               string    `json:"type"`
			Status             string    `json:"status"`
			LastTransitionTime time.Time `json:"lastTransitionTime"`
		} `json:"conditions"`
	} `json:"status"`
}

func (j kubernetesJob) expired(cutoff time.Time) bool {
	if j.Metadata.Name == "" || j.Status.Active > 0 || (j.Status.Succeeded == 0 && j.Status.Failed == 0 && j.Status.CompletionTime == nil) {
		return false
	}
	finishedAt := j.Metadata.CreationTimestamp
	if j.Status.CompletionTime != nil {
		finishedAt = *j.Status.CompletionTime
	} else {
		for _, condition := range j.Status.Conditions {
			if condition.Status == "True" && (condition.Type == "Complete" || condition.Type == "Failed") && condition.LastTransitionTime.After(finishedAt) {
				finishedAt = condition.LastTransitionTime
			}
		}
	}
	return !finishedAt.IsZero() && !finishedAt.After(cutoff)
}
