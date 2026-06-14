package traces

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"go.opentelemetry.io/otel/trace"
)

type TracesProvider struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
}

type TraceOptions struct {
	Url      string
	Protocol string
	Service  string
	Logger   *slog.Logger
}

func CreateTracesProvider(ctx context.Context, opts TraceOptions) *TracesProvider {
	var traceProvider TracesProvider
	if opts.Url == "" {
		opts.Logger.LogAttrs(context.Background(), slog.LevelWarn, "otlp", slog.String("error", "required otlp exporter url env for traces"))
		return &traceProvider
	}

	var err error
	var exporter *otlptrace.Exporter
	if opts.Protocol == "http" {
		if exporter, err = otlptracehttp.New(ctx,
			otlptracehttp.WithEndpoint(strings.TrimPrefix(strings.TrimPrefix(opts.Url, "http://"), "https://")),
			otlptracehttp.WithInsecure(),
		); err != nil {
			opts.Logger.LogAttrs(context.Background(), slog.LevelWarn, "otlp", slog.String("action", "create exporter"), slog.String("error", err.Error()))
			return &traceProvider
		}
	} else {
		if exporter, err = otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(strings.TrimPrefix(strings.TrimPrefix(opts.Url, "http://"), "https://")),
			otlptracegrpc.WithInsecure(),
		); err != nil {
			opts.Logger.LogAttrs(context.Background(), slog.LevelWarn, "otlp", slog.String("action", "create exporter"), slog.String("error", err.Error()))
			return &traceProvider
		}
	}

	var meta *resource.Resource
	if opts.Service == "" {
		opts.Service = "unknown_golang"
	}
	if meta, err = resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(opts.Service),
		),
	); err != nil {
		opts.Logger.LogAttrs(context.Background(), slog.LevelWarn, "otlp", slog.String("action", "create meta"), slog.String("error", err.Error()))

		return &traceProvider
	}

	var provider *sdktrace.TracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(meta),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)
	return &TracesProvider{
		provider: provider,
		tracer:   otel.Tracer(opts.Service),
	}
}

func (t *TracesProvider) Exist() bool {
	return t.provider != nil
}

func (t *TracesProvider) Close(ctx context.Context) {
	if !t.Exist() {
		return
	}
	t.provider.Shutdown(ctx)
}

func (t *TracesProvider) StartRequest(r *http.Request) (context.Context, func()) {
	if !t.Exist() {
		return r.Context(), func() {}
	}

	var ctx = otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	var spanName = fmt.Sprintf("HTTP %s %s", r.Method, r.URL.Path)
	var span trace.Span
	ctx, span = t.tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(
		attribute.String("http.method", r.Method),
		attribute.String("http.url", r.URL.String()),
		attribute.String("http.scheme", r.URL.Scheme),
		attribute.String("http.host", r.Host),
		attribute.String("http.remote", r.RemoteAddr),
		attribute.String("http.user_agent", r.UserAgent()),
	))

	return ctx, func() {
		span.End()
	}
}

func (t *TracesProvider) Start(rootCtx context.Context, name string) (context.Context, func()) {
	if !t.Exist() {
		return rootCtx, func() {}
	}

	var span trace.Span
	var ctx context.Context
	ctx, span = t.tracer.Start(rootCtx, name)
	return ctx, func() {
		span.End()
	}
}

func (t *TracesProvider) GetTraceId(ctx context.Context) *string {
	if !t.Exist() {
		return nil
	}
	var spanContext = trace.SpanFromContext(ctx).SpanContext()
	if !spanContext.IsValid() {
		return nil
	}
	var traceId = spanContext.TraceID().String()
	if traceId == "" {
		return nil
	}
	return &traceId
}

func (t *TracesProvider) SetAttributes(ctx context.Context, keyValues ...any) {
	if !t.Exist() {
		return
	}

	if len(keyValues) == 0 {
		return
	}

	var span = trace.SpanFromContext(ctx)
	if span == nil || !span.IsRecording() {
		return
	}

	var attrs []attribute.KeyValue
	for i := 0; i < len(keyValues); i += 2 {
		var key, ok = keyValues[i].(string)
		if !ok {
			continue
		}
		var value = keyValues[i+1]
		switch v := value.(type) {
		case string:
			attrs = append(attrs, attribute.String(key, v))
		case int:
			attrs = append(attrs, attribute.Int(key, v))
		case int64:
			attrs = append(attrs, attribute.Int64(key, v))
		case float64:
			attrs = append(attrs, attribute.Float64(key, v))
		case bool:
			attrs = append(attrs, attribute.Bool(key, v))
		}
	}
	span.SetAttributes(attrs...)

}
