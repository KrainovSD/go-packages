package logs

import (
	"context"
	"log/slog"

	"github.com/KrainovSD/go-packages/traces"
)

type TraceHandlerOptions struct {
	Handler       slog.Handler
	TraceProvider *traces.Provider
	Key           string
}

type TraceHandler struct {
	handler       slog.Handler
	traceProvider *traces.Provider
	key           string
}

func NewTraceHandler(opts *TraceHandlerOptions) *TraceHandler {
	var key = "traceId"
	if opts.Key != "" {
		key = opts.Key
	}
	return &TraceHandler{handler: opts.Handler, traceProvider: opts.TraceProvider, key: key}
}

func (h *TraceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *TraceHandler) Handle(ctx context.Context, r slog.Record) error {
	var traceId = h.traceProvider.GetTraceID(ctx)
	if traceId != nil {
		r.AddAttrs(slog.String(h.key, *traceId))
	}
	return h.handler.Handle(ctx, r)
}

func (h *TraceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TraceHandler{handler: h.handler.WithAttrs(attrs), traceProvider: h.traceProvider, key: h.key}
}

func (h *TraceHandler) WithGroup(name string) slog.Handler {
	return &TraceHandler{handler: h.handler.WithGroup(name), traceProvider: h.traceProvider, key: h.key}
}
