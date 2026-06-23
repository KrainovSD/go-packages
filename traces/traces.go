package traces

import (
	"context"
	"log/slog"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"go.opentelemetry.io/otel/trace"
)

type Provider struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
}

type ProviderOptions struct {
	Url      string
	Protocol string
	Service  string
	Logger   *slog.Logger
}

func CreateTracesProvider(ctx context.Context, opts ProviderOptions) *Provider {
	var traceProvider Provider
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
	return &Provider{
		provider: provider,
		tracer:   otel.Tracer(opts.Service),
	}
}

func (t *Provider) Exist() bool {
	return t.provider != nil
}

func (t *Provider) GetTracer() trace.Tracer {
	return t.tracer
}

func (t *Provider) Close(ctx context.Context) {
	if !t.Exist() {
		return
	}
	t.provider.Shutdown(ctx)
}

func (t *Provider) Start(rootCtx context.Context, name string, attributes ...attribute.KeyValue) (context.Context, func()) {
	if !t.Exist() {
		return rootCtx, func() {}
	}
	var ctx, span = t.tracer.Start(rootCtx, name, trace.WithAttributes(attributes...))
	return ctx, func() {
		span.End()
	}
}

func (t *Provider) StartAsync(parentCtx context.Context, name string, attributes ...attribute.KeyValue) (context.Context, func()) {
	if !t.Exist() {
		return context.Background(), func() {}
	}
	var parentSpan = trace.SpanFromContext(parentCtx).SpanContext()
	if !parentSpan.IsValid() {
		return context.Background(), func() {}
	}
	var attrs = make([]attribute.KeyValue, 0, 2+len(attributes))
	attrs = append(attrs, attribute.String("parent.trace_id", parentSpan.TraceID().String()))
	attrs = append(attrs, attribute.String("parent.span_id", parentSpan.SpanID().String()))
	if len(attributes) > 0 {
		attrs = append(attrs, attributes...)
	}
	var asyncCtx, asyncSpan = t.tracer.Start(
		context.Background(),
		name,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithLinks(trace.Link{
			SpanContext: parentSpan,
		}),
		trace.WithAttributes(attrs...),
	)
	return asyncCtx, func() {
		asyncSpan.End()
	}
}

func (t *Provider) SetError(ctx context.Context, err error) {
	var span = trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}
	span.SetStatus(codes.Error, err.Error())
	span.RecordError(err)
}

func (t *Provider) SetAttributes(ctx context.Context, attributes ...attribute.KeyValue) {
	if len(attributes) == 0 {
		return
	}
	if !t.Exist() {
		return
	}
	var span = trace.SpanFromContext(ctx)
	if span == nil || !span.IsRecording() {
		return
	}
	span.SetAttributes(attributes...)
}

func (t *Provider) GetTraceId(ctx context.Context) *string {
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
