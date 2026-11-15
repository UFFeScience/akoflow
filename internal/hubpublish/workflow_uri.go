package hubpublish

import (
	"github.com/google/uuid"
	"net/url"
	"strconv"
	"strings"
)

func parseWorkflowURI(value string) (string, int, error) {
	uri, err := url.Parse(strings.TrimSpace(value))
	invalid := failure(422, "invalid_workflow_uri", "Paste an AkôFlow Hub workflow URI with a publication and version.")
	if err != nil || uri.Scheme != "akoflow" || uri.Host != "hub" || uri.User != nil || uri.ForceQuery || uri.RawQuery != "" || uri.Fragment != "" || uri.Opaque != "" || uri.RawPath != "" {
		return "", 0, invalid
	}
	parts := strings.Split(strings.TrimPrefix(uri.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "publications" || parts[2] != "versions" {
		return "", 0, invalid
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return "", 0, invalid
	}
	version, err := strconv.Atoi(parts[3])
	if err != nil || version < 1 || strconv.Itoa(version) != parts[3] {
		return "", 0, invalid
	}
	return id.String(), version, nil
}
