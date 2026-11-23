package hubpublish

import (
	"context"
	"net/url"
	"strconv"
)

const maxUsageAttempts = 64
const maxUsageSamples = 4096
const maxUsageSamplesPerAttempt = 240

func (m *Manager) captureResourceUsage(ctx context.Context, runPath string, summaries []any) (map[string]any, error) {
	result := map[string]any{
		"method": "evenly spaced samples including first and last",
		"groups": []any{}, "observedSamples": 0, "includedSamples": 0, "omittedAttempts": 0,
	}
	groupCount := min(len(summaries), maxUsageAttempts)
	if groupCount == 0 {
		return result, nil
	}
	limit := min(maxUsageSamplesPerAttempt, maxUsageSamples/groupCount)
	var groups []any
	included, observed, omitted := 0, 0, 0
	for index, raw := range summaries {
		summary, _ := raw.(map[string]any)
		count := int(positiveMetric(summary["samples"]))
		observed += count
		if index >= groupCount {
			omitted++
			continue
		}
		group, err := m.readUsageAttempt(ctx, runPath, summary, count, limit)
		if err != nil {
			return nil, err
		}
		if group == nil {
			continue
		}
		included += len(group["samples"].([]any))
		groups = append(groups, group)
	}
	if groups != nil {
		result["groups"] = groups
	}
	result["observedSamples"], result["includedSamples"], result["omittedAttempts"] = observed, included, omitted
	return result, nil
}

func (m *Manager) readUsageAttempt(ctx context.Context, runPath string, summary map[string]any, count, limit int) (map[string]any, error) {
	activityID, _ := summary["activityId"].(string)
	attempt := int(positiveMetric(summary["attempt"]))
	if activityID == "" || attempt < 1 || count < 1 {
		return nil, nil
	}
	var response struct {
		Items []map[string]any `json:"items"`
	}
	path := runPath + "activities/" + url.PathEscape(activityID) + "/metrics/?attempt=" + strconv.Itoa(attempt)
	if err := m.readSource(ctx, path, &response); err != nil {
		return nil, err
	}
	if len(response.Items) == 0 {
		return nil, nil
	}
	samples := sampleResourceUsage(response.Items, limit)
	return map[string]any{
		"activityId": activityID, "attempt": attempt, "observedSamples": count, "samples": samples,
	}, nil
}

func sampleResourceUsage(items []map[string]any, limit int) []any {
	size := min(len(items), limit)
	result := make([]any, 0, size)
	for index := 0; index < size; index++ {
		offset := index
		if size > 1 && size < len(items) {
			offset = index * (len(items) - 1) / (size - 1)
		}
		item := items[offset]
		record := map[string]any{}
		for _, key := range []string{"observedAt", "cpuSeconds", "memoryBytes", "readBytes", "writeBytes"} {
			if value, ok := item[key].(float64); ok && value >= 0 {
				record[key] = value
			}
		}
		if positiveMetric(record["observedAt"]) > 0 {
			result = append(result, record)
		}
	}
	return result
}
