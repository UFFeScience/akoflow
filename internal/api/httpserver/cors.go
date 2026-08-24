package httpserver

import (
	"net/http"
	"strings"
)

// AllowCORS accepts only explicitly configured origins. An empty allowlist
// means same-origin requests only, which is the safe default for the local API.
func AllowCORS(h http.Handler) http.Handler {
	return AllowCORSFor(h, nil)
}

func AllowCORSFor(h http.Handler, allowed []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && allowedOrigin(origin, allowed) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if r.Method == "OPTIONS" {
			if origin != "" && !allowedOrigin(origin, allowed) {
				http.Error(w, "origin is not allowed", http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func allowedOrigin(origin string, allowed []string) bool {
	for _, value := range allowed {
		if strings.TrimSpace(value) == origin {
			return true
		}
	}
	return false
}
