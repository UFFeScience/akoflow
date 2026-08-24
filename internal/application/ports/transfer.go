package ports

import (
	"context"
	"io"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// TransferConnector owns bytes; the planner/materializer retain policy.
type TransferConnector interface {
	CanHandle(domain.TransferEndpoint) bool
	Exists(context.Context, domain.TransferEndpoint, string) (bool, error)
	Open(context.Context, domain.TransferEndpoint, string, int64) (io.ReadCloser, error)
	// Put writes bytes at offset. Connectors must preserve the prefix already
	// present at offset, enabling resume into the .partial object.
	Put(context.Context, domain.TransferEndpoint, string, io.Reader, int64) error
	Commit(context.Context, domain.TransferEndpoint, string, string) error
}

type TransferPlanner interface {
	Plan(domain.TransferLocation, domain.TransferLocation, []domain.BlobDescriptor, map[string]bool) domain.DataTransferPlan
}
type ArtifactMaterializer interface {
	Materialize(context.Context, domain.DataTransferPlan, domain.ArtifactMaterialization) (domain.ArtifactMaterialization, domain.DataTransferRun, error)
}
