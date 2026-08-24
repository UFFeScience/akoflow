package config

import (
	"testing"
	"time"
)

func TestVersionAndEnvironmentCollection(t *testing.T) {
	t.Setenv("AKOFLOW_SERVER_VERSION", "1.2.3")
	if GetVersion() != "1.2.3" {
		t.Fatal("version env")
	}
	t.Setenv("AKOFLOW_SERVER_VERSION", "")
	if GetVersion() != "dev-env" {
		t.Fatal("default version")
	}
}

func TestLoadSettings(t *testing.T) {
	t.Setenv("AKOFLOW_HTTP_ADDRESS", ":9090")
	t.Setenv("AKOFLOW_NAMESPACE", "science")
	t.Setenv("K8S_API_SERVER_HOST", "kind-control-plane:6443")
	t.Setenv("K8S_API_SERVER_TOKEN", "token")
	t.Setenv("K8S_API_SERVER_CA_FILE", "/tmp/kind-ca.crt")
	t.Setenv("K8S_API_SERVER_INSECURE_SKIP_TLS_VERIFY", "true")
	t.Setenv("AKOFLOW_KUBERNETES_HISTORY_CLEANUP_ENABLED", "true")
	t.Setenv("AKOFLOW_KUBERNETES_HISTORY_CLEANUP_INTERVAL", "5m")
	t.Setenv("AKOFLOW_KUBERNETES_HISTORY_RETENTION", "12h")
	t.Setenv("AKOFLOW_SLURM_SCRIPT_DIRECTORY", "/shared/akoflow/scripts")
	t.Setenv("AKOFLOW_SIMULATION_BACKEND", "simgrid")
	t.Setenv("AKOFLOW_SIMGRID_BINARY", "/usr/local/bin/runner")
	t.Setenv("AKOFLOW_SIMGRID_WORKSPACE", "/tmp/simulations")
	t.Setenv("AKOFLOW_SIMGRID_MAX_CONCURRENT", "3")
	t.Setenv("AKOFLOW_SIMGRID_TIMEOUT", "20m")
	t.Setenv("AKOFLOW_SIMGRID_REFERENCE_FLOPS", "2500000000")
	settings := Load()
	if settings.HTTPAddress != ":9090" || settings.DefaultNamespace != "science" {
		t.Fatalf("settings=%+v", settings)
	}
	if settings.KubernetesAPIServer != "kind-control-plane:6443" ||
		settings.KubernetesToken != "token" ||
		settings.KubernetesCAFile != "/tmp/kind-ca.crt" ||
		!settings.KubernetesInsecureSkipTLS ||
		!settings.KubernetesCleanupEnabled ||
		settings.KubernetesCleanupInterval != 5*time.Minute ||
		settings.KubernetesHistoryRetention != 12*time.Hour ||
		settings.SlurmScriptDirectory != "/shared/akoflow/scripts" ||
		settings.SimulationBackend != "simgrid" ||
		settings.SimGridBinaryPath != "/usr/local/bin/runner" ||
		settings.SimGridWorkspace != "/tmp/simulations" ||
		settings.SimGridMaxConcurrent != 3 ||
		settings.SimGridTimeout != 20*time.Minute ||
		settings.SimGridReferenceFLOPS != 2.5e9 {
		t.Fatalf("Kubernetes settings=%+v", settings)
	}
}
