package provenance

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/UFFeScience/akoflow/internal/application/ports"
)

type entitySpec struct {
	entity ports.ProvenanceEntity
	table  string
	order  string
}

type Repository struct {
	db    *sql.DB
	specs map[string]entitySpec
	order []string
}

func New(db *sql.DB) *Repository {
	specs, order := catalog()
	return &Repository{db: db, specs: specs, order: order}
}

func (r *Repository) Entities() []ports.ProvenanceEntity {
	items := make([]ports.ProvenanceEntity, 0, len(r.order))
	for _, name := range r.order {
		items = append(items, r.specs[name].entity)
	}
	return items
}

func (r *Repository) Query(ctx context.Context, query ports.ProvenanceQuery) (ports.ProvenancePage, error) {
	spec, ok := r.specs[query.Entity]
	if !ok {
		return ports.ProvenancePage{}, fmt.Errorf("unknown provenance entity %q", query.Entity)
	}
	page, pageSize := pagination(query.Page, query.PageSize)

	columns := make([]string, 0, len(spec.entity.Fields))
	searchParts := make([]string, 0, len(spec.entity.Fields))
	fieldExists := query.FilterField == ""
	for _, field := range spec.entity.Fields {
		columns = append(columns, field.Name)
		searchParts = append(searchParts, "CAST("+field.Name+" AS TEXT) LIKE ?")
		fieldExists = fieldExists || field.Name == query.FilterField
	}
	if !fieldExists {
		return ports.ProvenancePage{}, fmt.Errorf("unknown field %q for entity %q", query.FilterField, query.Entity)
	}

	where, args := "", []any{}
	clauses := []string{}
	if value := strings.TrimSpace(query.Search); value != "" {
		clauses = append(clauses, "("+strings.Join(searchParts, " OR ")+")")
		for range searchParts {
			args = append(args, "%"+value+"%")
		}
	}
	if query.FilterField != "" && strings.TrimSpace(query.FilterValue) != "" {
		clauses = append(clauses, query.FilterField+" = ?")
		args = append(args, query.FilterValue)
	}
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}

	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+spec.table+where, args...).Scan(&total); err != nil {
		return ports.ProvenancePage{}, fmt.Errorf("count provenance %s: %w", query.Entity, err)
	}
	order, err := validatedOrder(spec, query.SortField, query.SortOrder)
	if err != nil {
		return ports.ProvenancePage{}, err
	}
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+strings.Join(columns, ",")+" FROM "+spec.table+where+" ORDER BY "+order+" LIMIT ? OFFSET ?",
		append(args, pageSize, (page-1)*pageSize)...,
	)
	if err != nil {
		return ports.ProvenancePage{}, fmt.Errorf("query provenance %s: %w", query.Entity, err)
	}
	defer rows.Close()

	items, err := scanRows(rows, columns)
	if err != nil {
		return ports.ProvenancePage{}, fmt.Errorf("scan provenance %s: %w", query.Entity, err)
	}
	return ports.ProvenancePage{
		Entity: spec.entity, Items: items, Page: page, PageSize: pageSize,
		Total: total, HasNext: page*pageSize < total,
	}, nil
}

func validatedOrder(spec entitySpec, field, direction string) (string, error) {
	if strings.TrimSpace(field) == "" {
		return spec.order, nil
	}
	valid := false
	for _, candidate := range spec.entity.Fields {
		valid = valid || candidate.Name == field
	}
	if !valid {
		return "", fmt.Errorf("unknown sort field %q for entity %q", field, spec.entity.Name)
	}
	direction = strings.ToUpper(strings.TrimSpace(direction))
	if direction == "" {
		direction = "ASC"
	}
	if direction != "ASC" && direction != "DESC" {
		return "", fmt.Errorf("sort order must be asc or desc")
	}
	return field + " " + direction, nil
}

func pagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

