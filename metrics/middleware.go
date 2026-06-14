package metrics

import (
	"net/http"
)

type MiddlewareOptions struct {
	Metrics *MetricsProvider
}

type Middleware struct {
	metrics *MetricsProvider
}

func MiddlewareCreate(opts MiddlewareOptions) *Middleware {
	return &Middleware{
		metrics: opts.Metrics,
	}
}

func (m *Middleware) Register(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.metrics.IncreaseConnectionsHTTP()
		defer m.metrics.DecreaseConnectionsHTTP()
		next.ServeHTTP(w, r)
	})
}
