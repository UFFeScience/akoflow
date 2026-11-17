package workflow

import (
	"strings"
	"testing"
)

func TestInspectionSnapshotValidation(t *testing.T) {
	valid := ArtifactInspection{SchemaVersion: 1, Data: map[string]any{"format": "sif", "Id": "sha256:" + strings.Repeat("a", 64)}}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, value := range []ArtifactInspection{
		{SchemaVersion: 2, Data: valid.Data},
		{SchemaVersion: 1, Data: map[string]any{"format": "exe"}},
		{SchemaVersion: 1, Data: map[string]any{"format": "oci", "Id": "invalid"}},
		{SchemaVersion: 1, Data: map[string]any{"format": "oci", "history": strings.Repeat("x", MaxArtifactInspectionBytes)}},
	} {
		if err := value.Validate(); err == nil {
			t.Fatal("invalid metadata snapshot accepted")
		}
	}
}
