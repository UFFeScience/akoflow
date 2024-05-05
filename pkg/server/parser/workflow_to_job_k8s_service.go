package parser

import (
	"encoding/base64"
	"math/rand"
	"strconv"
	"strings"

	"github.com/ovvesley/akoflow/pkg/server/entities/k8s_job_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"gopkg.in/yaml.v3"
)

func WorkflowToJobK8sService(workflow workflow_entity.Workflow) []k8s_job_entity.K8sJob {

	k8sjobs := make([]k8s_job_entity.K8sJob, 0)
	for _, activity := range workflow.Spec.Activities {
		k8sJob := makeJobK8s(workflow, activity)
		k8sjobs = append(k8sjobs, k8sJob)
	}
	return k8sjobs
}

func makeJobK8s(workflow workflow_entity.Workflow, activity workflow.WorkflowActivities) k8s_job_entity.K8sJob {

	firstContainer := makeContainer(workflow, activity)

	k8sJob := k8s_job_entity.K8sJob{
		ApiVersion: "batch/v1",
		Kind:       "Job",
		Metadata: k8s_job_entity.K8sJobMetadata{
			//replace _ to - and add a random number
			Name: strings.ReplaceAll(workflow.Name, "_", "-") + "-" + strconv.Itoa(rand.Intn(100)),
		},
		Spec: k8s_job_entity.K8sJobSpec{
			Template: k8s_job_entity.K8sJobTemplate{
				Spec: k8s_job_entity.K8sJobSpecTemplate{
					Containers:    []k8s_job_entity.K8sJobContainer{firstContainer},
					RestartPolicy: "Never",
					BackoffLimit:  1,
				},
			},
		},
	}

	return k8sJob
}

func makeContainer(workflow workflow_entity.Workflow, activity workflow_entity.WorkflowActivities) k8s_job_entity.K8sJobContainer {
	command := base64.StdEncoding.EncodeToString([]byte(activity.Run))

	container := k8s_job_entity.K8sJobContainer{
		Name:    "activity-0" + strconv.Itoa(rand.Intn(100)),
		Image:   workflow.Spec.Image,
		Command: []string{"/bin/sh", "-c", "echo " + command + "| base64 -d| sh"},
	}

	return container
}

type ParamsDatabaseToWorkflowActivities struct {
	WorkflowActivityDatabase WorkflowActivityDatabase
}

func DatabaseToWorkflowActivities(params ParamsDatabaseToWorkflowActivities) WorkflowActivities {

	activityDecoding, err := base64.StdEncoding.DecodeString(params.WorkflowActivityDatabase.ResourceK8sBase64)
	if err != nil {
		return WorkflowActivities{}
	}

	activityString := string(activityDecoding)

	wfa := WorkflowActivities{}
	err = yaml.Unmarshal([]byte(activityString), &wfa)
	if err != nil {
		return WorkflowActivities{}
	}

	return WorkflowActivities{
		Id:          params.WorkflowActivityDatabase.Id,
		Name:        params.WorkflowActivityDatabase.Name,
		Status:      params.WorkflowActivityDatabase.Status,
		Run:         wfa.Run,
		WorkflowId:  params.WorkflowActivityDatabase.WorkflowId,
		MemoryLimit: wfa.MemoryLimit,
		CpuLimit:    wfa.CpuLimit,
		DependsOn:   wfa.DependsOn,
	}
}
