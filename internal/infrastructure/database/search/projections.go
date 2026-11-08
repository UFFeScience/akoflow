package search

// Projections read metadata only, never logs, definitions, credentials or runtime payloads.
var projections = map[string]string{
	"workflow": `SELECT 'workflow' AS type, w.id, w.name AS title, w.namespace AS subtitle,
		COALESCE(v.status,'') AS status, CAST(COALESCE(v.version,'') AS TEXT) AS version,
		w.id AS route_id, '' AS owner_id, '' AS owner_title, '' AS owner_type, w.external_id AS alternate_id
		FROM workflow_definitions w LEFT JOIN workflow_versions v ON v.workflow_id=w.id
		AND v.version=(SELECT MAX(latest.version) FROM workflow_versions latest WHERE latest.workflow_id=w.id)`,
	"execution": `SELECT 'execution' AS type, e.id, e.id AS title, e.mode AS subtitle,
		e.status, CAST(COALESCE(v.version,'') AS TEXT) AS version, e.schedule_plan_id AS route_id,
		COALESCE(w.id,'') AS owner_id, COALESCE(w.name,'') AS owner_title, 'workflow' AS owner_type, '' AS alternate_id
		FROM execution_runs e LEFT JOIN schedule_plans p ON p.id=e.schedule_plan_id
		LEFT JOIN workflow_versions v ON v.id=p.workflow_version_id
		LEFT JOIN workflow_definitions w ON w.id=v.workflow_id
		UNION ALL SELECT 'execution', s.id, 'Interactive session', '', s.status, '', s.id,
		COALESCE(r.id,''), COALESCE(r.name,''), 'resource', '' FROM console_sessions s LEFT JOIN resources r ON r.id=s.resource_id
		UNION ALL SELECT 'execution', c.id, 'Standalone command', '', c.status, '', c.id,
		COALESCE(r.id,''), COALESCE(r.name,''), 'resource', '' FROM console_commands c LEFT JOIN resources r ON r.id=c.resource_id`,
	"environment": `SELECT 'environment' AS type, id, name AS title, '' AS subtitle, status, '' AS version,
		id AS route_id, '' AS owner_id, '' AS owner_title, '' AS owner_type, '' AS alternate_id FROM environments`,
	"artifact": `SELECT 'artifact' AS type, a.id, a.name AS title, a.scope AS subtitle, '' AS status, a.version,
		a.artifact_id AS route_id, COALESCE(w.id,'') AS owner_id, COALESCE(w.name,'') AS owner_title,
		'workflow' AS owner_type, a.artifact_id AS alternate_id FROM artifact_versions a
		LEFT JOIN workflow_versions v ON a.scope='workflow' AND v.id=a.scope_id
		LEFT JOIN workflow_definitions w ON a.scope='workflow' AND w.id=COALESCE(v.workflow_id,
		(SELECT id FROM workflow_definitions WHERE id=a.scope_id),
		(SELECT id FROM workflow_definitions WHERE external_id=a.scope_id))`,
	"resource": `SELECT 'resource' AS type, r.id, r.name AS title, r.type AS subtitle, '' AS status,
		CAST(v.version AS TEXT) AS version, r.id AS route_id, e.id AS owner_id, e.name AS owner_title,
		'environment' AS owner_type, r.provider_id AS alternate_id FROM resources r
		JOIN environment_versions v ON v.id=r.environment_version_id JOIN environments e ON e.id=v.environment_id`,
	"plan": `SELECT 'plan' AS type, p.id, p.algorithm AS title, p.objective AS subtitle, p.status,
		CAST(v.version AS TEXT) AS version, p.id AS route_id, w.id AS owner_id, w.name AS owner_title,
		'workflow' AS owner_type, '' AS alternate_id FROM schedule_plans p
		JOIN workflow_versions v ON v.id=p.workflow_version_id JOIN workflow_definitions w ON w.id=v.workflow_id`,
	"scope": `SELECT 'scope' AS type, id, name AS title, '' AS subtitle, '' AS status, '' AS version,
		id AS route_id, '' AS owner_id, '' AS owner_title, '' AS owner_type, '' AS alternate_id FROM execution_scopes`,
	"materialization": `SELECT 'materialization' AS type, m.id, a.name AS title, '' AS subtitle, m.status, a.version,
		a.artifact_id AS route_id, a.artifact_id AS owner_id, a.name AS owner_title,
		'artifact' AS owner_type, '' AS alternate_id FROM artifact_materializations m
		JOIN artifact_variants v ON v.id=m.variant_id JOIN artifact_versions a ON a.id=v.artifact_version_id`,
}
