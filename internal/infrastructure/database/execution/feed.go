package execution

// Filter and paginate the cheap run headers before touching task/transfer history.
// Materializing the bounded selection prevents SQLite from moving aggregation
// ahead of LIMIT. The count query uses only the header union.
func runFeedPageQuery(selection string) string {
	return `WITH selected_keys AS MATERIALIZED (` + selection + `),
	selected_runs AS MATERIALIZED (` + runFeedDetailsSelect + `),
	tasks AS (
		SELECT execution_run_id, COUNT(*) AS activity_count,
			SUM(CASE WHEN status='completed' THEN 1 ELSE 0 END) AS completed_count,
			SUM(runtime_seconds) AS compute_seconds, SUM(queue_seconds) AS queue_seconds,
			SUM(interference_seconds) AS interference_seconds, SUM(overhead_seconds) AS overhead_seconds
		FROM task_executions
		WHERE execution_run_id IN (SELECT id FROM selected_runs WHERE kind='workflow')
		GROUP BY execution_run_id
	), transfers AS (
		SELECT execution_run_id, SUM(duration_seconds) AS transfer_seconds, SUM(bytes) AS transferred_bytes
		FROM data_transfer_observations
		WHERE execution_run_id IN (SELECT id FROM selected_runs WHERE kind='workflow')
		GROUP BY execution_run_id
	)
	SELECT e.id, e.schedule_plan_id, e.mode, e.seed, e.status, e.environment_snapshot_id,
		e.makespan_seconds, e.cost, e.failure_reason,
		COALESCE(t.activity_count, e.activity_count), COALESCE(t.completed_count, e.completed_count),
		COALESCE(t.compute_seconds, 0), COALESCE(d.transfer_seconds, 0),
		COALESCE(t.queue_seconds, 0), COALESCE(t.interference_seconds, 0),
		COALESCE(t.overhead_seconds, 0), COALESCE(d.transferred_bytes, 0),
		e.kind, e.title, e.resource_id, e.runtime_id, e.connection_id, e.actor_id, e.command,
		e.started_at, e.finished_at, COALESCE(w.id, ''), COALESCE(v.id, ''),
		COALESCE(w.name, ''), COALESCE(r.name, ''),
		COALESCE((SELECT json_group_array(name) FROM (
			SELECT env.name FROM execution_scope_environments se
			JOIN environment_versions ev ON ev.id=se.environment_version_id
			JOIN environments env ON env.id=ev.environment_id
			WHERE se.execution_scope_id=p.execution_scope_id ORDER BY se.environment_version_id
		)), '[]')
	FROM selected_runs e
	LEFT JOIN tasks t ON t.execution_run_id=e.id AND e.kind='workflow'
	LEFT JOIN transfers d ON d.execution_run_id=e.id AND e.kind='workflow'
	LEFT JOIN schedule_plans p ON p.id=e.schedule_plan_id AND e.kind='workflow'
	LEFT JOIN workflow_versions v ON v.id=p.workflow_version_id
	LEFT JOIN workflow_definitions w ON w.id=v.workflow_id
	LEFT JOIN resources r ON r.id=e.resource_id
	ORDER BY e.started_at DESC, e.id DESC`
}
