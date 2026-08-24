package kubernetes

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	runtimecommon "github.com/UFFeScience/akoflow/internal/provider"
)

const (
	activityContainer = "activity"
	manifestLogPrefix = "AKOFLOW_ARTIFACT_MANIFEST="
)

type Adapter struct {
	api       API
	namespace string
}

func New(api API, namespace string) *Adapter {
	if namespace == "" {
		namespace = "default"
	}
	return &Adapter{api: api, namespace: namespace}
}

func (*Adapter) Modes() []domain.ExecutionMode {
	return []domain.ExecutionMode{domain.ExecutionModeReal, domain.ExecutionModeInteractive}
}

func (a *Adapter) Start(ctx context.Context, execution domain.ActivityExecutionContext) (domain.ActivityHandle, error) {
	if a.api == nil {
		return domain.ActivityHandle{}, fmt.Errorf("Kubernetes API client is required")
	}
	activity := execution.Activity
	if activity.Command.Image == "" {
		return domain.ActivityHandle{}, fmt.Errorf("activity image is required for Kubernetes")
	}
	name := kubernetesName("akoflow-" + execution.Run.ID + "-" + activity.ID)
	job, service, err := resources(
		name, a.namespace, execution.Workflow, activity, execution.Resource, execution.Run.ID,
	)
	if err != nil {
		return domain.ActivityHandle{}, err
	}
	if err := a.api.Create(ctx, a.namespace, "jobs", job); err != nil {
		return domain.ActivityHandle{}, err
	}
	if service != nil {
		if err := a.api.Create(ctx, a.namespace, "services", service); err != nil {
			_ = a.api.Delete(ctx, a.namespace, "jobs", name)
			return domain.ActivityHandle{}, err
		}
	}
	submittedAt := runtimecommon.UnixSeconds(time.Now())
	return domain.ActivityHandle{ID: runtimecommon.NewID("activity"), RunID: execution.Run.ID,
		ActivityID: activity.ID, ResourceID: execution.Resource.ID,
		RuntimeID: execution.RuntimeID, ExternalID: name,
		Status: domain.HandleStarting, StartedAt: submittedAt,
		Endpoints: serviceEndpoints(name, a.namespace, activity),
		Metadata: map[string]any{
			domain.TimingSubmittedAt:    submittedAt,
			"artifactObservationDriver": "filesystem-diff",
			"artifactObservationRoot":   observationRoot(activity, execution.Run.ID),
			"artifactStorageType":       storageBindingFor(activity).Type,
			"artifactStorageResourceId": storageBindingFor(activity).ResourceID,
		}}, nil
}

func (a *Adapter) Inspect(ctx context.Context, handle domain.ActivityHandle) (domain.ActivityHandle, error) {
	output, err := a.api.Get(ctx, a.namespace, "jobs", handle.ExternalID)
	if err != nil {
		return handle, err
	}
	var job struct {
		Status struct {
			Active, Succeeded, Failed int
			Conditions                []struct{ Type, Status, Message string }
		}
	}
	if err := json.Unmarshal(output, &job); err != nil {
		return handle, fmt.Errorf("decode Kubernetes job: %w", err)
	}
	a.applyPodTiming(ctx, handle.ExternalID, &handle)
	switch {
	case job.Status.Succeeded > 0:
		handle.Status = domain.HandleCompleted
	case job.Status.Failed > 0:
		handle.Status = domain.HandleFailed
	case job.Status.Active > 0:
		handle.Status = domain.HandleRunning
	default:
		handle.Status = domain.HandleStarting
	}
	if handle.Status == domain.HandleCompleted || handle.Status == domain.HandleFailed {
		handle.FinishedAt = runtimecommon.UnixSeconds(time.Now())
	}
	for _, condition := range job.Status.Conditions {
		if condition.Type == "Failed" && condition.Status == "True" {
			handle.Failure = condition.Message
		}
	}
	if handle.Status == domain.HandleFailed {
		if failure := a.activityFailure(ctx, handle.ExternalID); failure != "" {
			handle.Failure = failure
		}
	}
	if log, logErr := a.activityLog(ctx, handle.ExternalID); logErr == nil {
		handle.Log = string(log)
	}
	if handle.Status == domain.HandleCompleted || handle.Status == domain.HandleFailed {
		manifest, observationErr := a.collectArtifacts(ctx, handle.ExternalID)
		if observationErr != nil {
			if handle.Metadata == nil {
				handle.Metadata = make(map[string]any)
			}
			handle.Metadata["artifactObservationError"] = observationErr.Error()
		} else {
			handle.Artifacts = manifest
		}
	}
	return handle, nil
}