func scanRows(rows *sql.Rows, columns []string) ([]map[string]any, error) {
	items := []map[string]any{}
	for rows.Next() {
		values, destinations := make([]any, len(columns)), make([]any, len(columns))
		for index := range values {
			destinations[index] = &values[index]
		}
		if err := rows.Scan(destinations...); err != nil {
			return nil, err
		}
		item := make(map[string]any, len(columns))
		for index, column := range columns {
			item[column] = normalizeValue(values[index])
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func normalizeValue(value any) any {
	if bytes, ok := value.([]byte); ok {
		return string(bytes)
	}
	return value
}

func fields(values ...string) []ports.ProvenanceField {
	result := make([]ports.ProvenanceField, 0, len(values)/3)
	for index := 0; index < len(values); index += 3 {
		result = append(result, ports.ProvenanceField{
			Name: values[index], Label: values[index+1], Type: values[index+2],
		})
	}
	return result
}

func catalog() (map[string]entitySpec, []string) {
	specs := entitySpecifications()
	result, order := make(map[string]entitySpec, len(specs)), make([]string, 0, len(specs))
	for _, spec := range specs {
		result[spec.entity.Name] = spec
		order = append(order, spec.entity.Name)
	}
	return result, order
}

func entitySpecifications() []entitySpec {
	return []entitySpec{
		environmentSpec(), resourceSpec(), executionScopeSpec(), workflowSpec(), workflowVersionSpec(), activitySpec(), planningSessionSpec(),
		planSpec(), runSpec(), taskExecutionSpec(), dataObjectSpec(), dataInstanceSpec(),
		transferSpec(), auditEventSpec(),
	}
}

func specification(name, label, description, table, order string, fieldList []ports.ProvenanceField) entitySpec {
	return entitySpec{
		entity: ports.ProvenanceEntity{Name: name, Label: label, Description: description, Fields: fieldList},
		table:  table, order: order,
	}
}

func environmentSpec() entitySpec {
	return specification("environments", "Environments", "Execution and simulation environments.",
		"environments", "created_at DESC", fields(
			"id", "ID", "identifier", "name", "Name", "text", "description", "Description", "text",
			"status", "Status", "status", "created_at", "Created", "datetime",
		))
}

func resourceSpec() entitySpec {
	spec := specification("resources", "Resources", "Schedulable compute resources used by plans and executions.",
		"resources", "name ASC", fields(
			"id", "ID", "identifier", "environment_version_id", "Environment version", "identifier",
			"parent_resource_id", "Parent resource", "identifier", "name", "Name", "text", "type", "Type", "status",
			"cpu_cores", "CPU cores", "number", "memory_bytes", "Memory", "bytes", "schedulable", "Schedulable", "boolean",
		))
	spec.entity.Links = []ports.ProvenanceRelationship{relationship("parent_resource_id", "resources")}
	return spec
}

func executionScopeSpec() entitySpec {
	return specification("execution_scopes", "Execution scopes", "Infrastructure universes evaluated by scheduling algorithms.",
		"execution_scopes", "name ASC", fields(
			"id", "ID", "identifier", "name", "Name", "text", "network_topology_id", "Network topology", "identifier",
		))
}

func relationship(field, target string) ports.ProvenanceRelationship {
	return ports.ProvenanceRelationship{Field: field, TargetEntity: target, TargetField: "id"}
}

func workflowSpec() entitySpec {
	return specification("workflows", "Workflows", "Workflow definitions and their creation history.",
		"workflow_definitions", "created_at DESC", fields(
			"id", "ID", "identifier", "external_id", "External ID", "text",
			"name", "Name", "text", "namespace", "Namespace", "text", "created_at", "Created", "datetime",
		))
}

func workflowVersionSpec() entitySpec {
	spec := specification("workflow_versions", "Workflow versions", "Immutable workflow revisions referenced by plans and executions.",
		"workflow_versions", "created_at DESC", fields(
			"id", "ID", "identifier", "workflow_id", "Workflow", "identifier", "version", "Version", "number",
			"definition_hash", "Definition hash", "text", "status", "Status", "status", "created_at", "Created", "datetime",
		))
	spec.entity.Links = []ports.ProvenanceRelationship{relationship("workflow_id", "workflows")}
	return spec
}

func activitySpec() entitySpec {
	spec := specification("activities", "Activities", "Versioned workflow activities used to create plans and runs.",
		"activity_definitions", "workflow_version_id, external_id", fields(
			"id", "ID", "identifier", "workflow_version_id", "Workflow version", "identifier",
			"external_id", "External ID", "text", "name", "Name", "text",
			"kind", "Kind", "status", "priority", "Priority", "number",
		))
	spec.entity.Links = []ports.ProvenanceRelationship{relationship("workflow_version_id", "workflow_versions")}
	return spec
}

func planningSessionSpec() entitySpec {
	spec := specification("planning_sessions", "Planning sessions", "Scheduling experiments and their selected plans.",
		"planning_sessions", "created_at DESC", fields(
			"id", "ID", "identifier", "workflow_version_id", "Workflow version", "identifier",
			"execution_scope_id", "Execution scope", "identifier", "status", "Status", "status",
			"progress", "Progress", "number", "candidate_count", "Candidates", "number",
			"selected_plan_id", "Selected plan", "identifier", "created_at", "Created", "datetime",
			"completed_at", "Completed", "datetime",
		))
	spec.entity.Links = []ports.ProvenanceRelationship{relationship("selected_plan_id", "plans")}
	spec.entity.Links = append(spec.entity.Links,
		relationship("workflow_version_id", "workflow_versions"), relationship("execution_scope_id", "execution_scopes"))
	return spec
}

func planSpec() entitySpec {
	spec := specification("plans", "Execution plans", "Generated or manual plans with predicted outcomes.",
		"schedule_plans", "created_at DESC", fields(
			"id", "ID", "identifier", "workflow_version_id", "Workflow version", "identifier",
			"execution_scope_id", "Execution scope", "identifier", "source", "Source", "status",
			"algorithm", "Algorithm", "text", "objective", "Objective", "text", "status", "Status", "status",
			"predicted_makespan_seconds", "Predicted makespan", "duration", "predicted_cost", "Predicted cost", "currency",
			"predicted_feasible", "Feasible", "boolean", "created_at", "Created", "datetime",
		))
	spec.entity.Links = []ports.ProvenanceRelationship{
		relationship("workflow_version_id", "workflow_versions"), relationship("execution_scope_id", "execution_scopes"),
	}
	return spec
}

func runSpec() entitySpec {
	spec := specification("runs", "Runs", "Workflow executions with observed makespan and cost.",
		"execution_runs", "created_at DESC", fields(
			"id", "ID", "identifier", "schedule_plan_id", "Plan", "identifier", "mode", "Mode", "status",
			"status", "Status", "status", "makespan_seconds", "Observed makespan", "duration",
			"cost", "Observed cost", "currency", "started_at", "Started", "datetime",
			"finished_at", "Finished", "datetime", "created_at", "Created", "datetime",
		))
	spec.entity.Links = []ports.ProvenanceRelationship{relationship("schedule_plan_id", "plans")}
	return spec
}

func taskExecutionSpec() entitySpec {
	spec := specification("task_executions", "Activity executions", "Observed runtime, queue, transfer and interference for each activity attempt.",
		"task_executions", "execution_run_id, started_at", fields(
			"id", "ID", "identifier", "execution_run_id", "Run", "identifier", "activity_id", "Activity", "identifier",
			"planned_resource_id", "Planned resource", "identifier", "allocated_resource_id", "Allocated resource", "identifier",
			"attempt", "Attempt", "number", "status", "Status", "status", "runtime_seconds", "Runtime", "duration",
			"queue_seconds", "Queue", "duration", "transfer_seconds", "Transfer", "duration",
			"transfer_bytes", "Transferred bytes", "bytes", "interference_seconds", "Interference", "duration",
			"overhead_seconds", "Overhead", "duration", "cost", "Cost", "currency",
		))
	spec.entity.Links = []ports.ProvenanceRelationship{
		relationship("execution_run_id", "runs"), relationship("activity_id", "activities"),
		relationship("planned_resource_id", "resources"), relationship("allocated_resource_id", "resources"),
	}
	return spec
}

func dataObjectSpec() entitySpec {
	spec := specification("data_objects", "Data objects", "Logical inputs and outputs declared or discovered by workflows.",
		"data_objects", "logical_name", fields(
			"id", "ID", "identifier", "workflow_version_id", "Workflow version", "identifier",
			"producer_activity_id", "Producer activity", "identifier", "logical_name", "Logical name", "text",
			"relative_path", "Relative path", "text", "declared", "Declared", "boolean",
		))
	spec.entity.Links = []ports.ProvenanceRelationship{
		relationship("workflow_version_id", "workflow_versions"), relationship("producer_activity_id", "activities"),
	}
	return spec
}

func dataInstanceSpec() entitySpec {
	spec := specification("data_instances", "Data instances", "Concrete produced files with size and checksum evidence.",
		"data_object_instances", "created_at DESC", fields(
			"id", "ID", "identifier", "data_object_id", "Data object", "identifier",
			"execution_run_id", "Run", "identifier", "producer_activity_id", "Producer activity", "identifier",
			"relative_path", "Relative path", "text", "size_bytes", "Size", "bytes", "checksum", "Checksum", "text",
			"media_type", "Media type", "text", "discovered", "Discovered", "boolean", "created_at", "Created", "datetime",
		))
	spec.entity.Links = []ports.ProvenanceRelationship{
		relationship("data_object_id", "data_objects"), relationship("execution_run_id", "runs"),
		relationship("producer_activity_id", "activities"),
	}
	return spec
}

func transferSpec() entitySpec {
	spec := specification("transfers", "Data transfers", "Observed data movement between resources during a run.",
		"data_transfer_observations", "execution_run_id, started_at", fields(
			"id", "ID", "identifier", "execution_run_id", "Run", "identifier", "data_object_id", "Data object", "identifier",
			"data_object_instance_id", "Data instance", "identifier",
			"producer_activity_id", "Producer", "identifier", "consumer_activity_id", "Consumer", "identifier",
			"source_resource_id", "Source resource", "identifier", "target_resource_id", "Target resource", "identifier",
			"bytes", "Bytes", "bytes", "status", "Status", "status",
			"duration_seconds", "Duration", "duration", "cost", "Cost", "currency",
		))
	spec.entity.Links = []ports.ProvenanceRelationship{
		relationship("execution_run_id", "runs"), relationship("data_object_id", "data_objects"),
		relationship("data_object_instance_id", "data_instances"), relationship("producer_activity_id", "activities"),
		relationship("consumer_activity_id", "activities"), relationship("source_resource_id", "resources"),
		relationship("target_resource_id", "resources"),
	}
	return spec
}

func auditEventSpec() entitySpec {
	return specification("audit_events", "Audit events", "Chronological operational evidence recorded by the control plane.",
		"audit_events", "occurred_at DESC", fields(
			"id", "ID", "identifier", "event_type", "Event", "text", "environment_id", "Environment", "identifier",
			"resource_id", "Resource", "identifier", "execution_id", "Execution", "identifier",
			"outcome", "Outcome", "status", "summary", "Summary", "text", "occurred_at", "Occurred", "datetime",
		))
}
