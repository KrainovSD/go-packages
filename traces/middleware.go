package traces

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"

	"go.opentelemetry.io/otel/attribute"
)

type Writer interface {
	Status() int
}

type MiddlewareOptions struct {
	Traces        *Provider
	ExcludeStatic bool
}

type Middleware struct {
	excludeStatic bool
	traces        *Provider
}

func MiddlewareCreate(opts *MiddlewareOptions) *Middleware {
	return &Middleware{
		traces:        opts.Traces,
		excludeStatic: opts.ExcludeStatic,
	}
}

func (m *Middleware) Register(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ctx context.Context
		var skip bool
		if m.excludeStatic {
			var ext = strings.ToLower(filepath.Ext(r.URL.Path))
			if ext != "" {
				skip = true
			}
		}
		if m.traces.Exist() && !skip {
			var closeSpan func()
			ctx, closeSpan = m.traces.StartRequest(r)
			defer closeSpan()
			r = r.WithContext(ctx)

			var traceId = m.traces.GetTraceId(ctx)
			if traceId != nil {
				w.Header().Set("Trace-Id", *traceId)
			}
		}
		next.ServeHTTP(w, r)
		if m.traces.Exist() && !skip {
			var status = 200
			if writer, ok := w.(Writer); ok {
				if writer.Status() != 0 {
					status = writer.Status()
				}
			}
			m.traces.SetAttributes(ctx, attribute.Int("status", status))
		}
	})
}
