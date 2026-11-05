package httpserver

import (
	"crypto/subtle"
	"github.com/UFFeScience/akoflow/internal/application/auditcontext"
	"github.com/google/uuid"
	"net"
	"net/http"
	"strings"
)

// SecurityOptions keeps local development usable while making an accidental
// public listener fail closed. A public API must always have a bearer token.
type SecurityOptions struct {
	BearerToken    string
	AllowedOrigins []string
	LoopbackOnly   bool
}

func SecureAPI(next http.Handler, options SecurityOptions) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if options.LoopbackOnly && !isLoopbackRequest(r) {
			setAccessIdentity(r, "", "anonymous", "forbidden")
			http.Error(w, "API is restricted to loopback", http.StatusForbidden)
			return
		}
		if options.BearerToken != "" && !isPublicBootstrapRequest(r) && !validBearer(r, options.BearerToken) {
			setAccessIdentity(r, "", "anonymous", "rejected")
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		if options.BearerToken != "" && validBearer(r, options.BearerToken) {
			setAccessIdentity(r, "api-service", "service_client", "authenticated")
			if trace := auditcontext.From(r.Context()); trace != nil {
				if correlation, err := uuid.Parse(r.Header.Get("X-Correlation-ID")); err == nil {
					trace.CorrelationID = correlation.String()
					w.Header().Set("X-Correlation-ID", trace.CorrelationID)
				}
			}
		} else {
			setAccessIdentity(r, "", "anonymous", "not_required")
		}
		next.ServeHTTP(w, r)
	})
}

// isPublicBootstrapRequest keeps remote environment discovery possible before
// credentials for this control plane have been exchanged. The instance record
// contains only the installation identity; every mutable or operational API
// remains protected by the bearer token.
func isPublicBootstrapRequest(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	return r.URL.Path == "/akoflow-api/instance" || r.URL.Path == "/akoflow-api/instance/" ||
		r.URL.Path == "/akoflow-api/preflight/"
}

func validBearer(r *http.Request, expected string) bool {
	value := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return value != "" && subtle.ConstantTimeCompare([]byte(value), []byte(expected)) == 1
}

func isLoopbackRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}
