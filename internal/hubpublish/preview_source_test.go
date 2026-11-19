package hubpublish

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestImportedExecutionRequiresExactSourceAndVerifiedDefinition(t *testing.T) {
	const pinned = "name: pinned\n"
	const executed = "name: local\n"
	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/publications/source/versions/1/yaml" {
			t.Errorf("unexpected source path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(pinned))
	}))
	defer hub.Close()

	manager := &Manager{state: savedState{HubURL: hub.URL}}
	source := WorkflowSource{
		PublicationID: "source", Version: 1, HubOrigin: hub.URL,
		Hash: digest([]byte(pinned)), LocalHash: digest([]byte(executed)),
	}
	capture := capturedExperiment{yaml: executed}
	for _, test := range []struct {
		name, destination, code string
		version                 int
	}{
		{"wrong publication", "other", "workflow_source_mismatch", 1},
		{"wrong version", "source", "workflow_source_mismatch", 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			var failure *apiError
			err := manager.verifyImportedDestinationLocked(context.Background(), &capture, source, test.destination, test.version)
			if !errors.As(err, &failure) || failure.Code != test.code {
				t.Fatalf("error = %v", err)
			}
		})
	}
	var failure *apiError
	tampered := source
	tampered.Hash = digest([]byte("another Hub definition"))
	if err := manager.verifyImportedDestinationLocked(context.Background(), &capture, tampered, "source", 1); !errors.As(err, &failure) ||
		failure.Code != "workflow_source_changed" {
		t.Fatalf("tampered checksum: %v", err)
	}
	if err := manager.verifyImportedDestinationLocked(context.Background(), &capture, source, "source", 1); err != nil || capture.yaml != pinned {
		t.Fatalf("verified definition = %q, %v", capture.yaml, err)
	}
}

func TestWorkflowChecksumIgnoresDerivedOCIImageButRejectsChangedCommand(t *testing.T) {
	const ref = "example.test/tool@sha256:1234"
	command := domain.ActivityCommand{
		Entrypoint: "run",
		Executable: &domain.ExecutableReference{Source: domain.ExecutableSource{
			Type: domain.ExecutableSourceOCI, Reference: ref,
		}},
	}
	definition := domain.WorkflowDefinition{Name: "study", Version: domain.WorkflowVersion{
		Activities: []domain.Activity{{Name: "step", Command: command}},
	}}
	original, err := workflowChecksum(definition, true)
	if err != nil {
		t.Fatal(err)
	}
	definition.Version.Activities[0].Command.Image = ref
	executed, err := workflowChecksum(definition, true)
	if err != nil || executed != original {
		t.Fatalf("derived image changed checksum: %v", err)
	}
	definition.Version.Activities[0].Command.Entrypoint = "changed"
	modified, err := workflowChecksum(definition, true)
	if err != nil || modified == original {
		t.Fatalf("modified command preserved checksum: %v", err)
	}
}
