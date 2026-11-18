package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestKubernetesObservesImageFromStartedContainersAndSeparatesAttempts(t *testing.T) {
	ctx := context.Background()
	api := &apiFake{getOutput: []byte(`{"status":{"nodeInfo":{"operatingSystem":"linux","architecture":"amd64"}}}`)}
	adapter := New(api, "default")
	requested := "registry.example/workflow:latest"
	newHandle := func(id string) domain.ActivityHandle {
		return domain.ActivityHandle{ID: id, Metadata: map[string]any{
			"imageObservation": map[string]any{"requestedReference": requested, "status": "unavailable"},
		}}
	}
	first, retry := newHandle("run:activity"), newHandle("run:activity:attempt-2")
	api.listOutput = []byte(`{"items":[{"status":{"containerStatuses":[{"name":"activity","imageID":"cached","state":{"waiting":{}}}]}}]}`)
	adapter.applyPodObservation(ctx, "first-job", &first)
	if image := first.Metadata["imageObservation"].(map[string]any); image["status"] != "unavailable" {
		t.Fatalf("waiting container reported an observed image: %v", image)
	}
	digest := "sha256:" + strings.Repeat("a", 64)
	api.listOutput = imagePodStatus("docker-pullable://registry.example/workflow@" + digest)
	adapter.applyPodObservation(ctx, "first-job", &first)
	api.listOutput = imagePodStatus("containerd://sha256:" + strings.Repeat("b", 64))
	adapter.applyPodObservation(ctx, "retry-job", &retry)

	original := first.Metadata["imageObservation"].(map[string]any)
	retried := retry.Metadata["imageObservation"].(map[string]any)
	if original["registryDigest"] != digest || original["runtimeImageId"] != nil || original["platform"] != "linux/amd64" {
		t.Fatalf("incorrect registry evidence: %v", original)
	}
	if retried["runtimeImageId"] == nil || retried["registryDigest"] != nil || retried["requestedReference"] != requested {
		t.Fatalf("incorrect retry evidence: %v", retried)
	}
	if path := resourcePath("", "nodes", "fixture-node"); path != "/api/v1/nodes/fixture-node" {
		t.Fatalf("unexpected node lookup path: %s", path)
	}
}

func imagePodStatus(imageID string) []byte {
	return fmt.Appendf(nil, `{"items":[{"spec":{"nodeName":"fixture-node"},"status":{"containerStatuses":[
		{"name":"activity","imageID":%q,"state":{"terminated":{"startedAt":"2026-08-24T06:30:04Z"}}}
	]}}]}`, imageID)
}
