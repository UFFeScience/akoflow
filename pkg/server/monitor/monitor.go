package monitor

import "time"

func StartMonitor() {
	for {
		println("Monitor is running")
		time.Sleep(5 * time.Second)
	}
}
