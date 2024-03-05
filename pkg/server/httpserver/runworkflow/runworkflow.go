package runworkflow

import (
	"encoding/json"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/parser"
	"net/http"
)

type RequestPostRunWorkflow struct {
	Workflow string `json:"workflow"`
}

func RunWorkflowHandler(w http.ResponseWriter, r *http.Request) {
	payload := RequestPostRunWorkflow{}
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	parser.Base64ToWorkflow(payload.Workflow)

	w.WriteHeader(http.StatusOK)
	return

}
