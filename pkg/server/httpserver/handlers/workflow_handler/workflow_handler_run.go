package workflow_handler

import (
	"encoding/json"
	"github.com/ovvesley/scik8sflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/scik8sflow/pkg/server/services/create_workflow_in_database_service"
	"net/http"
)

type RequestPostRunWorkflow struct {
	Workflow string `json:"workflow"`
}

func Run(w http.ResponseWriter, r *http.Request) {
	payload := RequestPostRunWorkflow{}
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	wf := workflow_entity.New(workflow_entity.WorkflowNewParams{WorkflowBase64: payload.Workflow})

	create_workflow_service := create_workflow_in_database_service.New()
	_ = create_workflow_service.Create(wf)

	response, err := json.Marshal(struct {
		Workflow string `json:"workflow_entity"`
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
