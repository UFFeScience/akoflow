package mcpbridge

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/UFFeScience/akoflow/internal/application/auditcontext"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

// OAuthOptions integrates an existing authorization server. The MCP endpoint
// validates access tokens by authenticated RFC 7662 introspection; it never
// forwards client tokens to the daemon or implements its own login system.
type OAuthOptions struct {
	Issuer           string
	Resource         string
	IntrospectionURL string
	ClientID         string
	ClientSecret     string
	Client           *http.Client
}

func HTTPOptionsFromEnvironment(token string, origins []string) HTTPOptions {
	options := HTTPOptions{Token: token, AllowedOrigins: origins, PublicURL: os.Getenv("AKOFLOW_MCP_PUBLIC_URL")}
	if os.Getenv("AKOFLOW_MCP_OAUTH_ISSUER") != "" {
		options.OAuth = &OAuthOptions{
			Issuer:           os.Getenv("AKOFLOW_MCP_OAUTH_ISSUER"),
			Resource:         os.Getenv("AKOFLOW_MCP_PUBLIC_URL"),
			IntrospectionURL: os.Getenv("AKOFLOW_MCP_OAUTH_INTROSPECTION_URL"),
			ClientID:         os.Getenv("AKOFLOW_MCP_OAUTH_CLIENT_ID"),
			ClientSecret:     os.Getenv("AKOFLOW_MCP_OAUTH_CLIENT_SECRET"),
		}
	}
	return options
}

func (b *Bridge) oauthHandler(next http.Handler, options OAuthOptions) (http.Handler, error) {
	for _, value := range []string{options.Issuer, options.Resource, options.IntrospectionURL} {
		if !secureOAuthURL(value) {
			return nil, errors.New("OAuth issuer, public MCP URL and introspection URL must be HTTPS URLs (HTTP is allowed only on loopback)")
		}
	}
	resource, _ := url.Parse(options.Resource)
	if resource.Path != "/mcp" {
		return nil, errors.New("OAuth public MCP URL must end in /mcp")
	}
	if options.ClientID == "" || options.ClientSecret == "" {
		return nil, errors.New("OAuth introspection requires client credentials")
	}
	client := options.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	options.Client = &copyClient
	scope := "akoflow:" + string(b.options.Permissions)
	metadataURL := *resource
	metadataURL.Path = "/.well-known/oauth-protected-resource/mcp"
	protected := auth.RequireBearerToken(options.verifier, &auth.RequireBearerTokenOptions{
		ResourceMetadataURL: metadataURL.String(), Scopes: []string{scope},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if info := auth.TokenInfoFromContext(r.Context()); info != nil {
			auditcontext.Identify(r.Context(), info.UserID, "principal", "authenticated")
		}
		next.ServeHTTP(w, r)
	}))
	metadata := auth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource: options.Resource, AuthorizationServers: []string{options.Issuer},
		ScopesSupported: []string{scope}, BearerMethodsSupported: []string{"header"},
	})
	mux := http.NewServeMux()
	mux.Handle("/mcp", protected)
	mux.Handle("/.well-known/oauth-protected-resource/mcp", metadata)
	mux.Handle("/.well-known/oauth-protected-resource", metadata)
	return mux, nil
}

func secureOAuthURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	ip := net.ParseIP(parsed.Hostname())
	return parsed.Scheme == "http" && (parsed.Hostname() == "localhost" || (ip != nil && ip.IsLoopback()))
}

func (options OAuthOptions) verifier(ctx context.Context, token string, request *http.Request) (*auth.TokenInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	form := url.Values{"token": []string{token}, "token_type_hint": []string{"access_token"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, options.IntrospectionURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, auth.ErrInvalidToken
	}
	req.SetBasicAuth(options.ClientID, options.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := options.Client.Do(req)
	if err != nil {
		return nil, auth.ErrInvalidToken
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, auth.ErrInvalidToken
	}
	var info struct {
		Active   bool            `json:"active"`
		Exp      int64           `json:"exp"`
		Issuer   string          `json:"iss"`
		Audience json.RawMessage `json:"aud"`
		Subject  string          `json:"sub"`
		Scope    string          `json:"scope"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&info) != nil {
		return nil, auth.ErrInvalidToken
	}
	if info.Subject == "" || !info.Active || info.Exp <= time.Now().Unix() || info.Issuer != options.Issuer || !hasAudience(info.Audience, options.Resource) {
		return nil, auth.ErrInvalidToken
	}
	return &auth.TokenInfo{UserID: info.Subject, Expiration: time.Unix(info.Exp, 0), Scopes: strings.Fields(info.Scope)}, nil
}

func hasAudience(raw json.RawMessage, expected string) bool {
	var single string
	if json.Unmarshal(raw, &single) == nil {
		return single == expected
	}
	var values []string
	if json.Unmarshal(raw, &values) != nil {
		return false
	}
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
