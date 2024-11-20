package channel

import (
	"sync"
)

var lock = &sync.Mutex{}

type Manager struct {
	WorfklowChannel chan DataChannel
}

type DataChannel struct {
	Namespace string
	Job       interface{}
	Id        int
}

var singleInstance *Manager

func GetInstance() *Manager {
	if singleInstance == nil {
		lock.Lock()
		defer lock.Unlock()
		if singleInstance == nil {
			//config.App().Logger.Info("Creating single instance now.")
			singleInstance = &Manager{}
			singleInstance.WorfklowChannel = make(chan DataChannel, 1000)
		} else {
			//config.App().Logger.Info("Single instance already created.")
		}
	} else {
		//config.App().Logger.Info("Single instance already created.")
	}

	return singleInstance
}
