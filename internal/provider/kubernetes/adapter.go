package kubernetes

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	runtimecommon "github.com/UFFeScience/akoflow/internal/provider"
)

const (
	activityContainer = "activity"
	manifestLogPrefix = "AKOFLOW_ARTIFACT_MANIFEST="
)

var kubernetesLabelValue = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9_.-]{0,61}[A-Za-z0-9])?$`)

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
	activity = withPreparedWorkspace(activity, execution.Preparation)
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
	claim, claimName, err := preparedWorkspaceClaim(activity, execution.Preparation, a.namespace, execution.Run.ID)
	if err != nil {
		return domain.ActivityHandle{}, err
	}
	claimCreated := false
	if claim != nil {
		claimCreated, err = a.createOrAdopt(ctx, "persistentvolumeclaims", claimName, claim, execution.Run.ID, activity.ID)
		if err != nil {
			return domain.ActivityHandle{}, err
		}
	}
	jobCreated, err := a.createOrAdopt(ctx, "jobs", name, job, execution.Run.ID, activity.ID)
	if err != nil {
		if claimCreated {
			_ = a.api.Delete(ctx, a.namespace, "persistentvolumeclaims", claimName)
		}
		return domain.ActivityHandle{}, err
	}
	if service != nil {
		if _, err := a.createOrAdopt(ctx, "services", name, service, execution.Run.ID, activity.ID); err != nil {
			if jobCreated {
				_ = a.api.Delete(ctx, a.namespace, "jobs", name)
			}
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

func preparedWorkspaceClaim(activity domain.Activity, preparation *domain.PreparationGate, namespace, runID string) ([]byte, string, error) {
	if preparation == nil || preparation.Workspace == nil {
		return nil, "", nil
	}
	u, err := url.Parse(preparation.Workspace.Destination.URI)
	if err != nil || u.Scheme != "kubernetes" || u.Query().Get("createClaim") != "true" {
		return nil, "", err
	}
	claimName := u.Query().Get("claim")
	if claimName == "" {
		return nil, "", fmt.Errorf("prepared Kubernetes workspace has no claim")
	}
	claimBytes, _ := strconv.ParseInt(u.Query().Get("claimBytes"), 10, 64)
	if claimBytes < 64<<20 {
		claimBytes = 64 << 20
	}
	claim := map[string]any{
		"apiVersion": "v1", "kind": "PersistentVolumeClaim",
		"metadata": map[string]any{
			"name": claimName, "namespace": namespace,
			"labels":      map[string]string{"app.kubernetes.io/managed-by": "akoflow"},
			"annotations": map[string]string{"akoflow.io/run-id": runID, "akoflow.io/activity-id": activity.ID},
		},
		"spec": map[string]any{
			"accessModes": []string{"ReadWriteOnce"},
			"resources":   map[string]any{"requests": map[string]string{"storage": strconv.FormatInt(claimBytes, 10)}},
		},
	}
	payload, err := json.Marshal(claim)
	return payload, claimName, err
}

func withPreparedWorkspace(activity domain.Activity, preparation *domain.PreparationGate) domain.Activity {
	if preparation == nil || preparation.Workspace == nil {
		return activity
	}
	u, err := url.Parse(preparation.Workspace.Destination.URI)
	if err != nil || u.Scheme != "kubernetes" || u.Query().Get("claim") == "" {
		return activity
	}
	metadata := make(map[string]any, len(activity.Metadata)+2)
	for key, value := range activity.Metadata {
		metadata[key] = value
	}
	metadata["storage"] = map[string]any{
		"type": "pvc", "claimName": u.Query().Get("claim"), "mountPath": u.Path,
	}
	metadata["artifactObservationRoot"] = u.Path
	activity.Metadata = metadata
	return activity
}

// createOrAdopt makes submission idempotent across a process crash between the
// Kubernetes create call and persistence of the activity handle.
func (a *Adapter) createOrAdopt(
	ctx context.Context,
	resource, name string,
	body []byte,
	runID, activityID string,
) (bool, error) {
	if err := a.api.Create(ctx, a.namespace, resource, body); err == nil {
		return true, nil
	} else if !errors.Is(err, ErrConflict) {
		return false, err
	}

	existing, err := a.api.Get(ctx, a.namespace, resource, name)
	if err != nil {
		return false, fmt.Errorf("inspect conflicting Kubernetes %s %q: %w", resource, name, err)
	}
	if err := validateOwnership(existing, resource, runID, activityID); err != nil {
		return false, fmt.Errorf("Kubernetes %s %q already exists but cannot be adopted: %w", resource, name, err)
	}
	return false, nil
}

func validateOwnership(payload []byte, resource, runID, activityID string) error {
	var object struct {
		Metadata struct {
			Labels      map[string]string `json:"labels"`
			Annotations map[string]string `json:"annotations"`
		} `json:"metadata"`
		Spec struct {
			Selector json.RawMessage `json:"selector"`
			Template struct {
				Metadata struct {
					Labels map[string]string `json:"labels"`
				} `json:"metadata"`
			} `json:"template"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(payload, &object); err != nil {
		return fmt.Errorf("decode existing resource: %w", err)
	}
	if object.Metadata.Labels["app.kubernetes.io/managed-by"] != "akoflow" {
		return fmt.Errorf("resource is not managed by AkôFlow")
	}
	if owner := object.Metadata.Annotations["akoflow.io/run-id"]; owner != "" && owner != runID {
		return fmt.Errorf("run owner is %q, expected %q", owner, runID)
	}
	if owner := object.Metadata.Annotations["akoflow.io/activity-id"]; owner != "" && owner != activityID {
		return fmt.Errorf("activity owner is %q, expected %q", owner, activityID)
	}

	// Legacy resources only carried the activity label in the pod template or
	// Service selector. Accept those so in-flight runs survive this upgrade.
	actualActivity := object.Spec.Template.Metadata.Labels["akoflow.io/activity"]
	if resource == "services" {
		var selector map[string]string
		if err := json.Unmarshal(object.Spec.Selector, &selector); err != nil {
			return fmt.Errorf("decode existing Service selector: %w", err)
		}
		actualActivity = selector["akoflow.io/activity"]
	}
	if resource == "persistentvolumeclaims" && object.Metadata.Annotations["akoflow.io/activity-id"] == activityID {
		return nil
	}
	if resource == "persistentvolumeclaims" &&
		object.Metadata.Labels["akoflow.io/purpose"] == "workspace-transfer" &&
		object.Metadata.Annotations["akoflow.io/run-id"] == "" {
		// Compatibility with claims created by the streaming connector before
		// ownership annotations were added. The deterministic claim name is
		// already checked by the caller.
		return nil
	}
	if actualActivity != activityID {
		return fmt.Errorf("activity label is %q, expected %q", actualActivity, activityID)
	}
	return nil
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
	ownership := map[string]string{"akoflow.io/run-id": runID, "akoflow.io/activity-id": activity.ID}
	job := map[string]any{"apiVersion": "batch/v1", "kind": "Job",
		"metadata": map[string]any{"name": name, "namespace": namespace,
			"labels": map[string]string{"app.kubernetes.io/managed-by": "akoflow"}, "annotations": ownership},
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
			"labels": map[string]string{"app.kubernetes.io/managed-by": "akoflow"}, "annotations": ownership},
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
	if nodeName := kubernetesNodeName(resource); nodeName != "" {
		podSpec["nodeSelector"] = map[string]string{
			"kubernetes.io/hostname": nodeName,
		}
	}
	return podSpec
}

func kubernetesNodeName(resource domain.Resource) string {
	if resource.Type != domain.ResourceKubernetesMachine {
		return ""
	}
	if name, _ := resource.Metadata["observedHostname"].(string); validKubernetesLabelValue(name) {
		return name
	}
	if validKubernetesLabelValue(resource.ProviderID) {
		return resource.ProviderID
	}
	return ""
}

func validKubernetesLabelValue(value string) bool {
	value = strings.TrimSpace(value)
	return len(value) <= 63 && kubernetesLabelValue.MatchString(value)
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
		digest := sha256.Sum256([]byte(value))
		value = strings.Trim(value[:54], "-") + fmt.Sprintf("-%x", digest[:4])
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
