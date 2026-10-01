package validation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	apirequests "github.com/UFFeScience/akoflow/internal/api/requests"
	"github.com/UFFeScience/akoflow/internal/domain"
	"gopkg.in/yaml.v3"
)

// FileKind enumerates the YAML files shipped under examples/. Each kind maps
// to the concrete request DTO the production server uses for that endpoint.
type FileKind string

const (
	KindWorkflow         FileKind = "workflow.yaml"
	KindEnvironment      FileKind = "environment.yaml"
	KindScope            FileKind = "scope.yaml"
	KindTopology         FileKind = "topology.yaml"
	KindPlan             FileKind = "plan.yaml"
	KindExecutionRequest FileKind = "execution-request.yaml"
	KindPlanRequest      FileKind = "plan-request.yaml"
)

// ValidationIssue records a single problem found while validating a fixture.
type ValidationIssue struct {
	Fixture string
	File    string
	Kind    FileKind
	Message string
}

// IsValidationError reports whether the issue is a failure (true) or a
// notice that the file is intentionally not validated (false).
func (i ValidationIssue) IsValidationError() bool {
	return i.Message != ""
}

// FixtureReport is the validation result for a single example category.
type FixtureReport struct {
	Fixture string
	Issues  []ValidationIssue
}

// HasErrors reports whether the report contains at least one failure.
func (r FixtureReport) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.IsValidationError() {
			return true
		}
	}
	return false
}

// Total returns the number of issues recorded.
func (r FixtureReport) Total() int { return len(r.Issues) }

// ValidateFixture runs every applicable decoder against the files shipped in a
// fixture category. It records (without returning) any failure so the caller
// can aggregate a single report per fixture.
func ValidateFixture(category FixtureCategory) FixtureReport {
	report := FixtureReport{Fixture: category.Name}
	validateWorkflowFile(category, &report)
	validateEnvironmentFile(category, &report)
	validateScopeFile(category, &report)
	validateTopologyFile(category, &report)
	validatePlanFile(category, &report)
	validateExecutionRequestFile(category, &report)
	validateScriptReferences(category, &report)
	return report
}

func validateWorkflowFile(category FixtureCategory, report *FixtureReport) {
	if !category.HasFile("workflow.yaml") {
		return
	}
	payload, err := category.ReadFile("workflow.yaml")
	if err != nil {
		report.recordIssue(category.Name, "workflow.yaml", KindWorkflow, err.Error())
		return
	}
	var request apirequests.Workflow
	if err := decodeFixture(payload, &request); err != nil {
		report.recordIssue(category.Name, "workflow.yaml", KindWorkflow, err.Error())
		return
	}
	if _, err := request.Domain(); err != nil {
		report.recordIssue(category.Name, "workflow.yaml", KindWorkflow, err.Error())
		return
	}
	report.recordIssue(category.Name, "workflow.yaml", KindWorkflow, "")
}

func validateEnvironmentFile(category FixtureCategory, report *FixtureReport) {
	if !category.HasFile("environment.yaml") {
		return
	}
	payload, err := category.ReadFile("environment.yaml")
	if err != nil {
		report.recordIssue(category.Name, "environment.yaml", KindEnvironment, err.Error())
		return
	}
	var definition domain.EnvironmentDefinition
	if err := decodeFixture(payload, &definition); err != nil {
		report.recordIssue(category.Name, "environment.yaml", KindEnvironment, err.Error())
		return
	}
	report.recordIssue(category.Name, "environment.yaml", KindEnvironment, "")
}

func validateScopeFile(category FixtureCategory, report *FixtureReport) {
	if !category.HasFile("scope.yaml") {
		return
	}
	payload, err := category.ReadFile("scope.yaml")
	if err != nil {
		report.recordIssue(category.Name, "scope.yaml", KindScope, err.Error())
		return
	}
	var scope domain.ExecutionScope
	if err := decodeFixture(payload, &scope); err != nil {
		report.recordIssue(category.Name, "scope.yaml", KindScope, err.Error())
		return
	}
	if scope.ID == "" {
		report.recordIssue(category.Name, "scope.yaml", KindScope, "scope.id is required")
		return
	}
	report.recordIssue(category.Name, "scope.yaml", KindScope, "")
}

func validateTopologyFile(category FixtureCategory, report *FixtureReport) {
	if !category.HasFile("topology.yaml") {
		return
	}
	payload, err := category.ReadFile("topology.yaml")
	if err != nil {
		report.recordIssue(category.Name, "topology.yaml", KindTopology, err.Error())
		return
	}
	var topology domain.NetworkTopology
	if err := decodeFixture(payload, &topology); err != nil {
		report.recordIssue(category.Name, "topology.yaml", KindTopology, err.Error())
		return
	}
	report.recordIssue(category.Name, "topology.yaml", KindTopology, "")
}

