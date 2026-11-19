package data

import (
	"context"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestArtifactDisplayNamesPreserveIdentityAndDisambiguateCollisions(t *testing.T) {
	ctx := context.Background()
	repository := testRepository(t)
	for _, id := range []string{"origin-first", "origin-second"} {
		version := domain.ArtifactVersion{ID: id + "-version", ArtifactID: id, Name: "Validate graph", Version: "1"}
		if err := repository.RegisterArtifactVersion(ctx, version); err != nil {
			t.Fatal(err)
		}
	}
	artifacts, err := repository.ListArtifacts(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, item := range artifacts {
		if strings.HasPrefix(item.ID, "origin-") {
			names[item.ID] = item.Name
		}
	}
	for _, id := range []string{"origin-first", "origin-second"} {
		if !strings.HasPrefix(names[id], "Validate graph · ") {
			t.Fatalf("unexpected display label for %s: %q", id, names[id])
		}
	}
	if names["origin-first"] == names["origin-second"] {
		t.Fatal("display labels did not distinguish artifact identities")
	}
}
