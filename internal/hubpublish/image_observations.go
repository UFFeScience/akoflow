package hubpublish

import "fmt"

func exportImageObservations(detail map[string]any, handles []any) []map[string]any {
	run, _ := detail["run"].(map[string]any)
	runID, _ := run["id"].(string)
	attempts, _ := detail["activityAttempts"].([]any)
	if len(attempts) == 0 {
		attempts, _ = detail["activities"].([]any)
	}
	indexed := indexEnvironmentRecords(handles)
	var result []map[string]any
	for _, value := range attempts {
		task, _ := value.(map[string]any)
		activityID, _ := task["activityId"].(string)
		attempt := max(1, positiveMetric(task["attempt"]))
		handleID := runID + ":" + activityID
		if attempt > 1 {
			handleID += fmt.Sprintf(":attempt-%d", int(attempt))
		}
		metadata, _ := indexed[handleID]["metadata"].(map[string]any)
		observation, _ := metadata["imageObservation"].(map[string]any)
		if len(observation) == 0 {
			continue
		}
		record := map[string]any{"activityId": activityID, "attempt": attempt}
		for _, key := range []string{"status", "requestedReference", "registryDigest", "runtimeImageId", "platform"} {
			if value, ok := observation[key].(string); ok && value != "" {
				record[key] = value
			}
		}
		result = append(result, record)
	}
	return result
}
