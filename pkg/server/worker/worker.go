package worker

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/channel"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/services/run_activity_in_cluster_service"
)

func StartWorker() {

	for {
		managerChannel := channel.GetInstance()
		result := <-managerChannel.WorfklowChannel
		handleWorker(result)
		println("Worker is Listening...")
	}
}

func handleWorker(result channel.DataChannel) {

	runActivityInClusterService := run_activity_in_cluster_service.New()

	runActivityInClusterService.Run(result.Id)
}
