package kubernetes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type apiFake struct {
	created    map[string][]byte
	createErr  map[string]error
	getOutput  []byte
	listOutput []byte
	logsOutput []byte
	deleteErr  map[string]error
	deleted    []string
}

func (f *apiFake) Create(_ context.Context, _, resource string, body []byte) error {
	if err := f.createErr[resource]; err != nil {
		return err
	}
	if f.created == nil {
		f.created = map[string][]byte{}
	}
	f.created[resource] = body
	return nil
}

func TestAdapterAdoptsExistingJobOwnedBySameActivity(t *testing.T) {
	api := &apiFake{
		createErr: map[string]error{"jobs": ErrConflict},
		getOutput: []byte(`{"metadata":{"labels":{"app.kubernetes.io/managed-by":"akoflow"}},"spec":{"template":{"metadata":{"labels":{"akoflow.io/activity":"activity"}}}}}`),
	}
	handle, err := New(api, "science").Start(context.Background(), domain.ActivityExecutionContext{
		Run:       domain.ExecutionRun{ID: "run"},
		Activity:  domain.Activity{ID: "activity", Command: domain.ActivityCommand{Image: "image:1"}},
		RuntimeID: "kubernetes",
		Resource:  domain.Resource{ID: "node", Type: domain.ResourceKubernetesMachine},
	})
	if err != nil {
		t.Fatal(err)
	}
	if handle.ExternalID != "akoflow-run-activity" || handle.Status != domain.HandleStarting {
		t.Fatalf("handle=%+v", handle)
	}
}

func TestAdapterRejectsExistingJobOwnedByAnotherActivity(t *testing.T) {
	existing := `{"metadata":{"labels":{"app.kubernetes.io/managed-by":"akoflow"},` +
		`"annotations":{"akoflow.io/run-id":"other-run","akoflow.io/activity-id":"activity"}},` +
		`"spec":{"template":{"metadata":{"labels":{"akoflow.io/activity":"activity"}}}}}`
	api := &apiFake{
		createErr: map[string]error{"jobs": ErrConflict},
		getOutput: []byte(existing),
	}
	_, err := New(api, "science").Start(context.Background(), domain.ActivityExecutionContext{
		Run:      domain.ExecutionRun{ID: "run"},
		Activity: domain.Activity{ID: "activity", Command: domain.ActivityCommand{Image: "image:1"}},
		Resource: domain.Resource{Type: domain.ResourceKubernetesMachine},
	})
	if err == nil || !strings.Contains(err.Error(), "run owner") {
		t.Fatalf("error=%v", err)
	}
}
func (f *apiFake) Get(context.Context, string, string, string) ([]byte, error) {
	return f.getOutput, nil
}
func (f *apiFake) List(context.Context, string, string, string) ([]byte, error) {
	return f.listOutput, nil
}
func (f *apiFake) Logs(context.Context, string, string, string) ([]byte, error) {
	return f.logsOutput, nil
}
func (f *apiFake) Delete(_ context.Context, _, resource, name string) error {
	f.deleted = append(f.deleted, resource+"/"+name)
	return f.deleteErr[resource]
}

