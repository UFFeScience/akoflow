package command

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/UFFeScience/akoflow-other-algorithms/internal/planner"
)

func Run(algorithm planner.Algorithm, arguments []string) error {
	flags := flag.NewFlagSet(string(algorithm), flag.ContinueOnError)
	inputPath := flags.String("input", "", "offline planning input YAML/JSON; use - for stdin")
	outputPath := flags.String("output", "-", "import envelope destination; use - for stdout")
	format := flags.String("format", "yaml", "output format: yaml or json")
	planID := flags.String("plan-id", "", "unique schedule plan ID")
	alpha := flags.Float64("alpha", 0.5, "AkôScore time weight from 0 to 1")
	defaultRuntime := flags.Float64("default-runtime", 1, "fallback activity runtime in seconds")
	apiURL := flags.String("api-url", "", "AkôFlow API base URL")
	token := flags.String("token", "", "AkôFlow API token; prefer AKOFLOW_API_TOKEN")
	scopeID := flags.String("scope-id", "", "execution scope ID for API mode")
	workflowID := flags.String("workflow-id", "", "workflow definition ID for API mode")
	topologyID := flags.String("topology-id", "", "network topology ID for API mode")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	var input planner.Input
	var err error
	if *inputPath != "" {
		input, err = planner.ReadInput(*inputPath)
	} else {
		resolvedToken := *token
		if resolvedToken == "" {
			resolvedToken = os.Getenv("AKOFLOW_API_TOKEN")
		}
		if *apiURL == "" {
			return fmt.Errorf("provide -input or -api-url with scope, workflow, and topology IDs")
		}
		input, err = (planner.APILoader{BaseURL: *apiURL, Token: resolvedToken}).Load(*scopeID, *workflowID, *topologyID)
	}
	if err != nil {
		return err
	}
	if *planID == "" {
		return fmt.Errorf("-plan-id is required")
	}
	envelope, err := planner.BuildPlan(input, planner.Options{
		Algorithm:             algorithm,
		PlanID:                *planID,
		Alpha:                 *alpha,
		DefaultRuntimeSeconds: *defaultRuntime,
	})
	if err != nil {
		return err
	}
	writer := os.Stdout
	if *outputPath != "-" {
		writer, err = os.Create(*outputPath)
		if err != nil {
			return err
		}
		defer writer.Close()
	}
	if err := planner.WriteEnvelope(writer, envelope, strings.ToLower(*format)); err != nil {
		return err
	}
	return nil
}
