package workflow_engine_api_handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/UFFeScience/akoflow/internal/domain"
	domainworkflow "github.com/UFFeScience/akoflow/internal/domain/workflow"
	"github.com/UFFeScience/akoflow/internal/hubpublish"
)

func (h *Handler) inspectArtifactSource(ctx context.Context, source domain.ExecutableSource) (map[string]any, error) {
	if source.Inspection != nil && source.Format == "" {
		format, _ := source.Inspection.Data["format"].(string)
		source.Format = domain.ExecutableFormat(format)
	}
	live, err := h.inspectLiveArtifactSource(ctx, source)
	if source.Inspection == nil {
		return live, err
	}
	if validation := source.Inspection.Validate(); validation != nil {
		return nil, validation
	}
	if source.Format != "" && source.Inspection.Data["format"] != string(source.Format) {
		return live, err
	}
	if err == nil && live["available"] == true {
		if live["platforms"] == nil && source.Inspection.Data["platforms"] != nil {
			live["platforms"] = source.Inspection.Data["platforms"]
		}
		return live, nil
	}
	data, marshalErr := json.Marshal(source.Inspection.Data)
	if marshalErr != nil {
		return nil, marshalErr
	}
	var snapshot map[string]any
	if json.Unmarshal(data, &snapshot) != nil {
		return nil, fmt.Errorf("invalid artifact inspection snapshot")
	}
	if live["platforms"] != nil {
		snapshot["platforms"] = live["platforms"]
	}
	snapshot["available"] = false
	snapshot["metadataProvenance"] = "published-snapshot"
	return snapshot, nil
}

func (h *Handler) captureArtifactInspection(ctx context.Context, source *domain.ExecutableSource) error {
	if source.Inspection != nil {
		if err := source.Inspection.Validate(); err != nil {
			return err
		}
	}
	if _, ok := h.build.(executableInspector); !ok {
		if source.Inspection != nil {
			return nil
		}
		source.Inspection = &domainworkflow.ArtifactInspection{
			SchemaVersion: 1,
			Data:          map[string]any{"format": "oci", "reference": source.Reference, "metadataUnavailable": true},
		}
		return nil
	}
	inspection, err := h.inspectArtifactSource(ctx, *source)
	if err != nil {
		if source.Inspection != nil {
			return source.Inspection.Validate()
		}
		return fmt.Errorf("capture image metadata: %w", err)
	}
	if source.Inspection != nil && inspection["metadataProvenance"] == "published-snapshot" {
		if platforms := inspection["platforms"]; platforms != nil {
			source.Inspection.Data["platforms"] = platforms
		}
		return source.Inspection.Validate()
	}
	if format, ok := inspection["format"].(string); !ok || format == "" {
		format = string(source.Format)
		if format == "" {
			format = "oci"
		}
		inspection["format"] = format
	}
	for _, key := range []string{"available", "metadataProvenance", "metadataCapturedAt", "variant", "location"} {
		delete(inspection, key)
	}
	if rows, ok := inspection["history"].([]map[string]any); ok {
		for _, row := range rows {
			delete(row, "CreatedSince")
		}
	}
	inspection, err = hubpublish.SanitizeArtifactInspection(inspection)
	if err != nil {
		return err
	}
	snapshot := &domainworkflow.ArtifactInspection{SchemaVersion: 1, Data: inspection}
	if err := snapshot.Validate(); err != nil {
		return err
	}
	source.Inspection = snapshot
	return nil
}
