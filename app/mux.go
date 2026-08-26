package app

import (
	"net/http"
	"slices"
)

type Middleware = func(next http.Handler) http.Handler

type MuxMiddleware struct {
	ID string
	Fn Middleware
}

type Mux struct {
	mux         *http.ServeMux
	middlewares []MuxMiddleware
}

func (m *Mux) Middlewares() []MuxMiddleware {
	return slices.Clone(m.middlewares)
}

func (m *Mux) SelectMiddlewares(order []string) []MuxMiddleware {
	var middlewares = make([]MuxMiddleware, 0, len(order))
	for _, ID := range order {
		for _, mid := range m.middlewares {
			if ID == mid.ID {
				middlewares = append(middlewares, mid)
				break
			}
		}
	}
	return middlewares
}

func (m *Mux) Mux() *http.ServeMux {
	return m.mux
}

func (m *Mux) PushMiddlewares(middlewares []MuxMiddleware) {
	m.middlewares = append(m.middlewares, middlewares...)
}

func (m *Mux) UnshiftMiddlewares(middlewares []MuxMiddleware) {
	var newMiddlewares = make([]MuxMiddleware, 0, len(m.middlewares)+len(middlewares))
	newMiddlewares = append(newMiddlewares, middlewares...)
	newMiddlewares = append(newMiddlewares, m.middlewares...)
	m.middlewares = newMiddlewares
}

func (m *Mux) Clone() *Mux {
	var clone = &Mux{
		mux:         m.mux,
		middlewares: slices.Clone(m.middlewares),
	}
	return clone
}

func (m *Mux) CloneWith(middlewares []MuxMiddleware) *Mux {
	var clone = &Mux{
		mux:         m.mux,
		middlewares: middlewares,
	}
	return clone
}

func (m *Mux) HandleExclude(pattern string, handler http.Handler, excludeIds ...string) {
	m.handle(pattern, handler, excludeIds)
}

func (m *Mux) HandleFuncExclude(pattern string, handler func(http.ResponseWriter, *http.Request), excludeIds ...string) {
	m.handle(pattern, http.HandlerFunc(handler), excludeIds)
}

func (m *Mux) Handle(pattern string, handler http.Handler) {
	m.handle(pattern, handler, nil)
}

func (m *Mux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	m.handle(pattern, http.HandlerFunc(handler), nil)
}

func (m *Mux) handle(pattern string, handler http.Handler, excludeIds []string) {
	var h = handler
	for i := len(m.middlewares) - 1; i >= 0; i-- {
		var middleware = m.middlewares[i]
		if slices.Contains(excludeIds, middleware.ID) {
			continue
		}
		h = middleware.Fn(h)
	}
	m.mux.Handle(pattern, h)
}
