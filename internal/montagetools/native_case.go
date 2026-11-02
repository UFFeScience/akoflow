package montagetools

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/provider/simgrid"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type dockerNativeCommand struct{ root, image string }

func (d dockerNativeCommand) Run(ctx context.Context, binary string, args []string, _ []byte) ([]byte, error) {
	check(strings.HasPrefix(binary, "/case/") || binary == "/build/output/akoflow-simgrid-runner", "native binary must be a case runner or known legacy runner")
	mapped := append([]string(nil), args...)
	for i, arg := range mapped {
		if filepath.IsAbs(arg) {
			rel, err := filepath.Rel(d.root, arg)
			if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
				return nil, fmt.Errorf("native case path outside repository")
			}
			mapped[i] = "/case/" + filepath.ToSlash(rel)
		}
	}
	argv := []string{"run", "--rm", "-v", d.root + ":/case", d.image, binary}
	argv = append(argv, mapped...)
	output, err := exec.CommandContext(ctx, "docker", argv...).CombinedOutput()
	if err != nil {
		for _, line := range strings.Split(string(output), "\n") {
			if strings.HasPrefix(line, "akoflow-simgrid-runner:") {
				return output, fmt.Errorf("%w: %s", err, line)
			}
		}
	}
	return output, err
}

// Exercise the production executor capability/trace checks, not only the C++
// binary. Mode is checked before any container runs. No daemon or real runtime.
func verifyNativeCase(requestPath, image, binary, out string) {
	raw, err := os.ReadFile(requestPath)
	check(err == nil, "native case request unavailable")
	var request ports.ExecutionRequest
	check(json.Unmarshal(raw, &request) == nil, "invalid native case request")
	check(string(request.Run.Mode) == "simulation", "verification is simulation only")
	root, err := os.Getwd()
	check(err == nil, "repository path unavailable")
	output, err := filepath.Abs(out)
	check(err == nil, "invalid verification output")
	_, err = os.Stat(output)
	check(os.IsNotExist(err), "verification output must be new")
	executor, err := simgrid.NewProcessExecutor(dockerNativeCommand{root: root, image: image}, simgrid.ProcessConfig{BinaryPath: binary, Workspace: output, Timeout: 10 * time.Minute, ReferenceFLOPS: 1e9})
	check(err == nil, "cannot create native verification executor")
	trace, err := executor.Execute(context.Background(), request)
	check(err == nil, fmt.Sprintf("native executor conformance: %v", err))
	payload, err := json.Marshal(trace)
	check(err == nil, "cannot encode native verification trace")
	var result Object
	check(json.Unmarshal(payload, &result) == nil, "cannot decode verification trace")
	write(filepath.Join(output, "verified-result.json"), result)
}
