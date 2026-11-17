package build

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type inspectionRunner struct{ calls []string }

func (r *inspectionRunner) Run(_ context.Context, name string, args []string, _ []byte) ([]byte, error) {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	if args[1] == "history" {
		return []byte("{\"CreatedBy\":\"RUN echo hello\",\"Size\":\"1kB\"}\n{\"CreatedBy\":\"FROM scratch\",\"Size\":\"0B\"}\n"), nil
	}
	return json.Marshal([]map[string]any{{"Id": contentDigest([]byte("image")), "Os": "linux", "Architecture": "amd64", "RepoDigests": []string{"example/image@" + contentDigest([]byte("registry"))}, "Size": 1024, "RootFS": map[string]any{"Layers": []string{contentDigest([]byte("layer"))}}, "Config": map[string]any{"Cmd": []string{"/bin/sh"}}}})
}
func TestImageInspectionReportsLayersAndHistoryWithoutPreparingImage(t *testing.T) {
	runner := &inspectionRunner{}
	origin := domain.ArtifactOrigin{Image: "example/image@" + contentDigest([]byte("registry")), Platform: "linux/amd64"}
	result, err := (Manager{Executor: Executor{Runner: runner}}).InspectOrigin(context.Background(), origin)
	if err != nil || result["available"] != true || result["Size"] != float64(1024) || len(result["history"].([]map[string]any)) != 2 || result["RootFS"] == nil {
		t.Fatalf("image inspection: %+v %v", result, err)
	}
	for _, call := range runner.calls {
		if !strings.Contains(call, "inspect") && !strings.Contains(call, "history") {
			t.Fatalf("inspection mutated image: %s", call)
		}
	}
}
