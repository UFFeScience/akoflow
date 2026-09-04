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
	Query(context.Context, ProvenanceQuery) (ProvenancePage, error)
}
