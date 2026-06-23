package metrics

import (
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type Provider struct {
	registry          *prometheus.Registry
	handler           http.Handler
	activeConnections prometheus.Gauge
}

type ProviderOptions struct {
	Service string
	Logger  *slog.Logger
}

func NewProvider(opts *ProviderOptions) *Provider {
	if opts.Service == "" {
		opts.Service = "unknown_golang"
	}

	var activeConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: opts.Service,
		Name:      "http_connections_active_total",
		Help:      "Number of active http connections in one time",
	})

	var registry = prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector())
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(collectors.NewBuildInfoCollector())
	registry.MustRegister(activeConnections)

	var exporter *otelprom.Exporter
	var err error
	if exporter, err = otelprom.New(otelprom.WithRegisterer(registry)); err != nil {
		opts.Logger.Error("create metrics provider", "error", err.Error())
		return &Provider{
			registry:          nil,
			handler:           nil,
			activeConnections: nil,
		}
	}
	var meterProvider = metric.NewMeterProvider(
		metric.WithReader(exporter),
		metric.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(opts.Service),
		)),
	)
	otel.SetMeterProvider(meterProvider)

	var handler http.Handler = promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		EnableOpenMetrics: false,
	})

	return &Provider{
		registry:          registry,
		handler:           handler,
		activeConnections: activeConnections,
	}
}

func (p *Provider) Exist() bool {
	return p.registry != nil
}

func (p *Provider) Register(cs ...prometheus.Collector) {
	if p.registry == nil {
		return
	}
	p.registry.MustRegister(cs...)
}

func (p *Provider) IncreaseConnectionsHTTP() {
	if p.registry == nil {
		return
	}
	p.activeConnections.Inc()
}

func (p *Provider) DecreaseConnectionsHTTP() {
	if p.registry == nil {
		return
	}
	p.activeConnections.Dec()
}

func (p *Provider) Handle() http.Handler {
	if p.registry == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
			return
		})
	}
	return p.handler
}
