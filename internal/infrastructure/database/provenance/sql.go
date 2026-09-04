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

var safeSQLTables = map[string]bool{
	"environments": true, "environment_versions": true, "resources": true,
	"resource_snapshots": true, "resource_relations": true, "execution_scopes": true,
	"execution_scope_environments": true, "network_topologies": true, "network_links": true,
	"workflow_definitions": true, "workflow_versions": true, "activity_types": true,
	"activity_definitions": true, "activity_dependencies": true, "workflow_data_dependencies": true,
	"activity_resource_profiles": true, "schedule_plans": true, "schedule_plan_assignments": true,
	"planning_sessions": true, "planning_algorithm_runs": true, "planning_candidates": true,
	"execution_runs": true, "task_executions": true, "activity_lifecycle_events": true,
	"data_objects": true, "data_object_instances": true, "data_locations": true,
	"data_transfer_observations": true, "audit_events": true,
}

var blockedSQLColumns = map[string]bool{
	"configuration": true, "metadata": true, "raw_definition": true,
	"credential_ref": true, "credential_reference": true, "payload": true,
}

func (r *Repository) Schema(ctx context.Context) ([]ports.ProvenanceSQLTable, error) {
	tables := make([]ports.ProvenanceSQLTable, 0, len(safeSQLTables))
	for table := range safeSQLTables {
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
			if !blockedSQLColumns[strings.ToLower(name)] {
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
				if safeSQLTables[argument1] && !blockedSQLColumns[strings.ToLower(argument2)] {
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
