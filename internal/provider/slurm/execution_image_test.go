package slurm

import (
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestSIFUsageRequiresContainerStartEvidence(t *testing.T) {
	handle := domain.ActivityHandle{Metadata: map[string]any{}}
	command := domain.ActivityCommand{
		Image: "example/tool:latest",
		ResolvedExecutable: &domain.ResolvedExecutable{
			VariantID: "sif", Digest: "verified", LocalPath: "/scratch/tool.sif", MaterializationDone: true,
		},
	}
	prepareImageObservation(&handle, command)
	handle, _ = applySentinelStatus(handle, "state=failed\nexit_code=127\ncontainer_started_at=0\n")
	observation := handle.Metadata["imageObservation"].(map[string]any)
	if observation["status"] != "unavailable" {
		t.Fatalf("preparation became usage: %v", observation)
	}
	handle, _ = applySentinelStatus(handle, "state=failed\nexit_code=1\ncontainer_started_at=10\nallocated_node=node\nnode_arch=x86_64\nnode_os=linux\n")
	observation = handle.Metadata["imageObservation"].(map[string]any)
	if observation["status"] != "observed" || observation["sifVariantId"] != "sif" || observation["platform"] != "linux/amd64" || observation["requestedReference"] != command.Image {
		t.Fatalf("missing observed usage: %v", observation)
	}
}
