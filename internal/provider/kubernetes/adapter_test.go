package kubernetes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type apiFake struct {
	created    map[string][]byte
	getOutput  []byte
	listOutput []byte
	logsOutput []byte
}

func (f *apiFake) Create(_ context.Context, _, resource string, body []byte) error {
	if f.created == nil {
		f.created = map[string][]byte{}
	}
	f.created[resource] = body
	return nil
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
func (f *apiFake) Delete(context.Context, string, string, string) error { return nil }

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
		manifest.Files[0].Checksum == "" {
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
