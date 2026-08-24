package http_config

import "net/http"

type Middleware struct {
	action func(HttpRequestHandler) HttpRequestHandler
}

type HttpRequestHandler func(w http.ResponseWriter, r *http.Request)

func GetMiddleware(name string) Middleware {
	if _, ok := MapMiddleware[name]; !ok {
		println("Middleware not found: ", name, " using default middleware")
		return DefaultMiddleware()
	}
	return MapMiddleware[name]
}

func handleMiddleware(requestHandler HttpRequestHandler, middlewaresStr []string) HttpRequestHandler {
	middlewares := Middlewares()
	for _, name := range middlewaresStr {
		middlewares = append(middlewares, GetMiddleware(name))
	}

	for i := len(middlewares) - 1; i >= 0; i-- {
		requestHandler = middlewares[i].action(requestHandler)
	}
	return requestHandler
}