func validatePlanFile(category FixtureCategory, report *FixtureReport) {
	// Plan files come in two shapes: an ImportPlan envelope ({"plan": {...}})
	// or a CreatePlan envelope ({"plan": {...}, "workflow": ..., ...}).
	// Kind examples ship the bare SchedulePlan at the top level. We try the
	// most permissive shape first and record the outcome.
	for _, file := range []string{"plan.yaml", "plan-request.yaml"} {
		if !category.HasFile(file) {
			continue
		}
		payload, err := category.ReadFile(file)
		if err != nil {
			report.recordIssue(category.Name, file, KindPlan, err.Error())
			return
		}
		// Try the bare SchedulePlan shape first (kind examples).
		var bare domain.SchedulePlan
		bareErr := decodeFixture(payload, &bare)
		// Then try the full CreatePlan envelope.
		var envelope struct {
			Plan            domain.SchedulePlan            `json:"plan"`
			Workflow        domain.WorkflowVersion         `json:"workflow"`
			Resources       []domain.Resource              `json:"resources"`
			ExecutionScope  domain.ExecutionScope          `json:"executionScope"`
			NetworkTopology domain.NetworkTopology         `json:"networkTopology"`
			Runtimes        []domain.EnvironmentRuntime    `json:"runtimes"`
			RuntimeBindings []domain.ResourceRuntimeBinding `json:"runtimeBindings"`
		}
		envelopeErr := decodeFixture(payload, &envelope)
		switch {
		case bareErr == nil && bare.ID != "":
			report.recordIssue(category.Name, file, KindPlan, "")
		case envelopeErr == nil && envelope.Plan.ID != "":
			report.recordIssue(category.Name, file, KindPlan, "")
		case bareErr != nil:
			report.recordIssue(category.Name, file, KindPlan, bareErr.Error())
		default:
			report.recordIssue(category.Name, file, KindPlan, envelopeErr.Error())
		}
	}
}

func validateExecutionRequestFile(category FixtureCategory, report *FixtureReport) {
	if !category.HasFile("execution-request.yaml") {
		return
	}
	payload, err := category.ReadFile("execution-request.yaml")
	if err != nil {
		report.recordIssue(category.Name, "execution-request.yaml", KindExecutionRequest, err.Error())
		return
	}
	var request struct {
		Run              domain.ExecutionRun             `json:"run"`
		Plan             domain.SchedulePlan             `json:"plan"`
		Workflow         domain.WorkflowVersion          `json:"workflow"`
		Resources        []domain.Resource               `json:"resources"`
		ExecutionScope   domain.ExecutionScope           `json:"executionScope"`
		NetworkTopology  domain.NetworkTopology          `json:"networkTopology"`
		Runtimes         []domain.EnvironmentRuntime     `json:"runtimes"`
		RuntimeBindings  []domain.ResourceRuntimeBinding `json:"runtimeBindings"`
		ActivityProfiles []domain.ActivityResourceProfile `json:"activityProfiles"`
	}
	if err := decodeFixture(payload, &request); err != nil {
		report.recordIssue(category.Name, "execution-request.yaml", KindExecutionRequest, err.Error())
		return
	}
	report.recordIssue(category.Name, "execution-request.yaml", KindExecutionRequest, "")
}

// validateScriptReferences walks every run.sh shipped in the fixture and
// checks that it references the YAML files that the fixture actually ships.
// This catches stale run.sh that still posts files that were renamed or
// removed.
func validateScriptReferences(category FixtureCategory, report *FixtureReport) {
	for _, file := range category.scripts() {
		payload, err := category.ReadFile(file)
		if err != nil {
			continue
		}
		references := scriptYAMLReferences(payload)
		if len(references) == 0 {
			continue
		}
		for _, reference := range references {
			if category.HasFile(reference) {
				continue
			}
			report.recordIssue(category.Name, file, FileKind(file),
				fmt.Sprintf("run.sh posts %q but fixture does not ship it", reference))
		}
	}
}

func (c FixtureCategory) scripts() []string {
	scripts := make([]string, 0)
	for name := range c.Files {
		if strings.HasSuffix(name, ".sh") {
			scripts = append(scripts, name)
		}
	}
	return scripts
}

func scriptYAMLReferences(payload string) []string {
	references := make([]string, 0)
	for _, line := range strings.Split(payload, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "post ") {
			continue
		}
		fields := strings.Fields(line)
		// Format: post <endpoint> <file>
		if len(fields) < 3 {
			continue
		}
		name := fields[2]
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			references = append(references, name)
		}
	}
	return references
}

// decodeFixture reproduces the server's decode() function: parse YAML,
// normalise to JSON, then unmarshal with strict (DisallowUnknownFields)
// semantics so any undocumented field is flagged immediately.
func decodeFixture(payload string, target any) error {
	var document any
	if err := yaml.Unmarshal([]byte(payload), &document); err != nil {
		return fmt.Errorf("decode YAML: %w", err)
	}
	normalised, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("normalise YAML: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(normalised))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil && err != io.EOF {
		return fmt.Errorf("decode JSON: %w", err)
	}
	return nil
}

func (r *FixtureReport) recordIssue(fixture, file string, kind FileKind, message string) {
	r.Issues = append(r.Issues, ValidationIssue{
		Fixture: fixture,
		File:    file,
		Kind:    kind,
		Message: message,
	})
}
