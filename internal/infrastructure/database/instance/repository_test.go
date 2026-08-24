package instance

import (
	"context"
	"database/sql"
	"testing"

	domaininstance "github.com/UFFeScience/akoflow/internal/domain/instance"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
	_ "github.com/mattn/go-sqlite3"
)

func TestRepositoryLifecycle(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	if err := database.Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	repository := New(db)
	value, err := repository.Find(ctx)
	if err != nil || value != nil {
		t.Fatalf("expected an empty instance: %+v %v", value, err)
	}
	if err := repository.Save(ctx, domaininstance.Instance{ID: "lab", Name: "Lab"}); err != nil {
		t.Fatal(err)
	}
	value, err = repository.Find(ctx)
	if err != nil || value == nil || value.Name != "Lab" {
		t.Fatalf("unexpected stored instance: %+v %v", value, err)
	}
	if err := repository.Save(ctx, domaininstance.Instance{ID: "lab", Name: "Updated"}); err != nil {
		t.Fatal(err)
	}
	value, _ = repository.Find(ctx)
	if value.Name != "Updated" {
		t.Fatalf("instance was not updated: %+v", value)
	}
}
