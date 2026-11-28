package hubpublish

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

type PublicationPatch struct {
	AcknowledgePublic bool                 `json:"acknowledgePublic,omitempty"`
	Title             *string              `json:"title,omitempty"`
	Slug              *string              `json:"slug,omitempty"`
	Description       *string              `json:"description,omitempty"`
	Readme            *string              `json:"readme,omitempty"`
	Visibility        *string              `json:"visibility,omitempty"`
	Authorship        *string              `json:"authorship,omitempty"`
	Tags              *[]string            `json:"tags,omitempty"`
	ImageTags         *map[string][]string `json:"imageTags,omitempty"`
}

func (m *Manager) ReadPublication(ctx context.Context, id, section string, query url.Values) (json.RawMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.identityLocked(ctx); err != nil {
		return nil, err
	}
	path, err := publicationResourcePath(id, section)
	if err != nil {
		return nil, err
	}
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	var result json.RawMessage
	err = m.requestLocked(ctx, "GET", path, nil, nil, true, &result)
	return result, err
}

func publicationResourcePath(id, section string) (string, error) {
	if id == "" {
		return "", failure(422, "publication_required", "Choose a publication.")
	}
	switch section {
	case "", "versions", "results", "compare":
	default:
		return "", failure(422, "invalid_resource", "Choose a supported publication resource.")
	}
	path := "/desktop/publications/" + url.PathEscape(id)
	if section != "" {
		path += "/" + section
	}
	return path, nil
}

func (m *Manager) UpdatePublication(ctx context.Context, id string, input PublicationPatch) (json.RawMessage, error) {
	if m.options.ReadOnly {
		return nil, failure(403, "read_only", "Read-only instances cannot update Hub publications.")
	}
	if input.Visibility != nil && *input.Visibility != "private" && !input.AcknowledgePublic {
		return nil, failure(422, "public_confirmation", "Acknowledge the public visibility change explicitly.")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.identityLocked(ctx); err != nil {
		return nil, err
	}
	path, err := publicationResourcePath(id, "")
	if err != nil {
		return nil, err
	}
	if err = m.requestLocked(ctx, "PATCH", path, input, nil, true, nil); err != nil {
		return nil, err
	}
	m.invalidatePublicationPreviewsLocked(id)
	var result json.RawMessage
	err = m.requestLocked(ctx, "GET", path, nil, nil, true, &result)
	return result, err
}

func (m *Manager) DeletePublication(ctx context.Context, id string) error {
	if m.options.ReadOnly {
		return failure(403, "read_only", "Read-only instances cannot delete Hub publications.")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.identityLocked(ctx); err != nil {
		return err
	}
	path, err := publicationResourcePath(id, "")
	if err != nil {
		return err
	}
	if err = m.requestLocked(ctx, "DELETE", path, nil, nil, true, nil); err != nil {
		return err
	}
	m.invalidatePublicationPreviewsLocked(id)
	if m.state.DefaultPublicationID == id {
		m.state.DefaultPublicationID = ""
		return m.persistLocked()
	}
	return nil
}

func (m *Manager) RetireVersion(ctx context.Context, id string, version, replacement int) error {
	if m.options.ReadOnly {
		return failure(403, "read_only", "Read-only instances cannot retire Hub workflow versions.")
	}
	if version < 1 || replacement <= version {
		return failure(422, "invalid_version_retirement", "Choose an older version and its verified replacement.")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.identityLocked(ctx); err != nil {
		return err
	}
	publication, err := m.publicationLocked(ctx, id)
	if err != nil {
		return err
	}
	latest := 0
	for _, candidate := range publication.Versions {
		latest = max(latest, candidate.Number)
	}
	if latest != replacement {
		return failure(409, "replacement_version_changed", "The verified replacement is no longer the latest version.")
	}
	path := "/desktop/publications/" + url.PathEscape(id) + "/versions/" + strconv.Itoa(version)
	if err := m.requestLocked(ctx, "DELETE", path, nil, nil, true, nil); err != nil {
		return err
	}
	m.invalidatePublicationPreviewsLocked(id)
	return nil
}

func (m *Manager) invalidatePublicationPreviewsLocked(id string) {
	for key, item := range m.previews {
		if item.Publication.ID == id {
			delete(m.previews, key)
		}
	}
}
