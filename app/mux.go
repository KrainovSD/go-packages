package app

import (
	"net/http"
)

type MuxMiddleware = func(next http.Handler) http.Handler

type GlobalMux struct {
	mux              *http.ServeMux
	middlewareGroups map[string][]MuxMiddleware
}

func newMux(middlewareGroups map[string][]MuxMiddleware) *GlobalMux {
	if middlewareGroups == nil {
		middlewareGroups = make(map[string][]MuxMiddleware, 2)
	}
	return &GlobalMux{
		mux:              http.NewServeMux(),
		middlewareGroups: middlewareGroups,
	}
}

func (m *GlobalMux) WithMiddlewares(group string) *Mux {
	var middlewares = m.middlewareGroups[group]
	return &Mux{
		mux:         m.mux,
		middlewares: middlewares,
	}
}

func (m *GlobalMux) FullMiddlewares() *Mux {
	return m.WithMiddlewares("full")
}

func (m *GlobalMux) BaseMiddlewares() *Mux {
	return m.WithMiddlewares("base")
}

func (m *GlobalMux) AddMiddlewareGroup(name string, group []MuxMiddleware) {
	m.middlewareGroups[name] = group
}

func (m *GlobalMux) ExtendMiddlewareGroup(parent string, name string, group []MuxMiddleware) {
	var parentMiddlewares = m.middlewareGroups[parent]
	var middlewares = make([]MuxMiddleware, len(parentMiddlewares), len(parentMiddlewares)+len(group))
	copy(middlewares, parentMiddlewares)
	for _, m := range group {
		middlewares = append(middlewares, m)
	}
	m.middlewareGroups[name] = middlewares
}

type Mux struct {
	mux         *http.ServeMux
	middlewares []MuxMiddleware
}

func (m *Mux) Handle(pattern string, handler http.Handler) {
	var h = handler
	for i := len(m.middlewares) - 1; i >= 0; i-- {
		h = m.middlewares[i](h)
	}
	m.mux.Handle(pattern, h)
}

func (m *Mux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	var h http.Handler = http.HandlerFunc(handler)
	for i := len(m.middlewares) - 1; i >= 0; i-- {
		h = m.middlewares[i](h)
	}
	m.mux.Handle(pattern, h)
}
