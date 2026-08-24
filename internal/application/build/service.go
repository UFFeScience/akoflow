// Package build executes immutable build specifications through a configured
// builder command. The API never accepts a host path: ContextDigest is looked
// up by the ContextResolver, which is normally backed by the artifact store.
package build

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type CommandRunner interface {
	Run(context.Context, string, []string, []byte) ([]byte, error)
}
type ContextResolver interface {
	ResolveBuildContext(context.Context, string) (string, error)
}
type Catalog interface {
	SaveBuildRun(context.Context, domain.BuildRun) error
	SaveBuildContext(context.Context, domain.BuildContextArtifact) error
	FindBuildContext(context.Context, string) (*domain.BuildContextArtifact, error)
	PublishBuildOutput(context.Context, string, domain.ArtifactVariant, domain.ArtifactLocation) error
}

type Executor struct {
	Catalog           Catalog
	Contexts          ContextResolver
	Runner            CommandRunner
	Buildctl          string
	Apptainer         string
	ArtifactStoreRoot string
}

func (s Executor) Execute(ctx context.Context, spec domain.ArtifactBuild, run domain.BuildRun) (domain.BuildRun, error) {
	if s.Runner == nil || (spec.SourceType != "docker-image" && s.Contexts == nil) {
		return s.fail(ctx, run, "builder is not configured")
	}
	contextPath := ""
	if spec.SourceType != "docker-image" {
		var err error
		contextPath, err = s.Contexts.ResolveBuildContext(ctx, spec.ContextDigest)
		if err != nil {
			return s.fail(ctx, run, fmt.Sprintf("resolve immutable build context: %v", err))
		}
		if strings.TrimSpace(contextPath) == "" {
			return s.fail(ctx, run, "build context is unavailable")
		}
	}
	run.Status = "running"
	_ = s.save(ctx, run)
	outputPath, logs, err := s.buildOutput(ctx, spec, run, contextPath)
	run.Logs += logs
	if err != nil {
		return s.fail(ctx, run, err.Error())
	}
	return s.publish(ctx, spec, run, outputPath)
}

func (s Executor) buildOutput(ctx context.Context, spec domain.ArtifactBuild, run domain.BuildRun, contextPath string) (string, string, error) {
	if s.ArtifactStoreRoot == "" {
		return "", "", fmt.Errorf("artifact store is not configured")
	}
	if err := os.MkdirAll(filepath.Join(s.ArtifactStoreRoot, "outputs"), 0700); err != nil {
		return "", "", err
	}
	if spec.SourceType == "docker-image" {
		if spec.TargetFormat != "sif" {
			return "", "", fmt.Errorf("Docker image artifacts must target SIF")
		}
		image := strings.TrimPrefix(strings.TrimSpace(spec.RecipePath), "docker://")
		if image == "" || strings.ContainsAny(image, " \t\n\r") {
			return "", "", fmt.Errorf("a valid Docker image reference is required")
		}
		apptainer := s.Apptainer
		if apptainer == "" {
			return "", "", fmt.Errorf("Apptainer builder is not configured")
		}
		sifPath := filepath.Join(s.ArtifactStoreRoot, "outputs", run.ID+".sif")
		// Apptainer otherwise selects the image matching the architecture of the
		// control-plane host.  That is incorrect when the build is intended for a
		// different execution environment (for example an arm64 laptop building a
		// SIF for an amd64 Slurm cluster).
		architecture := strings.TrimSpace(spec.TargetArchitecture)
		if architecture == "" {
			architecture = "amd64"
		}
		output, err := s.Runner.Run(ctx, apptainer, []string{"build", "--arch", architecture, sifPath, "docker://" + image}, nil)
		if err != nil {
			return "", string(output), err
		}
		return sifPath, string(output), nil
	}
	buildctl := s.Buildctl
	if buildctl == "" {
		buildctl = "buildctl"
	}
	outputPath := filepath.Join(s.ArtifactStoreRoot, "outputs", run.ID+".oci.tar")
	args := []string{
		"build", "--frontend=dockerfile.v0",
		"--local", "context=" + contextPath,
		"--local", "dockerfile=" + contextPath,
		"--opt", "filename=" + spec.RecipePath,
		"--opt", "platform=" + spec.TargetOS + "/" + spec.TargetArchitecture,
		"--output", "type=oci,dest=" + outputPath,
	}
	output, err := s.Runner.Run(ctx, buildctl, args, nil)
	logs := string(output)
	if err != nil {
		return "", logs, err
	}
	if spec.TargetFormat == "sif" {
		apptainer := s.Apptainer
		if apptainer == "" {
			return "", logs, fmt.Errorf("Apptainer builder is not configured")
		}
		sifPath := filepath.Join(s.ArtifactStoreRoot, "outputs", run.ID+".sif")
		output, err = s.Runner.Run(ctx, apptainer, []string{"build", sifPath, "oci-archive:" + outputPath}, nil)
		logs += "\n" + string(output)
		if err != nil {
			return "", logs, err
		}
		outputPath = sifPath
	}
	return outputPath, logs, nil
}

func (s Executor) publish(ctx context.Context, spec domain.ArtifactBuild, run domain.BuildRun, outputPath string) (domain.BuildRun, error) {
	bytes, err := os.ReadFile(outputPath)
	if err != nil {
		return s.fail(ctx, run, fmt.Sprintf("read build output: %v", err))
	}
	sum := sha256.Sum256(bytes)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	format := spec.TargetFormat
	if format == "" {
		format = "oci"
	}
	variant := domain.ArtifactVariant{
		ID: "variant-" + run.ID, Digest: digest, Format: format,
		Architecture: spec.TargetArchitecture, SizeBytes: int64(len(bytes)),
	}
	location := domain.ArtifactLocation{
		ID: "location-" + run.ID, VariantID: variant.ID, Digest: digest,
		URI:   "artifact://outputs/" + filepath.Base(outputPath),
		Scope: domain.CatalogScope("system"), EndpointID: "artifact-store", Available: true,
	}
	run.Status = "publishing"
	_ = s.save(ctx, run)
	if s.Catalog == nil {
		return s.fail(ctx, run, "build catalog is not configured")
	}
	if err := s.Catalog.PublishBuildOutput(ctx, run.ID, variant, location); err != nil {
		return s.fail(ctx, run, err.Error())
	}
	run.Status = "completed"
	run.OutputVariantID = variant.ID
	run.OutputDigest = digest
	if err := s.save(ctx, run); err != nil {
		return run, err
	}
	return run, nil
}
func (s Executor) fail(ctx context.Context, run domain.BuildRun, message string) (domain.BuildRun, error) {
	run.Status, run.Error = "failed", message
	_ = s.save(ctx, run)
	return run, fmt.Errorf("build %s: %s", run.ID, message)
}
func (s Executor) save(ctx context.Context, run domain.BuildRun) error {
	if s.Catalog == nil {
		return nil
	}
	return s.Catalog.SaveBuildRun(ctx, run)
}
