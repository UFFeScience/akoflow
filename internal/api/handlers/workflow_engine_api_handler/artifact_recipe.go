package workflow_engine_api_handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	applicationbuild "github.com/UFFeScience/akoflow/internal/application/build"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func (h *Handler) RegisterArtifactRecipe(w http.ResponseWriter, r *http.Request) {
	builder, ok := h.build.(interface {
		RecipeOrigin(context.Context, string, []byte, string, string, map[string]string) (*domain.ArtifactOrigin, error)
	})
	if !ok || h.data == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("portable build storage is unavailable"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 7<<20)
	var input ArtifactRecipeInput
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		var err error
		input, err = multipartArtifactRecipe(w, r)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, err)
			return
		}
	} else if !decode(w, r, &input) {
		return
	}
	arguments := input.Arguments
	origin, err := builder.RecipeOrigin(r.Context(), input.Dockerfile, input.ContextArchive, input.RecipePath, input.Platform, arguments)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}

	payload, _ := json.Marshal(origin)
	sum := sha256.Sum256(payload)
	key := hex.EncodeToString(sum[:])
	cacheKey := "recipe:" + key
	artifactID := "image-" + key[:16]
	existing, err := h.data.FindArtifactBuildByCacheKey(r.Context(), cacheKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusOK, map[string]any{"origin": origin, "build": existing, "artifactId": artifactID})
		return
	}
	version := domain.ArtifactVersion{ID: "recipe-version-" + key, ArtifactID: artifactID, Version: "1", Scope: domain.CatalogScope("system")}
	if err := h.data.RegisterArtifactVersion(r.Context(), version); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	spec := recipeBuildSpecification(key, version.ID, origin, arguments)

	if err := h.data.SaveArtifactBuild(r.Context(), spec); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"origin": origin, "build": spec, "artifact": version, "artifactId": artifactID})
}

func (h *Handler) InspectArtifactImage(w http.ResponseWriter, r *http.Request) {
	var source domain.ExecutableSource
	if !decode(w, r, &source) {
		return
	}
	ctx := r.Context()
	if r.URL.Query().Get("localOnly") == "true" {
		ctx = applicationbuild.LocalInspectionOnly(ctx)
	}
	result, err := h.inspectArtifactSource(ctx, source)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func recipeBuildSpecification(key, versionID string, origin *domain.ArtifactOrigin, arguments map[string]string) domain.ArtifactBuild {
	cacheKey := "recipe:" + key
	args, _ := json.Marshal(arguments)
	return domain.ArtifactBuild{
		ID:                 "recipe-build-" + key,
		ArtifactVersionID:  versionID,
		SourceType:         "dockerfile",
		ContextDigest:      origin.Build.ContextDigest,
		RecipePath:         origin.Build.RecipePath,
		RecipeDigest:       origin.Build.RecipeDigest,
		TargetFormat:       "oci",
		TargetOS:           "linux",
		TargetArchitecture: strings.TrimPrefix(origin.Platform, "linux/"),
		BuildArguments:     string(args),
		CacheKey:           cacheKey,
	}

}
