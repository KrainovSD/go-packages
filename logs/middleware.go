package logs

import (
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"github.com/KrainovSD/go-packages/web"
)

type MiddlewareOptions struct {
	Log           *slog.Logger
	ExcludeStatic bool
}

type Middleware struct {
	excludeStatic bool
	log           *slog.Logger
}

func CreateMiddleware(opts *MiddlewareOptions) *Middleware {
	return &Middleware{
		log:           opts.Log,
		excludeStatic: opts.ExcludeStatic,
	}
}

func (m *Middleware) Register(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var start = time.Now()
		next.ServeHTTP(w, r)
		if m.excludeStatic && filepath.Ext(r.URL.Path) != "" {
			return
		}
		var attrs = make([]slog.Attr, 0, 10)
		attrs = append(attrs, slog.String("host", r.Host))
		attrs = append(attrs, slog.String("userAgent", web.GetUserAgent(r)))
		attrs = append(attrs, slog.String("method", r.Method))
		attrs = append(attrs, slog.String("path", r.URL.Path))
		if r.URL.RawQuery != "" {
			attrs = append(attrs, slog.String("queries", r.URL.RawQuery))
		}
		attrs = append(attrs, slog.Duration("duration", time.Since(start)))
		var logLevel slog.Level = slog.LevelInfo
		if writer, ok := w.(*web.ResponseWriter); ok {
			var status = 200
			if writer.Status() != 0 {
				status = writer.Status()
			}
			attrs = append(attrs, slog.Int("status", status))
			if status >= 400 && status < 500 {
				logLevel = slog.LevelWarn
			} else if status >= 500 {
				logLevel = slog.LevelError
			}
			var err = writer.GetError()
			var p = writer.GetPanic()
			if p != nil {
				attrs = append(attrs, slog.String("error", p.Err.Error()))
				attrs = append(attrs, slog.String("stack", string(p.Stack)))
			} else if err != nil {
				attrs = append(attrs, slog.String("error", err.Error()))
			}
		} else {
			attrs = append(attrs, slog.Int("status", 200))
		}
		m.log.LogAttrs(r.Context(), logLevel, "request", attrs...)
	})
}
