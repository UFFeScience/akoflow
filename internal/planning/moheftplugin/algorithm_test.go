package moheftplugin

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	validator "github.com/UFFeScience/akoflow/internal/infrastructure/plugins/planning"
	"github.com/UFFeScience/akoflow/internal/planning/algorithms"
	"github.com/UFFeScience/akoflow/internal/planning/library"
	_ "github.com/mattn/go-sqlite3"
)

// The tests run the actual exported plugin against its generated Go contract,
// rather than a parallel reimplementation of MOHEFT in the test process.
func TestPublishedOperatorsAndPlacementInvariants(t *testing.T) {
	p, err := Package()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, content := range map[string]string{"contract.go": library.Contract, "algorithm.go": p.Source, "algorithm_test.go": operatorTests} {
		if err = os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-v", "contract.go", "algorithm.go", "algorithm_test.go")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GO111MODULE=off", "CGO_ENABLED=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin operator tests: %v\n%s", err, output)
	}
	t.Log(string(output))
}

type planSink struct{ plans []domain.SchedulePlan }

func (s *planSink) Emit(_ context.Context, p domain.SchedulePlan) error {
	s.plans = append(s.plans, p)
	return nil
}
func TestCompiledPluginUsesRegistryAndCoordinatorContract(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	registry, _ := algorithms.NewRegistry(algorithms.HEFT{})
	service, err := library.New(ctx, db, registry, t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Package()
	if err != nil {
		t.Fatal(err)
	}
	saved, err := service.Save(ctx, p)
	if err != nil || saved.Status != "ready" {
		t.Fatalf("save=%+v err=%v", saved, err)
	}
	resource := domain.Resource{ID: "cpu", EnvironmentVersionID: "env-v1", CPUCores: 1, CPUCapacity: 1, MemoryBytes: 1024, ComputeSpeedup: 1, Schedulable: true}
	request := domain.PlanningRequest{Workflow: domain.WorkflowVersion{ID: "workflow-v1", Activities: []domain.Activity{{ID: "a", Name: "A", Priority: 7, Resources: domain.ActivityResources{CPU: 1}, Simulation: &domain.ActivitySimulation{DurationSeconds: 3}}}}, Resources: []domain.Resource{resource}, ExecutionScope: domain.ExecutionScope{ID: "scope", EnvironmentVersionIDs: []string{"env-v1"}}, NetworkTopology: domain.NetworkTopology{ID: "network", ExecutionScopeID: "scope"}}
	plugin, ok := registry.Find("moheft")
	if !ok {
		t.Fatal("plugin not registered")
	}
	sink := &planSink{}
	if err = plugin.Schedule(ctx, request, map[string]any{"candidateCount": float64(10)}, nil, sink); err != nil {
		t.Fatal(err)
	}
	if len(sink.plans) != 1 {
		t.Fatalf("plans=%d", len(sink.plans))
	}
	plan := sink.plans[0]
	if plan.Assignments[0].Priority != 7 {
		t.Fatal("workflow dispatch priority lost across plugin contract")
	}
	if _, ok := plan.Assignments[0].Metadata["resourceAvailableAtSeconds"]; !ok {
		t.Fatal("plugin assignment metadata lost across JSON contract")
	}
	if err = validator.NewValidator().Validate(plan, request.Workflow, request.Resources, request.ExecutionScope, request.NetworkTopology); err != nil {
		t.Fatal(err)
	}
	if plan.Objective != "pareto" || plan.Predicted.MakespanSeconds != 3 || !plan.Predicted.Feasible || !strings.HasPrefix(plan.AlgorithmVersion, "1:") {
		b, _ := json.Marshal(plan)
		t.Fatalf("plan=%s", b)
	}
}

const operatorTests = `package main
import("testing";"encoding/json";"math";"fmt")
func fixture()Input{return Input{Request:Request{Workflow:Workflow{ID:"wf",Activities:[]Activity{{ID:"a",Resources:Requirements{CPU:1},Simulation:&Simulation{DurationSeconds:10}}}},Resources:[]Resource{{ID:"fast",EnvironmentVersionID:"env",CPUCores:1,CPUCapacity:1,MemoryBytes:100,ComputeSpeedup:2,PricePerSecond:10,Schedulable:true},{ID:"cheap",EnvironmentVersionID:"env",CPUCores:1,CPUCapacity:1,MemoryBytes:100,ComputeSpeedup:1,PricePerSecond:1,Schedulable:true}},ExecutionScope:Scope{ID:"scope",EnvironmentVersionIDs:[]string{"env"}}},Configuration:map[string]any{"candidateCount":float64(25)}}}
func TestBootContract(t *testing.T){in:=fixture();in.Request.Resources=in.Request.Resources[:1];in.Request.Resources[0].BootOverheadSeconds=12;in.Request.Resources[0].ContainerOverheadSeconds=2;in.Request.Workflow.Activities=append(in.Request.Workflow.Activities,Activity{ID:"b",Simulation:&Simulation{DurationSeconds:10}});in.Request.Workflow.Dependencies=[]Dependency{{ActivityID:"b",DependsOnActivityID:"a"}};plans,e:=Schedule(in);if e!=nil{t.Fatal(e)};p:=plans[0];if p.Predicted.MakespanSeconds!=26{t.Fatal(p.Predicted)};for _,a:=range p.Assignments{if a.Metadata["bootOverheadSeconds"]!=float64(0)||a.Metadata["resourceAvailableAtSeconds"]!=float64(12)||a.Metadata["containerOverheadSeconds"]!=float64(2){t.Fatal(a.Metadata)}}}
func TestDeclaredZeroWork(t *testing.T){in:=fixture();in.Request.Workflow.Activities[0].Simulation.DurationSeconds=0;plans,e:=Schedule(in);if e!=nil{t.Fatal(e)};for _,p:=range plans{if p.Predicted.MakespanSeconds!=0||p.Predicted.Cost!=0{t.Fatal(p.Predicted)}}}
func TestAnalyticTimeCostFront(t *testing.T){plans,e:=Schedule(fixture());if e!=nil{t.Fatal(e)};if len(plans)!=2{t.Fatalf("front=%d",len(plans))};found:=map[string]bool{};for _,p:=range plans{found[fmt.Sprintf("%.0f/%.0f",p.Predicted.MakespanSeconds,p.Predicted.Cost)]=true};if !found["5/50"]||!found["10/10"]{t.Fatal(found)}}
func TestDominatedPartialSchedulesRemoved(t *testing.T){front:=moSelect([]moState{{makespan:1,cost:2},{makespan:2,cost:3},{makespan:3,cost:1}},10);if len(front)!=2{t.Fatal(front)}}
func TestCrowdingKeepsEndpointsAndBounds(t *testing.T){front:=moSelect([]moState{{makespan:1,cost:9},{makespan:2,cost:6},{makespan:3,cost:4},{makespan:4,cost:3},{makespan:5,cost:1}},3);if len(front)!=3{t.Fatal(front)};a,b:=false,false;for _,p:=range front{a=a||p.makespan==1;b=b||p.cost==1};if !a||!b{t.Fatal("extreme points were lost")}}
func TestEqualObjectiveMappingsNotCollapsed(t *testing.T){front:=moSelect([]moState{{makespan:2,cost:2,signature:1},{makespan:2,cost:2,signature:2}},2);if len(front)!=2{t.Fatal("future communication alternatives lost")}}
func TestTwoTaskChainMatchesEnumeratedFront(t *testing.T){in:=fixture();in.Request.Workflow.Activities=append(in.Request.Workflow.Activities,Activity{ID:"b",Resources:Requirements{CPU:1},Simulation:&Simulation{DurationSeconds:10}});in.Request.Workflow.Dependencies=[]Dependency{{ActivityID:"b",DependsOnActivityID:"a"}};plans,e:=Schedule(in);if e!=nil{t.Fatal(e)};found:=map[string]bool{};for _,p:=range plans{found[fmt.Sprintf("%.0f/%.0f",p.Predicted.MakespanSeconds,p.Predicted.Cost)]=true};if len(found)!=3||!found["10/100"]||!found["15/60"]||!found["20/20"]{t.Fatal(found)}}
func TestTopologicalRankAndCycleRejection(t *testing.T){in:=fixture();in.Request.Workflow.Activities=append(in.Request.Workflow.Activities,Activity{ID:"b",Simulation:&Simulation{DurationSeconds:1}});in.Request.Workflow.Dependencies=[]Dependency{{ActivityID:"b",DependsOnActivityID:"a"}};m,e:=newMOModel(in);if e!=nil{t.Fatal(e)};if m.tasks[m.order[0]].ID!="a"{t.Fatal(m.order)};in.Request.Workflow.Dependencies=append(in.Request.Workflow.Dependencies,Dependency{ActivityID:"a",DependsOnActivityID:"b"});if _,e=Schedule(in);e==nil{t.Fatal("cycle accepted")}}
func TestInsertionUsesExistingIdleGap(t *testing.T){in:=fixture();in.Request.Resources=in.Request.Resources[:1];in.Request.Workflow.Activities[0].Simulation.DurationSeconds=2;m,e:=newMOModel(in);if e!=nil{t.Fatal(e)};s:=m.empty();s.slots[0][0]=[]moInterval{{start:10,finish:12}};next,ok:=m.place(s,0,0);if !ok||next.get(0).assignment.PredictedStartAt!=0{t.Fatal("idle gap ignored")};if len(s.slots[0][0])!=1{t.Fatal("parent state mutated")}}
func TestMemoryAdmissionAcrossCPULanes(t *testing.T){in:=fixture();in.Request.Resources=in.Request.Resources[:1];in.Request.Resources[0].CPUCores=2;in.Request.Resources[0].CPUCapacity=2;in.Request.Resources[0].MemoryBytes=10;in.Request.Workflow.Activities[0].Resources.MemoryBytes=10;in.Request.Workflow.Activities=append(in.Request.Workflow.Activities,Activity{ID:"b",Resources:Requirements{CPU:1,MemoryBytes:10},Simulation:&Simulation{DurationSeconds:10}});plans,e:=Schedule(in);if e!=nil{t.Fatal(e)};if plans[0].Predicted.MakespanSeconds!=10{t.Fatal(plans[0].Predicted)}}
func TestLimitsOnlyMarkFeasibility(t *testing.T){in:=fixture();in.Request.Budget=1;in.Request.DeadlineSeconds=1;plans,e:=Schedule(in);if e!=nil{t.Fatal(e)};for _,p:=range plans{if p.Predicted.Feasible{t.Fatal("infeasible plan advertised as feasible")}}}
func TestUnknownWorkAndMultiLaneAreRejected(t *testing.T){for _,mode:=range []string{"unknown","multi","memory"}{in:=fixture();if mode=="unknown"{in.Request.Workflow.Activities[0].Simulation=nil}else if mode=="multi"{in.Request.Workflow.Activities[0].Resources.CPU=2}else{in.Request.Workflow.Activities[0].Resources.MemoryBytes=1000};if _,e:=Schedule(in);e==nil{t.Fatal("unsupported input accepted",mode)}}}
func TestDirectedTransfersAndPriceTerms(t *testing.T){in:=fixture();in.Request.Resources[0].Metadata=map[string]any{"egressPricePerByte":float64(3)};in.Request.Resources[1].Metadata=map[string]any{"ingressPricePerByte":float64(2),"storagePricePerByteSecond":float64(1)};in.Request.Workflow.Activities=append(in.Request.Workflow.Activities,Activity{ID:"b",Resources:Requirements{CPU:1,StorageBytes:2},Simulation:&Simulation{DurationSeconds:2}});in.Request.Workflow.Dependencies=[]Dependency{{ActivityID:"b",DependsOnActivityID:"a"}};raw,_:=json.Marshal(moData{ProducerActivityID:"a",ConsumerActivityID:"b",SizeBytes:10});in.Request.Workflow.DataDependencies=[]json.RawMessage{raw};in.Request.NetworkTopology=json.RawMessage("{\"links\":[{\"sourceResourceId\":\"fast\",\"targetResourceId\":\"cheap\",\"bandwidthBitsPerSecond\":80}]} ");m,e:=newMOModel(in);if e!=nil{t.Fatal(e)};fast,cheap:=0,1;if m.resources[0].ID=="cheap"{fast,cheap=1,0};s,ok:=m.place(m.empty(),0,fast);if !ok{t.Fatal("parent")};s,ok=m.place(s,1,cheap);if !ok{t.Fatal("child")};p:=m.plan(s,0);if math.Abs(p.Predicted.Cost-109)>1e-9{t.Fatal("price terms differ",p.Predicted.Cost)};sum:=0.0;for _,a:=range p.Assignments{sum+=a.PredictedCost};if sum!=p.Predicted.Cost{t.Fatal("assignment costs do not sum",sum,p.Predicted.Cost)};if _,ok=m.transfer("cheap","fast",10);ok{t.Fatal("missing reverse link treated as free")}}
func TestPaperMaximumTransferTiming(t *testing.T){in:=fixture();in.Request.Resources=append(in.Request.Resources,Resource{ID:"target",EnvironmentVersionID:"env",CPUCores:1,CPUCapacity:1,MemoryBytes:100,ComputeSpeedup:1,Schedulable:true});in.Request.Workflow.Activities=[]Activity{{ID:"a",Simulation:&Simulation{DurationSeconds:1}},{ID:"b",Simulation:&Simulation{DurationSeconds:1}},{ID:"c",Simulation:&Simulation{DurationSeconds:1}}};in.Request.Workflow.Dependencies=[]Dependency{{ActivityID:"c",DependsOnActivityID:"a"},{ActivityID:"c",DependsOnActivityID:"b"}};for _,d:=range []moData{{"a","c",1},{"b","c",100}}{raw,_:=json.Marshal(d);in.Request.Workflow.DataDependencies=append(in.Request.Workflow.DataDependencies,raw)};in.Request.NetworkTopology=json.RawMessage("{\"links\":[{\"sourceResourceId\":\"fast\",\"targetResourceId\":\"target\",\"bandwidthBitsPerSecond\":8},{\"sourceResourceId\":\"cheap\",\"targetResourceId\":\"target\",\"bandwidthBitsPerSecond\":8}]}");m,e:=newMOModel(in);if e!=nil{t.Fatal(e)};s:=m.empty();s.blocks[0]=&moBlock{};find:=func(id string)int{for i,r:=range m.resources{if r.ID==id{return i}};panic(id)};s.blocks[0][0]=&moRecord{resource:find("fast"),assignment:Assignment{PredictedFinishAt:100}};s.blocks[0][1]=&moRecord{resource:find("cheap"),assignment:Assignment{PredictedFinishAt:1}};next,ok:=m.place(s,2,find("target"));if !ok||next.get(2).assignment.PredictedFinishAt!=201{t.Fatal("paper Eqs 1-2 were replaced by pairwise ready-time",next.get(2))}}
func TestFlatCrowdingAndDeterminism(t *testing.T){in:=fixture();a,e:=Schedule(in);if e!=nil{t.Fatal(e)};b,e:=Schedule(in);if e!=nil{t.Fatal(e)};x,_:=json.Marshal(a);y,_:=json.Marshal(b);if string(x)!=string(y){t.Fatal("nondeterministic output")};front:=moSelect([]moState{{makespan:1,cost:1,signature:3},{makespan:1,cost:1,signature:2},{makespan:1,cost:1,signature:1}},2);if len(front)!=2||front[0].signature!=1{t.Fatal(front)}}
func TestCandidateCountAndReferenceWorkProfile(t *testing.T){in:=fixture();in.Configuration["candidateCount"]=float64(0);if _,e:=Schedule(in);e==nil{t.Fatal("invalid K")};in=fixture();in.Request.Workflow.Activities[0].ActivityTypeID="type";raw:=json.RawMessage("{\"activityTypeId\":\"type\",\"resourceId\":\"fast\",\"runtimeSeconds\":0.25,\"modelVersion\":\"reference-work-v1\"}");in.Request.ActivityProfiles=[]json.RawMessage{raw};m,e:=newMOModel(in);if e!=nil{t.Fatal(e)};for i,r:=range m.resources{if r.ID=="fast"&&m.runtimes[0][i]!=2.5{t.Fatal(m.runtimes[0][i])}}}
`
