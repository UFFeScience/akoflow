package domain

// WorkflowCatalogEntry is a read model. Full commands and DAGs are loaded only
// when opening a workflow, rather than for every row in the catalog.
type WorkflowCatalogEntry struct {
	WorkflowDefinition
	ActivityCount     int                  `json:"activityCount"`
	PlanCount         int                  `json:"planCount"`
	RunCount          int                  `json:"runCount"`
	CompletedRunCount int                  `json:"completedRunCount"`
	ObservedCost      float64              `json:"observedCost"`
	LatestExecutionAt int64                `json:"latestExecutionAt"`
	RecentRuns        []WorkflowCatalogRun `json:"recentRuns"`
}

type WorkflowCatalogRun struct {
	Run  ExecutionRun `json:"run"`
	Plan SchedulePlan `json:"plan"`
}
