package build

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func (m Manager) InspectOrigin(ctx context.Context, origin domain.ArtifactOrigin) (map[string]any, error) {
	if err := origin.Validate(); err != nil {
		return nil, err
	}
	result := map[string]any{"platform": origin.Platform, "reference": origin.Image, "available": false}
	image := origin.Image
	if origin.Build != nil {
		if err := inspectPortableContext(*origin.Build); err != nil {
			return nil, err
		}
		payload, _ := json.Marshal(origin)
		image = "akoflow-reproduced:" + strings.TrimPrefix(contentDigest(payload), "sha256:")
		result["reference"] = image
		result["build"] = map[string]any{"recipePath": origin.Build.RecipePath, "recipeDigest": origin.Build.RecipeDigest, "contextDigest": origin.Build.ContextDigest, "contextSize": len(origin.Build.ContextArchive), "arguments": origin.Build.Arguments}
		gz, err := gzip.NewReader(bytes.NewReader(origin.Build.ContextArchive))
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		reader := tar.NewReader(gz)
		for {
			header, err := reader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			if path.Clean(header.Name) == origin.Build.RecipePath {
				data, err := io.ReadAll(reader)
				if err != nil {
					return nil, err
				}
				result["dockerfile"] = string(data)
				break
			}
		}
	}
	if m.Executor.Runner == nil {
		return result, nil
	}
	reproducer := &Reproducer{Runner: m.Executor.Runner}
	if _, err := reproducer.inspect(ctx, image, origin); err != nil {
		return result, nil
	}
	output, err := m.Executor.Runner.Run(ctx, "docker", []string{"image", "inspect", image}, nil)
	if err != nil {
		return nil, fmt.Errorf("read image metadata: %w", err)
	}
	var images []map[string]any
	if err := json.Unmarshal(output, &images); err != nil || len(images) != 1 {
		return nil, fmt.Errorf("invalid image metadata")
	}
	for _, key := range []string{"Id", "RepoTags", "RepoDigests", "Created", "Size", "Os", "Architecture", "RootFS", "Config"} {
		result[key] = images[0][key]
	}
	result["available"] = true
	history, err := m.Executor.Runner.Run(ctx, "docker", []string{"image", "history", "--no-trunc", "--format", "{{json .}}", image}, nil)
	if err != nil {
		result["historyUnavailable"] = true
		return result, nil
	}
	rows := []map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(string(history)), "\n") {
		if line == "" {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("invalid image history")
		}
		rows = append(rows, row)
	}
	result["history"] = rows
	return result, nil
}
