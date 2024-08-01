package httpserver

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/config/http_config"
	"github.com/ovvesley/akoflow/pkg/server/engine/httpserver/handlers/internal_storage_handler"
	"github.com/ovvesley/akoflow/pkg/server/engine/httpserver/handlers/storage_databasedump_handler"
	"github.com/ovvesley/akoflow/pkg/server/engine/httpserver/handlers/workflow_handler"

	"net/http"
)

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))

}
func StartServer() {

	http.HandleFunc("GET /akoflow-server/healtcheck", http_config.KernelHandler(HealthCheck, "hello"))
	http.HandleFunc("POST /akoflow-server/workflow/run", http_config.KernelHandler(workflow_handler.New().Run))

	http.HandleFunc("POST /akoflow-server/internal/storage/initial-file-list", http_config.KernelHandler(internal_storage_handler.New().InitialFileListHandler))
	http.HandleFunc("POST /akoflow-server/internal/storage/end-file-list", http_config.KernelHandler(internal_storage_handler.New().EndFileListHandler))
	http.HandleFunc("POST /akoflow-server/internal/storage/initial-disk-spec", http_config.KernelHandler(internal_storage_handler.New().InitialDiskSpecHandler))
	http.HandleFunc("POST /akoflow-server/internal/storage/end-disk-spec", http_config.KernelHandler(internal_storage_handler.New().EndDiskSpecHandler))

	http.HandleFunc("GET /akoflow-server/database-dump", http_config.KernelHandler(storage_databasedump_handler.New().DatabaseDumpHandler))

	//http.HandleFunc("GET /akoflow-admin/", ...) // Home page

	//http.HandleFunc("GET /akoflow-admin/api/workflows", ...)
	//http.HandleFunc("GET /akoflow-admin/api/workflows/{workflowId}", ...)

	//http.HandleFunc("GET /akoflow-admin/api/activities", ...)
	//http.HandleFunc("GET /akoflow-admin/api/activities/{activityId}", ...)

	//http.HandleFunc("GET /akoflow-admin/api/activities/{activityId}/logs", ...)
	//http.HandleFunc("GET /akoflow-admin/api/activities/{activityId}/metrics", ...)

	err := http.ListenAndServe(config.PORT_SERVER, nil)
	if err != nil {
		println("Error starting server", err)
		panic(err)
	}

}
