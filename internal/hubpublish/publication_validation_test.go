package hubpublish

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestInvalidPublicationIsRejectedBeforeConnectingToHub(t *testing.T) {
	for _, input := range []Publication{
		{Title: "Study", Slug: strings.Repeat("a", 65), Visibility: "unlisted"},
		{Title: "Study", Slug: "ab", Visibility: "private"},
		{Title: "Study", Slug: "Study", Visibility: "private"},
		{Title: strings.Repeat("é", 81), Slug: "study", Visibility: "private"},
		{Title: "Study", Slug: "study", Description: strings.Repeat("é", 1001), Visibility: "private"},
		{Title: "Study", Slug: "study", Readme: strings.Repeat("é", 32769), Visibility: "private"},
	} {
		manager := &Manager{}
		_, err := manager.CreatePublication(context.Background(), input)
		var failure *apiError
		if !errors.As(err, &failure) || failure.Status != 422 {
			t.Fatalf("invalid publication: %v", err)
		}
	}
	for _, visibility := range []string{"private", "unlisted", "public"} {
		if err := validatePublicationInput(Publication{Title: "Study", Slug: strings.Repeat("a", 64), Visibility: visibility}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestHubValidationErrorsExplainPublicEligibilityWithoutForwardingUnknownDetails(t *testing.T) {
	err := hubResponseError(422, []byte(`{"error":"invalid input","fieldErrors":{"visibility":"Public publishing requires administrator approval. You can save this publication as private.","internal":"sensitive-debug-value"}}`))
	if !strings.Contains(err.Error(), "administrator approval") || strings.Contains(err.Error(), "sensitive-debug-value") {
		t.Fatal(err)
	}
	err = hubResponseError(422, []byte(`{"error":"sensitive-debug-value"}`))
	if strings.Contains(err.Error(), "sensitive-debug-value") {
		t.Fatal(err)
	}
}
