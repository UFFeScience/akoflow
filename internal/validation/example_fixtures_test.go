package validation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiscoverFixturesIndexesLocalDirectExample(t *testing.T) {
	root := repositoryRoot(t)
	fixtures, err := DiscoverFixtures(root)
	require.NoError(t, err)

	// Every fixture under examples/local/direct-hello must be present.
	keys := fixtures.SortedNames()
	require.Contains(t, keys, "direct-hello")

	category := fixtures["direct-hello"]
	require.True(t, category.HasFile("workflow.yaml"))
	require.True(t, category.HasFile("environment.yaml"))
	require.True(t, category.HasFile("scope.yaml"))
	require.True(t, category.HasFile("topology.yaml"))
	require.True(t, category.HasFile("plan.yaml"))
	require.True(t, category.HasFile("execution-request.yaml"))
	require.True(t, category.HasFile("run.sh"))
}

func TestDiscoverFixturesCoversEveryExampleCategory(t *testing.T) {
	root := repositoryRoot(t)
	fixtures, err := DiscoverFixtures(root)
	require.NoError(t, err)

	// The CI catalog must include the canonical first-run example plus the
	// deterministic simulation fixtures used by the docs.
	for _, required := range []string{
		"direct-hello",
		"simulation",
		"30gb-fanout",
		"50core-fanout",
		"slurm",
	} {
		require.Contains(t, fixtures.SortedNames(), required, "missing fixture category %s", required)
	}
}

func TestReadFileSurfacesMissingFixtureError(t *testing.T) {
	category := FixtureCategory{Name: "test", Files: map[string]string{}}
	_, err := category.ReadFile("missing.yaml")
	require.ErrorContains(t, err, "does not contain missing.yaml")
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	// Walk up until we find go.mod.
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repository root above %s", wd)
		}
		dir = parent
	}
}
