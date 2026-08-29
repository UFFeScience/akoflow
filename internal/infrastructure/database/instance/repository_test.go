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
	if err := repository.Save(ctx, domaininstance.Instance{ID: "lab", Name: "Lab", TransferBufferBytes: 4 << 20}); err != nil {
		t.Fatal(err)
	}
	value, err = repository.Find(ctx)
	if err != nil || value == nil || value.Name != "Lab" || value.TransferBufferBytes != 4<<20 {
		t.Fatalf("unexpected stored instance: %+v %v", value, err)
	}
	if err := repository.Save(ctx, domaininstance.Instance{ID: "lab", Name: "Updated"}); err != nil {
		t.Fatal(err)
	}
	value, _ = repository.Find(ctx)
	if value.Name != "Updated" || value.TransferBufferBytes != domaininstance.DefaultTransferBufferBytes {
		t.Fatalf("instance was not updated: %+v", value)
	}
}

func TestUserPreferencesLifecycle(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	if err = database.Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	repository := New(db)
	value, err := repository.FindPreferences(ctx, "client")
	if err != nil || value != nil {
		t.Fatalf("empty preferences = %#v, %v", value, err)
	}
	preferences := domaininstance.UserPreferences{ClientID: "client", Theme: "dark", AnimationsEnabled: true}
	if err = repository.SavePreferences(ctx, preferences); err != nil {
		t.Fatal(err)
	}
	value, err = repository.FindPreferences(ctx, preferences.ClientID)
	if err != nil || value == nil || value.Theme != "dark" || !value.AnimationsEnabled || value.UpdatedAt.IsZero() {
		t.Fatalf("preferences = %#v, %v", value, err)
	}
	preferences.Theme, preferences.AnimationsEnabled = "light", false
	if err = repository.SavePreferences(ctx, preferences); err != nil {
		t.Fatal(err)
	}
	value, err = repository.FindPreferences(ctx, preferences.ClientID)
	if err != nil || value.Theme != "light" || value.AnimationsEnabled {
		t.Fatalf("updated preferences = %#v, %v", value, err)
	}
}
