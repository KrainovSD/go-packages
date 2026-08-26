package metrics

import (
	"net/http"
)

type MiddlewareOptions struct {
	Metrics *Provider
}

const MiddlewareID = "ksd-metrics"

func NewMiddleware(opts *MiddlewareOptions) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			opts.Metrics.IncreaseConnectionsHTTP()
			defer opts.Metrics.DecreaseConnectionsHTTP()
			next.ServeHTTP(w, r)
		})
	}
}
