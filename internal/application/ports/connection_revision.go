package ports

import (
	"context"
	"errors"

	"github.com/UFFeScience/akoflow/internal/domain"
)

var ErrConnectionRevisionConflict = errors.New("connection settings changed after this revision was read")

type ConnectionRevisionStore interface {
	UpdateConnectionRevision(context.Context, domain.EnvironmentConnection) (domain.EnvironmentConnection, error)
}
