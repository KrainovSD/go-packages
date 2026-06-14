package logs

import (
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/KrainovSD/go-packages/web"
)

type Writer interface {
	Status() int
	Error() string
}

type MiddlewareOptions struct {
	Log           *slog.Logger
	ExcludeStatic bool
}

type Middleware struct {
	excludeStatic bool
	log           *slog.Logger
}

func MiddlewareCreate(opts *MiddlewareOptions) *Middleware {
	return &Middleware{
		log:           opts.Log,
		excludeStatic: opts.ExcludeStatic,
	}
}

func (m *Middleware) Register(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var start = time.Now()
		var hostAttr = slog.String("host", r.Host)
		var userAgentAttr = slog.String("userAgent", r.UserAgent())
		var methodAttr = slog.String("method", r.Method)
		var urlAttr = slog.String("url", r.URL.String())

		defer func() {
			if err := recover(); err != nil {
				var durationAttr = slog.Duration("duration", time.Since(start))
				var errorMessageAttr = slog.Any("error", err)
				var stackTrace = debug.Stack()
				var stackTraceAttr = slog.String("stack", string(stackTrace))
				var statusAttr slog.Attr = slog.Int("status", 500)
				m.log.LogAttrs(r.Context(), slog.LevelError, "request", hostAttr, userAgentAttr, methodAttr, urlAttr, statusAttr, durationAttr, stackTraceAttr, errorMessageAttr)
				web.SendError(w, web.ErrorResponse{Error: fmt.Errorf("something went wrong"), Status: 500})
			}
		}()
		next.ServeHTTP(w, r)
		if m.excludeStatic {
			var ext = strings.ToLower(filepath.Ext(r.URL.Path))
			if ext != "" {
				return
			}
		}
		var durationAttr = slog.Duration("duration", time.Since(start))
		var statusAttr slog.Attr = slog.Int("status", 200)
		var logLevel slog.Level = slog.LevelInfo
		var attrs = []slog.Attr{hostAttr, userAgentAttr, methodAttr, urlAttr, statusAttr, durationAttr}
		if writer, ok := w.(Writer); ok {
			if writer.Status() != 0 {
				var index = slices.IndexFunc(attrs, func(a slog.Attr) bool {
					return a.Key == "status"
				})
				if index != -1 {
					attrs[index] = slog.Int("status", writer.Status())
				}
			}
			if writer.Status() >= 400 && writer.Status() < 500 {
				logLevel = slog.LevelWarn
			} else if writer.Status() >= 500 {
				logLevel = slog.LevelError
			}
			var err = writer.Error()
			if err != "" {
				attrs = append(attrs, slog.String("error", err))
			}
		}
		m.log.LogAttrs(r.Context(), logLevel, "request", attrs...)
	})
}
