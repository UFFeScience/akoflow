package httpserver

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/UFFeScience/akoflow/internal/api/handlers/workflow_engine_api_handler"
	"github.com/UFFeScience/akoflow/internal/infrastructure/config"
	"github.com/UFFeScience/akoflow/internal/infrastructure/config/http_config"
)

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func NewMux(workflowEngine *workflow_engine_api_handler.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", http_config.KernelHandler(HealthCheck))
	mux.HandleFunc("GET /akoflow-api/instance/", http_config.KernelHandler(workflowEngine.GetInstance))
	mux.HandleFunc("GET /akoflow-api/search/", http_config.KernelHandler(workflowEngine.Search))
	mux.HandleFunc("GET /akoflow-api/audit-events/", http_config.KernelHandler(workflowEngine.ListAuditEvents))
	mux.HandleFunc("GET /akoflow-api/console-commands/", http_config.KernelHandler(workflowEngine.ListConsoleCommands))
	mux.HandleFunc("POST /akoflow-api/console-commands/", http_config.KernelHandler(workflowEngine.ExecuteConsoleCommand))
	mux.HandleFunc("POST /akoflow-api/console-sessions/", http_config.KernelHandler(workflowEngine.OpenConsoleSession))
	mux.HandleFunc("GET /akoflow-api/console-sessions/", http_config.KernelHandler(workflowEngine.ListConsoleSessions))
	mux.HandleFunc("DELETE /akoflow-api/console-sessions/{sessionId}/", http_config.KernelHandler(workflowEngine.CloseConsoleSession))
	mux.HandleFunc("GET /akoflow-api/console-sessions/{sessionId}/log/", http_config.KernelHandler(workflowEngine.ExportConsoleSessionLog))
	mux.HandleFunc("GET /akoflow-api/console-sessions/{sessionId}/stream/", workflowEngine.StreamConsoleSession)
	mux.HandleFunc("GET /akoflow-api/ssh-keys/", http_config.KernelHandler(workflowEngine.ListSSHKeys))
	mux.HandleFunc("POST /akoflow-api/ssh-keys/", http_config.KernelHandler(workflowEngine.GenerateSSHKey))
	mux.HandleFunc("PUT /akoflow-api/instance/", http_config.KernelHandler(workflowEngine.SaveInstance))
	mux.HandleFunc("GET /akoflow-api/user-preferences/{clientId}/", http_config.KernelHandler(workflowEngine.GetUserPreferences))
	mux.HandleFunc("PUT /akoflow-api/user-preferences/{clientId}/", http_config.KernelHandler(workflowEngine.SaveUserPreferences))

	mux.HandleFunc("POST /akoflow-api/environments/", http_config.KernelHandler(workflowEngine.CreateEnvironment))
	mux.HandleFunc("GET /akoflow-api/environments/", http_config.KernelHandler(workflowEngine.ListEnvironments))
	mux.HandleFunc("GET /akoflow-api/environments/{environmentId}/", http_config.KernelHandler(workflowEngine.GetEnvironment))
	mux.HandleFunc("PUT /akoflow-api/environments/{environmentId}/", http_config.KernelHandler(workflowEngine.ReplaceEnvironment))
	mux.HandleFunc("DELETE /akoflow-api/environments/{environmentId}/", http_config.KernelHandler(workflowEngine.DeleteEnvironment))
	mux.HandleFunc("GET /akoflow-api/environments/{environmentId}/storages/", http_config.KernelHandler(workflowEngine.ListStorages))
	mux.HandleFunc("GET /akoflow-api/storages/{storageId}/roots/", http_config.KernelHandler(workflowEngine.StorageRoots))
	mux.HandleFunc("GET /akoflow-api/storages/{storageId}/entries/", http_config.KernelHandler(workflowEngine.BrowseStorage))
	mux.HandleFunc("DELETE /akoflow-api/storages/{storageId}/entries/", http_config.KernelHandler(workflowEngine.DeleteStorageEntry))
	mux.HandleFunc("GET /akoflow-api/storages/{storageId}/entry/", http_config.KernelHandler(workflowEngine.StatStorageEntry))
	mux.HandleFunc("GET /akoflow-api/storages/{storageId}/stat/", http_config.KernelHandler(workflowEngine.StatStorageEntry))
	mux.HandleFunc("POST /akoflow-api/storages/{storageId}/downloads/", http_config.KernelHandler(workflowEngine.CreateDownload))
	mux.HandleFunc("POST /akoflow-api/storages/{storageId}/checksum/", http_config.KernelHandler(workflowEngine.ChecksumStorageEntry))
	mux.HandleFunc("POST /akoflow-api/storages/{storageId}/copies/", http_config.KernelHandler(workflowEngine.CopyStorageEntry))
	mux.HandleFunc("POST /akoflow-api/storages/{storageId}/archives/", http_config.KernelHandler(workflowEngine.ArchiveStorageDirectory))
	mux.HandleFunc("POST /akoflow-api/storages/{storageId}/promote-data/", http_config.KernelHandler(workflowEngine.PromoteStorageData))
	mux.HandleFunc("POST /akoflow-api/storages/{storageId}/promote-artifact/", http_config.KernelHandler(workflowEngine.PromoteStorageArtifact))
	mux.HandleFunc("GET /akoflow-api/storages/{storageId}/index-runs/", http_config.KernelHandler(workflowEngine.ListStorageIndexRuns))
	mux.HandleFunc("POST /akoflow-api/storages/{storageId}/index-runs/", http_config.KernelHandler(workflowEngine.StartStorageIndex))
	mux.HandleFunc("GET /akoflow-api/storage-downloads/{downloadId}/", http_config.KernelHandler(workflowEngine.GetDownload))
	mux.HandleFunc("GET /akoflow-api/storage-downloads/{downloadId}/content/", workflowEngine.StreamDownload)
	mux.HandleFunc("POST /akoflow-api/environment-connections/{connectionId}/health/", http_config.KernelHandler(workflowEngine.CheckEnvironmentConnection))
	mux.HandleFunc("PUT /akoflow-api/environment-connections/{connectionId}/", http_config.KernelHandler(workflowEngine.UpdateEnvironmentConnection))
	mux.HandleFunc("POST /akoflow-api/environment-connections/{connectionId}/discover/", http_config.KernelHandler(workflowEngine.DiscoverEnvironmentConnection))
	mux.HandleFunc("GET /akoflow-api/environment-connections/{connectionId}/history/", http_config.KernelHandler(workflowEngine.ListEnvironmentConnectionHistory))
	mux.HandleFunc("GET /akoflow-api/resources/", http_config.KernelHandler(workflowEngine.ListResources))
	mux.HandleFunc("GET /akoflow-api/artifact-locations/", http_config.KernelHandler(workflowEngine.ListArtifactLocations))
	mux.HandleFunc("GET /akoflow-api/artifacts/", http_config.KernelHandler(workflowEngine.ListArtifacts))
	mux.HandleFunc("GET /akoflow-api/artifact-materializations/", http_config.KernelHandler(workflowEngine.ListArtifactMaterializations))
	mux.HandleFunc("POST /akoflow-api/artifact-materializations/", http_config.KernelHandler(workflowEngine.SaveArtifactMaterialization))
	mux.HandleFunc("POST /akoflow-api/build-contexts/", http_config.KernelHandler(workflowEngine.SaveBuildContext))
	mux.HandleFunc("POST /akoflow-api/artifact-builds/", http_config.KernelHandler(workflowEngine.CreateArtifactBuild))
	mux.HandleFunc("POST /akoflow-api/artifacts/docker/", http_config.KernelHandler(workflowEngine.RegisterDockerArtifact))
	mux.HandleFunc("GET /akoflow-api/artifact-builds/{buildId}/", http_config.KernelHandler(workflowEngine.GetArtifactBuild))
	mux.HandleFunc("GET /akoflow-api/artifacts/{artifactId}/builds/", http_config.KernelHandler(workflowEngine.ListArtifactBuilds))
	mux.HandleFunc("GET /akoflow-api/artifact-builds/{buildId}/runs/", http_config.KernelHandler(workflowEngine.ListBuildRuns))
	mux.HandleFunc("POST /akoflow-api/artifact-builds/{buildId}/runs/", http_config.KernelHandler(workflowEngine.StartArtifactBuildRun))
	mux.HandleFunc("GET /akoflow-api/build-runs/{runId}/", http_config.KernelHandler(workflowEngine.GetBuildRun))
	mux.HandleFunc("GET /akoflow-api/build-runs/{runId}/output/", workflowEngine.StreamBuildOutput)
	mux.HandleFunc("GET /akoflow-api/resources/{resourceId}/", http_config.KernelHandler(workflowEngine.GetResource))
	mux.HandleFunc("GET /akoflow-api/resources/{resourceId}/snapshot/", http_config.KernelHandler(workflowEngine.GetResourceSnapshot))
	mux.HandleFunc("POST /akoflow-api/resources/", http_config.KernelHandler(workflowEngine.CreateResource))
	mux.HandleFunc("POST /akoflow-api/network-topologies/", http_config.KernelHandler(workflowEngine.CreateNetworkTopology))
	mux.HandleFunc("GET /akoflow-api/network-topologies/", http_config.KernelHandler(workflowEngine.ListNetworkTopologies))
	mux.HandleFunc("GET /akoflow-api/network-topologies/{topologyId}/", http_config.KernelHandler(workflowEngine.GetNetworkTopology))
	mux.HandleFunc("POST /akoflow-api/execution-scopes/", http_config.KernelHandler(workflowEngine.CreateExecutionScope))
	mux.HandleFunc("GET /akoflow-api/execution-scopes/", http_config.KernelHandler(workflowEngine.ListExecutionScopes))
	mux.HandleFunc("GET /akoflow-api/execution-scopes/{scopeId}/", http_config.KernelHandler(workflowEngine.GetExecutionScope))
	mux.HandleFunc("DELETE /akoflow-api/execution-scopes/{scopeId}/", http_config.KernelHandler(workflowEngine.DeleteExecutionScope))
	mux.HandleFunc("POST /akoflow-api/workflow-definitions/", http_config.KernelHandler(workflowEngine.CreateWorkflow))
	mux.HandleFunc("POST /akoflow-api/workflow-definitions/import/", http_config.KernelHandler(workflowEngine.CreateWorkflow))
	mux.HandleFunc("GET /akoflow-api/workflow-definitions/", http_config.KernelHandler(workflowEngine.ListWorkflows))
	mux.HandleFunc("GET /akoflow-api/workflow-definitions/{workflowId}/", http_config.KernelHandler(workflowEngine.GetWorkflow))
	mux.HandleFunc("GET /akoflow-api/workflow-definitions/{workflowId}/export/", http_config.KernelHandler(workflowEngine.ExportWorkflow))
	mux.HandleFunc("POST /akoflow-api/workflow-definition-actions/duplicate/{workflowId}/", http_config.KernelHandler(workflowEngine.DuplicateWorkflow))
	mux.HandleFunc("POST /akoflow-api/schedule-plans/", http_config.KernelHandler(workflowEngine.CreatePlan))
	mux.HandleFunc("GET /akoflow-api/schedule-plans/", http_config.KernelHandler(workflowEngine.ListPlans))
	mux.HandleFunc("GET /akoflow-api/schedule-plans/{planId}/", http_config.KernelHandler(workflowEngine.GetPlan))
	mux.HandleFunc("POST /akoflow-api/execution-runs/", http_config.KernelHandler(workflowEngine.CreateExecution))
	mux.HandleFunc("GET /akoflow-api/execution-runs/", http_config.KernelHandler(workflowEngine.ListExecutions))
	mux.HandleFunc("GET /akoflow-api/execution-runs/{runId}/", http_config.KernelHandler(workflowEngine.GetExecution))
	return mux
}

func Serve(ctx context.Context, address string, workflowEngine *workflow_engine_api_handler.Handler) error {
	settings := config.Load()
	return ServeSecure(ctx, address, workflowEngine, SecurityOptions{
		BearerToken: settings.APIToken, AllowedOrigins: settings.APIAllowedOrigins,
		LoopbackOnly: !isLoopbackAddress(address) && settings.APIToken == "",
	})
}

func ServeSecure(ctx context.Context, address string, workflowEngine *workflow_engine_api_handler.Handler, security SecurityOptions) error {
	handler := AllowCORSFor(SecureAPI(NewMux(workflowEngine), security), security.AllowedOrigins)
	server := &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	go shutdownWhenCanceled(ctx, server)
	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func isLoopbackAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func shutdownWhenCanceled(ctx context.Context, server *http.Server) {
	<-ctx.Done()
	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownContext)
}
