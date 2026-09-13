package workflow_engine_api_handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
	cloudrepo "github.com/UFFeScience/akoflow/internal/infrastructure/database/cloud"
)

func TestMachineConfigurationDocumentationRequests(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(filepath.Join(t.TempDir(), "machine-configurations.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	store := cloudrepo.New(db)
	handler := &Handler{cloud: store}

	create := httptest.NewRequest(http.MethodPost, "/machine-configurations/", strings.NewReader(`{"id":"example-machine-setup","name":"Example machine setup"}`))
	create.Header.Set("Content-Type", "application/json")
	created := httptest.NewRecorder()
	handler.CreateMachineConfiguration(created, create)
	if created.Code != http.StatusCreated {
		t.Fatalf("create configuration: %d %s", created.Code, created.Body.String())
	}

	version := httptest.NewRequest(http.MethodPost, "/machine-configurations/example-machine-setup/versions/", strings.NewReader(`{"version":1,"playbookYaml":"- hosts: all\n  tasks:\n    - ansible.builtin.debug:\n        msg: ready\n"}`))
	version.Header.Set("Content-Type", "application/json")
	version.SetPathValue("configurationId", "example-machine-setup")
	versioned := httptest.NewRecorder()
	handler.CreateMachineConfigurationVersion(versioned, version)
	if versioned.Code != http.StatusCreated {
		t.Fatalf("create configuration version: %d %s", versioned.Code, versioned.Body.String())
	}

	stored, err := store.FindMachineConfiguration(ctx, "example-machine-setup")
	if err != nil || stored == nil || stored.Name != "Example machine setup" || len(stored.Versions) != 1 || stored.Versions[0].Version != 1 || stored.Versions[0].ContentSHA256 == "" {
		t.Fatalf("stored documentation requests: %#v, %v", stored, err)
	}
}
