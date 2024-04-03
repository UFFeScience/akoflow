package httpserver

import (
	"github.com/ovvesley/scik8sflow/pkg/server/httpserver/runworkflow"
	"net/http"
)

func StartServer() {

	http.HandleFunc("POST /runworkflow", runworkflow.RunWorkflowHandler)

	err := http.ListenAndServe(":8080", nil)

	if err != nil {
		println("Error starting server", err)
		panic(err)
	}

}
