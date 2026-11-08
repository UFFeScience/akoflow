package search

import (
	"context"
	"database/sql"
	"net/url"
	"strings"

	domainsearch "github.com/UFFeScience/akoflow/internal/domain/search"
)

type Repository struct{ db *sql.DB }

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Search(ctx context.Context, q domainsearch.Query) (domainsearch.Page, error) {
	page := domainsearch.Page{Query: q.Text, Results: []domainsearch.Result{}}
	c, err := decodeCursor(q)
	if err != nil {
		return page, err
	}
	if q.Limit < 1 || q.Limit > 50 {
		q.Limit = 20
	}
	query, args := searchSQL(q, c)
	if query == "" || q.Text == "" {
		return page, nil
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		item, err := scanResult(rows)
		if err != nil {
			return page, err
		}
		page.Results = append(page.Results, item)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	if len(page.Results) > q.Limit {
		page.Results = page.Results[:q.Limit]
		page.NextCursor = encodeCursor(q, page.Results[len(page.Results)-1])
	}
	page.Total = len(page.Results)
	return page, nil
}

func searchSQL(q domainsearch.Query, c cursor) (string, []any) {
	var parts []string
	for _, kind := range q.Types {
		if projection := projections[kind]; projection != "" {
			parts = append(parts, projection)
		}
	}
	if len(parts) == 0 {
		return "", nil
	}
	query := `WITH needle AS (SELECT LOWER(?) AS value), metadata AS (` + strings.Join(parts, " UNION ALL ") + `),
	 ranked AS (SELECT m.*, CASE
	 WHEN LOWER(id)=n.value OR LOWER(alternate_id)=n.value OR LOWER(title)=n.value THEN 3
	 WHEN SUBSTR(LOWER(id),1,LENGTH(n.value))=n.value OR SUBSTR(LOWER(title),1,LENGTH(n.value))=n.value THEN 2
	 ELSE 1 END AS score FROM metadata m CROSS JOIN needle n
	 WHERE INSTR(LOWER(id),n.value)>0 OR INSTR(LOWER(alternate_id),n.value)>0
	 OR INSTR(LOWER(title),n.value)>0 OR INSTR(LOWER(owner_title),n.value)>0)
	 SELECT type,id,title,subtitle,status,version,route_id,owner_id,owner_title,owner_type,score FROM ranked`
	args := []any{q.Text}
	if c.ID != "" {
		query += ` WHERE score < ? OR (score=? AND (type>? OR (type=? AND id>?)))`
		args = append(args, c.Score, c.Score, c.Type, c.Type, c.ID)
	}
	query += ` ORDER BY score DESC,type,id LIMIT ?`
	return query, append(args, q.Limit+1)
}

func scanResult(rows *sql.Rows) (domainsearch.Result, error) {
	var item domainsearch.Result
	var routeID, ownerType string
	err := rows.Scan(&item.Type, &item.ID, &item.Title, &item.Subtitle, &item.Status, &item.Version,
		&routeID, &item.OwnerID, &item.OwnerTitle, &ownerType, &item.Score)
	item.Path = entityPath(item.Type, routeID)
	if ownerType == "workflow" && item.OwnerID != "" {
		if item.Type == "execution" {
			item.Path = entityPath("workflow", item.OwnerID) + "/plans/" + url.PathEscape(routeID) + "/executions/" + url.PathEscape(item.ID)
		} else if item.Type == "plan" {
			item.Path = entityPath("workflow", item.OwnerID) + "/plans/" + url.PathEscape(item.ID)
		}
	}
	if item.OwnerID != "" {
		item.OwnerPath = entityPath(ownerType, item.OwnerID)
	}
	return item, err
}

func entityPath(kind, id string) string {
	prefix := map[string]string{"workflow": "/workflows/", "execution": "/executions/", "environment": "/environments/",
		"artifact": "/artifacts/", "resource": "/resources/", "plan": "/plans/", "scope": "/execution-scopes/", "materialization": "/artifacts/"}
	return prefix[kind] + url.PathEscape(id)
}
