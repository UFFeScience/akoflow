package hubpublish

import (
	"context"
	"net/url"
)

func (m *Manager) Publications(ctx context.Context) ([]Publication, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.identityLocked(ctx); err != nil {
		return nil, err
	}
	var items []Publication
	err := m.requestLocked(ctx, "GET", "/desktop/publications", nil, nil, true, &items)
	return items, err
}
func (m *Manager) CreatePublication(ctx context.Context, input Publication) (Publication, error) {
	if err := validatePublicationInput(input); err != nil {
		return Publication{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.identityLocked(ctx); err != nil {
		return Publication{}, err
	}
	var receipt struct {
		ID string `json:"id"`
	}
	fields := map[string]string{"title": input.Title, "slug": input.Slug, "description": input.Description, "readme": input.Readme, "visibility": input.Visibility}
	if err := m.requestLocked(ctx, "POST", "/desktop/publications", fields, nil, true, &receipt); err != nil {
		return Publication{}, err
	}
	item, err := m.publicationLocked(ctx, receipt.ID)
	if err != nil {
		return Publication{}, err
	}
	m.state.DefaultPublicationID = receipt.ID
	if err = m.persistLocked(); err != nil {
		return Publication{}, err
	}
	return item.Publication, nil
}
func (m *Manager) publicationLocked(ctx context.Context, id string) (publicationInfo, error) {
	var item publicationInfo
	if id == "" {
		return item, failure(422, "publication_required", "Choose a destination publication.")
	}
	err := m.requestLocked(ctx, "GET", "/desktop/publications/"+url.PathEscape(id), nil, nil, true, &item)
	return item, err
}

func (m *Manager) contributionDestinationLocked(ctx context.Context, id string) (publicationInfo, error) {
	var item publicationInfo
	if id == "" {
		return item, failure(422, "publication_required", "Choose a destination publication.")
	}
	path := "/desktop/publications/" + url.PathEscape(id) + "/contribution-destination"
	err := m.requestLocked(ctx, "GET", path, nil, nil, true, &item)
	return item, err
}

func (m *Manager) executionDestinationLocked(ctx context.Context, id string, contribution bool) (publicationInfo, error) {
	if contribution {
		return m.contributionDestinationLocked(ctx, id)
	}
	return m.publicationLocked(ctx, id)
}
