package kubernetes

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	runtimecommon "github.com/UFFeScience/akoflow/internal/provider"
)

var manifestDigest = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

type observedContainerState struct {
	StartedAt time.Time `json:"startedAt"`
}

type observedPod struct {
	Spec struct {
		NodeName string `json:"nodeName"`
	} `json:"spec"`
	Status struct {
		StartTime         *time.Time `json:"startTime"`
		ContainerStatuses []struct {
			Name    string `json:"name"`
			ImageID string `json:"imageID"`
			State   struct {
				Running    *observedContainerState `json:"running"`
				Terminated *observedContainerState `json:"terminated"`
			} `json:"state"`
		} `json:"containerStatuses"`
	} `json:"status"`
}

func (a *Adapter) applyPodObservation(ctx context.Context, jobName string, handle *domain.ActivityHandle) {
	payload, err := a.api.List(ctx, a.namespace, "pods", "job-name="+jobName)
	if err != nil {
		return
	}
	var pods struct {
		Items []observedPod `json:"items"`
	}
	if json.Unmarshal(payload, &pods) != nil || len(pods.Items) == 0 {
		return
	}
	if start := pods.Items[0].Status.StartTime; start != nil && !start.IsZero() {
		handle.StartedAt = runtimecommon.UnixSeconds(*start)
	}
	var newest time.Time
	var imageID, nodeName string
	for _, pod := range pods.Items {
		for _, container := range pod.Status.ContainerStatuses {
			if container.Name != activityContainer {
				continue
			}
			state := container.State.Running
			if state == nil {
				state = container.State.Terminated
			}
			if state != nil && state.StartedAt.After(newest) {
				newest, imageID, nodeName = state.StartedAt, container.ImageID, pod.Spec.NodeName
			}
		}
	}
	if newest.IsZero() {
		return
	}
	if handle.Metadata == nil {
		handle.Metadata = make(map[string]any)
	}
	startedAt := runtimecommon.UnixSeconds(newest)
	previous, _ := handle.Metadata[domain.TimingContainerStartedAt].(float64)
	if startedAt < previous {
		return
	}
	handle.Metadata[domain.TimingContainerStartedAt] = startedAt
	a.recordImageObservation(ctx, handle, imageID, nodeName, startedAt == previous)
}

func (a *Adapter) recordImageObservation(ctx context.Context, handle *domain.ActivityHandle, imageID, nodeName string, sameContainer bool) {
	prior, _ := handle.Metadata["imageObservation"].(map[string]any)
	if sameContainer && prior["status"] == "observed" {
		return
	}
	observation := map[string]any{"requestedReference": prior["requestedReference"], "status": "unavailable"}
	if imageID = strings.TrimSpace(imageID); imageID != "" {
		observation["status"] = "observed"
		reference := strings.TrimPrefix(imageID, "docker-pullable://")
		if repository, digest, ok := strings.Cut(reference, "@"); ok &&
			repository != "" && !strings.Contains(repository, "://") && manifestDigest.MatchString(digest) {
			observation["registryDigest"] = digest
		} else {
			observation["runtimeImageId"] = imageID
		}
		if platform := a.observedNodePlatform(ctx, nodeName); platform != "" {
			observation["platform"] = platform
		}
	}
	delete(handle.Metadata, "executionImageArtifact")
	handle.Metadata["imageObservation"] = observation
}

func (a *Adapter) observedNodePlatform(ctx context.Context, nodeName string) string {
	if nodeName == "" {
		return ""
	}
	payload, err := a.api.Get(ctx, "", "nodes", nodeName)
	if err != nil {
		return ""
	}
	var node struct {
		Status struct {
			NodeInfo struct {
				OperatingSystem string
				Architecture    string
			}
		}
	}
	if json.Unmarshal(payload, &node) != nil {
		return ""
	}
	info := node.Status.NodeInfo
	if info.OperatingSystem == "" || info.Architecture == "" {
		return ""
	}
	return info.OperatingSystem + "/" + info.Architecture
}
