package hubpublish

import (
	"regexp"
	"strings"
)

var publicationSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func validatePublicationInput(input Publication) error {
	if len(strings.TrimSpace(input.Title)) == 0 || len(strings.TrimSpace(input.Title)) > 160 {
		return failure(422, "publication_title", "Enter a publication title of up to 160 UTF-8 bytes.")
	}
	if len(input.Slug) < 3 || len(input.Slug) > 64 || !publicationSlugPattern.MatchString(input.Slug) {
		return failure(422, "publication_slug", "Publication slug must contain 3–64 lowercase letters, numbers or single hyphens between words.")
	}
	if len(input.Description) > 2000 {
		return failure(422, "publication_description", "Publication description must be at most 2,000 UTF-8 bytes.")
	}
	if len(input.Readme) > 65536 {
		return failure(422, "publication_readme", "Publication README must be at most 64 KiB of UTF-8 text.")
	}
	if input.Visibility != "private" && input.Visibility != "unlisted" && input.Visibility != "public" {
		return failure(422, "publication_visibility", "Choose private, anonymous link or public visibility.")
	}
	return nil
}

func hubValidationError(fields map[string]string) error {
	messages := []string{}
	for _, name := range []string{"title", "slug", "description", "readme", "visibility", "authorship", "projectId", "namespaceId"} {
		message := strings.TrimSpace(fields[name])
		if message != "" && len(message) <= 512 {
			messages = append(messages, name+": "+message)
		}
	}
	if len(messages) > 0 {
		return failure(422, "hub_validation", strings.Join(messages, " "))
	}
	return failure(422, "hub_validation", "Hub rejected these details. Check the publication fields or exported workflow and review again.")
}
