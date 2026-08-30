package ports

import (
	"context"
	"io"

	domaininstance "github.com/UFFeScience/akoflow/internal/domain/instance"
)

type InstanceArchive interface {
	List(context.Context) ([]domaininstance.ArchiveInstance, error)
	Export(context.Context, io.Writer, bool) error
	Import(context.Context, io.Reader, int64) (domaininstance.ArchiveInstance, error)
}