func (a *Adapter) applyPodTiming(ctx context.Context, jobName string, handle *domain.ActivityHandle) {
	payload, err := a.api.List(ctx, a.namespace, "pods", "job-name="+jobName)
	if err != nil {
		return
	}
	var pods struct {
		Items []struct {
			Status struct {
				StartTime         *time.Time `json:"startTime"`
				ContainerStatuses []struct {
					Name  string `json:"name"`
					State struct {
						Running *struct {
							StartedAt time.Time `json:"startedAt"`
						} `json:"running"`
					} `json:"state"`
				} `json:"containerStatuses"`
			} `json:"status"`
		} `json:"items"`
	}
	if json.Unmarshal(payload, &pods) != nil || len(pods.Items) == 0 {
		return
	}
	status := pods.Items[0].Status
	if status.StartTime != nil && !status.StartTime.IsZero() {
		handle.StartedAt = runtimecommon.UnixSeconds(*status.StartTime)
	}
	for _, container := range status.ContainerStatuses {
		if container.Name != activityContainer || container.State.Running == nil || container.State.Running.StartedAt.IsZero() {
			continue
		}
		if handle.Metadata == nil {
			handle.Metadata = make(map[string]any)
		}
		handle.Metadata[domain.TimingContainerStartedAt] = runtimecommon.UnixSeconds(container.State.Running.StartedAt)
		return
	}
}

func (a *Adapter) activityLog(ctx context.Context, jobName string) ([]byte, error) {
	payload, err := a.api.List(ctx, a.namespace, "pods", "job-name="+jobName)
	if err != nil {
		return nil, err
	}
	var pods struct {
		Items []struct{ Metadata struct{ Name string } }
	}
	if err := json.Unmarshal(payload, &pods); err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("activity pod not found")
	}
	return a.api.Logs(ctx, a.namespace, pods.Items[0].Metadata.Name, activityContainer)
}

func (a *Adapter) activityFailure(ctx context.Context, jobName string) string {
	payload, err := a.api.List(ctx, a.namespace, "pods", "job-name="+jobName)
	if err != nil {
		return ""
	}
	var pods struct {
		Items []struct {
			Status struct {
				ContainerStatuses []struct {
					State struct {
						Waiting, Terminated *struct{ Reason, Message string }
					}
				}
			}
		}
	}
	if json.Unmarshal(payload, &pods) != nil {
		return ""
	}
	for _, pod := range pods.Items {
		for _, status := range pod.Status.ContainerStatuses {
			for _, state := range []*struct{ Reason, Message string }{status.State.Waiting, status.State.Terminated} {
				if state == nil {
					continue
				}
				message := strings.TrimSpace(state.Message)
				if strings.Contains(message, "/bin/sh") && strings.Contains(strings.ToLower(message), "no such file") {
					return "activity image is incompatible with the Kubernetes shell runtime: /bin/sh is required"
				}
				if message != "" {
					return state.Reason + ": " + message
				}
			}
		}
	}
	return ""
}

func (a *Adapter) collectArtifacts(ctx context.Context, jobName string) (*domain.ArtifactManifest, error) {
	payload, err := a.api.List(ctx, a.namespace, "pods", "job-name="+jobName)
	if err != nil {
		return nil, fmt.Errorf("list activity pods: %w", err)
	}
	var pods struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := json.Unmarshal(payload, &pods); err != nil {
		return nil, fmt.Errorf("decode activity pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pod found for job %q", jobName)
	}
	logs, err := a.api.Logs(ctx, a.namespace, pods.Items[0].Metadata.Name, activityContainer)
	if err != nil {
		return nil, fmt.Errorf("read artifact observer logs: %w", err)
	}
	for _, line := range strings.Split(string(logs), "\n") {
		prefixAt := strings.Index(line, manifestLogPrefix)
		if prefixAt < 0 {
			continue
		}
		encoded := strings.TrimSpace(line[prefixAt+len(manifestLogPrefix):])
		manifestJSON, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode artifact manifest: %w", err)
		}
		var manifest domain.ArtifactManifest
		if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
			return nil, fmt.Errorf("parse artifact manifest: %w", err)
		}
		return &manifest, nil
	}
	return nil, fmt.Errorf("artifact manifest was not published")
}

