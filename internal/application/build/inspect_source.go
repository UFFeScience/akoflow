package build

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
	domainworkflow "github.com/UFFeScience/akoflow/internal/domain/workflow"
)

func (m Manager) InspectSource(ctx context.Context, source domain.ExecutableSource) (map[string]any, error) {
	if strings.HasPrefix(source.URI, "artifact://outputs/") && source.Format == domain.ExecutableFormatSIF {
		name := strings.TrimPrefix(source.URI, "artifact://outputs/")
		if m.Root == "" || name == "" || filepath.Base(name) != name {
			return nil, fmt.Errorf("invalid SIF artifact-store location")
		}
		source.Type = domainworkflow.ExecutableSourceLocalFile
		source.Path = filepath.Join(m.Root, "outputs", name)
	}
	if source.Format == domain.ExecutableFormatSIF || strings.HasSuffix(strings.ToLower(source.Path), ".sif") || strings.HasSuffix(strings.ToLower(source.URI), ".sif") {
		return m.inspectSIF(ctx, source)
	}
	result := map[string]any{"format": "oci", "reference": source.Reference, "available": false}
	if source.Type != domain.ExecutableSourceOCI && source.Type != domainworkflow.ExecutableSourceLocalContainerImage {
		result["format"] = source.Format
		result["reference"] = source.Path
		if source.URI != "" {
			result["reference"] = source.URI
		}
		if source.ArtifactRef != nil {
			result["reference"] = source.ArtifactRef.ID
			result["version"] = source.ArtifactRef.Version
		}
		return result, nil
	}
	image := strings.TrimSpace(source.Reference)
	if image == "" || strings.HasPrefix(image, "-") || strings.ContainsAny(image, " \r\n\t") {
		return nil, fmt.Errorf("invalid OCI image reference")
	}
	if m.Executor.Runner == nil {
		return result, nil
	}
	if source.Type == domain.ExecutableSourceOCI {
		m.inspectRegistryPlatforms(ctx, image, result)
	}
	output, err := m.Executor.Runner.Run(ctx, "docker", []string{"image", "inspect", image}, nil)
	if err != nil {
		return result, nil
	}
	var images []map[string]any
	if err := json.Unmarshal(output, &images); err != nil || len(images) != 1 {
		return nil, fmt.Errorf("invalid image metadata")
	}
	if source.ExpectedDigest != "" && images[0]["Id"] != source.ExpectedDigest {
		return nil, fmt.Errorf("image checksum does not match expectedDigest")
	}
	for _, key := range []string{"Id", "RepoTags", "RepoDigests", "Created", "Size", "Os", "Architecture", "RootFS", "Config"} {
		result[key] = images[0][key]
	}
	result["available"] = true
	result["platform"] = fmt.Sprintf("%v/%v", images[0]["Os"], images[0]["Architecture"])
	return m.inspectImageHistory(ctx, image, result)
}

func (m Manager) inspectSIF(ctx context.Context, source domain.ExecutableSource) (map[string]any, error) {
	result := map[string]any{
		"format": "sif", "reference": source.Path, "available": false,
		"environmentRef": source.EnvironmentRef, "resourceRef": source.ResourceRef,
		"expectedDigest": source.ExpectedDigest,
	}
	if source.URI != "" {
		result["reference"] = source.URI
	}
	if source.Type != domainworkflow.ExecutableSourceLocalFile {
		return result, nil
	}
	if !filepath.IsAbs(source.Path) {
		return nil, fmt.Errorf("SIF inspection requires an absolute file path")
	}
	file, err := os.Open(source.Path)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open SIF artifact: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("SIF artifact is not a regular file")
	}
	result["Size"] = info.Size()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return nil, err
	}
	checksum := "sha256:" + hex.EncodeToString(digest.Sum(nil))
	if source.ExpectedDigest != "" && source.ExpectedDigest != checksum {
		return nil, fmt.Errorf("SIF checksum does not match expectedDigest")
	}
	result["Id"] = checksum
	result["available"] = true
	if m.Executor.Runner == nil {
		result["metadataUnavailable"] = true
		return result, nil
	}
	executable := m.Executor.Apptainer
	if executable == "" {
		executable = "apptainer"
	}
	output, err := m.Executor.Runner.Run(ctx, executable, []string{"inspect", "--all", source.Path}, nil)
	if err != nil {
		result["metadataUnavailable"] = true
		return result, nil
	}
	var metadata map[string]any
	if err := json.Unmarshal(output, &metadata); err != nil {
		return nil, fmt.Errorf("invalid SIF inspection metadata")
	}
	result["sifMetadata"] = metadata
	partitions, err := m.Executor.Runner.Run(ctx, executable, []string{"sif", "list", source.Path}, nil)
	if err == nil {
		result["sifPartitions"] = string(partitions)
	}
	return result, nil
}
