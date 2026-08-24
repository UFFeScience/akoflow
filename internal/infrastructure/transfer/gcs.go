package transfer

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// GCS is deliberately an explicit unavailable connector until a credential
// broker is configured. Treating gs:// as HTTP would silently lose Google
// credentials and produce an unusable transfer plan.
//
// A deployment can replace this connector with a Google Cloud Storage SDK
// implementation without changing the TransferConnector contract.
type GCS struct{}

func (GCS) CanHandle(endpoint domain.TransferEndpoint) bool {
	return strings.HasPrefix(endpoint.URI, "gs://")
}
func (GCS) unavailable() error {
	return fmt.Errorf("GCS transfer is unavailable: configure a GCS transfer agent or use a signed HTTPS URL")
}
func (g GCS) Exists(context.Context, domain.TransferEndpoint, string) (bool, error) {
	return false, g.unavailable()
}
func (g GCS) Open(context.Context, domain.TransferEndpoint, string, int64) (io.ReadCloser, error) {
	return nil, g.unavailable()
}
func (g GCS) Put(context.Context, domain.TransferEndpoint, string, io.Reader, int64) error {
	return g.unavailable()
}
func (g GCS) Commit(context.Context, domain.TransferEndpoint, string, string) error {
	return g.unavailable()
}
