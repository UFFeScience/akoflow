package http_config

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"net/http"
)

func HelloMiddleware() Middleware {
	return Middleware{
		action: func(next HttpRequestHandler) HttpRequestHandler {
			return func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("HELLO-MIDDLEWARE", config.GetVersion())
				next(w, r)
			}
		},
	}
}
