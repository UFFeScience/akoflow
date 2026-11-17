package build

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	domainworkflow "github.com/UFFeScience/akoflow/internal/domain/workflow"
)

type sifInspectionRunner struct{ calls []string }

func (r *sifInspectionRunner) Run(_ context.Context, name string, args []string, _ []byte) ([]byte, error) {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	if args[0] == "sif" {
		return []byte("ID | TYPE\n1 | Def.FILE\n2 | FS (Squashfs/*System/amd64)"), nil
	}
	return json.Marshal(map[string]any{"data": map[string]any{"attributes": map[string]any{"runscript": "#!/bin/sh\necho ready", "deffile": "Bootstrap: docker\nFrom: alpine"}}})
}
func TestSIFInspectionReadsMetadataAndPartitionsWithoutExecutingPayload(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "image.sif")
	data := []byte("SIF fixture")
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	runner := &sifInspectionRunner{}
	manager := Manager{Executor: Executor{Runner: runner, Apptainer: "apptainer"}}
	source := domain.ExecutableSource{Type: domainworkflow.ExecutableSourceLocalFile, Path: file, Format: domain.ExecutableFormatSIF, ExpectedDigest: contentDigest(data)}
	result, err := manager.InspectSource(context.Background(), source)
	if err != nil || result["available"] != true || result["sifMetadata"] == nil || result["sifPartitions"] == nil || result["Id"] != source.ExpectedDigest {
		t.Fatalf("SIF inspection: %+v %v", result, err)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, " exec ") || strings.Contains(call, " build ") || strings.Contains(call, " pull ") {
			t.Fatalf("inspection executed an image: %s", call)
		}
	}
	source.ExpectedDigest = contentDigest([]byte("different"))
	if _, err := manager.InspectSource(context.Background(), source); err == nil {
		t.Fatal("SIF digest mismatch accepted")
	}
}
func TestRemoteSIFRetainsLocationWithoutInspectingGatewayFilesystem(t *testing.T) {
	runner := &sifInspectionRunner{}
	source := domain.ExecutableSource{Type: domain.ExecutableSourceRemoteFile, Path: "/apps/tool.sif", EnvironmentRef: "cluster", Format: domain.ExecutableFormatSIF}
	result, err := (Manager{Executor: Executor{Runner: runner}}).InspectSource(context.Background(), source)
	if err != nil || result["format"] != "sif" || result["available"] != false || result["environmentRef"] != "cluster" || len(runner.calls) != 0 {
		t.Fatalf("remote SIF inspection: %+v %v %+v", result, err, runner.calls)
	}
}
func TestLegacyOCIReferenceCanBeInspectedWithoutPortableOrigin(t *testing.T) {
	runner := &inspectionRunner{}
	result, err := (Manager{Executor: Executor{Runner: runner}}).InspectSource(context.Background(), domain.ExecutableSource{Type: domain.ExecutableSourceOCI, Reference: "example/image:1"})
	if err != nil || result["available"] != true || result["format"] != "oci" || result["platform"] != "linux/amd64" {
		t.Fatalf("OCI source inspection: %+v %v", result, err)
	}
}
