package build

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"path"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/domain/workflow"
)

func (m Manager) RecipeOrigin(ctx context.Context, recipe string, archive []byte, recipePath, platform string, arguments map[string]string) (*domain.ArtifactOrigin, error) {
	if recipePath == "" {
		recipePath = "Dockerfile"
	}
	if len(archive) == 0 {
		if recipe == "" {
			return nil, fmt.Errorf("a Dockerfile or context archive is required")
		}
		var buffer bytes.Buffer
		gz := gzip.NewWriter(&buffer)
		writer := tar.NewWriter(gz)
		if err := writer.WriteHeader(&tar.Header{Name: recipePath, Mode: 0600, Size: int64(len(recipe))}); err != nil {
			return nil, err
		}
		if _, err := writer.Write([]byte(recipe)); err != nil {
			return nil, err
		}
		if err := writer.Close(); err != nil {
			return nil, err
		}
		if err := gz.Close(); err != nil {
			return nil, err
		}
		archive = buffer.Bytes()
	} else if recipe != "" {
		return nil, fmt.Errorf("provide either Dockerfile text or a context archive")
	}
	if len(archive) > workflow.MaxPortableContextBytes {
		return nil, fmt.Errorf("portable context exceeds 4 MiB")
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	reader := tar.NewReader(io.LimitReader(gz, 32<<20))
	recipeDigest := ""
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if path.Clean(header.Name) == recipePath && header.Typeflag == tar.TypeReg {
			if header.Size > 1<<20 {
				return nil, fmt.Errorf("Dockerfile exceeds 1 MiB")
			}
			data, err := io.ReadAll(reader)
			if err != nil {
				return nil, err
			}
			recipeDigest = contentDigest(data)
		}
	}
	origin := &domain.ArtifactOrigin{Platform: platform, Build: &domain.PortableArtifactBuild{ContextArchive: archive, ContextDigest: contentDigest(archive), RecipePath: recipePath, RecipeDigest: recipeDigest, Arguments: arguments}}
	if err := origin.Validate(); err != nil {
		return nil, err
	}
	if err := inspectPortableContext(*origin.Build); err != nil {
		return nil, err
	}
	m.MaxBytes = workflow.MaxPortableContextBytes
	if _, err := m.Upload(ctx, bytes.NewReader(archive)); err != nil {
		return nil, err
	}
	return origin, nil
}
