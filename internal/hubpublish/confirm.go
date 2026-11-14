package hubpublish

import (
	"context"
	"net/url"
	"time"
)

func (m *Manager) Confirm(ctx context.Context, id string, acknowledgePublic bool) (Receipt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result Receipt
	if _, err := m.identityLocked(ctx); err != nil {
		return result, err
	}
	prepared, ok := m.previews[id]
	if !ok || time.Now().After(prepared.ExpiresAt) || prepared.TokenHash != digest([]byte(m.state.AccessToken)) {
		return result, failure(409, "review_expired", "This review expired or the connected account changed. Review the result again.")
	}
	current, err := m.publicationLocked(ctx, prepared.Publication.ID)
	if err != nil {
		return result, err
	}
	if current.Publication.Visibility != prepared.Publication.Visibility {
		return result, failure(409, "visibility_changed", "Publication visibility changed. Review the destination again before publishing.")
	}
	if current.Publication.Visibility == "public" && !acknowledgePublic {
		return result, failure(422, "public_confirmation", "Confirm that you reviewed the content of this public publication.")
	}
	var receipt struct {
		ExperimentID string             `json:"experimentId"`
		ResultCount  int                `json:"resultCount"`
		Version      publicationVersion `json:"version"`
	}
	path := "/desktop/publications/" + url.PathEscape(prepared.Publication.ID) + "/import/confirm"
	if err = m.requestLocked(ctx, "POST", path, prepared.Input, nil, true, &receipt); err != nil {
		return result, err
	}
	ownerKind := "u"
	if current.Publication.Kind == "organization" {
		ownerKind = "o"
	}
	result = Receipt{
		URL:          m.options.BrowserURL + "/" + ownerKind + "/" + url.PathEscape(current.Publication.Owner) + "/" + url.PathEscape(current.Publication.Slug),
		ExperimentID: receipt.ExperimentID,
		ResultCount:  receipt.ResultCount,
		Version:      receipt.Version.Number,
	}
	m.state.DefaultPublicationID = prepared.Publication.ID
	if err = m.persistLocked(); err != nil {
		return result, err
	}
	return result, nil
}
