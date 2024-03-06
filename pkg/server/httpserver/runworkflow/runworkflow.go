package runworkflow

import (
	"encoding/json"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/manager"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/workflow"
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

	wf := workflow.New(payload.Workflow)

	manager.DeployWorkflow(wf)

	response, err := json.Marshal(struct {
		Workflow string `json:"workflow"`
		Message  string `json:"message"`
	}{
		Workflow: wf.Name,
		Message:  "Workflow has been deployed successfully.",
	})

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(response)
	if err != nil {
		return
	}

	return

}
