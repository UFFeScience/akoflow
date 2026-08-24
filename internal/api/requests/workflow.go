package requests

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/UFFeScience/akoflow/internal/domain"
	"gopkg.in/yaml.v3"
)

type Workflow struct {
	Name string       `json:"name"`
	Spec WorkflowSpec `json:"spec"`
}

// FromDomain converts a persisted workflow back to the portable authoring
// format accepted by Domain. It deliberately omits generated IDs and runtime
// resolution state, so an exported file can be imported as a new definition.
func FromDomain(definition domain.WorkflowDefinition) Workflow {
	dependencies := make(map[string][]string)
	for _, dependency := range definition.Version.Dependencies {
		dependencies[dependency.ActivityID] = append(dependencies[dependency.ActivityID], dependency.DependsOnActivityID)
	}
	activityNames := make(map[string]string, len(definition.Version.Activities))
	for _, activity := range definition.Version.Activities {
		activityNames[activity.ID] = activity.Name
	}
	request := Workflow{Name: definition.Name, Spec: WorkflowSpec{Namespace: definition.Namespace}}
	for _, activity := range definition.Version.Activities {
		command := activity.Command
		command.ResolvedExecutable = nil
		predecessors := make([]string, 0, len(dependencies[activity.ID]))
		for _, predecessorID := range dependencies[activity.ID] {
			if name := activityNames[predecessorID]; name != "" {
				predecessors = append(predecessors, name)
			}
		}
		metadata := activity.Metadata
		item := WorkflowActivity{
			Name: activity.Name, Command: command,
			CPULimit:    strconv.FormatFloat(activity.Resources.CPU, 'f', -1, 64),
			MemoryLimit: strconv.FormatInt(activity.Resources.MemoryBytes, 10),
			DependsOn:   predecessors,
		}
		if metadata != nil {
			item.Runtime, _ = metadata["runtime"].(string)
			item.ResourceSelector, _ = metadata["resourceSelector"].(string)
			item.KeepDisk, _ = metadata["keepDisk"].(bool)
			item.MountPath, _ = metadata["mountPath"].(string)
		}
		request.Spec.Activities = append(request.Spec.Activities, item)
	}
	return request
}

// YAML serializes through JSON keys first, which keeps the portable document
// stable even for nested domain types that only declare json tags.
func (request Workflow) YAML() ([]byte, error) {
	encoded, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	var document any
	if err := json.Unmarshal(encoded, &document); err != nil {
		return nil, err
	}
	return yaml.Marshal(document)
}

type WorkflowSpec struct {
	Namespace        string             `json:"namespace"`
	Image            string             `json:"image,omitempty"`
	StorageClassName string             `json:"storageClassName,omitempty"`
	StorageSize      string             `json:"storageSize,omitempty"`
	StoragePolicy    StoragePolicy      `json:"storagePolicy,omitempty"`
	MountPath        string             `json:"mountPath,omitempty"`
	Activities       []WorkflowActivity `json:"activities"`
}

type StoragePolicy struct {
	Type string `json:"type,omitempty"`
}

type WorkflowActivity struct {
	Name             string                 `json:"name"`
	Command          domain.ActivityCommand `json:"command"`
	Image            string                 `json:"image,omitempty"`
	Runtime          string                 `json:"runtime,omitempty"`
	Run              string                 `json:"run,omitempty"`
	MemoryLimit      string                 `json:"memoryLimit,omitempty"`
	CPULimit         string                 `json:"cpuLimit,omitempty"`
	DependsOn        []string               `json:"dependsOn,omitempty"`
	ResourceSelector string                 `json:"resourceSelector,omitempty"`
	KeepDisk         bool                   `json:"keepDisk,omitempty"`
	MountPath        string                 `json:"mountPath,omitempty"`
}

