package worker

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/channel"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/connector"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/k8sjob"
)

func StartWorker() {

	for {
		managerChannel := channel.GetInstance()

		result := <-managerChannel.WorfklowChannel
		println("Received job from channel")

		c := connector.New()

		c.ApplyJob(result.Namespace, result.Job.(k8sjob.K8sJob))
	}
}
