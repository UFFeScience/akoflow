package workflow_engine_api_handler

import (
	"encoding/json"
	"fmt"
	"github.com/UFFeScience/akoflow/internal/domain/workflow"
	"io"
	"net/http"
)

type ArtifactRecipeInput struct {
	Dockerfile     string            `json:"dockerfile,omitempty"`
	ContextArchive []byte            `json:"contextArchive,omitempty"`
	RecipePath     string            `json:"recipePath,omitempty"`
	Platform       string            `json:"platform"`
	Arguments      map[string]string `json:"arguments,omitempty"`
}

func multipartArtifactRecipe(w http.ResponseWriter, r *http.Request) (ArtifactRecipeInput, error) {
	var input ArtifactRecipeInput
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		return input, err
	}
	defer r.MultipartForm.RemoveAll()
	if len(r.MultipartForm.File["context"]) > 0 {
		file, _, err := r.FormFile("context")
		if err != nil {
			return input, err
		}
		defer file.Close()
		input.ContextArchive, err = io.ReadAll(io.LimitReader(file, workflow.MaxPortableContextBytes+1))
		if err != nil {
			return input, err
		}
	}
	input.Arguments = map[string]string{}
	if value := r.FormValue("arguments"); value != "" {
		if err := json.Unmarshal([]byte(value), &input.Arguments); err != nil {
			return input, fmt.Errorf("build arguments must be a JSON string map")
		}
	}
	input.Dockerfile = r.FormValue("dockerfile")
	input.Platform = r.FormValue("platform")
	input.RecipePath = r.FormValue("recipePath")
	return input, nil
}
