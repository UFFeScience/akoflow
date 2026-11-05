package audit

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
)

type auditReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// explorerQuery applies exactly the event filters, without the page cursor.
// It counts stored rows rather than the current frontend page.
func explorerQuery(filter domainaudit.Filter) (string, []any, error) {
	filter.Before = ""
	query, args, err := auditQuery(filter)
	if err != nil {
		return "", nil, err
	}
	return strings.TrimSuffix(query, " ORDER BY occurred_at DESC, id DESC LIMIT ?"), args[:len(args)-1], nil
}

func facetColumn(filter *domainaudit.Filter, field string) (string, error) {
	switch field {
	case "category":
		filter.Category = ""
		return "CASE WHEN instr(event_type,'.')>0 THEN substr(event_type,1,instr(event_type,'.')-1) ELSE event_type END", nil
	case "eventType":
		filter.EventType = ""
		return "event_type", nil
	case "outcome":
		filter.Outcome = ""
		return "outcome", nil
	case "transport":
		filter.Transport = ""
		return "transport", nil
	case "actorId":
		filter.ActorID = ""
		return "actor_id", nil
	case "method":
		filter.Method = ""
		return "method", nil
	case "status":
		filter.Status = 0
		return "CASE WHEN status>0 THEN CAST(status AS TEXT) ELSE '' END", nil
	case "route":
		filter.Route = ""
		return "route", nil
	case "environmentId":
		filter.EnvironmentID = ""
		return "environment_id", nil
	case "resourceId":
		filter.ResourceID = ""
		return "resource_id", nil
	case "connectionId":
		filter.ConnectionID = ""
		return "connection_id", nil
	case "runtimeId":
		filter.RuntimeID = ""
		return "runtime_id", nil
	case "sessionId":
		filter.SessionID = ""
		return "session_id", nil
	case "executionId":
		filter.ExecutionID = ""
		return "execution_id", nil
	case "activityId":
		filter.ActivityID = ""
		return "activity_id", nil
	case "operationId":
		filter.OperationID = ""
		return "operation_id", nil
	case "requestId":
		filter.RequestID = ""
		return "request_id", nil
	case "correlationId":
		filter.CorrelationID = ""
		return "correlation_id", nil
	default:
		return "", fmt.Errorf("unsupported audit facet")
	}
}

func (r *Repository) ListAuditFacet(ctx context.Context, filter domainaudit.Filter, field, search string, limit int) (domainaudit.FacetPage, error) {
	return listAuditFacet(ctx, r.db, filter, field, search, limit)
}

func listAuditFacet(ctx context.Context, reader auditReader, filter domainaudit.Filter, field, search string, limit int) (domainaudit.FacetPage, error) {
	column, err := facetColumn(&filter, field)
	if err != nil {
		return domainaudit.FacetPage{}, err
	}
	query, args, err := explorerQuery(filter)
	if err != nil {
		return domainaudit.FacetPage{}, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	statement := "SELECT value,COUNT(*) FROM (SELECT " + column + " AS value FROM (" + query + ")) WHERE value<>''"
	if search != "" {
		statement += " AND instr(lower(value),lower(?))>0"
		args = append(args, search)
	}
	statement += " GROUP BY value ORDER BY COUNT(*) DESC,value ASC LIMIT ?"
	args = append(args, limit+1)
	rows, err := reader.QueryContext(ctx, statement, args...)
	if err != nil {
		return domainaudit.FacetPage{}, err
	}
	defer rows.Close()
	page := domainaudit.FacetPage{Values: []domainaudit.FacetValue{}}
	for rows.Next() {
		var item domainaudit.FacetValue
		if err := rows.Scan(&item.Value, &item.Count); err != nil {
			return page, err
		}
		item.Value = clean(item.Value)
		page.Values = append(page.Values, item)
	}
	if len(page.Values) > limit {
		page.HasMore = true
		page.Values = page.Values[:limit]
	}
	return page, rows.Err()
}

func summaryRange(filter domainaudit.Filter) (time.Time, time.Time, error) {
	to := time.Now().UTC()
	var err error
	if filter.To != "" {
		to, err = time.Parse(time.RFC3339Nano, filter.To)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	from := to.Add(-90 * time.Minute)
	if filter.From != "" {
		from, err = time.Parse(time.RFC3339Nano, filter.From)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if !from.Before(to) {
		return from, to, fmt.Errorf("audit range must end after it starts")
	}
	return from.UTC(), to.UTC(), nil
}

func (r *Repository) SummarizeAudit(ctx context.Context, filter domainaudit.Filter) (domainaudit.Summary, error) {
	from, to, err := summaryRange(filter)
	if err != nil {
		return domainaudit.Summary{}, err
	}
	filter.From, filter.To = from.Format(time.RFC3339Nano), to.Format(time.RFC3339Nano)
	query, args, err := explorerQuery(filter)
	if err != nil {
		return domainaudit.Summary{}, err
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return domainaudit.Summary{}, err
	}
	defer tx.Rollback()
	result := domainaudit.Summary{From: from, To: to, Facets: map[string][]domainaudit.FacetValue{}, FacetMore: map[string]bool{}}
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM ("+query+")", args...).Scan(&result.Total); err != nil {
		return result, err
	}
	result.Buckets, err = auditBuckets(ctx, tx, query, args, from, to)
	if err != nil {
		return result, err
	}
	for _, field := range []string{"category", "outcome", "transport"} {
		page, err := listAuditFacet(ctx, tx, filter, field, "", 100)
		if err != nil {
			return result, err
		}
		result.Facets[field] = page.Values
		result.FacetMore[field] = page.HasMore
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}

func auditBuckets(ctx context.Context, reader auditReader, query string, args []any, from, to time.Time) ([]domainaudit.TimeBucket, error) {
	width := (to.Unix()-from.Unix())/60 + 1
	if width < 1 {
		width = 1
	}
	// Bucket origin and width are parameters, and all filter SQL uses bound values.
	statement := "SELECT CAST((CAST(strftime('%s',occurred_at) AS INTEGER)-?)/? AS INTEGER),COUNT(*) FROM (" + query + ") GROUP BY 1"
	parameters := append([]any{from.Unix(), width}, args...)
	rows, err := reader.QueryContext(ctx, statement, parameters...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	size := int(to.Unix()-from.Unix())/int(width) + 1
	buckets := make([]domainaudit.TimeBucket, size)
	for index := range buckets {
		buckets[index].Timestamp = time.Unix(from.Unix()+int64(index)*width, int64(from.Nanosecond())).UTC()
	}
	for rows.Next() {
		var index int
		var count int64
		if err := rows.Scan(&index, &count); err != nil {
			return nil, err
		}
		if index >= 0 && index < len(buckets) {
			buckets[index].Count = count
		}
	}
	return buckets, rows.Err()
}
