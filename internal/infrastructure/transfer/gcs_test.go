package transfer

import (
	"context"
	"strings"
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
	if _, err := gcs.Open(context.Background(), endpoint, "image.sif", 0); err == nil || !strings.Contains(err.Error(), "GCS transfer is unavailable") {
		t.Fatalf("open error=%v", err)
	}
	if err := gcs.Put(context.Background(), endpoint, "image.sif", strings.NewReader("data"), 0); err == nil {
		t.Fatal("unconfigured GCS put must fail")
	}
	if err := gcs.Commit(context.Background(), endpoint, "partial", "final"); err == nil {
		t.Fatal("unconfigured GCS commit must fail")
	}
	if gcs.CanHandle(domain.TransferEndpoint{URI: "https://storage.googleapis.com"}) {
		t.Fatal("HTTPS endpoint must not be claimed as native GCS")
	}
}
