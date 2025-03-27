package http_config

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"net/http"
)

func DefaultMiddleware() Middleware {
	return Middleware{
		action: func(next HttpRequestHandler) HttpRequestHandler {
			return func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("AKOFLOW-SERVER", config.GetVersion())
				next(w, r)
			}
		},
	}
}
