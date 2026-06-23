package traces

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/KrainovSD/go-packages/web"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
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

func CreateMiddleware(opts *MiddlewareOptions) *Middleware {
	return &Middleware{
		traces:        opts.Traces,
		excludeStatic: opts.ExcludeStatic,
	}
}

func (m *Middleware) Register(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var withTraces = m.traces.Exist()
		if withTraces && m.excludeStatic && filepath.Ext(r.URL.Path) != "" {
			withTraces = false
		}
		if withTraces {
			var ctx = otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
			var span trace.Span
			var attrs = make([]attribute.KeyValue, 0, 5)
			attrs = append(attrs, attribute.String("http.method", r.Method))
			attrs = append(attrs, attribute.String("http.path", r.URL.Path))
			if r.URL.RawQuery != "" {
				attrs = append(attrs, attribute.String("http.queries", r.URL.RawQuery))
			}
			attrs = append(attrs, attribute.String("http.host", r.URL.Host))
			attrs = append(attrs, attribute.String("http.user_agent", web.GetUserAgent(r)))
			ctx, span = m.traces.GetTracer().Start(
				ctx,
				fmt.Sprintf("HTTP %s %s", r.Method, r.URL.Path),
				trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(attrs...))
			w.Header().Set("Trace-Id", span.SpanContext().TraceID().String())
			r = r.WithContext(ctx)
			defer func() {
				var status = 200
				if writer, ok := w.(*web.ResponseWriter); ok {
					if writer.Status() != 0 {
						status = writer.Status()
					}
					var p = writer.GetPanic()
					if p != nil {
						span.SetStatus(codes.Error, p.Err.Error())
						span.RecordError(p.Err)
					}
				}
				span.SetAttributes(attribute.Int("status", status))
				span.End()
			}()
		}
		next.ServeHTTP(w, r)
	})
}
