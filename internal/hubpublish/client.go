package hubpublish

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func validateOrigin(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", failure(422, "invalid_hub_url", "Enter a Hub origin without a path, query or embedded credentials.")
	}
	if parsed.Scheme != "https" {
		ip := net.ParseIP(parsed.Hostname())
		local := parsed.Hostname() == "localhost" || ip != nil && (ip.IsPrivate() || ip.IsLoopback())
		if parsed.Scheme != "http" || !local {
			return "", failure(422, "invalid_hub_url", "Use HTTPS for Hub, or HTTP only for an explicit local network address.")
		}
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}
func (m *Manager) requestLocked(ctx context.Context, method, path string, input any, form url.Values, authenticated bool, output any) error {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, m.state.HubURL+"/api/v1"+path, body)
	if err != nil {
		return failure(422, "invalid_hub_url", "AkôFlow Hub is not configured for this installation. Contact the administrator.")
	}
	request.Header.Set("Content-Type", "application/json")
	if form != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if authenticated {
		request.Header.Set("Authorization", "Bearer "+m.state.AccessToken)
	}
	client := http.Client{Timeout: 60 * time.Second}
	if m.options.Client != nil {
		client = *m.options.Client
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(request)
	if err != nil {
		return failure(502, "hub_unavailable", "AkôFlow Hub cannot be reached. Try again or contact the administrator.")
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 24<<20+1))
	if err != nil || len(data) > 24<<20 {
		return failure(502, "hub_response", "Hub returned an invalid or oversized response.")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return hubResponseError(response.StatusCode, data)
	}
	if output != nil && json.Unmarshal(data, output) != nil {
		return failure(502, "hub_response", "Hub returned an invalid response.")
	}
	return nil
}
func hubResponseError(status int, data []byte) error {
	var result struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(data, &result)
	switch result.Error {
	case "authorization_pending", "slow_down", "access_denied", "expired_token", "invalid_grant":
		return &apiError{Status: 400, Code: result.Error, Message: "Hub authorization is not complete."}
	}
	switch status {
	case 401, 403:
		return failure(401, "hub_reconnect", "Hub access expired or was revoked. Sign in again in Hub settings.")
	case 404:
		return failure(404, "hub_not_found", "The selected publication is not available in this connected Hub project.")
	case 409:
		return failure(409, "hub_conflict", "The destination changed. Review the result again before publishing.")
	case 422:
		return failure(422, "hub_validation", "Hub rejected these details. Check the publication fields or exported workflow and review again.")
	case 429:
		return failure(429, "hub_rate_limit", "Too many Hub requests. Wait a minute and try again.")
	}
	return failure(502, "hub_response", "Hub could not complete the request. Try again.")
}
func isReconnect(err error) bool {
	var value *apiError
	return errors.As(err, &value) && value.Code == "hub_reconnect"
}
