package app

import (
	"log/slog"
	"net/http"
	"time"
)

type Config struct {
	ServiceName     string
	ServiceVersion  string
	StartupTimeout  time.Duration
	ShutdownTimeout time.Duration
	Server          *ServerConfig
	Observability   *ObservabilityConfig
}

func (config *Config) setDefaults() {
	if config.ServiceName == "" {
		config.ServiceName = "ksd-golang-app"
	}
	if config.ServiceVersion == "" {
		config.ServiceVersion = "0.0.1"
	}
	if config.StartupTimeout == 0 {
		config.StartupTimeout = 30 * time.Second
	}
	if config.ShutdownTimeout == 0 {
		config.ShutdownTimeout = 20 * time.Second
	}
	if config.Server == nil {
		config.Server = &ServerConfig{}
	}
	config.Server.setDefaults()
	if config.Observability == nil {
		config.Observability = &ObservabilityConfig{}
	}
	config.Observability.setDefaults()
}

type ServerConfig struct {
	Port              int
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	MaxHeaderBytes    int
	CompressRequest   bool
	ShouldCompress    func(w http.ResponseWriter) bool
	BodySizeLimit     int64
}

func (config *ServerConfig) setDefaults() {
	if config.Port == 0 {
		config.Port = 3000
	}
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 5 * time.Second
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = 10 * time.Second
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = 120 * time.Second
	}
	if config.ReadHeaderTimeout == 0 {
		config.ReadHeaderTimeout = 2 * time.Second
	}
	if config.MaxHeaderBytes == 0 {
		config.MaxHeaderBytes = 1 << 20
	}
	if config.BodySizeLimit == 0 {
		config.BodySizeLimit = 5 << 20
	}
}

type ObservabilityConfig struct {
	LogLevel        slog.Level
	LogColor        bool
	LogTraceIDKey   string
	OtlpExporterURL string
	OtlpProtocol    string
	Pprof           bool
}

func (config *ObservabilityConfig) setDefaults() {
	if config.LogTraceIDKey == "" {
		config.LogTraceIDKey = "traceId"
	}
	if config.OtlpProtocol == "" {
		config.OtlpProtocol = "grpc"
	}
}
