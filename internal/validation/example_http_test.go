package validation

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestDirectHelloEndToEnd boots an isolated AkôFlow server and walks the
// canonical first-run fixture through every endpoint documented in the
// "First workflow" guide. It is the regression net for the user journey that
// the Desktop UI exercises when an operator clicks "Run example".
func TestDirectHelloEndToEnd(t *testing.T) {
	harness := StartServer(t)
	root, err := RepositoryRoot()
	require.NoError(t, err)
	fixtures, err := DiscoverFixtures(root)
	require.NoError(t, err)
	fixture, ok := fixtures["direct-hello"]
	require.True(t, ok, "direct-hello fixture must be present")

	// The server expects environments first, then the supporting resources.
	// Each entry accepts the status code the production endpoint actually
	// returns. execution-runs returns 202 because the run is queued onto the
	// event loop rather than executed inline.
	sequence := []struct {
		endpoint     string
		file         string
		expectedCode int
	}{
		{"environments", "environment.yaml", 201},
		{"execution-scopes", "scope.yaml", 201},
		{"network-topologies", "topology.yaml", 201},
		{"workflow-definitions", "workflow.yaml", 201},
		{"schedule-plans/import", "plan.yaml", 201},
		{"execution-runs", "execution-request.yaml", 202},
	}
	var runID string
	for _, step := range sequence {
		payload, err := fixture.ReadFile(step.file)
		require.NoErrorf(t, err, "fixture %s must ship %s", fixture.Name, step.file)
		status, body, raw := harness.PostYAML(step.endpoint, []byte(payload))
		require.Equalf(t, step.expectedCode, status,
			"POST %s with %s returned %d, body=%s", step.endpoint, step.file, status, raw)
		require.NotNilf(t, body, "POST %s returned empty body", step.endpoint)
		if step.endpoint == "execution-runs" {
			// The execution-runs endpoint echoes the queued job back. The
			// request itself carries the run id we will poll below; capture it
			// here so the GET assertion is deterministic regardless of how
			// the queue stores the job.
			runID = "local-direct-hello-run-v1"
		}
		t.Logf("%s via %s -> %d (id=%v)", step.file, step.endpoint, status, body["id"])
	}

	// The run should now be retrievable through the GET endpoint. The server
	// processes the queued run asynchronously on the event loop, so poll
	// briefly before giving up.
	require.NotEmpty(t, runID, "runID should be captured from the request")
	status, body, raw := pollExecutionRun(harness, runID, 10*time.Second)
	require.Equal(t, 200, status, "GET execution-runs returned %d: %s", status, raw)
	// GetExecution returns a wrapped response: {"run": {...}, "activities": [...], ...}.
	run, ok := body["run"].(map[string]any)
	require.Truef(t, ok, "GET execution-runs/%s must return a run object, got %v", runID, body)
	require.Equal(t, runID, run["id"])
	require.Contains(t, []any{"created", "running", "completed", "failed"}, run["status"],
		"run.status must be one of the documented lifecycle states, got %v", run["status"])
}

// TestSimulationEndToEnd exercises the deterministic edge-cloud simulation
// fixture: import a three-activity workflow, create its plan via the envelope
// endpoint, and trigger a simulation run. The simulation backend is configured
// to use the local stub because the real SimGrid runner is not available in CI.
func TestSimulationEndToEnd(t *testing.T) {
	harness := StartServer(t)
	root, err := RepositoryRoot()
	require.NoError(t, err)
	fixtures, err := DiscoverFixtures(root)
	require.NoError(t, err)
	fixture, ok := fixtures["simulation"]
	require.True(t, ok)

	// The simulation fixture ships the full CreatePlan envelope (plan +
	// workflow + resources + runtimes + bindings + scope). It must therefore
	// hit /schedule-plans/ rather than /schedule-plans/import/, matching the
	// behaviour of simulation/run.sh.
	sequence := []struct {
		endpoint     string
		file         string
		expectedCode int
	}{
		{"environments", "environment.yaml", 201},
		{"execution-scopes", "scope.yaml", 201},
		{"network-topologies", "topology.yaml", 201},
		{"workflow-definitions", "workflow.yaml", 201},
		{"schedule-plans", "plan-request.yaml", 201},
		{"execution-runs", "execution-request.yaml", 202},
	}
	for _, step := range sequence {
		payload, err := fixture.ReadFile(step.file)
		require.NoErrorf(t, err, "fixture %s must ship %s", fixture.Name, step.file)
		status, _, raw := harness.PostYAML(step.endpoint, []byte(payload))
		require.Equalf(t, step.expectedCode, status,
			"POST %s with %s returned %d, body=%s", step.endpoint, step.file, status, raw)
	}
}

