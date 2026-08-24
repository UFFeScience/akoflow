package ports

import (
	"context"

	"github.com/UFFeScience/akoflow/internal/domain/instance"
)

type InstanceStore interface {
	Find(context.Context) (*instance.Instance, error)
	Save(context.Context, instance.Instance) error
	FindPreferences(context.Context, string) (*instance.UserPreferences, error)
	SavePreferences(context.Context, instance.UserPreferences) error
}
