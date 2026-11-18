package kubernetes

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestMutableTagRuntimeObservation(t *testing.T) {
	endpoint, image := os.Getenv("AKOFLOW_TEST_KUBE_PROXY"), os.Getenv("AKOFLOW_TEST_KUBE_IMAGE")
	if endpoint == "" || image == "" {
		t.Skip("requires a disposable Kubernetes API proxy and test image")
	}
	client, err := NewClient(ClientConfig{Endpoint: endpoint, Token: "integration-proxy"})
	if err != nil {
		t.Fatal(err)
	}
	adapter := New(client, "default")
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	runID := os.Getenv("AKOFLOW_TEST_KUBE_RUN")
	attempt, _ := strconv.Atoi(os.Getenv("AKOFLOW_TEST_KUBE_ATTEMPT"))
	execution := domain.ActivityExecutionContext{
		Run:     domain.ExecutionRun{ID: runID},
		Attempt: attempt,
		Activity: domain.Activity{ID: "fixture", Command: domain.ActivityCommand{
			Image: image, Entrypoint: "/bin/sh", Arguments: []string{"-c", "cat /fixture-version"},
		}},
	}
	handle, err := adapter.Start(ctx, execution)
	if err != nil {
		t.Fatal(err)
	}
	for ctx.Err() == nil {
		handle, err = adapter.Inspect(ctx, handle)
		if err != nil {
			t.Fatal(err)
		}
		if handle.Status == domain.HandleFailed {
			t.Fatalf("activity failed: %s", handle.Log)
		}
		if handle.Status == domain.HandleCompleted {
			observation, _ := handle.Metadata["imageObservation"].(map[string]any)
			if observation["status"] != "observed" || observation["requestedReference"] != image {
				t.Fatalf("unexpected runtime evidence: %v", observation)
			}
			output, _, _ := strings.Cut(handle.Log, "\n")
			t.Logf("requested=%s runtimeImageID=%v registryDigest=%v platform=%v output=%s",
				image, observation["runtimeImageId"], observation["registryDigest"], observation["platform"], output)
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatal("Kubernetes activity did not complete")
}
