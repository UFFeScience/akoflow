package worker

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/channel"
)

func StartWorker() {

	for {
		managerChannel := channel.GetInstance()

		result := <-managerChannel.WorfklowChannel

		println("Received job with namespace: ", result.Namespace, " with id: ", result.Id)

	}
}
