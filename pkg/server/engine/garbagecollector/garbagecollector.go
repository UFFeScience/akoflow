package garbagecollector

import (
	"time"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/services/garbage_collector_remove_storage_service"
)

const TimeToUpdateSeconds = 1

func StartGarbageCollector() {

	for {
		handleGarbageCollector()
		time.Sleep(TimeToUpdateSeconds * time.Second)
		config.App().Logger.Info("Garbage Collector is running")
	}
}

func handleGarbageCollector() {

	garbageCollectorRemoveStorageService := garbage_collector_remove_storage_service.New()
	garbageCollectorRemoveStorageService.RemoveStorages()

}
