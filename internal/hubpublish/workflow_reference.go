package hubpublish

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/google/uuid"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var workflowReferencePattern = regexp.MustCompile(`^/(u|o|a)/[a-z0-9][a-z0-9_.-]{1,63}/[a-z0-9][a-z0-9-]{1,63}:(latest|v?[1-9][0-9]*)$`)

func (m *Manager) workflowReference(ctx context.Context, reference string) (WorkflowDefinition, error) {
	reference = strings.TrimSpace(reference)
	if strings.HasPrefix(reference, "akoflow::") {
		reference = strings.TrimPrefix(reference, "akoflow::")
		if strings.HasPrefix(reference, "o/") || strings.HasPrefix(reference, "a/") {
			reference = "/" + reference
		} else {
			reference = "/u/" + reference
		}
	}
	if !workflowReferencePattern.MatchString(reference) {
		return WorkflowDefinition{}, failure(422, "invalid_workflow_uri", "Use akoflow::owner/workflow:latest or akoflow::owner/workflow:v1.")
	}
	m.mu.Lock()
	origin, token := m.state.HubURL, m.state.AccessToken
	m.mu.Unlock()
	if origin == "" {
		return WorkflowDefinition{}, failure(503, "hub_not_configured", "AkôFlow Hub is not configured for this installation.")
	}
	address := origin + "/api/v1/workflow-definitions/resolve?" + url.Values{"reference": {reference}}.Encode()
	data, status, headers, err := m.readWorkflowYAML(ctx, address, "")
	if err == nil && status == 404 && token != "" {
		data, status, headers, err = m.readWorkflowYAML(ctx, address, token)
	}
	if err != nil {
		return WorkflowDefinition{}, err
	}
	if status != 200 {
		if status == 404 || status == 401 || status == 403 {
			return WorkflowDefinition{}, failure(404, "workflow_not_found", "Publication or version unavailable. Check the link and version. For a private publication, connect Desktop to the Hub project that can access it.")
		}
		return WorkflowDefinition{}, hubResponseError(status, data)
	}
	version := headers.version
	if version < 1 {
		return WorkflowDefinition{}, failure(502, "hub_response", "Hub did not identify the resolved workflow version.")
	}
	hash := sha256.Sum256(data)
	base, tag, _ := strings.Cut(reference, ":")
	if tag != "latest" && strings.TrimPrefix(tag, "v") != strconv.Itoa(version) {
		return WorkflowDefinition{}, failure(502, "hub_response", "Hub returned a different workflow version. Reload the source before importing.")
	}
	parts := strings.Split(strings.TrimPrefix(base, "/"), "/")
	title, owner := headers.title, headers.owner
	if title == "" {
		title = parts[2]
	}
	if owner == "" || parts[0] == "a" {
		owner = parts[1]
	}
	publicationID := ""
	if id, err := uuid.Parse(headers.publicationID); err == nil {
		publicationID = id.String()
	}
	return WorkflowDefinition{
		origin: origin, PublicationID: publicationID, Publication: title, Owner: owner,
		URI:     "akoflow::" + strings.TrimPrefix(strings.TrimPrefix(base, "/u/"), "/") + ":v" + strconv.Itoa(version),
		Version: version, YAML: string(data), Hash: hex.EncodeToString(hash[:]),
	}, nil
}

func browserWorkflowReference(link, origin, browser string) (string, error) {
	invalid := failure(422, "invalid_workflow_link", "Paste a publication or version link from the configured AkôFlow Hub, or use an akoflow:: workflow URI.")
	uri, err := url.Parse(link)
	if err != nil || uri.User != nil || uri.Host == "" || uri.RawPath != "" {
		return "", invalid
	}
	linkOrigin := uri.Scheme + "://" + uri.Host
	if linkOrigin != origin && linkOrigin != browser {
		return "", invalid
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(uri.Path, "/"), "/"), "/")
	if len(parts) < 3 {
		return "", invalid
	}
	base := "/" + strings.Join(parts[:3], "/")
	version := "latest"
	switch {
	case len(parts) == 3:
	case len(parts) == 4 && (parts[3] == "versions" || parts[3] == "workflow" || parts[3] == "images" || parts[3] == "environment"):
	case len(parts) == 5 || len(parts) == 6:
		if parts[3] != "versions" || (len(parts) == 6 && parts[5] != "code" && parts[5] != "changes") {
			return "", invalid
		}
		number, err := strconv.Atoi(parts[4])
		if err != nil || number < 1 || strconv.Itoa(number) != parts[4] {
			return "", invalid
		}
		version = "v" + parts[4]
	default:
		return "", invalid
	}
	if uri.Fragment != "" {
		fragment := strings.TrimPrefix(uri.Fragment, "versions/v")
		if fragment != uri.Fragment && version == "latest" {
			number, err := strconv.Atoi(fragment)
			if err != nil || number < 1 || strconv.Itoa(number) != fragment {
				return "", invalid
			}
			version = "v" + fragment
		}
	}
	if reference := base + ":" + version; workflowReferencePattern.MatchString(reference) {
		return reference, nil
	}
	return "", invalid
}
