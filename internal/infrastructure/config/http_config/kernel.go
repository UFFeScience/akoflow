package http_config

import "net/http"

func Middlewares() []Middleware {
	return []Middleware{
		DefaultMiddleware(),
	}
}

var MapMiddleware = map[string]Middleware{
	"hello": HelloMiddleware(),
}

func KernelHandler(requestHandler HttpRequestHandler, middlewares ...string) func(w http.ResponseWriter, r *http.Request) {
	return handleMiddleware(requestHandler, middlewares)
}
