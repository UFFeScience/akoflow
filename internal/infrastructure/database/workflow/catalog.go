package workflow

import (
	"context"
	"database/sql"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

const latestVersions = `SELECT v.id, v.workflow_id, v.version, v.definition_hash
	FROM workflow_versions v WHERE v.version=(
		SELECT MAX(v2.version) FROM workflow_versions v2 WHERE v2.workflow_id=v.workflow_id)`

// ListCatalog returns counts and three recent headers per workflow in two
// queries, independent of the number of definitions or history pages.
func (r *Repository) ListCatalog(ctx context.Context) ([]domain.WorkflowCatalogEntry, error) {
	entries, err := r.listCatalogHeaders(ctx)
	if err != nil {
		return nil, err
	}
	return r.withRecentCatalogRuns(ctx, entries)
}

func (r *Repository) listCatalogHeaders(ctx context.Context) ([]domain.WorkflowCatalogEntry, error) {
	rows, err := r.db.QueryContext(ctx, `WITH latest AS (`+latestVersions+`),
	plan_counts AS (SELECT workflow_version_id, COUNT(*) AS count FROM schedule_plans GROUP BY workflow_version_id),
	activity_counts AS (SELECT workflow_version_id, COUNT(*) AS count FROM activity_definitions GROUP BY workflow_version_id),
	run_counts AS (
		SELECT p.workflow_version_id, COUNT(*) AS count,
			SUM(CASE WHEN e.status='completed' THEN 1 ELSE 0 END) AS completed,
			SUM(e.cost) AS cost
		FROM execution_runs e JOIN schedule_plans p ON p.id=e.schedule_plan_id
		GROUP BY p.workflow_version_id
	)
	SELECT w.id, w.external_id, w.name, w.namespace, v.id, v.version, v.definition_hash,
		COALESCE(a.count,0), COALESCE(p.count,0), COALESCE(e.count,0),
		COALESCE(e.completed,0), COALESCE(e.cost,0)
	FROM workflow_definitions w JOIN latest v ON v.workflow_id=w.id
	LEFT JOIN activity_counts a ON a.workflow_version_id=v.id
	LEFT JOIN plan_counts p ON p.workflow_version_id=v.id
	LEFT JOIN run_counts e ON e.workflow_version_id=v.id
	ORDER BY w.namespace, w.name, w.id`)
	if err != nil {
		return nil, err
	}
	entries := make([]domain.WorkflowCatalogEntry, 0)
	for rows.Next() {
		var entry domain.WorkflowCatalogEntry
		if err := rows.Scan(&entry.ID, &entry.ExternalID, &entry.Name, &entry.Namespace,
			&entry.Version.ID, &entry.Version.Version, &entry.Version.DefinitionHash,
			&entry.ActivityCount, &entry.PlanCount, &entry.RunCount,
			&entry.CompletedRunCount, &entry.ObservedCost); err != nil {
			rows.Close()
			return nil, err
		}
		entry.Version.WorkflowID = entry.ID
		entry.RecentRuns = make([]domain.WorkflowCatalogRun, 0)
		entries = append(entries, entry)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *Repository) withRecentCatalogRuns(ctx context.Context, entries []domain.WorkflowCatalogEntry) ([]domain.WorkflowCatalogEntry, error) {
	byVersion := make(map[string]int, len(entries))
	for index, entry := range entries {
		byVersion[entry.Version.ID] = index
	}
	// Rank only cheap run headers; task and transfer aggregates are not used by
	// the collapsed catalog and must never be computed for its entire history.
	recent, err := r.db.QueryContext(ctx, `WITH ranked AS (
		SELECT e.id, e.schedule_plan_id, e.mode, e.status, e.makespan_seconds, e.cost,
			e.started_at, e.finished_at, p.workflow_version_id,
			ROW_NUMBER() OVER (PARTITION BY p.workflow_version_id
				ORDER BY COALESCE(e.finished_at,e.started_at) DESC, e.id DESC) AS rank
		FROM execution_runs e JOIN schedule_plans p ON p.id=e.schedule_plan_id
		WHERE p.workflow_version_id IN (SELECT id FROM (`+latestVersions+`))
	) SELECT id, schedule_plan_id, mode, status, makespan_seconds, cost,
		started_at, finished_at, workflow_version_id FROM ranked WHERE rank<=3
	ORDER BY workflow_version_id, rank`)
	if err != nil {
		return nil, err
	}
	defer recent.Close()
	for recent.Next() {
		var item domain.WorkflowCatalogRun
		var startedAt, finishedAt sql.NullTime
		var versionID string
		if err := recent.Scan(&item.Run.ID, &item.Run.SchedulePlanID, &item.Run.Mode,
			&item.Run.Status, &item.Run.MakespanSeconds, &item.Run.Cost,
			&startedAt, &finishedAt, &versionID); err != nil {
			return nil, err
		}
		if startedAt.Valid {
			item.Run.StartedAt = &startedAt.Time
		}
		if finishedAt.Valid {
			item.Run.FinishedAt = &finishedAt.Time
		}
		item.Run.Kind = domain.ExecutionRunWorkflow
		item.Plan.ID, item.Plan.WorkflowVersionID = item.Run.SchedulePlanID, versionID
		index, found := byVersion[versionID]
		if !found {
			continue // A concurrently added version belongs to the next refresh.
		}
		entry := &entries[index]
		if len(entry.RecentRuns) == 0 {
			observed := item.Run.FinishedAt
			if observed == nil {
				observed = item.Run.StartedAt
			}
			if observed != nil && !observed.Equal(time.Time{}) {
				entry.LatestExecutionAt = observed.UnixMilli()
			}
		}
		entry.RecentRuns = append(entry.RecentRuns, item)
	}
	return entries, recent.Err()
}
