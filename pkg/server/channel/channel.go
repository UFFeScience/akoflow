package channel

import (
	"fmt"
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
			fmt.Println("Creating single instance now.")
			singleInstance = &Manager{}
			singleInstance.WorfklowChannel = make(chan DataChannel, 1000)
		} else {
			fmt.Println("Single instance already created.")
		}
	} else {
		fmt.Println("Single instance already created.")
	}

	return singleInstance
}
