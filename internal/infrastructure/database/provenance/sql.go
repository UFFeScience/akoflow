package provenance

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/mattn/go-sqlite3"
)

const sqlQueryTimeout = 10 * time.Second
const sqliteRecursive = 33

var safeSQLSchema = map[string]map[string]bool{
	"environments":                     allowedSQLColumns("id", "name", "description", "status", "created_at"),
	"environment_versions":             allowedSQLColumns("id", "environment_id", "version", "status", "network_model", "interference_model", "cost_model", "configuration_hash", "created_at", "published_at"),
	"environment_runtimes":             allowedSQLColumns("id", "environment_version_id", "name", "driver", "mode", "role"),
	"environment_runtime_capabilities": allowedSQLColumns("runtime_id", "capabilities"),
	"discovery_runs":                   allowedSQLColumns("id", "environment_version_id", "provider", "status", "resources_found", "error", "started_at", "finished_at"),
	"environment_connection_checks":    allowedSQLColumns("id", "connection_id", "status", "latency_ms", "checked_at"),
	"resources":                        allowedSQLColumns("id", "environment_version_id", "execution_target", "parent_resource_id", "type", "name", "provider_id", "tier", "region", "zone", "architecture", "cpu_cores", "cpu_capacity", "memory_bytes", "storage_bytes", "compute_speedup", "price_per_second", "boot_overhead_seconds", "container_overhead_seconds", "schedulable"),
	"resource_snapshots":               allowedSQLColumns("id", "resource_id", "captured_at", "available", "cpu_used", "memory_used_bytes", "network_in_bps", "network_out_bps", "disk_read_bps", "disk_write_bps", "queue_length"),
	"resource_relations":               allowedSQLColumns("environment_version_id", "source_resource_id", "target_resource_id", "relation_type"),
	"execution_scopes":                 allowedSQLColumns("id", "name", "network_topology_id"),
	"execution_scope_environments":     allowedSQLColumns("execution_scope_id", "environment_version_id"),
	"network_topologies":               allowedSQLColumns("id", "name", "version", "execution_scope_id", "created_at"),
	"network_links":                    allowedSQLColumns("id", "topology_id", "source_resource_id", "target_resource_id", "bandwidth_bits_per_second", "latency_seconds", "price_per_byte", "bidirectional", "sharing_policy", "max_concurrent_transfers"),
	"workflow_definitions":             allowedSQLColumns("id", "external_id", "name", "namespace", "created_at"),
	"workflow_versions":                allowedSQLColumns("id", "workflow_id", "version", "definition_hash", "status", "created_at"),
	"activity_types":                   allowedSQLColumns("id", "name", "application", "default_image", "cpu_intensity", "memory_intensity", "io_intensity", "network_intensity"),
	"activity_definitions":             allowedSQLColumns("id", "workflow_version_id", "activity_type_id", "external_id", "name", "kind", "capabilities", "command_spec", "resource_requirements", "service_spec", "simulation_spec", "policy", "priority"),
	"activity_dependencies":            allowedSQLColumns("activity_id", "depends_on_activity_id", "dependency_type"),
	"workflow_data_dependencies":       allowedSQLColumns("producer_activity_id", "consumer_activity_id", "logical_name", "size_bytes"),
	"activity_resource_profiles":       allowedSQLColumns("id", "activity_type_id", "resource_id", "runtime_seconds", "runtime_stddev_seconds", "cpu_utilization", "peak_memory_bytes", "disk_read_bytes", "disk_write_bytes", "energy_joules", "source", "sample_size", "model_version"),
	"schedule_plans":                   allowedSQLColumns("id", "workflow_version_id", "execution_scope_id", "network_topology_id", "source", "algorithm", "algorithm_version", "objective", "status", "deadline_seconds", "budget", "predicted_makespan_seconds", "predicted_cost", "predicted_feasible", "created_at"),
	"schedule_plan_assignments":        allowedSQLColumns("id", "schedule_plan_id", "activity_id", "resource_id", "core_id", "slot_id", "order_on_resource", "priority", "predicted_ready_at", "predicted_start_at", "predicted_finish_at", "predicted_runtime_seconds", "predicted_transfer_seconds", "predicted_cost"),
	"planning_sessions":                allowedSQLColumns("id", "workflow_version_id", "execution_scope_id", "network_topology_id", "status", "algorithms", "progress", "candidate_count", "selected_candidate_id", "selected_plan_id", "deadline_seconds", "budget", "failure_reason", "created_at", "started_at", "completed_at"),
	"planning_algorithm_runs":          allowedSQLColumns("id", "planning_session_id", "algorithm", "objective", "status", "progress", "candidate_count", "failure_reason", "started_at", "completed_at"),
	"planning_candidates":              allowedSQLColumns("id", "planning_session_id", "algorithm_run_id", "algorithm", "objective", "rank", "pareto_optimal", "dominated", "feasible", "predicted_makespan_seconds", "predicted_cost", "plan", "fingerprint", "created_at"),
	"execution_runs":                   allowedSQLColumns("id", "schedule_plan_id", "mode", "seed", "status", "environment_snapshot_id", "started_at", "finished_at", "makespan_seconds", "cost", "failure_reason", "created_at"),
	"task_executions":                  allowedSQLColumns("id", "execution_run_id", "plan_assignment_id", "activity_id", "planned_resource_id", "allocated_resource_id", "attempt", "status", "ready_at", "data_ready_at", "queued_at", "started_at", "finished_at", "runtime_seconds", "queue_seconds", "transfer_seconds", "transfer_bytes", "interference_seconds", "overhead_seconds", "cost", "failure_reason"),
	"activity_lifecycle_events":        allowedSQLColumns("id", "task_execution_id", "phase", "status", "started_at", "finished_at", "duration_seconds", "source", "error"),
	"activity_metric_samples":          allowedSQLColumns("execution_run_id", "activity_id", "attempt", "observed_at", "cpu_seconds", "memory_bytes", "read_bytes", "write_bytes", "source", "scope"),
	"data_objects":                     allowedSQLColumns("id", "workflow_version_id", "producer_activity_id", "logical_name", "relative_path", "declared"),
	"data_object_instances":            allowedSQLColumns("id", "data_object_id", "execution_run_id", "producer_activity_id", "attempt", "relative_path", "size_bytes", "checksum", "media_type", "discovered", "created_at"),
	"data_locations":                   allowedSQLColumns("id", "data_object_instance_id", "storage_resource_id", "resource_id", "execution_run_id", "uri", "available_at", "verified_at", "status"),
	"data_transfer_observations":       allowedSQLColumns("id", "execution_run_id", "data_object_id", "data_object_instance_id", "producer_activity_id", "consumer_activity_id", "source_resource_id", "target_resource_id", "bytes", "status", "started_at", "finished_at", "duration_seconds", "cost"),
	"storage_resources":                allowedSQLColumns("id", "environment_version_id", "name", "type", "capacity_bytes", "shared", "read_only"),
	"storage_runtime_bindings":         allowedSQLColumns("storage_resource_id", "environment_version_id", "runtime_id", "is_default", "container_path", "read_only"),
	"transfer_runs":                    allowedSQLColumns("id", "plan_id", "execution_run_id", "activity_id", "strategy", "status", "started_at", "finished_at", "transferred_bytes", "logical_bytes", "network_bytes", "error", "created_at"),
	"transfer_chunk_runs":              allowedSQLColumns("transfer_run_id", "chunk_index", "offset_bytes", "size_bytes", "digest", "status", "attempts", "updated_at"),
	"activity_workspaces":              allowedSQLColumns("id", "execution_run_id", "activity_id", "environment_id", "resource_id", "runtime_id", "driver", "state", "retention", "is_final", "pinned", "file_count", "size_bytes", "input_bytes", "output_bytes", "reclaimed_bytes", "release_reason", "last_error", "created_at", "sealed_at", "released_at"),
	"workspace_leases":                 allowedSQLColumns("id", "workspace_id", "execution_run_id", "producer_activity_id", "consumer_activity_id", "released", "released_at", "release_reason"),
	"cloud_operation_runs":             allowedSQLColumns("id", "kind", "status", "environment_id", "capacity_target_id", "instance_id", "execution_run_id", "activity_id", "phase", "failure_reason", "created_at", "started_at", "finished_at"),
	"cloud_operation_events":           allowedSQLColumns("operation_id", "sequence", "timestamp", "tool", "phase", "level", "event", "duration_seconds"),
	"cloud_provisioned_instances":      allowedSQLColumns("id", "capacity_target_id", "environment_id", "provider", "provider_id", "name", "status", "failure_reason", "created_at", "ready_at", "destroyed_at"),
	"planned_lifecycle_actions":        allowedSQLColumns("id", "schedule_plan_id", "capacity_target_id", "cloud_instance_id", "action", "earliest_start", "expected_duration", "depends_on"),
	"artifact_versions":                allowedSQLColumns("id", "artifact_id", "name", "version", "scope", "scope_id"),
	"artifact_variants":                allowedSQLColumns("id", "artifact_version_id", "digest", "format", "architecture", "size_bytes"),
	"artifact_locations":               allowedSQLColumns("id", "variant_id", "endpoint_id", "uri", "digest", "scope", "scope_id", "available"),
	"artifact_materializations":        allowedSQLColumns("id", "run_id", "activity_id", "variant_id", "digest", "resource_id", "environment_version_id", "destination_path", "status", "verified_digest", "updated_at"),
	"artifact_builds":                  allowedSQLColumns("id", "artifact_version_id", "source_type", "context_digest", "recipe_path", "recipe_digest", "target_format", "target_os", "target_architecture", "cache_key", "created_at"),
	"build_runs":                       allowedSQLColumns("id", "artifact_build_id", "status", "output_variant_id", "output_digest", "started_at", "finished_at", "created_at"),
	"simulation_engines":               allowedSQLColumns("id", "name", "driver", "enabled"),
	"simulation_scenarios":             allowedSQLColumns("id", "name", "environment_version_id", "environment_snapshot_id", "engine_id", "seed", "network_overrides", "interference_model", "cost_model", "data_scale", "created_at"),
	"simulation_runs":                  allowedSQLColumns("id", "scenario_id", "execution_run_id", "created_at"),
	"audit_events":                     allowedSQLColumns("id", "event_type", "actor_id", "actor_type", "environment_id", "resource_id", "connection_id", "runtime_id", "session_id", "execution_id", "external_id", "outcome", "summary", "occurred_at"),
}

