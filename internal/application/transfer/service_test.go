package transfer

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	infra "github.com/UFFeScience/akoflow/internal/infrastructure/transfer"
)

func TestMaterializerCommitsVerifiedBlob(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	content := []byte("portable artifact")
	if err := os.WriteFile(filepath.Join(source, "input"), content, 0600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(content)
	digest := fmt.Sprintf("sha256:%x", hash[:])
	plan := domain.DataTransferPlan{ID: "transfer", Source: domain.TransferLocation{URI: "file://" + source, Path: "input"}, Destination: domain.TransferLocation{URI: "file://" + destination}, Blobs: []domain.BlobDescriptor{{Digest: digest}}}
	materializer := Materializer{Connectors: []ports.TransferConnector{infra.LocalFilesystem{}}}
	result, run, err := materializer.Materialize(context.Background(), plan, domain.ArtifactMaterialization{Digest: digest})
	if err != nil || !result.Committed() || run.Status != domain.TransferCompleted {
		t.Fatalf("result=%+v run=%+v err=%v", result, run, err)
	}
	if run.TransferredBytes != int64(len(content)) || run.StartedAt == 0 || run.FinishedAt < run.StartedAt {
		t.Fatalf("transfer metrics=%+v", run)
	}
	if _, err = os.Stat(filepath.Join(destination, digest)); err != nil {
		t.Fatal(err)
	}
}

func TestMaterializerResumesPartialAndSkipsVerifiedDestination(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	content := []byte("portable artifact with a resumable tail")
	if err := os.WriteFile(filepath.Join(source, "input"), content, 0600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(content)
	digest := fmt.Sprintf("sha256:%x", hash[:])
	if err := os.WriteFile(filepath.Join(destination, digest+".partial"), content[:12], 0600); err != nil {
		t.Fatal(err)
	}
	plan := domain.DataTransferPlan{ID: "resume", Source: domain.TransferLocation{URI: "file://" + source, Path: "input"}, Destination: domain.TransferLocation{URI: "file://" + destination}, Blobs: []domain.BlobDescriptor{{Digest: digest}}}
	m := Materializer{Connectors: []ports.TransferConnector{infra.LocalFilesystem{}}}
	result, run, err := m.Materialize(context.Background(), plan, domain.ArtifactMaterialization{Digest: digest})
	if err != nil || !result.Committed() || len(run.VerifiedBlobs) != 1 {
		t.Fatalf("result=%+v run=%+v err=%v", result, run, err)
	}
	got, err := os.ReadFile(filepath.Join(destination, digest))
	if err != nil || string(got) != string(content) {
		t.Fatalf("got %q, err %v", got, err)
	}
	// A second call verifies the already committed object and performs no copy.
	_, second, err := m.Materialize(context.Background(), plan, domain.ArtifactMaterialization{Digest: digest})
	if err != nil || second.Status != domain.TransferCompleted || len(second.VerifiedBlobs) != 1 {
		t.Fatalf("run=%+v err=%v", second, err)
	}
}

func TestMaterializerRejectsGatewayExecutionOfDestinationPull(t *testing.T) {
	plan := domain.DataTransferPlan{ID: "pull", Strategy: domain.TransferDestinationPull,
		Source: domain.TransferLocation{URI: "file:///source"}, Destination: domain.TransferLocation{URI: "file:///destination"}}
	m := Materializer{Connectors: []ports.TransferConnector{infra.LocalFilesystem{}}}
	_, run, err := m.Materialize(context.Background(), plan, domain.ArtifactMaterialization{Digest: "sha256:abc"})
	if err == nil || run.Status != domain.TransferFailed {
		t.Fatalf("run=%+v err=%v", run, err)
	}
}
