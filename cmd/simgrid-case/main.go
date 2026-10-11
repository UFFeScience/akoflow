// simgrid-case exports a private simulation-only case using the production
// planner/SimGrid contract. It never submits an execution or provisions a VM.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/provider/simgrid"
)

func main() {
	requestPath := flag.String("request", "", "private simulation request JSON")
	outputPath := flag.String("output", "", "new private case directory (must not exist)")
	flag.Parse()
	if err := export(*requestPath, *outputPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func export(requestPath, outputPath string) error {
	if requestPath == "" || outputPath == "" {
		return fmt.Errorf("--request and --output required")
	}
	payload, err := os.ReadFile(requestPath)
	if err != nil {
		return err
	}
	var request ports.ExecutionRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return err
	}
	platform, input, err := simgrid.BuildOfflineCase(request, 1e9)
	if err != nil {
		return err
	}
	if err := os.Mkdir(outputPath, 0700); err != nil {
		return err
	}
	for name, content := range map[string][]byte{"platform.xml": platform, "simulation.json": input} {
		if err := os.WriteFile(filepath.Join(outputPath, name), content, 0600); err != nil {
			return err
		}
	}
	return nil
}
