package hubpublish

import (
	"math"
	"time"
)

type attemptKey struct {
	activityID string
	attempt    float64
}

type preparationDurations struct {
	pull, build, sif, push, register, reuse *float64
}

type publishedAttemptTiming struct {
	ReadyAt         *time.Time `json:"readyAt"`
	QueuedAt        *time.Time `json:"queuedAt"`
	StartedAt       *time.Time `json:"startedAt"`
	FinishedAt      *time.Time `json:"finishedAt"`
	QueueSeconds    *float64   `json:"queueSeconds"`
	PullSeconds     *float64   `json:"imagePullSeconds"`
	BuildSeconds    *float64   `json:"imageBuildSeconds"`
	SIFSeconds      *float64   `json:"sifBuildSeconds"`
	PushSeconds     *float64   `json:"imagePushSeconds"`
	RegisterSeconds *float64   `json:"sifRegisterSeconds"`
	ReuseSeconds    *float64   `json:"imageReuseSeconds"`
	TransferSeconds *float64   `json:"transferSeconds"`
	RuntimeSeconds  *float64   `json:"runtimeSeconds"`
}

func preparationTiming(timeline map[string]any) map[attemptKey]preparationDurations {
	result := map[attemptKey]preparationDurations{}
	phases, _ := timeline["phases"].([]any)
	for _, entry := range phases {
		phase, _ := entry.(map[string]any)
		activityID, _ := phase["activityId"].(string)
		kind, _ := phase["category"].(string)
		status, _ := phase["status"].(string)
		if activityID == "" || (status != "completed" && status != "failed" && status != "cancelled") {
			continue
		}
		seconds, ok := phase["durationSeconds"].(float64)
		if !ok || seconds < 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
			continue
		}
		key := attemptKey{activityID, max(1, positiveMetric(phase["attempt"]))}
		durations := result[key]
		switch kind {
		case "image-push":
			durations.push = addDuration(durations.push, seconds)
		case "sif-register":
			durations.register = addDuration(durations.register, seconds)
		case "image-cache-hit":
			durations.reuse = addDuration(durations.reuse, seconds)
		case "image-pull":
			durations.pull = addDuration(durations.pull, seconds)
		case "image-build":
			durations.build = addDuration(durations.build, seconds)
		case "sif-build":
			durations.sif = addDuration(durations.sif, seconds)
		default:
			continue
		}
		result[key] = durations
	}
	return result
}

func addDuration(previous *float64, seconds float64) *float64 {
	if previous != nil {
		seconds += *previous
	}
	return &seconds
}

func describeAttemptTiming(task map[string]any, preparation preparationDurations) publishedAttemptTiming {
	return publishedAttemptTiming{
		ReadyAt: utcTimestamp(task["readyAt"]), QueuedAt: utcTimestamp(task["queuedAt"]),
		StartedAt: utcTimestamp(task["startedAt"]), FinishedAt: utcTimestamp(task["finishedAt"]),
		QueueSeconds: measuredDuration(task["queueSeconds"]), RuntimeSeconds: measuredDuration(task["runtimeSeconds"]),
		TransferSeconds: measuredDuration(task["transferSeconds"]),
		PullSeconds:     preparation.pull, BuildSeconds: preparation.build, SIFSeconds: preparation.sif,
		PushSeconds: preparation.push, RegisterSeconds: preparation.register, ReuseSeconds: preparation.reuse,
	}
}

func measuredDuration(value any) *float64 {
	if number := positiveMetric(value); number > 0 {
		return &number
	}
	return nil
}

func utcTimestamp(value any) *time.Time {
	number := positiveMetric(value)
	if number == 0 || number >= 253402300800 {
		return nil
	}
	seconds, fraction := math.Modf(number)
	stamp := time.Unix(int64(seconds), int64(fraction*float64(time.Second))).UTC()
	return &stamp
}

func attemptStatus(value any) string {
	switch status, _ := value.(string); status {
	case "queued", "preparing", "running", "completed", "failed", "cancelled", "interrupted", "skipped":
		return status
	default:
		return "unknown"
	}
}
