package simgrid

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/provider"
)

type ProcessConfig struct {
	BinaryPath     string
	Workspace      string
	MaxConcurrent  int
	Timeout        time.Duration
	ReferenceFLOPS float64
}

type ProcessExecutor struct {
	commands  provider.CommandExecutor
	config    ProcessConfig
	semaphore chan struct{}
}

func NewProcessExecutor(commands provider.CommandExecutor, config ProcessConfig) (*ProcessExecutor, error) {
	if commands == nil {
		return nil, errors.New("SimGrid command executor is required")
	}
	if strings.TrimSpace(config.BinaryPath) == "" {
		return nil, errors.New("SimGrid runner binary is required")
	}
	if config.Workspace == "" {
		config.Workspace = "storage/simgrid"
	}
	if config.MaxConcurrent < 1 {
		config.MaxConcurrent = 1
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Minute
	}
	if config.ReferenceFLOPS <= 0 {
		config.ReferenceFLOPS = 1e9
	}
	return &ProcessExecutor{
		commands: commands, config: config,
		semaphore: make(chan struct{}, config.MaxConcurrent),
	}, nil
}

func (e *ProcessExecutor) Execute(ctx context.Context, request ports.ExecutionRequest) (domain.ExecutionTrace, error) {
	if request.Run.Mode != domain.ExecutionModeSimulation {
		return domain.ExecutionTrace{}, fmt.Errorf("SimGrid runner cannot execute mode %q", request.Run.Mode)
	}
	if err := e.acquire(ctx); err != nil {
		return domain.ExecutionTrace{}, err
	}
	defer e.release()

	bundle, err := e.createBundle(request)
	if err != nil {
		return domain.ExecutionTrace{}, err
	}
	runContext, cancel := context.WithTimeout(ctx, e.config.Timeout)
	defer cancel()
	output, err := e.commands.Run(runContext, e.config.BinaryPath, []string{
		"--platform", bundle.platformPath,
		"--input", bundle.inputPath,
		"--output", bundle.outputPath,
	}, nil)
	if err != nil {
		return domain.ExecutionTrace{}, fmt.Errorf("execute SimGrid run %q: %w", request.Run.ID, err)
	}
	if len(output) > 0 {
		_ = os.WriteFile(bundle.logPath, output, 0o600)
	}
	trace, err := readTrace(bundle.outputPath)
	if err != nil {
		return domain.ExecutionTrace{}, err
	}
	if trace.RunID != request.Run.ID || trace.PlanID != request.Plan.ID {
		return domain.ExecutionTrace{}, fmt.Errorf("SimGrid returned trace for run %q and plan %q", trace.RunID, trace.PlanID)
	}
	trace.Mode, trace.Predicted = request.Run.Mode, request.Plan.Predicted
	return trace, nil
}

func (e *ProcessExecutor) acquire(ctx context.Context) error {
	select {
	case e.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (e *ProcessExecutor) release() { <-e.semaphore }

type simulationBundle struct {
	platformPath string
	inputPath    string
	outputPath   string
	logPath      string
}

func (e *ProcessExecutor) createBundle(request ports.ExecutionRequest) (simulationBundle, error) {
	directory := filepath.Join(e.config.Workspace, safeRunID(request.Run.ID)+"-"+provider.NewID("simulation"))
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return simulationBundle{}, fmt.Errorf("create SimGrid workspace: %w", err)
	}
	bundle := simulationBundle{
		platformPath: filepath.Join(directory, "platform.xml"),
		inputPath:    filepath.Join(directory, "simulation.json"),
		outputPath:   filepath.Join(directory, "result.json"),
		logPath:      filepath.Join(directory, "runner.log"),
	}
	platform, err := buildPlatformXML(request.Resources, request.NetworkTopology, e.config.ReferenceFLOPS)
	if err != nil {
		return simulationBundle{}, err
	}
	input, err := buildRunnerInput(request, e.config.ReferenceFLOPS)
	if err != nil {
		return simulationBundle{}, err
	}
	if err := os.WriteFile(bundle.platformPath, platform, 0o600); err != nil {
		return simulationBundle{}, fmt.Errorf("write SimGrid platform: %w", err)
	}
	if err := os.WriteFile(bundle.inputPath, input, 0o600); err != nil {
		return simulationBundle{}, fmt.Errorf("write SimGrid input: %w", err)
	}
	return bundle, nil
}

func readTrace(path string) (domain.ExecutionTrace, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return domain.ExecutionTrace{}, fmt.Errorf("read SimGrid result: %w", err)
	}
	var trace domain.ExecutionTrace
	if err := json.Unmarshal(payload, &trace); err != nil {
		return domain.ExecutionTrace{}, fmt.Errorf("decode SimGrid result: %w", err)
	}
	return trace, nil
}

func safeRunID(value string) string {
	value = strings.Map(func(character rune) rune {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '-' || character == '_' {
			return character
		}
		return '-'
	}, value)
	value = strings.Trim(value, "-")
	if value == "" {
		return "run"
	}
	return value
}
