package httpserver

import (
	"github.com/ovvesley/akoflow/pkg/server/httpserver/handlers/internal_storage_handler"
	"net/http"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/httpserver/handlers/workflow_handler"
)

func StartServer() {

	http.HandleFunc("GET /akoflow-server/healtcheck", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
	http.HandleFunc("POST /akoflow-server/workflow/run", workflow_handler.New().Run)

	http.HandleFunc("POST /akoflow-server/internal/storage/initial-file-list", internal_storage_handler.New().InitialFileListHandler)
	http.HandleFunc("POST /akoflow-server/internal/storage/end-file-list", internal_storage_handler.New().EndFileListHandler)
	http.HandleFunc("POST /akoflow-server/internal/storage/initial-disk-spec", internal_storage_handler.New().InitialDiskSpecHandler)
	http.HandleFunc("POST /akoflow-server/internal/storage/end-disk-spec", internal_storage_handler.New().EndDiskSpecHandler)

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
