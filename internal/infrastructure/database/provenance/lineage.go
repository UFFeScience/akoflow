package provenance

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/UFFeScience/akoflow/internal/application/ports"
)

type lineageVisit struct {
	entity string
	id     string
	depth  int
}

func (r *Repository) Lineage(ctx context.Context, query ports.ProvenanceLineageQuery) (ports.ProvenanceLineage, error) {
	if _, ok := r.specs[query.Entity]; !ok {
		return ports.ProvenanceLineage{}, fmt.Errorf("unknown provenance entity %q", query.Entity)
	}
	query.ID = strings.TrimSpace(query.ID)
	if query.ID == "" {
		return ports.ProvenanceLineage{}, fmt.Errorf("lineage record ID is required")
	}
	direction, depth, maxNodes, err := lineageOptions(query)
	if err != nil {
		return ports.ProvenanceLineage{}, err
	}
	root := lineageKey(query.Entity, query.ID)
	result := ports.ProvenanceLineage{Root: root, Nodes: []ports.ProvenanceLineageNode{}, Edges: []ports.ProvenanceLineageEdge{}}
	queue := []lineageVisit{{entity: query.Entity, id: query.ID}}
	seen := map[string]bool{}
	edges := map[string]bool{}
	for len(queue) > 0 && len(result.Nodes) < maxNodes {
		visit := queue[0]
		queue = queue[1:]
		key := lineageKey(visit.entity, visit.id)
		if seen[key] {
			continue
		}
		record, err := r.recordByID(ctx, visit.entity, visit.id)
		if err != nil {
			return ports.ProvenanceLineage{}, err
		}
		if record == nil {
			if visit.depth == 0 {
				return ports.ProvenanceLineage{}, fmt.Errorf("provenance record %q was not found", key)
			}
			continue
		}
		seen[key] = true
		result.Nodes = append(result.Nodes, lineageNode(visit.entity, visit.id, record))
		if visit.depth >= depth {
			continue
		}
		if direction != "downstream" {
			r.expandUpstream(record, visit, &queue, &result, edges)
		}
		if direction != "upstream" {
			if err := r.expandDownstream(ctx, visit, maxNodes-len(result.Nodes), &queue, &result, edges); err != nil {
				return ports.ProvenanceLineage{}, err
			}
		}
	}
	result.Truncated = len(queue) > 0
	return result, nil
}

func lineageOptions(query ports.ProvenanceLineageQuery) (string, int, int, error) {
	direction := strings.ToLower(strings.TrimSpace(query.Direction))
	if direction == "" {
		direction = "both"
	}
	if direction != "upstream" && direction != "downstream" && direction != "both" {
		return "", 0, 0, fmt.Errorf("lineage direction must be upstream, downstream or both")
	}
	depth := query.Depth
	if depth < 1 {
		depth = 2
	}
	if depth > 5 {
		depth = 5
	}
	maxNodes := query.MaxNodes
	if maxNodes < 1 {
		maxNodes = 200
	}
	if maxNodes > 500 {
		maxNodes = 500
	}
	return direction, depth, maxNodes, nil
}

func (r *Repository) expandUpstream(
	record map[string]any,
	visit lineageVisit,
	queue *[]lineageVisit,
	result *ports.ProvenanceLineage,
	edges map[string]bool,
) {
	for _, link := range r.specs[visit.entity].entity.Links {
		id := strings.TrimSpace(fmt.Sprint(record[link.Field]))
		if id == "" || id == "<nil>" {
			continue
		}
		parent := lineageKey(link.TargetEntity, id)
		appendLineageEdge(result, edges, parent, lineageKey(visit.entity, visit.id), link.Field)
		*queue = append(*queue, lineageVisit{entity: link.TargetEntity, id: id, depth: visit.depth + 1})
	}
}

func (r *Repository) expandDownstream(
	ctx context.Context,
	visit lineageVisit,
	limit int,
	queue *[]lineageVisit,
	result *ports.ProvenanceLineage,
	edges map[string]bool,
) error {
	if limit < 1 {
		return nil
	}
	for _, entityName := range r.order {
		spec := r.specs[entityName]
		for _, link := range spec.entity.Links {
			if link.TargetEntity != visit.entity {
				continue
			}
			ids, err := r.relatedIDs(ctx, spec, link.Field, visit.id, limit)
			if err != nil {
				return err
			}
			for _, id := range ids {
				child := lineageKey(entityName, id)
				appendLineageEdge(result, edges, lineageKey(visit.entity, visit.id), child, link.Field)
				*queue = append(*queue, lineageVisit{entity: entityName, id: id, depth: visit.depth + 1})
			}
		}
	}
	return nil
}

func (r *Repository) recordByID(ctx context.Context, entity, id string) (map[string]any, error) {
	spec := r.specs[entity]
	columns := fieldNames(spec.entity.Fields)
	row := r.db.QueryRowContext(ctx, "SELECT "+strings.Join(columns, ",")+" FROM "+spec.table+" WHERE id = ?", id)
	values, destinations := make([]any, len(columns)), make([]any, len(columns))
	for index := range values {
		destinations[index] = &values[index]
	}
	if err := row.Scan(destinations...); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("read provenance %s %q: %w", entity, id, err)
	}
	record := make(map[string]any, len(columns))
	for index, column := range columns {
		record[column] = normalizeValue(values[index])
	}
	return record, nil
}

func (r *Repository) relatedIDs(ctx context.Context, spec entitySpec, field, id string, limit int) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id FROM "+spec.table+" WHERE "+field+" = ? LIMIT ?", id, limit)
	if err != nil {
		return nil, fmt.Errorf("expand provenance relationship: %w", err)
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var related string
		if err := rows.Scan(&related); err != nil {
			return nil, err
		}
		ids = append(ids, related)
	}
	return ids, rows.Err()
}

func fieldNames(fields []ports.ProvenanceField) []string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.Name)
	}
	return names
}

func lineageNode(entity, id string, record map[string]any) ports.ProvenanceLineageNode {
	label := id
	for _, field := range []string{"name", "logical_name", "external_id", "status"} {
		if value := strings.TrimSpace(fmt.Sprint(record[field])); value != "" && value != "<nil>" {
			label = value
			break
		}
	}
	return ports.ProvenanceLineageNode{Key: lineageKey(entity, id), Entity: entity, ID: id, Label: label, Record: record}
}

func appendLineageEdge(
	result *ports.ProvenanceLineage,
	edges map[string]bool,
	source string,
	target string,
	label string,
) {
	key := source + "\x00" + target + "\x00" + label
	if edges[key] {
		return
	}
	edges[key] = true
	result.Edges = append(result.Edges, ports.ProvenanceLineageEdge{Source: source, Target: target, Label: label})
}

func lineageKey(entity, id string) string { return entity + ":" + id }
