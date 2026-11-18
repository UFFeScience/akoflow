package hubpublish

import (
	"context"
	"net/url"
	"strconv"
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
	if prepared.Profile != nil {
		return m.confirmProfileExecutionLocked(ctx, *prepared.Profile, acknowledgePublic)
	}
	current, err := m.executionDestinationLocked(ctx, prepared.Publication.ID, prepared.Input.Contribution)
	if err != nil {
		return result, err
	}
	if current.Publication.Visibility != prepared.Publication.Visibility || current.Publication.Authorship != prepared.Publication.Authorship || current.Publication.AnonymousOwner != prepared.Publication.AnonymousOwner {
		return result, failure(409, "visibility_changed", "Publication visibility changed. Review the destination again before publishing.")
	}
	if current.Publication.Visibility != "private" && !acknowledgePublic {
		return result, failure(422, "public_confirmation", "Confirm that you reviewed the content of this public publication.")
	}
	receipt, err := m.confirmPreparedLocked(ctx, prepared)
	if err != nil {
		return result, err
	}
	ownerKind := "u"
	owner := current.Publication.Owner
	if current.Publication.Kind == "organization" {
		ownerKind = "o"
	}
	if current.Publication.Visibility == "unlisted" {
		ownerKind = "a"
		owner = current.Publication.AnonymousOwner
	}
	publicationURL := m.options.BrowserURL + "/" + ownerKind + "/" + url.PathEscape(owner) + "/" + url.PathEscape(current.Publication.Slug)
	if prepared.Input.TargetVersion > 0 && receipt.ExperimentID != "" && prepared.RunID != "" {
		publicationURL += "/results/" + strconv.Itoa(receipt.Version.Number) + "/executions/" +
			url.PathEscape(receipt.ExperimentID) + "/runs/" + url.PathEscape(prepared.RunID) + "/workflow"
	}
	result = Receipt{
		URL:                publicationURL,
		ContributionStatus: receipt.ContributionStatus,
		ExperimentID:       receipt.ExperimentID,
		ResultCount:        receipt.ResultCount,
		Version:            receipt.Version.Number,
	}
	if prepared.Input.Contribution {
		return result, nil
	}
	m.state.DefaultPublicationID = prepared.Publication.ID
	if err = m.persistLocked(); err != nil {
		return result, err
	}
	return result, nil
}

type publicationReceipt struct {
	ContributionStatus string             `json:"contributionStatus"`
	ExperimentID       string             `json:"experimentId"`
	ResultCount        int                `json:"resultCount"`
	Version            publicationVersion `json:"version"`
}

func (m *Manager) confirmPreparedLocked(ctx context.Context, prepared preparedPublication) (publicationReceipt, error) {
	var receipt publicationReceipt
	path := "/desktop/publications/" + url.PathEscape(prepared.Publication.ID)
	if prepared.VersionInput != nil {
		var version publicationVersion
		err := m.requestLocked(ctx, "POST", path+"/versions", prepared.VersionInput, nil, true, &version)
		receipt.Version = version
		return receipt, err
	}
	err := m.requestLocked(ctx, "POST", path+"/import/confirm", prepared.Input, nil, true, &receipt)
	return receipt, err
}