func (request Workflow) Domain() (domain.WorkflowDefinition, error) {
	workflowID := identifier(request.Name)
	if workflowID == "" || request.Spec.Namespace == "" {
		return domain.WorkflowDefinition{}, fmt.Errorf("workflow name and spec.namespace are required")
	}
	versionID := workflowID + "-v1"
	typeID := workflowID + "-activity"
	definition := domain.WorkflowDefinition{
		ID: workflowID, ExternalID: workflowID, Name: request.Name, Namespace: request.Spec.Namespace,
		Types:   []domain.ActivityType{{ID: typeID, Name: request.Name + " activity", DefaultImage: request.Spec.Image}},
		Version: domain.WorkflowVersion{ID: versionID, WorkflowID: workflowID, Version: 1, DefinitionHash: versionID},
	}
	names := make(map[string]string, len(request.Spec.Activities))
	for _, value := range request.Spec.Activities {
		id := identifier(value.Name)
		if id == "" {
			return domain.WorkflowDefinition{}, fmt.Errorf("activity name is required")
		}
		if _, exists := names[value.Name]; exists {
			return domain.WorkflowDefinition{}, fmt.Errorf("duplicate activity %q", value.Name)
		}
		names[value.Name] = id
	}
	for index, value := range request.Spec.Activities {
		activity, err := activityDomain(value, request.Spec.Image, versionID, typeID, index)
		if err != nil {
			return domain.WorkflowDefinition{}, err
		}
		definition.Version.Activities = append(definition.Version.Activities, activity)
		for _, predecessor := range value.DependsOn {
			predecessorID, exists := names[predecessor]
			if !exists {
				return domain.WorkflowDefinition{}, fmt.Errorf("activity %q depends on unknown activity %q", value.Name, predecessor)
			}
			definition.Version.Dependencies = append(definition.Version.Dependencies, domain.ActivityDependency{
				ActivityID: activity.ID, DependsOnActivityID: predecessorID, Type: "control",
			})
		}
	}
	return definition, nil
}

func activityDomain(value WorkflowActivity, defaultImage, versionID, typeID string, index int) (domain.Activity, error) {
	cpu, err := cpuValue(value.CPULimit)
	if err != nil {
		return domain.Activity{}, fmt.Errorf("activity %q cpuLimit: %w", value.Name, err)
	}
	memory, err := byteValue(value.MemoryLimit)
	if err != nil {
		return domain.Activity{}, fmt.Errorf("activity %q memoryLimit: %w", value.Name, err)
	}
	command := value.Command
	if command.Entrypoint == "" && value.Run != "" {
		command.Entrypoint = "sh"
		command.Arguments = []string{"-c", value.Run}
	}
	if command.Executable == nil {
		image := value.Image
		if image == "" {
			image = defaultImage
		}
		if image != "" {
			command.Executable = &domain.ExecutableReference{
				Source:   domain.ExecutableSource{Type: domain.ExecutableSourceOCI, Reference: image},
				Delivery: domain.ExecutableDelivery{Strategy: domain.DeliveryAuto},
			}
		}
	}
	if command.Entrypoint == "" || command.Executable == nil {
		return domain.Activity{}, fmt.Errorf("activity %q command.entrypoint and command.executable are required", value.Name)
	}
	if err := command.Executable.Validate(); err != nil {
		return domain.Activity{}, fmt.Errorf("activity %q: %w", value.Name, err)
	}
	return domain.Activity{
		ID: identifier(value.Name), WorkflowVersionID: versionID, ActivityTypeID: typeID,
		ExternalID: identifier(value.Name), Name: value.Name, Kind: domain.ActivityKindTask,
		Capabilities: []domain.ActivityCapability{domain.ActivityCapabilityReal},
		Command:      command,
		Resources:    domain.ActivityResources{CPU: cpu, MemoryBytes: memory},
		Policy:       domain.ActivityPolicy{TimeoutSeconds: 3600, MaxAttempts: 1}, Priority: len(value.DependsOn) + index,
		Metadata: map[string]any{"runtime": value.Runtime, "resourceSelector": value.ResourceSelector,
			"keepDisk": value.KeepDisk, "mountPath": value.MountPath},
	}, nil
}

func identifier(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.Trim(strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			return r
		}
		return '-'
	}, value), "-")
}

func cpuValue(value string) (float64, error) {
	if value == "" {
		return 0.1, nil
	}
	if strings.HasSuffix(value, "m") {
		milli, err := strconv.ParseFloat(strings.TrimSuffix(value, "m"), 64)
		return milli / 1000, err
	}
	return strconv.ParseFloat(value, 64)
}

func byteValue(value string) (int64, error) {
	if value == "" {
		return 16 * 1024 * 1024, nil
	}
	multipliers := map[string]float64{"Ki": 1024, "Mi": 1024 * 1024, "Gi": 1024 * 1024 * 1024}
	for suffix, multiplier := range multipliers {
		if strings.HasSuffix(value, suffix) {
			number, err := strconv.ParseFloat(strings.TrimSuffix(value, suffix), 64)
			return int64(number * multiplier), err
		}
	}
	return strconv.ParseInt(value, 10, 64)
}
