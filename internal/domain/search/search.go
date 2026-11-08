package search

import "errors"

var ErrInvalidCursor = errors.New("invalid search cursor")

var Types = []string{"workflow", "execution", "environment", "artifact", "resource", "plan", "scope", "materialization"}

type Query struct {
	Text   string
	Types  []string
	Limit  int
	Cursor string
}

type Result struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Title      string `json:"title"`
	Subtitle   string `json:"subtitle,omitempty"`
	Status     string `json:"status,omitempty"`
	Version    string `json:"version,omitempty"`
	Path       string `json:"path"`
	OwnerID    string `json:"ownerId,omitempty"`
	OwnerTitle string `json:"ownerTitle,omitempty"`
	OwnerPath  string `json:"ownerPath,omitempty"`
	Score      int    `json:"score"`
}

type Page struct {
	Query      string   `json:"query"`
	Results    []Result `json:"results"`
	NextCursor string   `json:"nextCursor"`
	Total      int      `json:"total"`
}
