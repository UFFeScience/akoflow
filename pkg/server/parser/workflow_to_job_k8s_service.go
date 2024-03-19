package parser

import (
	"encoding/base64"
	"github.com/ovvesley/scik8sflow/pkg/server/entities/workflow"
	"github.com/ovvesley/scik8sflow/pkg/server/k8sjob"
	"math/rand"
	"strconv"
	"strings"
)

func WorkflowToJobK8sService(workflow workflow.Workflow) []k8sjob.K8sJob {

	k8sjobs := make([]k8sjob.K8sJob, 0)
	for _, activity := range workflow.Spec.Activities {
		k8sJob := makeJobK8s(workflow, activity)
		k8sjobs = append(k8sjobs, k8sJob)
	}
	return k8sjobs
}

func makeJobK8s(workflow workflow.Workflow, activity workflow.WorkflowActivities) k8sjob.K8sJob {

	firstContainer := makeContainer(workflow, activity)

	k8sJob := k8sjob.K8sJob{
		ApiVersion: "batch/v1",
		Kind:       "Job",
		Metadata: k8sjob.K8sJobMetadata{
			//replace _ to - and add a random number
			Name: strings.ReplaceAll(workflow.Name, "_", "-") + "-" + strconv.Itoa(rand.Intn(100)),
		},
		Spec: k8sjob.K8sJobSpec{
			Template: k8sjob.K8sJobTemplate{
				Spec: k8sjob.K8sJobSpecTemplate{
					Containers:    []k8sjob.K8sJobContainer{firstContainer},
					RestartPolicy: "Never",
					BackoffLimit:  1,
				},
			},
		},
	}

	return k8sJob
}

func makeContainer(workflow workflow.Workflow, activity workflow.WorkflowActivities) k8sjob.K8sJobContainer {
	command := base64.StdEncoding.EncodeToString([]byte(activity.Run))

	container := k8sjob.K8sJobContainer{
		Name:    "activity-0" + strconv.Itoa(rand.Intn(100)),
		Image:   workflow.Spec.Image,
		Command: []string{"/bin/sh", "-c", "echo " + command + "| base64 -d| sh"},
	}

	return container
}
