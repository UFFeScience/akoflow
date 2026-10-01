package validation

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestEveryExampleFixtureValidates runs the production request DTOs against
// every example shipped under examples/. It is the regression net that keeps
// documentation in lockstep with the server contract.
func TestEveryExampleFixtureValidates(t *testing.T) {
	root := repositoryRoot(t)
	fixtures, err := DiscoverFixtures(root)
	require.NoError(t, err)

	reports := make([]FixtureReport, 0, len(fixtures))
	for _, name := range fixtures.SortedNames() {
		reports = append(reports, ValidateFixture(fixtures[name]))
	}

	// Collect failures only from fixtures that ship the YAMLs we expect the
	// docs to document. We surface all failures in a single message so the
	// developer can fix them in one pass.
	var failures []string
	for _, r := range reports {
		for _, issue := range r.Issues {
			if !issue.IsValidationError() {
				continue
			}
			failures = append(failures, fmt.Sprintf("[%s/%s] %s: %s",
				issue.Fixture, issue.File, issue.Kind, issue.Message))
		}
	}
	if len(failures) > 0 {
		t.Fatalf("%d example fixture(s) failed validation:\n%s",
			len(failures), strings.Join(failures, "\n"))
	}
}

// TestLocalDirectExampleParsesRoundTrip confirms the canonical first-run
// fixture survives the same decode pipeline the server uses for HTTP requests.
func TestLocalDirectExampleParsesRoundTrip(t *testing.T) {
	root := repositoryRoot(t)
	fixtures, err := DiscoverFixtures(root)
	require.NoError(t, err)
	report := ValidateFixture(fixtures["direct-hello"])
	for _, issue := range report.Issues {
		if issue.IsValidationError() {
			t.Fatalf("[%s/%s] %s: %s", issue.Fixture, issue.File, issue.Kind, issue.Message)
		}
	}
}

// TestScriptReferencesMatchFixtureFiles guards against stale run.sh scripts
// that post YAML files no longer shipped in the fixture.
func TestScriptReferencesMatchFixtureFiles(t *testing.T) {
	root := repositoryRoot(t)
	fixtures, err := DiscoverFixtures(root)
	require.NoError(t, err)
	var failures []string
	for _, name := range fixtures.SortedNames() {
		report := ValidateFixture(fixtures[name])
		for _, issue := range report.Issues {
			if !issue.IsValidationError() || !strings.HasSuffix(issue.File, ".sh") {
				continue
			}
			failures = append(failures, fmt.Sprintf("[%s/%s] %s", issue.Fixture, issue.File, issue.Message))
		}
	}
	if len(failures) > 0 {
		t.Fatalf("stale script references detected:\n%s", strings.Join(failures, "\n"))
	}
}
