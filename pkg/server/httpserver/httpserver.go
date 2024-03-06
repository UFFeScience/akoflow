package httpserver

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/httpserver/runworkflow"
	"net/http"
)

func StartServer() {

	http.HandleFunc("POST /runworkflow", runworkflow.RunWorkflowHandler)

	err := http.ListenAndServe(":8080", nil)

	if err != nil {
		return
	}

}
