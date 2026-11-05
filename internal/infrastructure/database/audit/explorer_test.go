package audit

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
)

func explorerRepository(t *testing.T) *Repository {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if err := database.Bootstrap(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return New(db)
}

func TestExplorerCountsAllMatchingHistoryAndBuildsBoundedTimeline(t *testing.T) {
	repo := explorerRepository(t)
	ctx := context.Background()
	end := time.Now().UTC().Truncate(time.Second)
	start := end.Add(-90 * time.Minute)
	for index := 0; index < 230; index++ {
		family, outcome := "activity.completed", domainaudit.OutcomeSucceeded
		if index%2 == 0 {
			family, outcome = "api.access", domainaudit.OutcomeFailed
		}
		if err := repo.RecordAuditEvent(ctx, domainaudit.Event{
			ID: fmt.Sprintf("event-%d", index), EventType: family, Outcome: outcome, Transport: "internal",
			ActorID: fmt.Sprintf("actor-%03d", index), OccurredAt: start.Add(time.Duration(index+1) * time.Second),
		}); err != nil {
			t.Fatal(err)
		}
	}
	filter := domainaudit.Filter{From: start.Format(time.RFC3339Nano), To: end.Format(time.RFC3339Nano), Limit: 1, Before: "event-0"}
	summary, err := repo.SummarizeAudit(ctx, filter)
	if err != nil || summary.Total != 230 {
		t.Fatalf("%+v %v", summary, err)
	}
	var sum int64
	for _, bucket := range summary.Buckets {
		sum += bucket.Count
	}
	if sum != 230 || len(summary.Buckets) > 61 {
		t.Fatalf("sum=%d buckets=%d", sum, len(summary.Buckets))
	}
	filter.Category = "activity"
	summary, err = repo.SummarizeAudit(ctx, filter)
	if err != nil || summary.Total != 115 {
		t.Fatalf("%+v %v", summary, err)
	}
	// The category facet clears only its own selection, preserving other filters.
	if len(summary.Facets["category"]) != 2 {
		t.Fatalf("%+v", summary.Facets)
	}
	page, err := repo.ListAuditFacet(ctx, filter, "actorId", "actor-", 10)
	if err != nil || len(page.Values) != 10 || !page.HasMore {
		t.Fatalf("%+v %v", page, err)
	}
	exact, err := repo.ListAuditFacet(ctx, filter, "actorId", "actor-001", 10)
	if err != nil || len(exact.Values) != 1 || exact.Values[0].Count != 1 {
		t.Fatalf("%+v %v", exact, err)
	}
}

func TestExplorerDefaultIsNinetyMinutesAndDoesNotAlterStoredHistory(t *testing.T) {
	repo := explorerRepository(t)
	ctx := context.Background()
	for _, event := range []domainaudit.Event{
		{ID: "recent", EventType: "api.access", Outcome: domainaudit.OutcomeSucceeded, OccurredAt: time.Now().UTC().Add(-time.Minute)},
		{ID: "old", EventType: "activity.completed", Outcome: domainaudit.OutcomeSucceeded, OccurredAt: time.Now().UTC().Add(-4 * time.Hour)},
	} {
		if err := repo.RecordAuditEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	summary, err := repo.SummarizeAudit(ctx, domainaudit.Filter{})
	if err != nil || summary.Total != 1 || summary.To.Sub(summary.From) != 90*time.Minute {
		t.Fatalf("%+v %v", summary, err)
	}
	summary, err = repo.SummarizeAudit(ctx, domainaudit.Filter{From: time.Now().Add(-6 * time.Hour).UTC().Format(time.RFC3339Nano)})
	if err != nil || summary.Total != 2 {
		t.Fatalf("%+v %v", summary, err)
	}
	items, err := repo.ListAuditEvents(ctx, domainaudit.Filter{})
	if err != nil || len(items) != 2 {
		t.Fatalf("%+v %v", items, err)
	}
}

func TestExplorerRejectsSQLFieldsTreatsSearchAsLiteralAndHonorsCancellation(t *testing.T) {
	repo := explorerRepository(t)
	ctx := context.Background()
	if _, err := repo.ListAuditFacet(ctx, domainaudit.Filter{}, "id);DROP TABLE audit_events;--", "", 10); err == nil {
		t.Fatal("unsafe field accepted")
	}
	page, err := repo.ListAuditFacet(ctx, domainaudit.Filter{}, "category", "%' OR 1=1 --", 10)
	if err != nil || len(page.Values) != 0 {
		t.Fatalf("%+v %v", page, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := repo.SummarizeAudit(cancelled, domainaudit.Filter{}); err == nil {
		t.Fatal("cancelled query succeeded")
	}
	if _, err := repo.SummarizeAudit(ctx, domainaudit.Filter{From: "2026-11-05T12:00:00Z", To: "2026-11-05T10:00:00Z"}); err == nil {
		t.Fatal("reversed range accepted")
	}
}
