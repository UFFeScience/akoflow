package transfer

import (
	"context"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestGCSFailsExplicitlyUntilAgentIsConfigured(t *testing.T) {
	gcs := GCS{}
	endpoint := domain.TransferEndpoint{URI: "gs://research-artifacts/images"}
	if !gcs.CanHandle(endpoint) {
		t.Fatal("GCS connector must claim gs:// endpoints to avoid an ambiguous route")
	}
	if _, err := gcs.Exists(context.Background(), endpoint, "image.sif"); err == nil {
		t.Fatal("unconfigured GCS must return an actionable error")
	}
}
