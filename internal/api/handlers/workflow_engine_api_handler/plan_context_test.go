package workflow_engine_api_handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
	dbenvironment "github.com/UFFeScience/akoflow/internal/infrastructure/database/environment"
	dbplanning "github.com/UFFeScience/akoflow/internal/infrastructure/database/planning"
	dbworkflow "github.com/UFFeScience/akoflow/internal/infrastructure/database/workflow"
)

func TestPlanContextUsesPersistedVersionsAndOnlyItsScope(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := database.Bootstrap(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`INSERT INTO environments(id,name) VALUES ('env','Historical'),('outside','Outside')`,
		`INSERT INTO environment_versions(id,environment_id,version,status,network_model,interference_model,cost_model,configuration_hash)
		 VALUES ('env-old','env',1,'published','','','','a'),('env-new','env',2,'published','','','','b')`,
		`INSERT INTO resources(id,environment_version_id,type,name,provider_id)
		 VALUES ('old-resource','env-old','local_machine','Old','old'),('new-resource','env-new','local_machine','New','new')`,
		`INSERT INTO workflow_definitions(id,external_id,name) VALUES ('workflow','workflow','Workflow name')`,
		`INSERT INTO workflow_versions(id,workflow_id,version,definition_hash)
		 VALUES ('wf-old','workflow',1,'a'),('wf-new','workflow',2,'b')`,
		`INSERT INTO execution_scopes(id,name) VALUES ('scope','Selected')`,
		`INSERT INTO execution_scope_environments(execution_scope_id,environment_version_id) VALUES ('scope','env-old')`,
		`INSERT INTO schedule_plans(id,workflow_version_id,execution_scope_id,source,algorithm)
		 VALUES ('plan','wf-old','scope','plugin','test')`,
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	h := newTestHandler()
	h.plans = dbplanning.New(db)
	h.workflows = dbworkflow.New(db)
	h.environments = dbenvironment.New(db)
	request := httptest.NewRequest(http.MethodGet, "/schedule-plans/plan/context/", nil)
	request.SetPathValue("planId", "plan")
	response := httptest.NewRecorder()
	h.GetPlanContext(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("context status=%d %s", response.Code, response.Body.String())
	}
	var result struct {
		Workflow     domain.WorkflowDefinition      `json:"workflow"`
		Environments []domain.EnvironmentDefinition `json:"environments"`
		Resources    []domain.Resource              `json:"resources"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Workflow.Version.ID != "wf-old" || result.Workflow.Name != "Workflow name" || len(result.Environments) != 1 || result.Environments[0].Version.ID != "env-old" {
		t.Fatalf("incorrect context: %+v", result)
	}
	if len(result.Resources) != 1 || result.Resources[0].ID != "old-resource" {
		t.Fatalf("unrelated resource: %+v", result.Resources)
	}
}
