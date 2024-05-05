package httpserver

import (
	"net/http"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/httpserver/handlers/workflow_handler"
)

func StartServer() {

	//
	http.HandleFunc("POST /akoflow-server/workflow/run", workflow_handler.Run)

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
