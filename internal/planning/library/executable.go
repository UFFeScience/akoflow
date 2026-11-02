package library

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type boundedOutput struct {
	bytes.Buffer
	limit int
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, fmt.Errorf("output exceeds %d bytes", b.limit)
	}
	return b.Buffer.Write(p)
}
func (s *Catalog) compile(ctx context.Context, p Package) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	binary, err := filepath.Abs(s.binary(p))
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(binary)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	userSource, _ := sourceUnit(p.Source)
	for name, source := range map[string]string{"contract.go": Contract, "runner.go": runner, "algorithm.go": userSource} {
		if err = os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			return "", err
		}
	}
	goBinary, err := exec.LookPath("go")
	if err != nil {
		if _, check := os.Stat("/usr/local/go/bin/go"); check != nil {
			return "", fmt.Errorf("Go compiler is unavailable on the daemon")
		}
		goBinary = "/usr/local/go/bin/go"
	}
	cmd := exec.CommandContext(ctx, goBinary, "build", "-trimpath", "-o", binary, "contract.go", "runner.go", "algorithm.go")
	cmd.Dir = dir
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "GOCACHE=" + filepath.Join(os.TempDir(), "akoflow-scheduler-go-cache"), "GO111MODULE=off", "GOWORK=off", "GOTOOLCHAIN=local", "CGO_ENABLED=0", "GOPROXY=off"}
	output := &boundedOutput{limit: 128 * 1024}
	cmd.Stdout = output
	cmd.Stderr = output
	if err = cmd.Run(); err != nil {
		return "", fmt.Errorf("Go compilation failed: %s (%v)", output.String(), err)
	}
	return binary, nil
}

type executable struct {
	descriptor ports.SchedulerDescriptor
	binary     string
	revision   int
	sourceHash string
}

func (e *executable) Descriptor() ports.SchedulerDescriptor { return e.descriptor }
func (e *executable) Schedule(ctx context.Context, request domain.PlanningRequest, configuration map[string]any, progress ports.ProgressReporter, sink ports.CandidateSink) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	input, err := json.Marshal(map[string]any{"request": request, "configuration": configuration})
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, e.binary)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "LANG=C.UTF-8"}
	cmd.WaitDelay = time.Second
	output := &boundedOutput{limit: 32 * 1024 * 1024}
	diagnostics := &boundedOutput{limit: 128 * 1024}
	cmd.Stdout = output
	cmd.Stderr = diagnostics
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("scheduler plugin failed: %s (%v)", diagnostics.String(), err)
	}
	var plans []domain.SchedulePlan
	if err = json.Unmarshal(output.Bytes(), &plans); err != nil {
		return fmt.Errorf("invalid plugin response: %w", err)
	}
	if len(plans) == 0 || len(plans) > 1000 {
		return fmt.Errorf("plugin must return between 1 and 1000 plans")
	}
	if !e.descriptor.Multiple && len(plans) > 1 {
		return fmt.Errorf("single-candidate plugin returned multiple plans")
	}
	for i, plan := range plans {
		plan.Source = domain.PlanningSourcePlugin
		plan.Algorithm = e.descriptor.ID
		plan.AlgorithmVersion = fmt.Sprintf("%d:%s", e.revision, e.sourceHash)
		plan.Objective = e.descriptor.Objective
		if err = sink.Emit(ctx, plan); err != nil {
			return err
		}
		if progress != nil {
			if err = progress.Report(ctx, 100*float64(i+1)/float64(len(plans)), "Plugin candidate validated"); err != nil {
				return err
			}
		}
	}
	return nil
}
