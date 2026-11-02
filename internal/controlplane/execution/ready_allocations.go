package execution

import "github.com/UFFeScience/akoflow/internal/domain"

type allocationResult struct {
	activityID string
	allocation domain.RuntimeAllocation
	err        error
}

// Drain only available completions; unfinished provision operations never
// hold the scheduling loop or delay unrelated eligible activities.
func drainReadyAllocations(results <-chan allocationResult, consume func(allocationResult) error) error {
	for {
		select {
		case result, ok := <-results:
			if !ok {
				return nil
			}
			if err := consume(result); err != nil {
				return err
			}
		default:
			return nil
		}
	}
}
