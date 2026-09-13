package workflow_engine_api_handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	domaininstance "github.com/UFFeScience/akoflow/internal/domain/instance"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
	instancerepo "github.com/UFFeScience/akoflow/internal/infrastructure/database/instance"
)

func TestInstanceDocumentationRequestsPreserveCurrentFields(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(filepath.Join(t.TempDir(), "instance.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	store := instancerepo.New(db)
	if err := store.Save(ctx, domaininstance.Instance{ID: "lab", Name: "Research lab", Description: "Keep this", Organization: "Institute", Location: "Campus"}); err != nil {
		t.Fatal(err)
	}
	handler := &Handler{instance: store}
	current := callHandler(t, http.MethodGet, "/instance/", "", nil, handler.GetInstance)
	if current.Code != http.StatusOK {
		t.Fatalf("read current instance: %d %s", current.Code, current.Body.String())
	}
	var value domaininstance.Instance
	if err := json.Unmarshal(current.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	value.TransferBufferBytes = 8388608
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	updated := callHandler(t, http.MethodPut, "/instance/", string(body), nil, handler.SaveInstance)
	if updated.Code != http.StatusOK {
		t.Fatalf("update current instance: %d %s", updated.Code, updated.Body.String())
	}
	stored, err := store.Find(ctx)
	if err != nil || stored == nil || stored.ID != "lab" || stored.Name != "Research lab" || stored.Description != "Keep this" || stored.Organization != "Institute" || stored.Location != "Campus" || stored.TransferBufferBytes != 8388608 {
		t.Fatalf("instance update lost fields: %#v, %v", stored, err)
	}

	preferences := httptest.NewRequest(http.MethodPut, "/user-preferences/docs-client-01/", strings.NewReader(`{"theme":"dark","animationsEnabled":false}`))
	preferences.SetPathValue("clientId", "docs-client-01")
	preferences.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.SaveUserPreferences(response, preferences)
	saved, err := store.FindPreferences(ctx, "docs-client-01")
	if err != nil || response.Code != http.StatusOK || saved == nil || saved.ClientID != "docs-client-01" || saved.Theme != "dark" || saved.AnimationsEnabled {
		t.Fatalf("preferences request: status=%d saved=%#v error=%v", response.Code, saved, err)
	}
}
