package ports

import "context"

// ProvenanceEntity describes a safe, read-only projection of persisted data.
// Names and fields are server-defined; clients never submit SQL identifiers.
type ProvenanceEntity struct {
	Name        string                   `json:"name"`
	Label       string                   `json:"label"`
	Description string                   `json:"description"`
	Fields      []ProvenanceField        `json:"fields"`
	Links       []ProvenanceRelationship `json:"links,omitempty"`
}

type ProvenanceField struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	Type  string `json:"type"`
}

type ProvenanceRelationship struct {
	Field        string `json:"field"`
	TargetEntity string `json:"targetEntity"`
	TargetField  string `json:"targetField"`
}

type ProvenanceQuery struct {
	Entity      string
	Search      string
	FilterField string
	FilterValue string
	Page        int
	PageSize    int
	SortField   string
	SortOrder   string
}

type ProvenanceSQLQuery struct {
	SQL        string         `json:"sql"`
	Parameters map[string]any `json:"parameters,omitempty"`
	Page       int            `json:"page,omitempty"`
	PageSize   int            `json:"pageSize,omitempty"`
}

type ProvenanceSQLColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type ProvenanceSQLTable struct {
	Name    string                `json:"name"`
	Columns []ProvenanceSQLColumn `json:"columns"`
}

type ProvenanceSQLResult struct {
	Columns             []ProvenanceSQLColumn `json:"columns"`
	Items               []map[string]any      `json:"items"`
	Page                int                   `json:"page"`
	PageSize            int                   `json:"pageSize"`
	Truncated           bool                  `json:"truncated"`
	ElapsedMilliseconds int64                 `json:"elapsedMilliseconds"`
}

type ProvenanceLineageQuery struct {
	Entity    string
	ID        string
	Direction string
	Depth     int
	MaxNodes  int
}

type ProvenanceLineageNode struct {
	Key    string         `json:"key"`
	Entity string         `json:"entity"`
	ID     string         `json:"id"`
	Label  string         `json:"label"`
	Record map[string]any `json:"record"`
}

type ProvenanceLineageEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Label  string `json:"label"`
}

type ProvenanceLineage struct {
	Root      string                  `json:"root"`
	Nodes     []ProvenanceLineageNode `json:"nodes"`
	Edges     []ProvenanceLineageEdge `json:"edges"`
	Truncated bool                    `json:"truncated"`
}

type ProvenancePage struct {
	Entity   ProvenanceEntity `json:"entity"`
	Items    []map[string]any `json:"items"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
	Total    int              `json:"total"`
	HasNext  bool             `json:"hasNext"`
}

type ProvenanceExplorer interface {
	Entities() []ProvenanceEntity
	Schema(context.Context) ([]ProvenanceSQLTable, error)
	Query(context.Context, ProvenanceQuery) (ProvenancePage, error)
	SQL(context.Context, ProvenanceSQLQuery) (ProvenanceSQLResult, error)
	Explain(context.Context, ProvenanceSQLQuery) (ProvenanceSQLResult, error)
	Lineage(context.Context, ProvenanceLineageQuery) (ProvenanceLineage, error)
}
