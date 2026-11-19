package build

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type publicationCatalog struct {
	ports.DataCatalog
	runs          []domain.BuildRun
	commit        bool
	published     bool
	lookupFailure bool
	lookupCalls   int
}

func (*publicationCatalog) FindArtifactBuildByCacheKey(context.Context, string) (*domain.ArtifactBuild, error) {
	return &domain.ArtifactBuild{ID: "build"}, nil
}
func (c *publicationCatalog) FindBuildOutput(context.Context, string) (*domain.ArtifactVariant, *domain.ArtifactLocation, error) {
	c.lookupCalls++
	if c.lookupCalls > 1 && c.lookupFailure {
		return nil, nil, errors.New("database offline")
	}
	if c.published {
		return &domain.ArtifactVariant{ID: "verified", Digest: "image"}, &domain.ArtifactLocation{ID: "location"}, nil
	}
	return nil, nil, nil
}
func (c *publicationCatalog) SaveBuildRun(ctx context.Context, run domain.BuildRun) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.runs = append(c.runs, run)
	return nil
}
func (c *publicationCatalog) PublishBuildOutput(context.Context, string, domain.ArtifactVariant, domain.ArtifactLocation) error {
	c.published = c.commit
	return errors.New("publication response lost")
}
func TestPortablePublicationReconcilesBeforeFinalizingFailure(t *testing.T) {
	for _, test := range []struct {
		name            string
		commit, unknown bool
		wantStatus      string
	}{
		{"rolled back", false, false, "failed"},
		{"committed response lost", true, false, "publishing"},
		{"outcome unknown", false, true, "publishing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			catalog := &publicationCatalog{commit: test.commit, lookupFailure: test.unknown}
			reproducer := &Reproducer{Catalog: catalog}
			id, err := reproducer.publishNamed(context.Background(), domain.ArtifactOrigin{}, "key", inspectedImage{ID: "image"}, "")
			if test.commit {
				if err != nil || id != "verified" {
					t.Fatalf("lost committed publication: %s %v", id, err)
				}
			} else if err == nil {
				t.Fatal("failure hidden")
			}
			if test.unknown && !strings.Contains(err.Error(), "reconciliation") {
				t.Fatalf("missing uncertainty: %v", err)
			}
			if catalog.runs[len(catalog.runs)-1].Status != test.wantStatus {
				t.Fatalf("incorrect publication status: %+v", catalog.runs)
			}
		})
	}
}
