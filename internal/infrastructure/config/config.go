package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Settings struct {
	HTTPAddress string
	// APIToken is required for non-loopback API listeners. It is deliberately
	// not generated at startup: an operator must be able to distribute it.
	APIToken                   string
	APIAllowedOrigins          []string
	LocalStorageRoot           string
	DefaultNamespace           string
	KubernetesAPIServer        string
	KubernetesToken            string
	KubernetesCAFile           string
	KubernetesInsecureSkipTLS  bool
	KubernetesCleanupEnabled   bool
	KubernetesCleanupInterval  time.Duration
	KubernetesHistoryRetention time.Duration
	ConnectionCheckInterval    time.Duration
	ConsoleEnabled             bool
	SlurmScriptDirectory       string
	SSHKeyDirectory            string
	SimulationBackend          string
	SimGridBinaryPath          string
	SimGridWorkspace           string
	SimGridMaxConcurrent       int
	SimGridTimeout             time.Duration
	SimGridReferenceFLOPS      float64
	ArtifactStoreRoot          string
	BuildContextMaxBytes       int64
	Buildctl                   string
	Apptainer                  string
}

func Load() Settings {
	settings := Settings{
		HTTPAddress: "127.0.0.1:8080", DefaultNamespace: "akoflow",
		KubernetesCleanupEnabled: true, KubernetesCleanupInterval: 15 * time.Minute,
		KubernetesHistoryRetention: 24 * time.Hour,
		ConnectionCheckInterval:    time.Minute,
		ConsoleEnabled:             false,
		SlurmScriptDirectory:       "storage/slurm/scripts",
		SSHKeyDirectory:            "storage/credentials/ssh",
		SimulationBackend:          "simgrid",
		SimGridBinaryPath:          "akoflow-simgrid-runner",
		SimGridWorkspace:           "storage/simgrid",
		SimGridMaxConcurrent:       2,
		SimGridTimeout:             30 * time.Minute,
		SimGridReferenceFLOPS:      1e9,
		ArtifactStoreRoot:          "storage/artifacts",
		BuildContextMaxBytes:       512 << 20,
	}
	loadBuild(&settings)
	loadServer(&settings)
	loadSimulation(&settings)
	return settings
}

func loadServer(settings *Settings) {
	if value := os.Getenv("AKOFLOW_HTTP_ADDRESS"); value != "" {
		settings.HTTPAddress = value
	}
	settings.APIToken = os.Getenv("AKOFLOW_API_TOKEN")
	if value := os.Getenv("AKOFLOW_API_ALLOWED_ORIGINS"); value != "" {
		for _, origin := range strings.Split(value, ",") {
			if origin = strings.TrimSpace(origin); origin != "" {
				settings.APIAllowedOrigins = append(settings.APIAllowedOrigins, origin)
			}
		}
	}
	settings.LocalStorageRoot = os.Getenv("AKOFLOW_LOCAL_STORAGE_ROOT")
	if value := os.Getenv("AKOFLOW_NAMESPACE"); value != "" {
		settings.DefaultNamespace = value
	}
	settings.KubernetesAPIServer = os.Getenv("K8S_API_SERVER_HOST")
	settings.KubernetesToken = os.Getenv("K8S_API_SERVER_TOKEN")
	settings.KubernetesCAFile = os.Getenv("K8S_API_SERVER_CA_FILE")
	settings.KubernetesInsecureSkipTLS = os.Getenv("K8S_API_SERVER_INSECURE_SKIP_TLS_VERIFY") == "true"
	if value := os.Getenv("AKOFLOW_SLURM_SCRIPT_DIRECTORY"); value != "" {
		settings.SlurmScriptDirectory = value
	}
	if value := os.Getenv("AKOFLOW_SSH_KEY_DIRECTORY"); value != "" {
		settings.SSHKeyDirectory = value
	}
	if value := os.Getenv("AKOFLOW_KUBERNETES_HISTORY_CLEANUP_ENABLED"); value != "" {
		settings.KubernetesCleanupEnabled = value == "true"
	}
	settings.KubernetesCleanupInterval = durationOrDefault(os.Getenv("AKOFLOW_KUBERNETES_HISTORY_CLEANUP_INTERVAL"), settings.KubernetesCleanupInterval)
	settings.KubernetesHistoryRetention = durationOrDefault(os.Getenv("AKOFLOW_KUBERNETES_HISTORY_RETENTION"), settings.KubernetesHistoryRetention)
	settings.ConnectionCheckInterval = durationOrDefault(os.Getenv("AKOFLOW_CONNECTION_CHECK_INTERVAL"), settings.ConnectionCheckInterval)
	settings.ConsoleEnabled = os.Getenv("AKOFLOW_CONSOLE_ENABLED") == "true"
}
func loadBuild(settings *Settings) {
	if value := os.Getenv("AKOFLOW_ARTIFACT_STORE_ROOT"); value != "" {
		settings.ArtifactStoreRoot = value
	}
	if value := os.Getenv("AKOFLOW_BUILD_CONTEXT_MAX_BYTES"); value != "" {
		if n, err := strconv.ParseInt(value, 10, 64); err == nil && n > 0 {
			settings.BuildContextMaxBytes = n
		}
	}
	settings.Buildctl = os.Getenv("AKOFLOW_BUILDCTL")
	settings.Apptainer = os.Getenv("AKOFLOW_APPTAINER")
}
func loadSimulation(settings *Settings) {
	if value := os.Getenv("AKOFLOW_SIMULATION_BACKEND"); value != "" {
		settings.SimulationBackend = value
	}
	if value := os.Getenv("AKOFLOW_SIMGRID_BINARY"); value != "" {
		settings.SimGridBinaryPath = value
	}
	if value := os.Getenv("AKOFLOW_SIMGRID_WORKSPACE"); value != "" {
		settings.SimGridWorkspace = value
	}
	settings.SimGridMaxConcurrent = integerOrDefault(
		os.Getenv("AKOFLOW_SIMGRID_MAX_CONCURRENT"), settings.SimGridMaxConcurrent,
	)
	settings.SimGridReferenceFLOPS = floatOrDefault(
		os.Getenv("AKOFLOW_SIMGRID_REFERENCE_FLOPS"), settings.SimGridReferenceFLOPS,
	)
	settings.SimGridTimeout = durationOrDefault(
		os.Getenv("AKOFLOW_SIMGRID_TIMEOUT"), settings.SimGridTimeout,
	)
}

func integerOrDefault(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if value == "" || err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

func floatOrDefault(value string, fallback float64) float64 {
	parsed, err := strconv.ParseFloat(value, 64)
	if value == "" || err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func durationOrDefault(value string, fallback time.Duration) time.Duration {
	parsed, err := time.ParseDuration(value)
	if value == "" || err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func GetVersion() string {

	versionEnv := os.Getenv("AKOFLOW_SERVER_VERSION")
	if versionEnv != "" {
		return versionEnv
	}
	return "dev-env"
}
