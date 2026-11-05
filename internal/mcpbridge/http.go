package mcpbridge

import (
	"crypto/subtle"
	"errors"
	"github.com/UFFeScience/akoflow/internal/application/auditcontext"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type HTTPOptions struct {
	PublicURL      string
	Token          string
	AllowedOrigins []string
	OAuth          *OAuthOptions
}

func (b *Bridge) HTTPHandler(options HTTPOptions) (http.Handler, error) {
	publicURL := options.PublicURL
	if options.OAuth != nil {
		publicURL = options.OAuth.Resource
	}
	publicHost := ""
	if publicURL != "" {
		if !secureOAuthURL(publicURL) {
			return nil, errors.New("public MCP URL must use HTTPS, except on loopback")
		}
		parsed, _ := url.Parse(publicURL)
		if parsed.Path != "/mcp" {
			return nil, errors.New("public MCP URL must end in /mcp")
		}
		publicHost = parsed.Host
	}
	transport := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server { return b.server },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, SessionTimeout: 10 * time.Minute, DisableLocalhostProtection: publicHost != ""},
	)
	var handler http.Handler
	if options.OAuth != nil {
		var err error
		handler, err = b.oauthHandler(transport, *options.OAuth)
		if err != nil {
			return nil, err
		}
	} else {
		if options.Token == "" {
			return nil, errors.New("HTTP MCP requires AKOFLOW_MCP_TOKEN or OAuth configuration")
		}
		b.redactor.mu.Lock()
		b.redactor.secrets[options.Token] = true
		b.redactor.mu.Unlock()
		handler = staticBearer(transport, options.Token, b.options.ClientID)
	}
	if publicHost != "" {
		handler = publicHostGate(handler, publicHost)
	}
	return originGate(handler, options.AllowedOrigins), nil
}

func staticBearer(next http.Handler, token string, clientIDs ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value := r.Header.Get("Authorization")
		valid := strings.HasPrefix(value, "Bearer ") &&
			subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(value, "Bearer ")), []byte(token)) == 1
		if !valid {
			auditcontext.Identify(r.Context(), "", "anonymous", "rejected")
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		clientID := "mcp-service"
		if len(clientIDs) > 0 && clientIDs[0] != "" {
			clientID = clientIDs[0]
		}
		auditcontext.Identify(r.Context(), clientID, "service_client", "authenticated")
		next.ServeHTTP(w, r)
	})
}

func originGate(next http.Handler, origins []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			allowed := false
			for _, candidate := range origins {
				if origin == candidate {
					allowed = true
					break
				}
			}
			if !allowed {
				http.Error(w, "origin is not allowed", http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, MCP-Protocol-Version, MCP-Session-Id")
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Expose-Headers", "MCP-Session-Id, MCP-Protocol-Version")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
		next.ServeHTTP(w, r)
	})
}

// A configured public URL permits a trusted reverse proxy to preserve its Host
// header while retaining an explicit host allowlist against DNS rebinding.
func publicHostGate(next http.Handler, publicHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := (&url.URL{Host: r.Host}).Hostname()
		ip := net.ParseIP(host)
		loopback := strings.EqualFold(host, "localhost") || (ip != nil && ip.IsLoopback())
		if !strings.EqualFold(r.Host, publicHost) && !loopback {
			http.Error(w, "host is not allowed", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
