package build

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/domain/workflow"
	"github.com/UFFeScience/akoflow/internal/operationerror"
	"github.com/UFFeScience/akoflow/internal/provider"
)

func contentDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func inspectPortableContext(b domain.PortableArtifactBuild) error {
	if contentDigest(b.ContextArchive) != b.ContextDigest {
		return fmt.Errorf("portable context checksum mismatch")
	}
	gz, err := gzip.NewReader(bytes.NewReader(b.ContextArchive))
	if err != nil {
		return fmt.Errorf("portable context must be gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(io.LimitReader(gz, 32<<20))
	seen := map[string]bool{}
	found := false
	var expanded int64
	for count := 0; ; count++ {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read portable context: %w", err)
		}
		if count > 2048 || h.Size < 0 || expanded+h.Size > 16<<20 {
			return fmt.Errorf("portable context exceeds expanded size or file limit")
		}
		expanded += h.Size
		name := filepath.ToSlash(filepath.Clean(h.Name))
		if !safeArchivePath(h.Name) || strings.Contains(h.Name, "\\") || seen[name] || (h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeDir) {
			return fmt.Errorf("unsafe portable context entry %q", h.Name)
		}
		seen[name] = true
		for _, part := range strings.Split(name, "/") {
			lower := strings.ToLower(part)
			if sensitiveContextName(lower) {
				return fmt.Errorf("portable context contains sensitive file %q; upload a sanitized context", name)
			}
		}
		if h.Typeflag == tar.TypeDir {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return err
		}
		if bytes.Contains(data, []byte("PRIVATE KEY-----")) {
			return fmt.Errorf("portable context contains a private key")
		}
		if name == b.RecipePath {
			if contentDigest(data) != b.RecipeDigest {
				return fmt.Errorf("portable recipe checksum mismatch")
			}
			if err := validatePortableRecipe(string(data)); err != nil {
				return err
			}
			found = true
		}
	}
	if !found {
		return fmt.Errorf("portable recipe is missing from context")
	}
	return nil
}

func validatePortableRecipe(recipe string) error {
	stages := map[string]bool{"scratch": true}
	for _, line := range strings.Split(strings.ReplaceAll(recipe, "\\\n", " "), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.ToUpper(fields[0]) != "FROM" {
			continue
		}
		index := 1
		for index < len(fields) && strings.HasPrefix(fields[index], "--") {
			index++
		}
		if index >= len(fields) {
			return fmt.Errorf("portable Dockerfile has an invalid FROM instruction")
		}
		base := fields[index]
		if !stages[strings.ToLower(base)] {
			origin := domain.ArtifactOrigin{Image: base, Platform: "linux/amd64"}
			if err := origin.Validate(); err != nil {
				return fmt.Errorf("pin Dockerfile base %q to reference@sha256 before publishing", base)
			}
		}
		if index+2 < len(fields) && strings.EqualFold(fields[index+1], "AS") {
			stages[strings.ToLower(fields[index+2])] = true
		}
	}
	return nil
}

func (m Manager) PortableOrigin(_ context.Context, spec domain.ArtifactBuild) (*domain.ArtifactOrigin, error) {
	origin := &domain.ArtifactOrigin{Platform: spec.TargetOS + "/" + normalizedArchitecture(spec.TargetArchitecture)}
	if spec.SourceType == "docker-image" {
		origin.Image = strings.TrimPrefix(spec.RecipePath, "docker://")
		return origin, nil
	}
	if spec.SourceType != "dockerfile" {
		return nil, fmt.Errorf("build source %q cannot be exported as a portable Dockerfile recipe", spec.SourceType)
	}
	if !strings.HasPrefix(spec.ContextDigest, "sha256:") || len(spec.ContextDigest) != 71 {
		return nil, fmt.Errorf("invalid context digest")
	}
	f, err := os.Open(filepath.Join(m.Root, "contexts", strings.TrimPrefix(spec.ContextDigest, "sha256:")+".tar.gz"))
	if err != nil {
		return nil, fmt.Errorf("portable context is unavailable; upload its source again: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, workflow.MaxPortableContextBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > workflow.MaxPortableContextBytes {
		return nil, fmt.Errorf("portable context exceeds 4 MiB; reduce the context before publishing")
	}
	b := &domain.PortableArtifactBuild{ContextArchive: data, ContextDigest: spec.ContextDigest, RecipePath: spec.RecipePath, RecipeDigest: spec.RecipeDigest}
	if spec.BuildArguments != "" {
		if err := json.Unmarshal([]byte(spec.BuildArguments), &b.Arguments); err != nil {
			return nil, fmt.Errorf("invalid build arguments: %w", err)
		}
	}
	origin.Build = b
	if err := origin.Validate(); err != nil {
		return nil, err
	}
	if err := inspectPortableContext(*b); err != nil {
		return nil, err
	}
	return origin, nil
}

type Reproducer struct {
	Manager  Manager
	Runner   CommandRunner
	Catalog  ports.DataCatalog
	Buildctl string
	mu       sync.Mutex
}

type inspectedImage struct {
	ID           string
	OS           string `json:"Os"`
	Architecture string
	RepoDigests  []string
	Size         int64
}

func (r *Reproducer) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if r.Runner == nil {
		return nil, fmt.Errorf("artifact reproduction runner is unavailable")
	}
	out, err := r.Runner.Run(ctx, name, args, nil)
	if err != nil {
		code, action, retryable := "artifact_preparation_failed", "Review artifact build logs and correct the build configuration.", false
		if name == "docker" && len(args) > 0 && args[0] == "pull" {
			code, action, retryable = "registry_pull_failed", "Check registry access and image availability, then retry preparation.", true
		}
		return nil, operationerror.Wrap(fmt.Errorf("%s failed while preparing artifact: %w: %s", name, err, operationerror.Sanitize(string(out))), code, "artifact_preparation", "", retryable, action)
	}
	return out, nil
}

func (r *Reproducer) inspect(ctx context.Context, image string, origin domain.ArtifactOrigin) (inspectedImage, error) {
	out, err := r.run(ctx, "docker", "image", "inspect", image)
	if err != nil {
		return inspectedImage{}, err
	}
	var images []inspectedImage
	if err := json.Unmarshal(out, &images); err != nil || len(images) != 1 {
		return inspectedImage{}, fmt.Errorf("Docker did not return exactly one verified image")
	}
	imageInfo := images[0]
	if origin.ExpectedDigest != "" && imageInfo.ID != origin.ExpectedDigest {
		return inspectedImage{}, fmt.Errorf("prepared image checksum does not match executable expectedDigest")
	}
	if len(imageInfo.ID) != 71 || !strings.HasPrefix(imageInfo.ID, "sha256:") || imageInfo.OS+"/"+imageInfo.Architecture != strings.Join(strings.Split(origin.Platform, "/")[:2], "/") {
		return inspectedImage{}, fmt.Errorf("image platform differs from origin %q", origin.Platform)
	}
	if origin.Image != "" {
		digest := strings.Split(origin.Image, "@")[1]
		matched := false
		for _, ref := range imageInfo.RepoDigests {
			if strings.HasSuffix(ref, "@"+digest) {
				matched = true
			}
		}
		if !matched {
			return inspectedImage{}, fmt.Errorf("downloaded image does not match the origin registry digest")
		}
	}
	return imageInfo, nil
}

func (r *Reproducer) Reproduce(ctx context.Context, origin domain.ArtifactOrigin) (*domain.ArtifactMaterialization, error) {
	if err := origin.Validate(); err != nil {
		return nil, err
	}
	if origin.Build != nil {
		if err := inspectPortableContext(*origin.Build); err != nil {
			return nil, err
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(origin)
	key := strings.TrimPrefix(contentDigest(payload), "sha256:")
	if catalog, ok := r.Catalog.(portableAvailability); ok {
		if err := catalog.SetPortableOriginAvailability(ctx, key, "", false); err != nil {
			return nil, err
		}
	}
	image := origin.Image
	if origin.Build != nil {
		var err error
		image, err = r.build(ctx, origin, key)
		if err != nil {
			return nil, err
		}
	} else {
		if _, err := r.inspect(ctx, image, origin); err != nil {
			if _, err := r.run(ctx, "docker", "pull", "--platform", origin.Platform, image); err != nil {
				return nil, err
			}
		}
	}
	info, err := r.inspect(ctx, image, origin)
	if err != nil {
		return nil, err
	}
	variantID, err := r.publish(ctx, origin, key, info)
	if err != nil {
		return nil, err
	}
	return &domain.ArtifactMaterialization{ID: "reproduced-" + key, VariantID: variantID, Digest: info.ID, VerifiedDigest: info.ID, DestinationPath: info.ID, OCIReference: info.ID, Status: domain.MaterializationCommitted}, nil
}

func (r *Reproducer) build(ctx context.Context, origin domain.ArtifactOrigin, key string) (string, error) {
	b := origin.Build
	manager := r.Manager
	manager.MaxBytes = workflow.MaxPortableContextBytes
	if _, err := manager.Upload(ctx, bytes.NewReader(b.ContextArchive)); err != nil {
		return "", err
	}
	// Extract into an attempt-owned root so a prior interrupted extraction cannot
	// turn a cached directory into a partially prepared build context.
	temp, err := os.MkdirTemp(manager.Root, "reproduce-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temp)
	isolated := Manager{Root: temp, MaxBytes: workflow.MaxPortableContextBytes}
	if _, err := isolated.Upload(ctx, bytes.NewReader(b.ContextArchive)); err != nil {
		return "", err
	}
	dir, err := isolated.ResolveBuildContext(ctx, b.ContextDigest)
	if err != nil {
		return "", err
	}
	image := "akoflow-reproduced:" + key
	if _, err := r.inspect(ctx, image, origin); err == nil {
		return image, nil
	}
	output := filepath.Join(temp, "image.tar")
	args := []string{
		"build", "--frontend=dockerfile.v0",
		"--local", "context=" + dir, "--local", "dockerfile=" + dir,
		"--opt", "filename=" + b.RecipePath, "--opt", "platform=" + origin.Platform,
		"--output", "type=docker,name=" + image + ",dest=" + output,
	}
	keys := make([]string, 0, len(b.Arguments))
	for key := range b.Arguments {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, "--opt", "build-arg:"+key+"="+b.Arguments[key])
	}
	buildctl := r.Buildctl
	if buildctl == "" {
		buildctl = "buildctl"
	}
	if _, err := r.run(ctx, buildctl, args...); err != nil {
		return "", err
	}
	if _, err := r.run(ctx, "docker", "load", "--input", output); err != nil {
		return "", err
	}
	return image, nil
}

func (r *Reproducer) publish(ctx context.Context, origin domain.ArtifactOrigin, key string, image inspectedImage) (string, error) {
	if r.Catalog == nil {
		return "variant-" + key, nil
	}
	spec, err := r.ensureOriginSpec(ctx, origin, key, image)
	if err != nil {
		return "", err
	}
	variant, location, err := r.Catalog.FindBuildOutput(ctx, spec.ID)
	if err != nil {
		return "", err
	}
	if variant != nil && location != nil && variant.Digest == image.ID {
		if catalog, ok := r.Catalog.(portableAvailability); ok {
			if err := catalog.SetPortableOriginAvailability(ctx, key, variant.ID, true); err != nil {
				return "", err
			}
		}
		return variant.ID, nil
	}
	run := domain.BuildRun{ID: provider.NewID("origin-run"), ArtifactBuildID: spec.ID, Status: "publishing"}
	if err := r.Catalog.SaveBuildRun(ctx, run); err != nil {
		return "", err
	}
	variant = &domain.ArtifactVariant{ID: "variant-" + run.ID, Digest: image.ID, Format: "oci", Architecture: image.Architecture, SizeBytes: image.Size}
	location = &domain.ArtifactLocation{ID: "location-" + run.ID, VariantID: variant.ID, Digest: image.ID, URI: "docker-image://" + image.ID, EndpointID: "local-docker", Scope: domain.CatalogScope("system"), Available: true}
	if err := r.Catalog.PublishBuildOutput(ctx, run.ID, *variant, *location); err != nil {
		reconcile, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		published, stored, lookupErr := r.Catalog.FindBuildOutput(reconcile, spec.ID)
		if lookupErr != nil {
			return "", errors.Join(err, fmt.Errorf("publication outcome requires reconciliation: %w", lookupErr))
		}
		if published != nil && stored != nil && published.Digest == image.ID {
			return published.ID, nil
		}
		run.Status, run.Error = "failed", err.Error()
		if saveErr := r.Catalog.SaveBuildRun(reconcile, run); saveErr != nil {
			return "", errors.Join(err, fmt.Errorf("persist publication failure: %w", saveErr))
		}
		return "", err
	}
	return variant.ID, nil
}

func sensitiveContextName(name string) bool {
	for _, value := range []string{".git", ".ssh", ".aws", ".npmrc", ".netrc", "credentials", ".env"} {
		if name == value {
			return true
		}
	}
	if strings.HasPrefix(name, ".env.") {
		return true
	}
	for _, suffix := range []string{".pem", ".key", ".p12", ".pfx"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

type portableAvailability interface {
	SetPortableOriginAvailability(context.Context, string, string, bool) error
}

func (r *Reproducer) ensureOriginSpec(ctx context.Context, origin domain.ArtifactOrigin, key string, image inspectedImage) (domain.ArtifactBuild, error) {
	cacheKey := key + ":" + image.ID
	outputKey := strings.TrimPrefix(contentDigest([]byte(cacheKey)), "sha256:")
	existing, err := r.Catalog.FindArtifactBuildByCacheKey(ctx, cacheKey)
	if err != nil {
		return domain.ArtifactBuild{}, err
	}
	if existing != nil {
		return *existing, nil
	}
	spec := domain.ArtifactBuild{
		ID: "build-" + outputKey, ArtifactVersionID: "origin-version-" + outputKey,
		SourceType: "docker-image", RecipePath: origin.Image,
		ContextDigest: contentDigest([]byte(origin.Image)), RecipeDigest: contentDigest([]byte(origin.Image)),
		TargetFormat: "oci", TargetOS: image.OS, TargetArchitecture: image.Architecture, CacheKey: cacheKey,
	}
	if origin.Build != nil {
		spec.SourceType = "dockerfile"
		spec.ContextDigest = origin.Build.ContextDigest
		spec.RecipeDigest = origin.Build.RecipeDigest
		spec.RecipePath = origin.Build.RecipePath
		args, _ := json.Marshal(origin.Build.Arguments)
		spec.BuildArguments = string(args)
	}
	version := domain.ArtifactVersion{
		ID: spec.ArtifactVersionID, ArtifactID: "origin-" + key, Version: image.ID, Scope: domain.CatalogScope("system"),
	}
	if err := r.Catalog.RegisterArtifactVersion(ctx, version); err != nil {
		return spec, err
	}
	return spec, r.Catalog.SaveArtifactBuild(ctx, spec)
}
