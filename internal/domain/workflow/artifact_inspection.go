package workflow

import (
	"encoding/json"
	"fmt"
)

const MaxArtifactInspectionBytes = 1 << 20

type ArtifactInspection struct {
	SchemaVersion int            `json:"schemaVersion"`
	Data          map[string]any `json:"data"`
}

func (s ArtifactInspection) Validate() error {
	if s.SchemaVersion != 1 || s.Data == nil {
		return fmt.Errorf("artifact inspection requires schemaVersion 1 and data")
	}
	format, _ := s.Data["format"].(string)
	if format != "oci" && format != "sif" {
		return fmt.Errorf("artifact inspection format must be oci or sif")
	}
	encoded, err := json.Marshal(s.Data)
	if err != nil || len(encoded) > MaxArtifactInspectionBytes {
		return fmt.Errorf("artifact inspection metadata exceeds 1 MiB")
	}
	if digest, ok := s.Data["Id"].(string); ok && digest != "" && !portableDigest.MatchString(digest) {
		return fmt.Errorf("artifact inspection digest must be SHA-256")
	}
	return nil
}
