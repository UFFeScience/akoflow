package local

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/provider"
)

func ExecutionContainerName(runID, activityID string, attempt int) string {
	name := "akoflow-" + strings.NewReplacer(":", "-", "/", "-").Replace(runID+"-"+activityID)
	if attempt > 1 {
		name += fmt.Sprintf("-attempt-%d", attempt)
	}
	return name
}

func ContainerReservationReleased(ctx context.Context, executor provider.CommandExecutor, name string) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	output, err := executor.Run(ctx, "docker", []string{"container", "inspect", "--format", "{{.State.Status}}", name}, nil)
	state := strings.TrimSpace(string(output))
	if err != nil {
		return state == "Error: No such container: "+name || state == "Error response from daemon: No such container: "+name
	}
	return state == "exited" || state == "dead"
}
