package garbagecollector

import (
	"github.com/ovvesley/akoflow/pkg/server/services/garbage_collector_remove_storage_service"
	"time"
)

const TimeToUpdateSeconds = 1

func StartGarbageCollector() {

	for {
		handleGarbageCollector()
		time.Sleep(TimeToUpdateSeconds * time.Second)
		println("GarbageCollector is Listening...")
	}
}

func handleGarbageCollector() {

	garbageCollectorRemoveStorageService := garbage_collector_remove_storage_service.New()
	garbageCollectorRemoveStorageService.RemoveStorages()

}
