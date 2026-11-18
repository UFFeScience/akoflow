package data

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestCatalogExecutionImageReusesConcurrentObservationsAndKeepsChangedTags(t *testing.T) {
	ctx := context.Background()
	repository := testRepository(t)
	digest := "sha256:" + strings.Repeat("a", 64)
	image := func(digest string) domain.ActivityHandle {
		return domain.ActivityHandle{Metadata: map[string]any{
			"executionTarget": "local-docker",
			"imageObservation": map[string]any{
				"status": "observed", "requestedReference": "workflow:latest", "platform": "linux/amd64", "runtimeImageId": digest,
			},
		}}
	}
	links := make([]map[string]string, 8)
	var workers sync.WaitGroup
	for i := range links {
		workers.Go(func() {
			link, err := repository.CatalogExecutionImage(ctx, image(digest))
			if err != nil {
				t.Error(err)
			}
			links[i] = link
		})
	}
	workers.Wait()
	for _, link := range links {
		if link["digest"] != digest || link["variantId"] != links[0]["variantId"] {
			t.Fatalf("concurrent usage produced different variants: %+v", links)
		}
	}
	changed, err := repository.CatalogExecutionImage(ctx, image("sha256:"+strings.Repeat("b", 64)))
	if err != nil || changed["variantId"] == "" || changed["variantId"] == links[0]["variantId"] {
		t.Fatalf("changed tag did not receive its own variant: %v %v", changed, err)
	}
	unavailable, err := repository.CatalogExecutionImage(ctx, image("workflow:latest"))
	if err != nil || unavailable != nil {
		t.Fatalf("mutable reference became a variant: %v %v", unavailable, err)
	}
	locations, err := repository.ListArtifactLocations(ctx)
	if err != nil || len(locations) != 2 {
		t.Fatalf("unexpected cache locations: %+v %v", locations, err)
	}
	for _, location := range locations {
		if location.EndpointID != "local-docker" || location.URI != "docker-image://"+location.Digest {
			t.Fatalf("unexpected local cache location: %+v", location)
		}
	}
}
