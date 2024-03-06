package orchestrator

import "time"

const TimeToUpdateSeconds = 5

func StartOrchestrator() {

	for {
		handleOrchestrator()
		handleMonitoring()
		time.Sleep(TimeToUpdateSeconds * time.Second)
	}

}

func handleOrchestrator() {
	// This function will be called every TIME_TO_UPDATE_SECONDS
	// and will be responsible for updating the state of the workflow
	// and checking if there are any jobs that need to be executed
	// or if the workflow is finished
	// If the workflow is finished, it will send a message to the user
	// saying that the workflow is finished
	// If there are jobs that need to be executed, it will send the jobs
	// to the worker
	// If there are no jobs to be executed, it will do nothing
	// and wait for the next TIME_TO_UPDATE_SECONDS
	println("Orchestrator running awaiting jobs")

	/// mark workflow as finished in database if all act jobs are finished

	/// deploy workflow that exists in the database and is not running

}

func handleMonitoring() {
	// This function will be called every TIME_TO_UPDATE_SECONDS
	// and will be responsible for monitoring the state of the jobs
	// that are being executed
	// If a job is finished, it will update the state of the workflow
	// and check if there are any more jobs to be executed
	// If there are no more jobs to be executed, it will send a message
	// to the user saying that the workflow is finished
	// If there are more jobs to be executed, it will send the jobs to
	// the worker
	// If there are no jobs to be executed, it will do nothing
	// and wait for the next TIME_TO_UPDATE_SECONDS
	println("Monitoring jobs")

	/// get metrics from k8s and update the database
	/// get logs and update the database

}