func (a *Adapter) Stop(ctx context.Context, handle domain.ActivityHandle) error {
	jobErr := ignoreNotFound(a.api.Delete(ctx, a.namespace, "jobs", handle.ExternalID))
	serviceErr := ignoreNotFound(a.api.Delete(ctx, a.namespace, "services", handle.ExternalID))
	return errors.Join(jobErr, serviceErr)
}

func resources(
	name string,
	namespace string,
	workflow domain.WorkflowVersion,
	activity domain.Activity,
	resource domain.Resource,
	runID string,
) ([]byte, []byte, error) {
	podSpec := observedPodSpec(workflow, activity, resource, runID)
	job := map[string]any{"apiVersion": "batch/v1", "kind": "Job",
		"metadata": map[string]any{"name": name, "namespace": namespace,
			"labels": map[string]string{"app.kubernetes.io/managed-by": "akoflow"}},
		"spec": map[string]any{"backoffLimit": max(activity.Policy.MaxAttempts-1, 0),
			"template": map[string]any{
				"metadata": map[string]any{"labels": map[string]string{"akoflow.io/activity": activity.ID}},
				"spec":     podSpec,
			}}}
	jobJSON, err := json.Marshal(job)
	if err != nil {
		return nil, nil, err
	}
	if activity.Service == nil || len(activity.Service.Ports) == 0 {
		return jobJSON, nil, nil
	}
	ports := make([]map[string]any, 0, len(activity.Service.Ports))
	for _, port := range activity.Service.Ports {
		ports = append(ports, map[string]any{"port": port, "targetPort": port})
	}
	service := map[string]any{"apiVersion": "v1", "kind": "Service",
		"metadata": map[string]any{"name": name, "namespace": namespace,
			"labels": map[string]string{"app.kubernetes.io/managed-by": "akoflow"}},
		"spec": map[string]any{"selector": map[string]string{"akoflow.io/activity": activity.ID}, "ports": ports}}
	serviceJSON, err := json.Marshal(service)
	return jobJSON, serviceJSON, err
}

func observedPodSpec(
	workflow domain.WorkflowVersion,
	activity domain.Activity,
	resource domain.Resource,
	runID string,
) map[string]any {
	environment := make([]map[string]string, 0, len(activity.Command.Environment))
	for key, value := range activity.Command.Environment {
		environment = append(environment, map[string]string{"name": key, "value": value})
	}
	environment = append(environment,
		map[string]string{"name": "AKOFLOW_RUN_ID", "value": runID},
		map[string]string{"name": "AKOFLOW_ACTIVITY_ID", "value": activity.ID},
		map[string]string{"name": "AKOFLOW_OBSERVATION_ROOT", "value": observationRoot(activity, runID)},
	)
	if binding := storageBindingFor(activity); binding.Type == "pvc" || binding.Type == "nfs" {
		environment = append(environment,
			map[string]string{"name": "AKOFLOW_RUN_DATA_ROOT", "value": path.Join(binding.MountPath, "runs", runID)},
			map[string]string{"name": "AKOFLOW_ACTIVITY_DATA_ROOT", "value": observationRoot(activity, runID)},
		)
		for _, dependency := range workflow.DataDependencies {
			if dependency.ConsumerActivityID != activity.ID || dependency.LogicalName == "" {
				continue
			}
			environment = append(environment, map[string]string{
				"name":  inputEnvironmentName(dependency.LogicalName),
				"value": path.Join(binding.MountPath, "runs", runID, dependency.ProducerActivityID, dependency.LogicalName),
			})
		}
	}
	arguments := []string{
		"-c", renderShellLifecycle(), "akoflow-entrypoint", activity.Command.Entrypoint,
	}
	arguments = append(arguments, activity.Command.Arguments...)
	container := map[string]any{"name": activityContainer, "image": activity.Command.Image,
		"command": []string{"/bin/sh"}, "args": arguments,
		"env": environment,
		"resources": map[string]any{"requests": map[string]string{
			"cpu":    fmt.Sprintf("%g", activity.Resources.CPU),
			"memory": fmt.Sprintf("%d", activity.Resources.MemoryBytes)}}}
	binding := storageBindingFor(activity)
	if binding.Type != "" {
		container["volumeMounts"] = []map[string]any{{
			"name": "akoflow-data", "mountPath": binding.MountPath, "readOnly": binding.ReadOnly,
		}}
	}
	podSpec := map[string]any{
		"restartPolicy": "Never",
		"containers":    []any{container},
	}
	switch binding.Type {
	case "pvc":
		podSpec["volumes"] = []map[string]any{{"name": "akoflow-data",
			"persistentVolumeClaim": map[string]any{"claimName": binding.ClaimName, "readOnly": binding.ReadOnly}}}
	case "nfs":
		podSpec["volumes"] = []map[string]any{{"name": "akoflow-data",
			"nfs": map[string]any{"server": binding.Server, "path": binding.Path, "readOnly": binding.ReadOnly}}}
	}
	if resource.Type == domain.ResourceKubernetesMachine && resource.ProviderID != "" {
		podSpec["nodeSelector"] = map[string]string{
			"kubernetes.io/hostname": resource.ProviderID,
		}
	}
	return podSpec
}

