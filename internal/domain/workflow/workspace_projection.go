package workflow

import (
	"encoding/json"
	"fmt"
	"math"
	"path"
	"strings"
)

// WorkspaceProjection describes expected files, not observed future execution.
// Inputs are staged before execution; Outputs replace paths after execution.
// ContentID can be a calibrated symbolic identity when an output hash is unknown.
type WorkspaceProjection struct {
	Contract     string          `json:"contract"`
	Source       string          `json:"source"`
	Inputs       []ProjectedFile `json:"inputs,omitempty"`
	Outputs      []ProjectedFile `json:"outputs,omitempty"`
	RemovedPaths []string        `json:"removedPaths,omitempty"`
	// Runtime packaging can put additional files into a cloud workspace while
	// an HPC image reads those files outside the exported workspace. Inputs is
	// the conservative search default; a complete assignment selects a variant.
	InputsByResourceType map[string][]ProjectedFile `json:"inputsByResourceType,omitempty"`
}

type ProjectedFile struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
	ContentID string `json:"contentId"`
}

// ProjectWorkspaceDependencies preserves legacy edge semantics unless an
// explicit whole-workspace model is supplied. Partial models fail closed: a
// missing producer manifest must not silently turn into zero transfer bytes.
// This pure transformation is shared by planning and both simulation adapters.
func ProjectWorkspaceDependencies(version WorkflowVersion) (WorkflowVersion, error) {
	return ProjectWorkspaceDependenciesForResourceTypes(version, nil)
}

func ProjectWorkspaceDependenciesForResourceTypes(version WorkflowVersion, resourceTypes map[string]string) (WorkflowVersion, error) {
	enabled := false
	for _, activity := range version.Activities {
		if activity.Metadata["workspaceProjection"] != nil {
			enabled = true
		}
	}
	if !enabled {
		return version, nil
	}
	models := map[string]WorkspaceProjection{}
	for _, activity := range version.Activities {
		raw := activity.Metadata["workspaceProjection"]
		if raw == nil {
			return version, fmt.Errorf("workspace projection missing for activity %q", activity.ID)
		}
		encoded, err := json.Marshal(raw)
		if err != nil {
			return version, err
		}
		var model WorkspaceProjection
		if err = json.Unmarshal(encoded, &model); err != nil {
			return version, fmt.Errorf("workspace projection %q: %w", activity.ID, err)
		}
		if model.Contract != "whole-workspace-v1" || strings.TrimSpace(model.Source) == "" {
			return version, fmt.Errorf("workspace projection %q requires whole-workspace-v1 and provenance", activity.ID)
		}
		if resourceTypes != nil && len(model.InputsByResourceType) > 0 {
			resourceType, exists := resourceTypes[activity.ID]
			if !exists {
				return version, fmt.Errorf("workspace projection resource type missing for %q", activity.ID)
			}
			inputs, exists := model.InputsByResourceType[resourceType]
			if !exists {
				return version, fmt.Errorf("workspace projection has no %q runtime inputs for %q", resourceType, activity.ID)
			}
			model.Inputs = inputs
		}
		if _, exists := models[activity.ID]; exists {
			return version, fmt.Errorf("duplicate activity %q", activity.ID)
		}
		models[activity.ID] = model
	}
	preds := map[string][]string{}
	for _, edge := range version.Dependencies {
		if _, ok := models[edge.ActivityID]; !ok {
			return version, fmt.Errorf("unknown consumer %q", edge.ActivityID)
		}
		if _, ok := models[edge.DependsOnActivityID]; !ok {
			return version, fmt.Errorf("unknown producer %q", edge.DependsOnActivityID)
		}
		preds[edge.ActivityID] = append(preds[edge.ActivityID], edge.DependsOnActivityID)
	}
	manifests := map[string]map[string]ProjectedFile{}
	for len(manifests) < len(models) {
		progress := false
		for _, activity := range version.Activities {
			if _, done := manifests[activity.ID]; done {
				continue
			}
			ready := true
			for _, producer := range preds[activity.ID] {
				if _, done := manifests[producer]; !done {
					ready = false
				}
			}
			if !ready {
				continue
			}
			manifest := map[string]ProjectedFile{}
			merge := func(file ProjectedFile, replace bool) error {
				if file.Path == "" || strings.ContainsRune(file.Path, '\x00') || path.IsAbs(file.Path) || path.Clean(file.Path) != file.Path || file.Path == "." || file.Path == ".." || strings.HasPrefix(file.Path, "../") || strings.Contains(file.Path, "\\") || file.SizeBytes < 0 || file.ContentID == "" {
					return fmt.Errorf("invalid projected file %q in %q", file.Path, activity.ID)
				}
				if previous, exists := manifest[file.Path]; exists && !replace && (previous.ContentID != file.ContentID || previous.SizeBytes != file.SizeBytes) {
					return fmt.Errorf("projected workspace conflict in %q at %q", activity.ID, file.Path)
				}
				manifest[file.Path] = file
				return nil
			}
			for _, producer := range preds[activity.ID] {
				for _, file := range manifests[producer] {
					if err := merge(file, false); err != nil {
						return version, err
					}
				}
			}
			model := models[activity.ID]
			for _, file := range model.Inputs {
				if err := merge(file, false); err != nil {
					return version, err
				}
			}
			for _, removed := range model.RemovedPaths {
				if _, exists := manifest[removed]; !exists {
					return version, fmt.Errorf("removed path %q absent in %q", removed, activity.ID)
				}
				delete(manifest, removed)
			}
			outputPaths := map[string]bool{}
			for _, file := range model.Outputs {
				if outputPaths[file.Path] {
					return version, fmt.Errorf("duplicate projected output %q in %q", file.Path, activity.ID)
				}
				outputPaths[file.Path] = true
				if err := merge(file, true); err != nil {
					return version, err
				}
			}
			manifests[activity.ID] = manifest
			progress = true
		}
		if !progress {
			return version, fmt.Errorf("workspace projection dependency cycle")
		}
	}
	// Each dependency transfers the producer's complete workspace, even when
	// siblings inherit identical paths. Merge deduplication is a separate cost;
	// this must not assume a destination cache or skip a route optimistically.
	version.DataDependencies = make([]ActivityDataDependency, 0, len(version.Dependencies))
	for _, edge := range version.Dependencies {
		var bytes int64
		for _, file := range manifests[edge.DependsOnActivityID] {
			if file.SizeBytes > math.MaxInt64-bytes {
				return version, fmt.Errorf("workspace size overflow for %q", edge.DependsOnActivityID)
			}
			bytes += file.SizeBytes
		}
		version.DataDependencies = append(version.DataDependencies, ActivityDataDependency{ProducerActivityID: edge.DependsOnActivityID, ConsumerActivityID: edge.ActivityID, LogicalName: "whole-workspace-v1", SizeBytes: bytes})
	}
	return version, nil
}