func allowedSQLColumns(names ...string) map[string]bool {
	columns := make(map[string]bool, len(names))
	for _, name := range names {
		columns[name] = true
	}
	return columns
}

func (r *Repository) Schema(ctx context.Context) ([]ports.ProvenanceSQLTable, error) {
	tables := make([]ports.ProvenanceSQLTable, 0, len(safeSQLSchema))
	for table, allowedColumns := range safeSQLSchema {
		rows, err := r.db.QueryContext(ctx, "PRAGMA table_info("+quotedIdentifier(table)+")")
		if err != nil {
			return nil, fmt.Errorf("inspect provenance table %s: %w", table, err)
		}
		columns := []ports.ProvenanceSQLColumn{}
		for rows.Next() {
			var index int
			var name, dataType string
			var notNull, primaryKey int
			var defaultValue any
			if err := rows.Scan(&index, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
				rows.Close()
				return nil, err
			}
			if allowedColumns[strings.ToLower(name)] {
				columns = append(columns, ports.ProvenanceSQLColumn{Name: name, Type: strings.ToLower(dataType)})
			}
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		if len(columns) > 0 {
			tables = append(tables, ports.ProvenanceSQLTable{Name: table, Columns: columns})
		}
	}
	slices.SortFunc(tables, func(left, right ports.ProvenanceSQLTable) int { return strings.Compare(left.Name, right.Name) })
	return tables, nil
}

func (r *Repository) SQL(ctx context.Context, query ports.ProvenanceSQLQuery) (ports.ProvenanceSQLResult, error) {
	statement, err := selectStatement(query.SQL)
	if err != nil {
		return ports.ProvenanceSQLResult{}, err
	}
	page, pageSize := pagination(query.Page, query.PageSize)
	wrapped := "SELECT * FROM (" + statement + ") LIMIT ? OFFSET ?"
	args := namedArguments(query.Parameters)
	args = append(args, pageSize+1, (page-1)*pageSize)
	return r.executeSQL(ctx, wrapped, args, page, pageSize)
}

func (r *Repository) Explain(ctx context.Context, query ports.ProvenanceSQLQuery) (ports.ProvenanceSQLResult, error) {
	statement, err := selectStatement(query.SQL)
	if err != nil {
		return ports.ProvenanceSQLResult{}, err
	}
	return r.executeSQL(ctx, "EXPLAIN QUERY PLAN "+statement, namedArguments(query.Parameters), 1, 200)
}

func (r *Repository) executeSQL(
	ctx context.Context,
	statement string,
	args []any,
	page int,
	pageSize int,
) (ports.ProvenanceSQLResult, error) {
	started := time.Now()
	queryCtx, cancel := context.WithTimeout(ctx, sqlQueryTimeout)
	defer cancel()
	connection, err := r.db.Conn(queryCtx)
	if err != nil {
		return ports.ProvenanceSQLResult{}, fmt.Errorf("open provenance connection: %w", err)
	}
	defer connection.Close()
	if err := configureReadAuthorizer(connection); err != nil {
		return ports.ProvenanceSQLResult{}, err
	}
	defer func() { _ = clearReadAuthorizer(connection) }()
	rows, err := connection.QueryContext(queryCtx, statement, args...)
	if err != nil {
		return ports.ProvenanceSQLResult{}, fmt.Errorf("execute read-only provenance query: %w", err)
	}
	defer rows.Close()
	columns, err := sqlColumns(rows)
	if err != nil {
		return ports.ProvenanceSQLResult{}, err
	}
	names := make([]string, len(columns))
	for index := range columns {
		names[index] = columns[index].Name
	}
	items, err := scanRows(rows, names)
	if err != nil {
		return ports.ProvenanceSQLResult{}, fmt.Errorf("scan provenance query: %w", err)
	}
	truncated := len(items) > pageSize
	if truncated {
		items = items[:pageSize]
	}
	return ports.ProvenanceSQLResult{
		Columns: columns, Items: items, Page: page, PageSize: pageSize,
		Truncated: truncated, ElapsedMilliseconds: time.Since(started).Milliseconds(),
	}, nil
}

func configureReadAuthorizer(connection *sql.Conn) error {
	return connection.Raw(func(driverConnection any) error {
		sqliteConnection, ok := driverConnection.(*sqlite3.SQLiteConn)
		if !ok {
			return fmt.Errorf("provenance database is not SQLite")
		}
		sqliteConnection.RegisterAuthorizer(func(operation int, argument1, argument2, _ string) int {
			switch operation {
			case sqlite3.SQLITE_SELECT, sqliteRecursive:
				return sqlite3.SQLITE_OK
			case sqlite3.SQLITE_READ:
				allowedColumns, tableAllowed := safeSQLSchema[argument1]
				if tableAllowed && (argument2 == "" || allowedColumns[strings.ToLower(argument2)]) {
					return sqlite3.SQLITE_OK
				}
				return sqlite3.SQLITE_DENY
			case sqlite3.SQLITE_FUNCTION:
				if strings.EqualFold(argument2, "load_extension") || strings.EqualFold(argument2, "readfile") || strings.EqualFold(argument2, "writefile") {
					return sqlite3.SQLITE_DENY
				}
				return sqlite3.SQLITE_OK
			default:
				return sqlite3.SQLITE_DENY
			}
		})
		return nil
	})
}

// clearReadAuthorizer prevents a per-query SQLite authorizer from leaking into
// a pooled connection. Schema discovery uses PRAGMA and must not inherit the
// read-only query policy after the connection is returned to database/sql.
func clearReadAuthorizer(connection *sql.Conn) error {
	return connection.Raw(func(driverConnection any) error {
		sqliteConnection, ok := driverConnection.(*sqlite3.SQLiteConn)
		if !ok {
			return fmt.Errorf("provenance database is not SQLite")
		}
		sqliteConnection.RegisterAuthorizer(nil)
		return nil
	})
}

func quotedIdentifier(value string) string { return `"` + strings.ReplaceAll(value, `"`, `""`) + `"` }

func selectStatement(value string) (string, error) {
	statement := strings.TrimSpace(value)
	statement = strings.TrimSpace(strings.TrimSuffix(statement, ";"))
	if statement == "" {
		return "", fmt.Errorf("SQL query is required")
	}
	parts := strings.FieldsFunc(statement, func(character rune) bool {
		return unicode.IsSpace(character) || character == '('
	})
	if len(parts) == 0 {
		return "", fmt.Errorf("SQL query is required")
	}
	first := strings.ToUpper(parts[0])
	if first != "SELECT" && first != "WITH" {
		return "", fmt.Errorf("only SELECT and WITH queries are allowed")
	}
	return statement, nil
}

func namedArguments(parameters map[string]any) []any {
	args := make([]any, 0, len(parameters))
	for name, value := range parameters {
		args = append(args, sql.Named(name, value))
	}
	return args
}

func sqlColumns(rows *sql.Rows) ([]ports.ProvenanceSQLColumn, error) {
	types, err := rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("inspect provenance columns: %w", err)
	}
	columns := make([]ports.ProvenanceSQLColumn, 0, len(types))
	for _, column := range types {
		columns = append(columns, ports.ProvenanceSQLColumn{
			Name: column.Name(), Type: strings.ToLower(column.DatabaseTypeName()),
		})
	}
	return columns, nil
}
