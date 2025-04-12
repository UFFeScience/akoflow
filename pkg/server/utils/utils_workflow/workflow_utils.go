package utils_workflow

import (
	"fmt"
	"time"

	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository/activity_repository"
)

func GetIds(workflows []workflow_entity.Workflow) []int {
	var ids []int
	for _, wf := range workflows {
		ids = append(ids, wf.Id)
	}
	return ids
}

func HydrateWorkflows(workflows []workflow_entity.Workflow, mapWfActivities activity_repository.ResultGetActivitiesByWorkflowIds) []workflow_entity.Workflow {
	var workflowsToReturn []workflow_entity.Workflow
	for _, wf := range workflows {
		if mapWfActivities[wf.Id] == nil {
			continue
		}
		wf.Spec.Activities = mapWfActivities[wf.Id]
		workflowsToReturn = append(workflowsToReturn, wf)
	}
	return workflowsToReturn
}

func HydrateWorkflow(workflow workflow_entity.Workflow, mapWfActivities activity_repository.ResultGetActivitiesByWorkflowIds) workflow_entity.Workflow {
	if mapWfActivities[workflow.Id] == nil {
		return workflow
	}
	workflow.Spec.Activities = mapWfActivities[workflow.Id]
	return workflow
}

func ParseTimestamp(timestamp string) string {
	// Define o formato da data esperado, por exemplo: "2006-01-02 15:04:05"
	layout := "2006-01-02 15:04:05"

	// Converte a string de timestamp para o formato time.Time
	t, err := time.Parse(layout, timestamp)
	if err != nil {
		// Em caso de erro na conversão, retorna uma mensagem de erro
		return fmt.Sprintf("Error parsing timestamp: %v", err)
	}

	// Retorna o timestamp no formato Unix (segundos desde 1º de janeiro de 1970)
	return fmt.Sprintf("%d", t.Unix())
}
