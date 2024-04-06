package httpserver

import (
	"github.com/ovvesley/scik8sflow/pkg/server/config"
	"github.com/ovvesley/scik8sflow/pkg/server/httpserver/handlers/workflow_handler"
	"net/http"
)

func StartServer() {

	//
	http.HandleFunc("POST /scik8sflow-server/workflow/run", workflow_handler.Run)

	//http.HandleFunc("GET /scik8sflow-admin/", ...) // Home page

	//http.HandleFunc("GET /scik8sflow-admin/api/workflows", ...)
	//http.HandleFunc("GET /scik8sflow-admin/api/workflows/{workflowId}", ...)

	//http.HandleFunc("GET /scik8sflow-admin/api/activities", ...)
	//http.HandleFunc("GET /scik8sflow-admin/api/activities/{activityId}", ...)

	//http.HandleFunc("GET /scik8sflow-admin/api/activities/{activityId}/logs", ...)
	//http.HandleFunc("GET /scik8sflow-admin/api/activities/{activityId}/metrics", ...)

	err := http.ListenAndServe(config.PORT_SERVER, nil)
	if err != nil {
		println("Error starting server", err)
		panic(err)
	}

}
