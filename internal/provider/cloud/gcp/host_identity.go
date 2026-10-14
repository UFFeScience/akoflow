package gcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/crypto/ssh"
)

var errHostKeyPublicationPending = errors.New("provider-backed host key publication pending")

// HostIdentity contains only public evidence obtained through authenticated
// Compute API reads. It does not enroll a key or relax existing SSH trust.
type HostIdentity struct {
	Generation  string
	PublicKey   string
	Fingerprint string
}

func (c *Catalog) VerifyHostIdentity(ctx context.Context, credential []byte, project, zone, name, expectedGeneration string) (HostIdentity, error) {
	if project == "" || zone == "" || name == "" || expectedGeneration == "" {
		return HostIdentity{}, fmt.Errorf("cloud host identity requires project, zone, name and generation")
	}
	var account serviceAccount
	if err := json.Unmarshal(credential, &account); err != nil {
		return HostIdentity{}, fmt.Errorf("decode GCP credential: %w", err)
	}
	token, err := c.accessToken(ctx, account)
	if err != nil {
		return HostIdentity{}, err
	}
	endpoint := fmt.Sprintf("%s/projects/%s/zones/%s/instances/%s", c.computeEndpoint, url.PathEscape(project), url.PathEscape(zone), url.PathEscape(name))
	var before struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := c.get(ctx, token, endpoint, &before); err != nil {
		return HostIdentity{}, err
	}
	if before.ID != expectedGeneration || before.Name != name {
		return HostIdentity{}, fmt.Errorf("cloud instance generation does not match expected identity")
	}
	var attributes struct {
		QueryValue struct {
			Items []struct{ Namespace, Key, Value string } `json:"items"`
		} `json:"queryValue"`
	}
	if err := c.get(ctx, token, endpoint+"/getGuestAttributes?queryPath=hostkeys/", &attributes); err != nil {
		var apiError *APIError
		if errors.As(err, &apiError) && apiError.StatusCode == 404 {
			return HostIdentity{}, errHostKeyPublicationPending
		}
		return HostIdentity{}, fmt.Errorf("read provider-backed host keys: %w", err)
	}
	var publicKey ssh.PublicKey
	for _, item := range attributes.QueryValue.Items {
		if item.Namespace != "hostkeys" || item.Key != "ssh-ed25519" {
			continue
		}
		if publicKey != nil {
			return HostIdentity{}, fmt.Errorf("ambiguous provider-backed ED25519 keys")
		}
		key, _, options, rest, err := ssh.ParseAuthorizedKey([]byte(item.Key + " " + strings.TrimSpace(item.Value)))
		if err != nil || len(options) != 0 || len(strings.TrimSpace(string(rest))) != 0 || key.Type() != ssh.KeyAlgoED25519 {
			return HostIdentity{}, fmt.Errorf("invalid provider-backed ED25519 key")
		}
		publicKey = key
	}
	if publicKey == nil {
		return HostIdentity{}, errHostKeyPublicationPending
	}
	// Fence replacement between the identity and guest-attribute reads.
	var after struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := c.get(ctx, token, endpoint, &after); err != nil {
		return HostIdentity{}, err
	}
	if after.ID != before.ID || after.Name != before.Name {
		return HostIdentity{}, fmt.Errorf("cloud instance generation changed during host verification")
	}
	return HostIdentity{Generation: before.ID, PublicKey: strings.TrimSpace(string(ssh.MarshalAuthorizedKey(publicKey))), Fingerprint: ssh.FingerprintSHA256(publicKey)}, nil
}