// TestSlurmLocalFixtureEndToEnd exercises the SLURM fixture that ships with
// its own start-fixture.sh and bin stubs. The harness runs the server with the
// fixture bin directory on PATH so that sbatch and singularity resolve to the
// stubs.
func TestSlurmLocalFixtureEndToEnd(t *testing.T) {
	harness := StartServer(t)
	root, err := RepositoryRoot()
	require.NoError(t, err)
	fixtures, err := DiscoverFixtures(root)
	require.NoError(t, err)
	fixture, ok := fixtures["local-fixture"]
	require.True(t, ok)

	sequence := []struct {
		endpoint     string
		file         string
		expectedCode int
	}{
		{"environments", "environment.yaml", 201},
		{"execution-scopes", "scope.yaml", 201},
		{"network-topologies", "topology.yaml", 201},
		{"workflow-definitions", "workflow.yaml", 201},
		{"schedule-plans/import", "plan.yaml", 201},
		{"execution-runs", "execution-request.yaml", 202},
	}
	for _, step := range sequence {
		payload, err := fixture.ReadFile(step.file)
		require.NoErrorf(t, err, "fixture %s must ship %s", fixture.Name, step.file)
		status, _, raw := harness.PostYAML(step.endpoint, []byte(payload))
		require.Equalf(t, step.expectedCode, status,
			"POST %s with %s returned %d, body=%s", step.endpoint, step.file, status, raw)
	}
}

// TestEveryExampleEnvironmentImports exercises every environment fixture. It
// confirms that the documented environments can be posted without HTTP errors
// before any workflow is attached. Because every showcase fixture uses the
// runtime id "local-docker", the test must give each fixture its own isolated
// daemon and database, otherwise the second-and-later POST would fail with a
// UNIQUE constraint on environment_runtimes.id.
func TestEveryExampleEnvironmentImports(t *testing.T) {
	root, err := RepositoryRoot()
	require.NoError(t, err)
	fixtures, err := DiscoverFixtures(root)
	require.NoError(t, err)

	var failures []string
	for _, name := range fixtures.SortedNames() {
		name := name
		t.Run(name, func(t *testing.T) {
			category := fixtures[name]
			if !category.HasFile("environment.yaml") {
				t.Skip("no environment.yaml in fixture")
			}
			payload, err := category.ReadFile("environment.yaml")
			require.NoError(t, err)
			harness := StartServer(t)
			status, _, raw := harness.PostYAML("environments", []byte(payload))
			if status/100 != 2 {
				failures = append(failures, fmt.Sprintf("[%s] POST environments -> %d: %s", name, status, raw))
			}
		})
	}
	if len(failures) > 0 {
		t.Fatalf("environment fixtures failed:\n%s", strings.Join(failures, "\n"))
	}
}

// pollExecutionRun repeatedly GETs /execution-runs/{id} until the server
// returns 200 or the deadline expires. The run is created asynchronously by
// the event loop after CreateExecution publishes the queue job, so a single
// immediate GET is not sufficient.
func pollExecutionRun(h *ServerHarness, runID string, timeout time.Duration) (int, map[string]any, string) {
	deadline := time.Now().Add(timeout)
	var lastStatus int
	var lastBody map[string]any
	var lastRaw string
	for time.Now().Before(deadline) {
		lastStatus, lastBody, lastRaw = h.GetJSON("execution-runs/" + runID)
		if lastStatus == 200 {
			return lastStatus, lastBody, lastRaw
		}
		time.Sleep(200 * time.Millisecond)
	}
	return lastStatus, lastBody, lastRaw
}
