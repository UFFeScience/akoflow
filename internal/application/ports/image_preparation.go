package ports

import (
	"context"

	domainexecution "github.com/UFFeScience/akoflow/internal/domain/execution"
)

type ImagePreparationRecorder interface {
	RecordImagePreparation(context.Context, string, domainexecution.ImagePreparationObservation) error
}
