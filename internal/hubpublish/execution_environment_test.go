package hubpublish

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPublishedEnvironmentSeparatesCapacityFromRequestsAndOmitsPrivateFields(t *testing.T) {
	var context, execution map[string]any
	if err := json.Unmarshal([]byte(`{
		"resources":[
			{"id":"private-node","name":"private-host.example","providerId":"192.168.25.10","architecture":"aarch64","cpuCores":32,"memoryBytes":4096,"metadata":{"password":"secret"}},
			{"id":"other-node","architecture":"amd64","cpuCores":16}
		],
		"workflow":{"version":{"activities":[{"id":"activity","resources":{"cpu":4,"memoryBytes":128,"gpu":1}}]}}
	}`), &context); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{
		"run":{"mode":"real"},
		"activityAttempts":[
			{"activityId":"activity","attempt":1,"startedAt":100,"allocatedResourceId":"private-node"},
			{"activityId":"activity","attempt":2,"startedAt":200,"allocatedResourceId":"other-node"},
			{"activityId":"activity","attempt":3,"startedAt":0,"allocatedResourceId":"another-private-node"}
		]
	}`), &execution); err != nil {
		t.Fatal(err)
	}
	snapshot := executionEnvironment(context, execution)
	if len(snapshot.Resources) != 2 || len(snapshot.Attempts) != 2 ||
		snapshot.Resources[0].Source != "configured" || snapshot.Resources[0].Architecture != "arm64" ||
		snapshot.Resources[0].CPUCores != 32 || snapshot.Attempts[0].RequestedCPU != 4 ||
		snapshot.Attempts[0].RequestedGPU != 1 || snapshot.Attempts[1].ResourceNumber != 2 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"private-node", "private-host", "192.168", "password", "secret"} {
		if strings.Contains(string(encoded), private) {
			t.Fatalf("published private field %q", private)
		}
	}
}

func TestPublishedEnvironmentUsesDockerObservationsAndDistinguishesSlurmNodes(t *testing.T) {
	var execution map[string]any
	if err := json.Unmarshal([]byte(`{
		"run":{"id":"run","mode":"real"},
        "timeline":{"totals":{"imagePullSeconds":2.5,"imageBuildSeconds":8}},
		"activityAttempts":[
			{"activityId":"container","attempt":2,"startedAt":100,"allocatedResourceId":"docker"},
			{"activityId":"native","startedAt":150,"allocatedResourceId":"docker"},
            {"activityId":"first","startedAt":200,"allocatedResourceId":"cluster"},
			{"activityId":"second","startedAt":300,"allocatedResourceId":"cluster"}
		],
		"handles":[
			{"id":"run:container:attempt-2","metadata":{"dockerEnvironment":{
				"architecture":"x86_64","cpuCores":16,"memoryBytes":4096,"version":"29.1.0","executionStyle":"emulated"
			},"dockerLimits":{"cpu":2,"memoryBytes":1024}}},
			{"id":"run:native","metadata":{"nativeEnvironment":{
                "architecture":"x86_64","cpuCapacity":4,"memoryBytes":2048,"cpuModel":"Test CPU (TM)","executionStyle":"native"
            }}},
			{"id":"run:first","metadata":{"allocatedNode":"private-first","slurmAllocatedGpus":2,"slurmEnvironment":{"architecture":"aarch64","cpuCores":32,"gpuCapacity":8,"version":"1.5.5"}}},
			{"id":"run:second","metadata":{"allocatedNode":"private-second"}}
		]
	}`), &execution); err != nil {
		t.Fatal(err)
	}
	snapshot := executionEnvironment(nil, execution)
	if len(snapshot.Resources) != 4 || len(snapshot.Attempts) != 4 || snapshot.ImagePullSeconds != 2.5 || snapshot.ImageBuildSeconds != 8 {
		t.Fatalf("unexpected resources or attempts: %+v", snapshot)
	}
	docker := snapshot.Resources[0]
	if docker.Source != "observed" || docker.Runtime != "Docker" || docker.RuntimeVersion != "29.1.0" ||
		docker.CPUCores != 16 || docker.MemoryBytes != 4096 || docker.Architecture != "amd64" ||
		snapshot.Attempts[0].ExecutionStyle != "emulated" || snapshot.Attempts[0].AllocatedCPU != 2 || snapshot.Attempts[0].AllocatedRAM != 1024 {
		t.Fatalf("missing observation: %+v", snapshot)
	}
	if native := snapshot.Resources[1]; native.Runtime != "Native process" || native.CPUModel != "Test CPU (TM)" ||
		native.CPUCapacity != 4 || snapshot.Attempts[1].ExecutionStyle != "native" {
		t.Fatalf("missing native observation: %+v", snapshot)
	}
	if snapshot.Attempts[2].AllocatedGPU == nil || *snapshot.Attempts[2].AllocatedGPU != 2 ||
		snapshot.Resources[2].Runtime != "Slurm" || snapshot.Resources[2].Architecture != "arm64" ||
		snapshot.Resources[2].GPUCapacity == nil || *snapshot.Resources[2].GPUCapacity != 8 || snapshot.Resources[2].ContainerRuntimeVersion != "1.5.5" || snapshot.Resources[2].RuntimeVersion != "" ||
		snapshot.Resources[3].Source != "unknown" || snapshot.Attempts[2].ResourceNumber == snapshot.Attempts[3].ResourceNumber {
		t.Fatalf("expected separate Slurm nodes with unknown capacity: %+v", snapshot)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil || strings.Contains(string(encoded), "private-") {
		t.Fatalf("expected anonymous node references, got %s (%v)", encoded, err)
	}
}

func TestPublishedAttemptTimingTracksRetriesAndSanitizesKernel(t *testing.T) {
	var run map[string]any
	err := json.Unmarshal([]byte(`{
		"run":{"id":"run","mode":"real"},
		"activityAttempts":[
			{"activityId":"step","attempt":1,"status":"failed","startedAt":10.25},
			{"activityId":"step","attempt":2,"status":"completed","queuedAt":12,"startedAt":14,"finishedAt":19,"queueSeconds":2,"runtimeSeconds":5,"transferSeconds":1}
		],
		"timeline":{"phases":[
			{"category":"image-pull","activityId":"step","attempt":1,"status":"failed","durationSeconds":4},
			{"category":"image-pull","activityId":"step","attempt":2,"status":"completed","durationSeconds":2},
			{"category":"sif-build","activityId":"step","attempt":2,"status":"completed","durationSeconds":3},
			{"category":"sif-register","activityId":"step","attempt":2,"status":"completed","durationSeconds":0.5},
			{"category":"image-build","activityId":"other","attempt":2,"status":"completed","durationSeconds":99}
		]},
		"handles":[{"id":"run:step:attempt-2","metadata":{"slurmEnvironment":{"os":"linux","kernel":"6.12.62-1-private-host.example","architecture":"amd64"}},"artifacts":{"runId":"run","activityId":"step","attempt":2,"initialSnapshot":[{"path":"private-input-path","sizeBytes":14},{"path":"other","sizeBytes":6}]}}]
	}`), &run)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := executionEnvironment(nil, run)
	first, second := snapshot.Attempts[0], snapshot.Attempts[1]
	if first.Timing.PullSeconds == nil || *first.Timing.PullSeconds != 4 || first.Timing.FinishedAt != nil ||
		second.ActivityID != "step" || second.Status != "completed" || first.InputBytes != nil ||
		second.InputBytes == nil || *second.InputBytes != 20 ||
		second.Timing.PullSeconds == nil || *second.Timing.PullSeconds != 2 ||
		second.Timing.SIFSeconds == nil || *second.Timing.SIFSeconds != 3 ||
		second.Timing.RegisterSeconds == nil || *second.Timing.RegisterSeconds != 0.5 || second.Timing.BuildSeconds != nil || second.Timing.RuntimeSeconds == nil || *second.Timing.RuntimeSeconds != 5 {
		t.Fatalf("unexpected timing by attempt: %+v", snapshot.Attempts)
	}
	if snapshot.Resources[0].OS != "linux" || snapshot.Resources[0].Kernel != "6.12.62-1" {
		t.Fatalf("unexpected OS or kernel: %+v", snapshot.Resources[0])
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil || !strings.Contains(string(encoded), `"startedAt":"1970-01-01T00:00:10.25Z"`) ||
		strings.Contains(string(encoded), "private-host.example") || strings.Contains(string(encoded), "private-input-path") {
		t.Fatalf("unexpected published snapshot: %s (%v)", encoded, err)
	}
}
