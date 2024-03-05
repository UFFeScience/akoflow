package httpserver

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/httpserver/health"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/httpserver/index"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/httpserver/runworkflow"
	"net/http"
)

func StartServer() {

	http.HandleFunc("/", index.IndexHandler)
	http.HandleFunc("/health", health.HealthHandler)
	http.HandleFunc("/health/ready", health.HealthReadyHandler)
	http.HandleFunc("/runworkflow", runworkflow.RunWorkflowHandler)

	err := http.ListenAndServe(":8080", nil)

	if err != nil {
		return
	}

}