func TestAdapterCreatesJobAndServiceFromActivity(t *testing.T) {
	api := &apiFake{}
	adapter := New(api, "science")
	activity := domain.Activity{
		ID: "A 1",
		Command: domain.ActivityCommand{
			Image: "image:1", Entrypoint: "python", Arguments: []string{"run.py"},
		},
		Resources: domain.ActivityResources{CPU: 2, MemoryBytes: 1048576},
		Service:   &domain.ServiceSpec{Ports: []int{8080}},
	}
	handle, err := adapter.Start(context.Background(), domain.ActivityExecutionContext{
		Run: domain.ExecutionRun{ID: "run"}, Activity: activity,
		RuntimeID: "kubernetes",
		Resource: domain.Resource{ID: "node",
			Type: domain.ResourceKubernetesMachine, ProviderID: "kind-worker"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if handle.Status != domain.HandleStarting || len(handle.Endpoints) != 1 {
		t.Fatalf("handle=%+v", handle)
	}
	if len(api.created) != 2 {
		t.Fatalf("created resources=%d", len(api.created))
	}
	var job map[string]any
	if err := json.Unmarshal(api.created["jobs"], &job); err != nil {
		t.Fatal(err)
	}
	template := job["spec"].(map[string]any)["template"].(map[string]any)
	podSpec := template["spec"].(map[string]any)
	selector := podSpec["nodeSelector"].(map[string]any)
	if selector["kubernetes.io/hostname"] != "kind-worker" {
		t.Fatalf("node selector=%v", selector)
	}
	containers := podSpec["containers"].([]any)
	if len(containers) != 1 {
		t.Fatalf("activity must be the only container: %v", podSpec)
	}
	if _, exists := podSpec["initContainers"]; exists {
		t.Fatalf("observer must not require an init container: %v", podSpec)
	}
	activityContainer := containers[0].(map[string]any)
	command := activityContainer["command"].([]any)
	if command[0] != "/bin/sh" {
		t.Fatalf("activity command=%v", command)
	}
	if _, exists := podSpec["volumes"]; exists {
		t.Fatalf("shell lifecycle must not inject volumes: %v", podSpec)
	}
}

func TestAdapterDoesNotUseAPIEndpointAsKubernetesNodeSelector(t *testing.T) {
	podSpec := observedPodSpec(domain.WorkflowVersion{}, domain.Activity{}, domain.Resource{
		Type: domain.ResourceKubernetesMachine, ProviderID: "https://host.docker.internal:61640",
	}, "run")
	if _, exists := podSpec["nodeSelector"]; exists {
		t.Fatalf("API endpoint must not be emitted as node selector: %v", podSpec)
	}
}

func TestAdapterUsesObservedHostnameForKubernetesNodeSelector(t *testing.T) {
	podSpec := observedPodSpec(domain.WorkflowVersion{}, domain.Activity{}, domain.Resource{
		Type: domain.ResourceKubernetesMachine, ProviderID: "https://host.docker.internal:61640",
		Metadata: map[string]any{"observedHostname": "kind-worker"},
	}, "run")
	selector, ok := podSpec["nodeSelector"].(map[string]string)
	if !ok || selector["kubernetes.io/hostname"] != "kind-worker" {
		t.Fatalf("node selector=%v", podSpec["nodeSelector"])
	}
}

func TestAdapterExplainsMissingShellContract(t *testing.T) {
	api := &apiFake{
		getOutput:  []byte(`{"status":{"failed":1,"conditions":[{"type":"Failed","status":"True","message":"Backoff limit exceeded"}]}}`),
		listOutput: []byte(`{"items":[{"metadata":{"name":"job-pod"},"status":{"containerStatuses":[{"state":{"waiting":{"reason":"StartError","message":"exec: \"/bin/sh\": stat /bin/sh: no such file or directory"}}}]}}]}`),
	}
	handle, err := New(api, "default").Inspect(context.Background(), domain.ActivityHandle{ExternalID: "job"})
	if err != nil || handle.Status != domain.HandleFailed ||
		handle.Failure != "activity image is incompatible with the Kubernetes shell runtime: /bin/sh is required" {
		t.Fatalf("handle=%+v err=%v", handle, err)
	}
}

func TestAdapterSeparatesKubernetesQueueAndContainerStartup(t *testing.T) {
	api := &apiFake{
		getOutput:  []byte(`{"status":{"active":1}}`),
		listOutput: []byte(`{"items":[{"status":{"startTime":"2026-08-24T06:30:00Z","containerStatuses":[{"name":"activity","state":{"running":{"startedAt":"2026-08-24T06:30:04Z"}}}]}}]}`),
	}
	handle, err := New(api, "default").Inspect(context.Background(), domain.ActivityHandle{
		ExternalID: "job", StartedAt: 0,
		Metadata: map[string]any{domain.TimingSubmittedAt: 1.0},
	})
	if err != nil || handle.StartedAt != 1787553000 || handle.Metadata[domain.TimingContainerStartedAt] != 1787553004.0 {
		t.Fatalf("handle=%+v err=%v", handle, err)
	}
}

func TestShellLifecycleExecutesActivityAndPublishesManifest(t *testing.T) {
	command := exec.Command(
		"/bin/sh", "-c", renderShellLifecycle(), "akoflow-entrypoint",
		"/bin/sh", "-c", "printf result > result.txt",
	)
	command.Dir = t.TempDir()
	command.Env = append(os.Environ(),
		"AKOFLOW_RUN_ID=run", "AKOFLOW_ACTIVITY_ID=activity", "AKOFLOW_OBSERVATION_ROOT=.",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("execute shell lifecycle: %v: %s", err, output)
	}
	prefixAt := strings.Index(string(output), manifestLogPrefix)
	if prefixAt < 0 {
		t.Fatalf("manifest was not published: %s", output)
	}
	encoded := strings.TrimSpace(string(output)[prefixAt+len(manifestLogPrefix):])
	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var manifest domain.ArtifactManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		t.Fatalf("decode manifest: %v: %s", err, payload)
	}
	if manifest.ExitCode != 0 || manifest.RunID != "run" || manifest.Summary.FinalFiles < 1 ||
		manifest.Summary.CreatedFiles != 1 || len(manifest.Files) != 1 ||
		manifest.Files[0].Checksum == "" || len(manifest.FinalSnapshot) != 1 {
		t.Fatalf("manifest=%+v", manifest)
	}
}

func TestObservationRootUsesIsolatedWorkspaceByDefault(t *testing.T) {
	activity := domain.Activity{}
	if root := observationRoot(activity, "run"); root != "/tmp/akoflow/workspace" {
		t.Fatalf("default observation root=%q", root)
	}
	activity.Command.WorkingDirectory = "/work"
	if root := observationRoot(activity, "run"); root != "/work" {
		t.Fatalf("working directory observation root=%q", root)
	}
	activity.Metadata = map[string]any{"artifactObservationRoot": "/outputs"}
	if root := observationRoot(activity, "run"); root != "/outputs" {
		t.Fatalf("configured observation root=%q", root)
	}
}

func TestObservedPodMountsDeclaredPVC(t *testing.T) {
	activity := domain.Activity{ID: "activity", Metadata: map[string]any{"storage": map[string]any{
		"type": "pvc", "claimName": "results", "mountPath": "/data",
	}}}
	spec := observedPodSpec(domain.WorkflowVersion{}, activity, domain.Resource{}, "run")
	if observationRoot(activity, "run") != "/data/runs/run/activity" {
		t.Fatalf("root=%s", observationRoot(activity, "run"))
	}
	if _, ok := spec["volumes"]; !ok {
		t.Fatalf("PVC volume was not created: %v", spec)
	}
}

func TestAdapterMapsKubernetesStatus(t *testing.T) {
	manifest := []byte(`{"schemaVersion":1,"runId":"run","activityId":"activity","files":[{"path":"result.csv","change":"created","sizeBytes":42}]}`)
	api := &apiFake{
		getOutput:  []byte(`{"status":{"succeeded":1}}`),
		listOutput: []byte(`{"items":[{"metadata":{"name":"job-pod"}}]}`),
		logsOutput: []byte(manifestLogPrefix + base64.StdEncoding.EncodeToString(manifest)),
	}
	handle, err := New(api, "default").Inspect(context.Background(), domain.ActivityHandle{
		ExternalID: "job", Status: domain.HandleRunning,
	})
	if err != nil || handle.Status != domain.HandleCompleted || handle.Artifacts == nil {
		t.Fatalf("handle=%+v err=%v", handle, err)
	}
	if len(handle.Artifacts.Files) != 1 || handle.Artifacts.Files[0].SizeBytes != 42 {
		t.Fatalf("artifacts=%+v", handle.Artifacts)
	}
}

func TestObservationFailureDoesNotChangeExecutionStatus(t *testing.T) {
	api := &apiFake{getOutput: []byte(`{"status":{"succeeded":1}}`), listOutput: []byte(`{"items":[]}`)}
	handle, err := New(api, "default").Inspect(context.Background(), domain.ActivityHandle{ExternalID: "job"})
	if err != nil || handle.Status != domain.HandleCompleted || handle.Metadata["artifactObservationError"] == nil {
		t.Fatalf("handle=%+v err=%v", handle, err)
	}
}

func TestAdapterPreparedWorkspaceClaimAndDependencies(t *testing.T) {
	api := &apiFake{}
	activity := domain.Activity{ID: "consumer", Command: domain.ActivityCommand{Image: "image:1"}, Metadata: map[string]any{"existing": true}}
	preparation := &domain.PreparationGate{Workspace: &domain.WorkspaceMaterialization{
		Status:      domain.MaterializationCommitted,
		Destination: domain.TransferLocation{URI: "kubernetes:///workspace?claim=run-workspace&createClaim=true&claimBytes=1024"},
	}}
	workflow := domain.WorkflowVersion{DataDependencies: []domain.ActivityDataDependency{{ProducerActivityID: "producer", ConsumerActivityID: "consumer", LogicalName: "input data.csv"}}}
	handle, err := New(api, "science").Start(context.Background(), domain.ActivityExecutionContext{
		Run: domain.ExecutionRun{ID: "run"}, Activity: activity, Workflow: workflow,
		Preparation: preparation, RuntimeID: "kubernetes",
	})
	if err != nil {
		t.Fatal(err)
	}
	if handle.Metadata["artifactObservationRoot"] != "/workspace" || api.created["persistentvolumeclaims"] == nil || api.created["jobs"] == nil {
		t.Fatalf("handle=%+v created=%+v", handle, api.created)
	}
	var claim map[string]any
	if err := json.Unmarshal(api.created["persistentvolumeclaims"], &claim); err != nil {
		t.Fatal(err)
	}
	storage := claim["spec"].(map[string]any)["resources"].(map[string]any)["requests"].(map[string]any)["storage"]
	if storage != "67108864" {
		t.Fatalf("minimum claim size=%v", storage)
	}
	prepared := withPreparedWorkspace(activity, preparation)
	spec := observedPodSpec(workflow, prepared, domain.Resource{}, "run")
	container := spec["containers"].([]any)[0].(map[string]any)
	environment := container["env"].([]map[string]string)
	foundInput := false
	for _, variable := range environment {
		if variable["name"] == "AKOFLOW_INPUT_INPUT_DATA_CSV" && strings.Contains(variable["value"], "/producer/input data.csv") {
			foundInput = true
		}
	}
	if !foundInput {
		t.Fatalf("environment=%+v", environment)
	}
}

func TestPreparedWorkspaceValidation(t *testing.T) {
	activity := domain.Activity{ID: "activity"}
	if claim, name, err := preparedWorkspaceClaim(activity, nil, "default", "run"); err != nil || claim != nil || name != "" {
		t.Fatalf("claim=%q name=%q err=%v", claim, name, err)
	}
	unchanged := withPreparedWorkspace(activity, &domain.PreparationGate{Workspace: &domain.WorkspaceMaterialization{Destination: domain.TransferLocation{URI: "file:///tmp"}}})
	if unchanged.Metadata != nil {
		t.Fatalf("activity changed=%+v", unchanged)
	}
	preparation := &domain.PreparationGate{Workspace: &domain.WorkspaceMaterialization{Destination: domain.TransferLocation{URI: "kubernetes:///workspace?createClaim=true"}}}
	if _, _, err := preparedWorkspaceClaim(activity, preparation, "default", "run"); err == nil {
		t.Fatal("claim name is required")
	}
}

func TestAdapterModesStopAndStartValidation(t *testing.T) {
	adapter := New(nil, "")
	if len(adapter.Modes()) != 2 || adapter.namespace != "default" {
		t.Fatalf("adapter=%+v modes=%+v", adapter, adapter.Modes())
	}
	if _, err := adapter.Start(context.Background(), domain.ActivityExecutionContext{}); err == nil {
		t.Fatal("nil API must fail")
	}
	if _, err := New(&apiFake{}, "default").Start(context.Background(), domain.ActivityExecutionContext{Activity: domain.Activity{ID: "activity"}}); err == nil {
		t.Fatal("missing image must fail")
	}
	api := &apiFake{deleteErr: map[string]error{"jobs": ErrNotFound}}
	if err := New(api, "default").Stop(context.Background(), domain.ActivityHandle{ExternalID: "job"}); err != nil || len(api.deleted) != 2 {
		t.Fatalf("deleted=%+v err=%v", api.deleted, err)
	}
	api.deleteErr = map[string]error{"jobs": errors.New("delete failed")}
	if err := New(api, "default").Stop(context.Background(), domain.ActivityHandle{ExternalID: "job"}); err == nil {
		t.Fatal("delete failure expected")
	}
}

func TestValidateOwnershipVariants(t *testing.T) {
	for _, test := range []struct {
		name, resource, payload string
		wantErr                 bool
	}{
		{"malformed", "jobs", `{`, true},
		{"not managed", "jobs", `{}`, true},
		{"activity owner", "jobs", `{"metadata":{"labels":{"app.kubernetes.io/managed-by":"akoflow"},"annotations":{"akoflow.io/activity-id":"other"}}}`, true},
		{"service", "services", `{"metadata":{"labels":{"app.kubernetes.io/managed-by":"akoflow"}},"spec":{"selector":{"akoflow.io/activity":"activity"}}}`, false},
		{"claim", "persistentvolumeclaims", `{"metadata":{"labels":{"app.kubernetes.io/managed-by":"akoflow"},"annotations":{"akoflow.io/activity-id":"activity"}}}`, false},
		{"legacy claim", "persistentvolumeclaims", `{"metadata":{"labels":{"app.kubernetes.io/managed-by":"akoflow","akoflow.io/purpose":"workspace-transfer"}}}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateOwnership([]byte(test.payload), test.resource, "run", "activity")
			if (err != nil) != test.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, test.wantErr)
			}
		})
	}
}