func inputEnvironmentName(logicalName string) string {
	name := strings.Map(func(character rune) rune {
		if character >= 'a' && character <= 'z' {
			return character - ('a' - 'A')
		}
		if character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' {
			return character
		}
		return '_'
	}, logicalName)
	return "AKOFLOW_INPUT_" + strings.Trim(name, "_")
}

const manifestPrefixPlaceholder = "__AKOFLOW_MANIFEST_PREFIX__"

//go:embed scripts/lifecycle.sh
var shellLifecycleTemplate string

func renderShellLifecycle() string {
	return strings.ReplaceAll(shellLifecycleTemplate, manifestPrefixPlaceholder, manifestLogPrefix)
}

type storageBinding struct {
	Type, ResourceID, ClaimName, Server, Path, MountPath string
	ReadOnly                                             bool
}

func storageBindingFor(activity domain.Activity) storageBinding {
	value, ok := activity.Metadata["storage"].(map[string]any)
	if !ok {
		return storageBinding{}
	}
	binding := storageBinding{
		Type: stringValue(value["type"]), ResourceID: stringValue(value["resourceId"]),
		ClaimName: stringValue(value["claimName"]),
		Server:    stringValue(value["server"]), Path: stringValue(value["path"]),
		MountPath: stringValue(value["mountPath"]),
	}
	binding.ReadOnly, _ = value["readOnly"].(bool)
	if binding.MountPath == "" {
		binding.MountPath = "/akoflow/data"
	}
	return binding
}

func observationRoot(activity domain.Activity, runID string) string {
	if configured, ok := activity.Metadata["artifactObservationRoot"].(string); ok && configured != "" {
		return configured
	}
	if binding := storageBindingFor(activity); binding.Type == "pvc" || binding.Type == "nfs" {
		return path.Join(binding.MountPath, "runs", runID, activity.ID)
	}
	if activity.Command.WorkingDirectory != "" {
		return activity.Command.WorkingDirectory
	}
	return "/tmp/akoflow/workspace"
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func ignoreNotFound(err error) error {
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func kubernetesName(value string) string {
	value = strings.ToLower(value)
	value = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '-'
	}, value)
	value = strings.Trim(value, "-")
	if len(value) > 63 {
		value = value[:63]
	}
	return strings.Trim(value, "-")
}

func serviceEndpoints(name, namespace string, activity domain.Activity) []string {
	if activity.Service == nil {
		return nil
	}
	result := make([]string, 0, len(activity.Service.Ports))
	for _, port := range activity.Service.Ports {
		result = append(result, fmt.Sprintf("tcp://%s.%s.svc:%d", name, namespace, port))
	}
	return result
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
