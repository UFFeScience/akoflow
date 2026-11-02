package library

import (
	"context"
	"database/sql"
	planningplugin "github.com/UFFeScience/akoflow/internal/infrastructure/plugins/planning"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/planning/algorithms"
	_ "github.com/mattn/go-sqlite3"
)

type capture struct{ plans []domain.SchedulePlan }

func (c *capture) Emit(_ context.Context, p domain.SchedulePlan) error {
	c.plans = append(c.plans, p)
	return nil
}
func TestPluginCompilesRunsPersistsAndKeepsLastWorkingRevision(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "plugins.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	registry, _ := algorithms.NewRegistry(algorithms.HEFT{}, algorithms.NewPRISMTime(), algorithms.NewPRISMCost())
	root := t.TempDir()
	s, err := New(ctx, db, registry, root, false)
	if err != nil {
		t.Fatal(err)
	}
	p := Package{ID: "custom-test", Name: "Custom test", Objective: "time", Source: `func Schedule(input Input)([]Plan,error){return []Plan{{ID:"test",WorkflowVersionID:input.Request.Workflow.ID,ExecutionScopeID:input.Request.ExecutionScope.ID,Predicted:Metrics{MakespanSeconds:2,Feasible:true},Assignments:[]Assignment{{ActivityID:"activity",ResourceID:"resource",PredictedFinishAt:2}}}},nil}`}
	saved, err := s.Save(ctx, p)
	if err != nil || saved.Status != "ready" {
		t.Fatalf("save=%+v err=%v", saved, err)
	}
	scheduler, ok := registry.Find(p.ID)
	if !ok {
		t.Fatal("not registered")
	}
	sink := &capture{}
	request := domain.PlanningRequest{
		Workflow:       domain.WorkflowVersion{ID: "wf-v1", Activities: []domain.Activity{{ID: "activity", Name: "Activity"}}},
		ExecutionScope: domain.ExecutionScope{ID: "scope", EnvironmentVersionIDs: []string{"env-v1"}},
		Resources:      []domain.Resource{{ID: "resource", EnvironmentVersionID: "env-v1", Schedulable: true}},
	}
	if err = scheduler.Schedule(ctx, request, nil, nil, sink); err != nil {
		t.Fatal(err)
	}
	if len(sink.plans) != 1 || sink.plans[0].WorkflowVersionID != "wf-v1" || sink.plans[0].Assignments[0].PredictedFinishAt != 2 {
		t.Fatalf("plans=%+v", sink.plans)
	}
	if err = planningplugin.NewValidator().Validate(sink.plans[0], request.Workflow, request.Resources, request.ExecutionScope, request.NetworkTopology); err != nil {
		t.Fatalf("compiled output failed coordinator validation: %v", err)
	}
	if !strings.HasPrefix(sink.plans[0].AlgorithmVersion, "1:") {
		t.Fatal("missing source provenance")
	}
	p.Source = "package main\nfunc Schedule("
	failed, err := s.Save(ctx, p)
	if err != nil || failed.Status != "compile_failed" || failed.ActiveRevision != 1 || failed.Revision != 2 {
		t.Fatalf("failed=%+v err=%v", failed, err)
	}
	stillActive, _ := registry.Find(p.ID)
	var revisions int
	if err = db.QueryRow(`SELECT count(*) FROM scheduler_plugin_revisions WHERE id=?`, p.ID).Scan(&revisions); err != nil || revisions != 2 {
		t.Fatalf("revision history count=%d err=%v", revisions, err)
	}
	if stillActive != scheduler {
		t.Fatal("failed revision replaced active code")
	}
	if err = os.Remove(s.binary(*saved)); err != nil {
		t.Fatal(err)
	}
	registry2, _ := algorithms.NewRegistry()
	_, err = New(ctx, db, registry2, root, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok = registry2.Find(p.ID); !ok {
		t.Fatal("active revision not reloaded")
	}
	exported, err := s.Get(ctx, p.ID)
	if err != nil || exported.Source != p.Source {
		t.Fatal("draft source not preserved")
	}
	for _, id := range []string{"heft", "prism-time", "prism-cost", "../escape"} {
		p.ID = id
		if _, err = s.Save(ctx, p); err == nil {
			t.Fatalf("accepted reserved/invalid id %q", id)
		}
	}
}
func TestNativeSourceAndClassification(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	registry, _ := algorithms.NewRegistry(algorithms.HEFT{}, algorithms.NewPRISMTime(), algorithms.NewPRISMCost())
	s, err := New(context.Background(), db, registry, t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"heft", "prism-time", "prism-cost"} {
		p, err := s.Get(context.Background(), id)
		if err != nil || p.Origin != "native" || p.Exclusive != (id != "heft") || p.Files["heft.go"] == "" {
			t.Fatalf("native=%+v error=%v", p, err)
		}
	}
	var _ ports.Scheduler = (*executable)(nil)
}
