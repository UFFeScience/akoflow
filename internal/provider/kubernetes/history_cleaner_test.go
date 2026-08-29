package kubernetes

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type cleanupAPIFake struct {
	jobsPayload []byte
	podsByJob   map[string][]byte
	lists       []string
	deletes     []string
	deleteErr   map[string]error
}

func (*cleanupAPIFake) Create(context.Context, string, string, []byte) error { return nil }
func (*cleanupAPIFake) Get(context.Context, string, string, string) ([]byte, error) {
	return nil, nil
}
func (f *cleanupAPIFake) List(_ context.Context, _, resource, selector string) ([]byte, error) {
	f.lists = append(f.lists, resource+"?"+selector)
	if resource == "jobs" {
		return f.jobsPayload, nil
	}
	return f.podsByJob[selector], nil
}
func (*cleanupAPIFake) Logs(context.Context, string, string, string) ([]byte, error) {
	return nil, nil
}
func (f *cleanupAPIFake) Delete(_ context.Context, _, resource, name string) error {
	key := resource + "/" + name
	f.deletes = append(f.deletes, key)
	return f.deleteErr[key]
}

func TestHistoryCleanerRemovesOnlyExpiredFinishedJobs(t *testing.T) {
	api := &cleanupAPIFake{
		jobsPayload: []byte(`{"items":[
			{"metadata":{"name":"old","creationTimestamp":"2026-08-10T00:00:00Z"},"status":{"succeeded":1,"completionTime":"2026-08-10T01:00:00Z"}},
			{"metadata":{"name":"recent","creationTimestamp":"2026-08-15T23:00:00Z"},"status":{"succeeded":1,"completionTime":"2026-08-15T23:30:00Z"}},
			{"metadata":{"name":"running","creationTimestamp":"2026-08-10T00:00:00Z"},"status":{"active":1,"failed":1}}
		]}`),
		podsByJob: map[string][]byte{
			"job-name=old": []byte(`{"items":[{"metadata":{"name":"old-pod"}}]}`),
		},
		deleteErr: map[string]error{},
	}
	cleaner, err := NewHistoryCleaner(api, "akoflow", 24*time.Hour)
	require.NoError(t, err)
	cleaner.now = func() time.Time { return time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC) }

	result, err := cleaner.Cleanup(context.Background())
	require.NoError(t, err)
	require.Equal(t, CleanupResult{JobsDeleted: 1, PodsDeleted: 1}, result)
	require.Equal(t, []string{"jobs?" + managedByAkoflowSelector, "pods?job-name=old"}, api.lists)
	require.Equal(t, []string{"pods/old-pod", "services/old", "jobs/old"}, api.deletes)
}

func TestHistoryCleanerReportsPartialDeletionErrors(t *testing.T) {
	api := &cleanupAPIFake{
		jobsPayload: []byte(`{"items":[{"metadata":{"name":"old","creationTimestamp":"2026-08-10T00:00:00Z"},"status":{"failed":1}}]}`),
		podsByJob:   map[string][]byte{"job-name=old": []byte(`{"items":[]}`)},
		deleteErr:   map[string]error{"jobs/old": fmt.Errorf("forbidden")},
	}
	cleaner, err := NewHistoryCleaner(api, "akoflow", time.Hour)
	require.NoError(t, err)
	cleaner.now = func() time.Time { return time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC) }

	result, err := cleaner.Cleanup(context.Background())
	require.ErrorContains(t, err, "forbidden")
	require.Zero(t, result.JobsDeleted)
}

func TestHistoryCleanerRemovesWorkspaceClaimOwnedByExpiredJob(t *testing.T) {
	api := &cleanupAPIFake{
		jobsPayload: []byte(`{"items":[{"metadata":{"name":"old","creationTimestamp":"2026-08-10T00:00:00Z","annotations":{"akoflow.io/run-id":"run-1","akoflow.io/activity-id":"activity-1"}},"status":{"succeeded":1}}]}`),
		podsByJob: map[string][]byte{
			"job-name=old": []byte(`{"items":[]}`),
			managedByAkoflowSelector: []byte(`{"items":[
				{"metadata":{"name":"workspace-1","annotations":{"akoflow.io/run-id":"run-1","akoflow.io/activity-id":"activity-1"}}},
				{"metadata":{"name":"workspace-2","annotations":{"akoflow.io/run-id":"run-2","akoflow.io/activity-id":"activity-1"}}}
			]}`),
		},
		deleteErr: map[string]error{},
	}
	cleaner, err := NewHistoryCleaner(api, "akoflow", time.Hour)
	require.NoError(t, err)
	cleaner.now = func() time.Time { return time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC) }

	result, err := cleaner.Cleanup(context.Background())
	require.NoError(t, err)
	require.Equal(t, CleanupResult{JobsDeleted: 1, ClaimsDeleted: 1}, result)
	require.Contains(t, api.deletes, "persistentvolumeclaims/workspace-1")
	require.NotContains(t, api.deletes, "persistentvolumeclaims/workspace-2")
}

func TestHistoryCleanerValidatesConfiguration(t *testing.T) {
	_, err := NewHistoryCleaner(nil, "", time.Hour)
	require.Error(t, err)
	_, err = NewHistoryCleaner(&cleanupAPIFake{}, "", -time.Second)
	require.Error(t, err)
}
