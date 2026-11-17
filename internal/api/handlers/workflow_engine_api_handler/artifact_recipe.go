package workflow_engine_api_handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/domain/workflow"
)

func (h *Handler) RegisterArtifactRecipe(w http.ResponseWriter, r *http.Request) {
	builder, ok := h.build.(interface {
		RecipeOrigin(context.Context, string, []byte, string, string, map[string]string) (*domain.ArtifactOrigin, error)
	})
	if !ok || h.data == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("portable build storage is unavailable"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, workflow.MaxPortableContextBytes+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer r.MultipartForm.RemoveAll()
	var archive []byte
	if len(r.MultipartForm.File["context"]) > 0 {
		file, _, err := r.FormFile("context")
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, err)
			return
		}
		defer file.Close()
		archive, err = io.ReadAll(io.LimitReader(file, workflow.MaxPortableContextBytes+1))
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, err)
			return
		}
	}
	arguments := map[string]string{}
	if value := r.FormValue("arguments"); value != "" {
		if err := json.Unmarshal([]byte(value), &arguments); err != nil {
			writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("build arguments must be a JSON string map"))
			return
		}
	}
	origin, err := builder.RecipeOrigin(r.Context(), r.FormValue("dockerfile"), archive, r.FormValue("recipePath"), r.FormValue("platform"), arguments)
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
	result, err := h.inspectArtifactSource(r.Context(), source)
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
