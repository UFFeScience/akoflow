package hubpublish

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

type publishedEnvironment struct {
	ImagePullSeconds  float64               `json:"imagePullSeconds,omitempty"`
	ImageBuildSeconds float64               `json:"imageBuildSeconds,omitempty"`
	MachineLabel      string                `json:"machineLabel,omitempty"`
	Mode              string                `json:"mode"`
	Resources         []publishedResource   `json:"resources"`
	Attempts          []publishedAllocation `json:"attempts"`
}

type publishedResource struct {
	OS             string  `json:"os"`
	Kernel         string  `json:"kernel"`
	Number         int     `json:"number"`
	Source         string  `json:"source"`
	CPUModel       string  `json:"cpuModel,omitempty"`
	Architecture   string  `json:"architecture"`
	CPUCores       float64 `json:"cpuCores,omitempty"`
	CPUCapacity    float64 `json:"cpuCapacity,omitempty"`
	Runtime        string  `json:"runtime,omitempty"`
	RuntimeVersion string  `json:"runtimeVersion,omitempty"`
	MemoryBytes    float64 `json:"memoryBytes,omitempty"`
}

type publishedAllocation struct {
	ActivityID     string                 `json:"activityId"`
	Status         string                 `json:"status"`
	Timing         publishedAttemptTiming `json:"timing"`
	ResourceNumber int                    `json:"resourceNumber,omitempty"`
	ExecutionStyle string                 `json:"executionStyle,omitempty"`
	Attempt        float64                `json:"attempt"`
	AllocatedGPU   *float64               `json:"allocatedGpus,omitempty"`
	AllocatedCPU   float64                `json:"allocatedCpu,omitempty"`
	AllocatedRAM   float64                `json:"allocatedMemoryBytes,omitempty"`
	RequestedCPU   float64                `json:"requestedCpu,omitempty"`
	RequestedRAM   float64                `json:"requestedMemoryBytes,omitempty"`
	RequestedGPU   float64                `json:"requestedGpus,omitempty"`
}

func executionEnvironment(context, execution map[string]any) publishedEnvironment {
	run, _ := execution["run"].(map[string]any)
	mode, _ := run["mode"].(string)
	switch mode {
	case "real", "interactive", "simulation":
	default:
		mode = "unknown"
	}
	result := publishedEnvironment{Mode: mode, Resources: []publishedResource{}, Attempts: []publishedAllocation{}}
	timeline, _ := execution["timeline"].(map[string]any)
	totals, _ := timeline["totals"].(map[string]any)
	result.ImagePullSeconds = positiveMetric(totals["imagePullSeconds"])
	result.ImageBuildSeconds = positiveMetric(totals["imageBuildSeconds"])
	timingByAttempt := preparationTiming(timeline)
	resources := indexEnvironmentRecords(context["resources"])
	workflow, _ := context["workflow"].(map[string]any)
	version, _ := workflow["version"].(map[string]any)
	activities := indexEnvironmentRecords(version["activities"])
	attempts, _ := execution["activityAttempts"].([]any)
	if len(attempts) == 0 {
		attempts, _ = execution["activities"].([]any)
	}
	runID, _ := run["id"].(string)
	handles := indexEnvironmentRecords(execution["handles"])
	type resourceKey struct{ resourceID, node, runtime string }
	numbers := map[resourceKey]int{}
	for _, entry := range attempts {
		task, _ := entry.(map[string]any)
		if positiveMetric(task["startedAt"]) == 0 {
			continue
		}
		resourceID, _ := task["allocatedResourceId"].(string)
		if resourceID == "" && mode == "simulation" {
			resourceID, _ = task["plannedResourceId"].(string)
		}
		activityID, _ := task["activityId"].(string)
		attempt := max(1, positiveMetric(task["attempt"]))
		handleID := runID + ":" + activityID
		if attempt > 1 {
			handleID += fmt.Sprintf(":attempt-%d", int(attempt))
		}
		metadata, _ := handles[handleID]["metadata"].(map[string]any)
		node, _ := metadata["allocatedNode"].(string)
		facts, runtimeName := observedRuntime(metadata)
		key := resourceKey{resourceID, node, runtimeName}
		number := numbers[key]
		if (resourceID != "" || node != "" || len(facts) > 0) && number == 0 {
			number = len(result.Resources) + 1
			numbers[key] = number
			item := resources[resourceID]
			if node != "" {
				item = nil
			}
			result.Resources = append(result.Resources, describePublishedResource(number, mode, item))
		}
		if len(facts) > 0 {
			result.Resources[number-1] = describeObservedResource(number, mode, facts, runtimeName)
		}
		style, _ := facts["executionStyle"].(string)
		if style != "native" && style != "emulated" {
			style = ""
		}
		limits, _ := metadata["dockerLimits"].(map[string]any)
		var allocatedGPU *float64
		if gpus, ok := metadata["slurmAllocatedGpus"].(float64); node != "" && ok && gpus >= 0 && gpus <= 65535 && math.Trunc(gpus) == gpus {
			allocatedGPU = &gpus
		}
		request, _ := activities[activityID]["resources"].(map[string]any)
		result.Attempts = append(result.Attempts, publishedAllocation{
			ActivityID: activityID, Status: attemptStatus(task["status"]),
			Timing:         describeAttemptTiming(task, timingByAttempt[attemptKey{activityID, attempt}]),
			ResourceNumber: number, Attempt: attempt, ExecutionStyle: style, AllocatedGPU: allocatedGPU,
			AllocatedCPU: positiveMetric(limits["cpu"]), AllocatedRAM: positiveMetric(limits["memoryBytes"]),
			RequestedCPU: positiveMetric(request["cpu"]), RequestedRAM: positiveMetric(request["memoryBytes"]),
			RequestedGPU: positiveMetric(request["gpu"]),
		})
	}
	return result
}

