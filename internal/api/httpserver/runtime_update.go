package httpserver

import (
	"context"
	"net/http"
	"sync"
	"time"
)

const runtimeUpdatePath = "/_desktop/runtime-update"

type RuntimeUpdateGate struct {
	mu       sync.Mutex
	inFlight int
	until    time.Time
	active   func(context.Context) (bool, error)
}

func NewRuntimeUpdateGate(active func(context.Context) (bool, error), hold bool) *RuntimeUpdateGate {
	gate := &RuntimeUpdateGate{active: active}
	if hold {
		gate.until = time.Now().Add(2 * time.Minute)
	}
	return gate
}

func (g *RuntimeUpdateGate) Admit() bool {
	if g == nil {
		return true
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if time.Now().Before(g.until) {
		return false
	}
	g.inFlight++
	return true
}

func (g *RuntimeUpdateGate) Done() {
	if g != nil {
		g.mu.Lock()
		g.inFlight--
		g.mu.Unlock()
	}
}

func (g *RuntimeUpdateGate) Wrap(next http.Handler, token string) http.Handler {
	if g == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == runtimeUpdatePath {
			if token == "" || !validBearer(r, token) {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			g.reserve(w, r)
			return
		}
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		if !g.Admit() {
			http.Error(w, "AkôFlow is preparing a runtime update", http.StatusServiceUnavailable)
			return
		}
		defer g.Done()
		next.ServeHTTP(w, r)
	})
}

func (g *RuntimeUpdateGate) reserve(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch r.Method {
	case http.MethodPost:
		if g.inFlight > 0 || time.Now().Before(g.until) {
			http.Error(w, "AkôFlow is busy", http.StatusConflict)
			return
		}
		active, err := g.active(r.Context())
		if err != nil {
			http.Error(w, "Could not check execution state", http.StatusServiceUnavailable)
			return
		}
		if active {
			http.Error(w, "Executions are active", http.StatusConflict)
			return
		}
		g.until = time.Now().Add(2 * time.Minute)
	case http.MethodDelete:
		g.until = time.Time{}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
