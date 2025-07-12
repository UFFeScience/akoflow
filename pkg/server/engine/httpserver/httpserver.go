package httpserver

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/config/http_config"
	"github.com/ovvesley/akoflow/pkg/server/engine/httpserver/handlers/akoflow_admin_handler"
	"github.com/ovvesley/akoflow/pkg/server/engine/httpserver/handlers/internal_storage_handler"
	"github.com/ovvesley/akoflow/pkg/server/engine/httpserver/handlers/public_static_handler"
	"github.com/ovvesley/akoflow/pkg/server/engine/httpserver/handlers/runtime_api_handler"
	"github.com/ovvesley/akoflow/pkg/server/engine/httpserver/handlers/schedule_api_handler"
	"github.com/ovvesley/akoflow/pkg/server/engine/httpserver/handlers/storage_databasedump_handler"
	"github.com/ovvesley/akoflow/pkg/server/engine/httpserver/handlers/workflow_api_handler"
	"github.com/ovvesley/akoflow/pkg/server/engine/httpserver/handlers/workflow_handler"

	"net/http"
)

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))

}
func StartServer() {

	http.HandleFunc("GET /", http_config.KernelHandler(public_static_handler.New().Static))

	http.HandleFunc("GET /akoflow-server/check-service/", http_config.KernelHandler(HealthCheck, "hello"))
	http.HandleFunc("POST /akoflow-server/workflow/", http_config.KernelHandler(workflow_handler.New().Create))

	http.HandleFunc("POST /akoflow-server/internal/storage/initial-file-list/", http_config.KernelHandler(internal_storage_handler.New().InitialFileListHandler))
	http.HandleFunc("POST /akoflow-server/internal/storage/end-file-list/", http_config.KernelHandler(internal_storage_handler.New().EndFileListHandler))
	http.HandleFunc("POST /akoflow-server/internal/storage/initial-disk-spec/", http_config.KernelHandler(internal_storage_handler.New().InitialDiskSpecHandler))
	http.HandleFunc("POST /akoflow-server/internal/storage/end-disk-spec/", http_config.KernelHandler(internal_storage_handler.New().EndDiskSpecHandler))

	http.HandleFunc("GET /akoflow-server/database-dump/", http_config.KernelHandler(storage_databasedump_handler.New().DatabaseDumpHandler))

	http.HandleFunc("GET /akoflow-admin/", http_config.KernelHandler(akoflow_admin_handler.New().Home))
	http.HandleFunc("GET /akoflow-admin/runtimes", http_config.KernelHandler(akoflow_admin_handler.New().Runtime))
	http.HandleFunc("GET /akoflow-admin/schedules", http_config.KernelHandler(akoflow_admin_handler.New().Schedule))
	http.HandleFunc("GET /akoflow-admin/workflows/{namespace}/{workflowId}/", http_config.KernelHandler(akoflow_admin_handler.New().WorkflowDetail))

	http.HandleFunc("GET /akoflow-api/workflows/", http_config.KernelHandler(workflow_api_handler.New().ListAllWorkflows))
	//http.HandleFunc("POST /akoflow-api/workflows/", http_config.KernelHandler(workflow_api_handler.New().CreateWorkflow))
	//http.HandleFunc("POST /akoflow-api/validate-workflow/", http_config.KernelHandler(workflow_api_handler.New().ValidateWorkflow))
	//
	http.HandleFunc("GET /akoflow-api/workflows/{workflowId}/", http_config.KernelHandler(workflow_api_handler.New().GetWorkflow))
	http.HandleFunc("GET /akoflow-api/runtimes/", http_config.KernelHandler(runtime_api_handler.New().ListAllRuntimes))

	http.HandleFunc("GET /akoflow-api/schedules/", http_config.KernelHandler(schedule_api_handler.New().ListAllSchedules))
	http.HandleFunc("POST /akoflow-api/schedules/", http_config.KernelHandler(schedule_api_handler.New().CreateSchedule))
	http.HandleFunc("GET /akoflow-api/schedules/{scheduleId}/", http_config.KernelHandler(schedule_api_handler.New().GetSchedule))

	//http.HandleFunc("GET /akoflow-api/workflows/{workflowId}/activities/", http_config.KernelHandler(workflow_api_handler.New().ListAllActivities))
	//http.HandleFunc("GET /akoflow-api/workflows/{workflowId}/activities/{activityId}/", http_config.KernelHandler(workflow_api_handler.New().GetActivity))
	//http.HandleFunc("GET /akoflow-api/workflows/{workflowId}/activities/{activityId}/logs/", http_config.KernelHandler(workflow_api_handler.New().ListAllLogs))
	//http.HandleFunc("GET /akoflow-api/workflows/{workflowId}/activities/{activityId}/metrics-cpu/", http_config.KernelHandler(workflow_api_handler.New().ListAllMetricsCPU))
	//http.HandleFunc("GET /akoflow-api/workflows/{workflowId}/activities/{activityId}/metrics-memory/", http_config.KernelHandler(workflow_api_handler.New().ListAllMetricsMemory))
	//
	//http.HandleFunc("GET /akoflow-api/workflows/{workflowId}/metrics-cpu/", http_config.KernelHandler(workflow_api_handler.New().ListAllMetricsCPU))
	//http.HandleFunc("GET /akoflow-api/workflows/{workflowId}/metrics-memory/", http_config.KernelHandler(workflow_api_handler.New().ListAllMetricsMemory))
	//http.HandleFunc("GET /akoflow-api/workflows/{workflowId}/metrics-timeline/", http_config.KernelHandler(workflow_api_handler.New().ListAllMetricsTimeline))
	//
	//http.HandleFunc("GET /akoflow-api/workflows/{workflowId}/storages/", http_config.KernelHandler(workflow_api_handler.New().GetStorages))
	//http.HandleFunc("GET /akoflow-api/workflows/{workflowId}/storages/{storageId}/", http_config.KernelHandler(workflow_api_handler.New().GetStorage))
	//http.HandleFunc("GET /akoflow-api/workflows/{workflowId}/storages/{storageId}/download-file/", http_config.KernelHandler(workflow_api_handler.New().DownloadFile))

	err := http.ListenAndServe(config.PORT_SERVER, nil)
	if err != nil {
		println("Error starting server", err)
		panic(err)
	}

}
