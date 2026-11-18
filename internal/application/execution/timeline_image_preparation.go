package execution

import (
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	domainexecution "github.com/UFFeScience/akoflow/internal/domain/execution"
)

type preparationActivity struct {
	id      string
	attempt int
}

func appendImagePreparation(timeline *domain.ExecutionTimeline, observations []domainexecution.ImagePreparationObservation, now time.Time) map[preparationActivity][]timelineInterval {
	byActivity := make(map[preparationActivity][]timelineInterval)
	owners := make(map[string]bool)
	for _, observation := range observations {
		if !observation.Shared {
			owners[observation.OperationID] = true
		}
	}
	seen := make(map[string]struct{})
	for _, observation := range observations {
		if observation.OperationID == "" || observation.StartedAt.IsZero() {
			continue
		}
		switch observation.Kind {
		case domainexecution.ImagePreparationPull, domainexecution.ImagePreparationBuild, domainexecution.ImagePreparationPush, domainexecution.ImagePreparationSIF, domainexecution.ImagePreparationRegister, domainexecution.ImagePreparationCacheHit:
		default:
			continue
		}
		start := unixSeconds(observation.StartedAt)
		active := observation.FinishedAt == nil && timeline.FinishedAt == 0
		finish := timeline.FinishedAt
		if observation.FinishedAt != nil {
			finish = unixSeconds(*observation.FinishedAt)
		} else if active {
			finish = unixSeconds(now)
		}
		finish = maxTimeline(start, finish)
		key := preparationActivity{observation.ActivityID, max(1, observation.Attempt)}
		byActivity[key] = append(byActivity[key], timelineInterval{start, finish})
		if _, exists := seen[observation.OperationID]; exists || observation.Shared && owners[observation.OperationID] {
			continue
		}
		seen[observation.OperationID] = struct{}{}
		item := phase(observation.OperationID, string(observation.Kind), observation.ActivityID,
			observation.OperationID, "", observation.Status, "image-preparation", start, finish)
		item.Attempt = observation.Attempt
		item.Active = active
		item.Progress = observation.Progress
		timeline.Phases = append(timeline.Phases, item)
	}
	return byActivity
}