func observedRuntime(metadata map[string]any) (map[string]any, string) {
	for _, runtime := range []struct{ key, name string }{
		{"slurmEnvironment", "Slurm"}, {"nativeEnvironment", "Native process"}, {"dockerEnvironment", "Docker"},
	} {
		if facts, _ := metadata[runtime.key].(map[string]any); len(facts) > 0 {
			return facts, runtime.name
		}
	}
	return nil, ""
}

func describeObservedResource(number int, mode string, facts map[string]any, runtimeName string) publishedResource {
	observation := describePublishedResource(number, mode, facts)
	observation.Source = "observed"
	observation.Runtime = runtimeName
	switch facts["os"] {
	case "linux", "darwin", "windows", "freebsd":
		observation.OS = facts["os"].(string)
	}
	if kernel, _ := facts["kernel"].(string); linuxKernel.MatchString(kernel) {
		observation.Kernel = linuxKernel.FindString(kernel)
	}
	if model, _ := facts["cpuModel"].(string); runtimeName != "Docker" && cpuModel.MatchString(model) {
		observation.CPUModel = model
	}
	if version, _ := facts["version"].(string); runtimeName == "Docker" && dockerVersion.MatchString(version) {
		observation.RuntimeVersion = version
	}
	return observation
}

var linuxKernel = regexp.MustCompile(`^[0-9]{1,5}\.[0-9]{1,5}\.[0-9]{1,5}(-[0-9]{1,5})?`)

var cpuModel = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ().+_-]{0,95}$`)

var dockerVersion = regexp.MustCompile(`^[0-9]{1,5}\.[0-9]{1,5}\.[0-9]{1,5}$`)

func describePublishedResource(number int, mode string, item map[string]any) publishedResource {
	source := "unknown"
	if item != nil {
		source = "configured"
		if mode == "simulation" {
			source = "simulated"
		}
	}
	architecture, _ := item["architecture"].(string)
	switch strings.ToLower(architecture) {
	case "amd64", "x86_64":
		architecture = "amd64"
	case "arm64", "aarch64":
		architecture = "arm64"
	case "ppc64le", "s390x", "riscv64":
		architecture = strings.ToLower(architecture)
	default:
		architecture = "unknown"
	}
	return publishedResource{Number: number, Source: source, Architecture: architecture, OS: "unknown", Kernel: "unknown",
		CPUCores: positiveMetric(item["cpuCores"]), CPUCapacity: positiveMetric(item["cpuCapacity"]),
		MemoryBytes: positiveMetric(item["memoryBytes"])}
}

func indexEnvironmentRecords(value any) map[string]map[string]any {
	result := map[string]map[string]any{}
	if records, ok := value.([]any); ok {
		for _, entry := range records {
			item, _ := entry.(map[string]any)
			if id, ok := item["id"].(string); ok && id != "" {
				result[id] = item
			}
		}
	}
	return result
}

func positiveMetric(value any) float64 {
	number, ok := value.(float64)
	if !ok || number <= 0 || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0
	}
	return number
}
