package worker

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/engine/channel"
	"github.com/ovvesley/akoflow/pkg/server/services/run_activity_in_cluster_service"
)

var FLAG_ID_WORKER_STOP_LISTENING = -1

type Worker struct {
}

func New() *Worker {
	return &Worker{}
}

func (w *Worker) StartWorker() {
	for {

		managerChannel := channel.GetInstance()
		result := <-managerChannel.WorfklowChannel

		if result.Id == FLAG_ID_WORKER_STOP_LISTENING {
			break
		}

		runActivityInClusterService := run_activity_in_cluster_service.New()
		runActivityInClusterService.Run(result.Id)

		config.App().Logger.Info("Worker: Activity finished", result.Id)
	}
}
