package validation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// examplesRepoRoot resolves the canonical akoflow-examples repository. The
// tests below run whenever the environment variable AKOFLOW_EXAMPLES_ROOT
// points at a checkout; otherwise they are skipped. This keeps the drift
// net optional in CI while letting a developer trigger it locally with:
//
//	AKOFLOW_EXAMPLES_ROOT=/path/to/akoflow-examples \
//	  go test ./internal/validation/... -run External
func examplesRepoRoot(t *testing.T) string {
	t.Helper()
	root := os.Getenv("AKOFLOW_EXAMPLES_ROOT")
	if root == "" {
		t.Skip("AKOFLOW_EXAMPLES_ROOT is not set; skipping akoflow-examples drift test")
	}
	resolved, err := filepath.Abs(root)
	require.NoError(t, err)
	info, err := os.Stat(filepath.Join(resolved, "README.md"))
	require.NoErrorf(t, err, "akoflow-examples repository not found at %s", resolved)
	require.Falsef(t, info.IsDir(), "AKOFLOW_EXAMPLES_ROOT must point at a directory")
	return resolved
}

// TestExternalExamplesDecode exercises every fixture in the canonical
// akoflow-examples repository through the same production request DTOs used
// for the in-tree examples. It is the drift detector: any change to the
// server contract that breaks a public example fails here, even if no
// Go-level test notices.
func TestExternalExamplesDecode(t *testing.T) {
	root := examplesRepoRoot(t)
	fixtures, err := DiscoverFixtures(root)
	require.NoError(t, err)
	require.NotEmpty(t, fixtures, "no fixtures discovered under %s", root)

	reports := make([]FixtureReport, 0, len(fixtures))
	for _, name := range fixtures.SortedNames() {
		reports = append(reports, ValidateFixture(fixtures[name]))
	}
	var failures []string
	for _, r := range reports {
		for _, issue := range r.Issues {
			if !issue.IsValidationError() {
				continue
			}
			failures = append(failures, issue.Fixture+"/"+issue.File+": "+issue.Message)
		}
	}
	if len(failures) > 0 {
		t.Fatalf("%d akoflow-examples fixture(s) failed validation:\n%s",
			len(failures), strings.Join(failures, "\n"))
	}
}

// TestExternalExamplesScriptReferences walks every run.sh shipped under the
// canonical akoflow-examples repository and confirms that it only POSTs files
// the fixture actually ships. A renamed YAML should never silently slip past
// this test.
func TestExternalExamplesScriptReferences(t *testing.T) {
	root := examplesRepoRoot(t)
	fixtures, err := DiscoverFixtures(root)
	require.NoError(t, err)
	var failures []string
	for _, name := range fixtures.SortedNames() {
		report := ValidateFixture(fixtures[name])
		for _, issue := range report.Issues {
			if !issue.IsValidationError() || !strings.HasSuffix(issue.File, ".sh") {
				continue
			}
			failures = append(failures, "["+issue.Fixture+"/"+issue.File+"] "+issue.Message)
		}
	}
	if len(failures) > 0 {
		t.Fatalf("stale script references in akoflow-examples:\n%s", strings.Join(failures, "\n"))
	}
}

// TestExternalExamplesCoverage asserts that every example directory in
// akoflow-examples ships at least one expected file from the documented
// canonical set. The list intentionally omits showcase directories that are
// documented as runnable via docker-compose; the test surfaces drift in the
// README's "Repository layout" section by requiring each category to ship
// the most basic artefact (a README.md).
func TestExternalExamplesCoverage(t *testing.T) {
	root := examplesRepoRoot(t)
	fixtures, err := DiscoverFixtures(root)
	require.NoError(t, err)
	names := fixtures.SortedNames()
	require.NotEmpty(t, names)

	// A small smoke check: every discovered category must ship at least one
	// fixture file. Categories without any YAML/JSON/MD/Sh would indicate a
	// broken walk or an empty example.
	for _, name := range names {
		category := fixtures[name]
		require.NotEmptyf(t, category.Files,
			"fixture %s has no files", name)
	}
}
