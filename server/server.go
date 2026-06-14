package server

import (
	"compress/gzip"
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/KrainovSD/go-packages/helpers"
	"github.com/KrainovSD/go-packages/logs"
	"github.com/KrainovSD/go-packages/metrics"
	"github.com/KrainovSD/go-packages/traces"
	"github.com/KrainovSD/go-packages/web"
)

type Server struct {
	Logger         *slog.Logger
	Traces         *traces.TracesProvider
	Metrics        *metrics.MetricsProvider
	StaticMux      *http.ServeMux
	ApiMux         *http.ServeMux
	ServerHandler  *http.ServeMux
	StartupCtx     context.Context
	ShutdownWait   *sync.WaitGroup
	ShutdownCtx    context.Context
	instance       *http.Server
	cleanups       []func(ctx context.Context)
	shutdownCancel context.CancelFunc
}

type ServerOptions struct {
	Port            int
	Pprof           bool
	StartupTime     time.Duration
	ApiPrefix       []string
	StaticPrefix    []string
	LogLevel        slog.Level
	LogColor        bool
	LogTraceIdKey   string
	ServiceName     string
	OtlpExporterUrl string
	OtlpProtocol    string // http, grpc
	CompressRequest bool
}

func (o *ServerOptions) SetDefaults() {
	if o.Port == 0 {
		o.Port = 3000
	}
	if o.StartupTime == 0 {
		o.StartupTime = 1 * time.Minute
	}
	o.OtlpProtocol = strings.ToLower(o.OtlpProtocol)
	if o.ServiceName == "" {
		o.ServiceName = "unknown_golang"
	}

}

func Create(opts *ServerOptions) *Server {
	opts.SetDefaults()
	if opts.Pprof {
		go startPprof()
	}

	var cleanups = make([]func(ctx context.Context), 0, 10)
	var startupCtx, startupCancel = context.WithTimeout(context.Background(), opts.StartupTime)
	cleanups = append(cleanups, func(ctx context.Context) {
		startupCancel()
	})

	var logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level:     opts.LogLevel,
		AddSource: false,
	}))
	var tracesProvider = traces.CreateTracesProvider(startupCtx, traces.TraceOptions{
		Url:      opts.OtlpExporterUrl,
		Protocol: opts.OtlpProtocol,
		Service:  opts.ServiceName,
		Logger:   logger,
	})
	var metricsProvider = metrics.CreateMetricsProvider(&metrics.MetricsProviderOpts{
		Service: opts.ServiceName,
		Logger:  logger,
	})
	cleanups = append(cleanups, tracesProvider.Close)
	if !opts.LogColor {
		logger = slog.New(logs.NewTraceHandler(&logs.TraceHandlerOptions{
			Handler: slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level:     opts.LogLevel,
				AddSource: false,
			}),
			TraceProvider: tracesProvider,
			Key:           helpers.StrToPtr(opts.LogTraceIdKey),
		}))
	} else {
		logger = slog.New(logs.NewTraceHandler(&logs.TraceHandlerOptions{
			Handler: logs.NewFormatHandler(os.Stdout, &logs.FormatHandlerOptions{
				Colors: true,
				Level:  opts.LogLevel,
			}),
			TraceProvider: tracesProvider,
			Key:           helpers.StrToPtr(opts.LogTraceIdKey),
		}))
	}

	var writerMiddleware = web.WriterMiddlewareCreate(&web.WriterMiddlewareOptions{
		Gzip:      opts.CompressRequest,
		GzipLevel: gzip.DefaultCompression,
	})
	var tracesMiddleware = traces.MiddlewareCreate(&traces.MiddlewareOptions{
		Traces:        tracesProvider,
		ExcludeStatic: true,
	})
	var metricsMiddleware = metrics.MiddlewareCreate(metrics.MiddlewareOptions{
		Metrics: metricsProvider,
	})
	var loggerMiddleware = logs.MiddlewareCreate(&logs.MiddlewareOptions{
		Log:           logger,
		ExcludeStatic: true,
	})
	var apiMux = http.NewServeMux()
	var apiHandler = http.Handler(apiMux)
	apiHandler = loggerMiddleware.Register(apiHandler)
	apiHandler = metricsMiddleware.Register(apiHandler)
	apiHandler = tracesMiddleware.Register(apiHandler)
	apiHandler = writerMiddleware.Register(apiHandler)
	var staticMux = http.NewServeMux()
	var staticHandler = http.Handler(staticMux)
	staticHandler = writerMiddleware.Register(staticHandler)

	var serverHandler = http.NewServeMux()
	for _, prefix := range opts.ApiPrefix {
		serverHandler.Handle(prefix, apiHandler)
	}
	for _, prefix := range opts.StaticPrefix {
		serverHandler.Handle(prefix, staticHandler)
	}

	var shutdownCtx, shutdownCancel = context.WithCancel(context.Background())
	cleanups = append(cleanups, func(ctx context.Context) {
		shutdownCancel()
	})
	var shutdownWait sync.WaitGroup

	return &Server{
		Logger:        logger,
		Traces:        tracesProvider,
		Metrics:       metricsProvider,
		ApiMux:        apiMux,
		StaticMux:     staticMux,
		ServerHandler: serverHandler,
		StartupCtx:    startupCtx,
		ShutdownWait:  &shutdownWait,
		ShutdownCtx:   shutdownCtx,
		instance: &http.Server{
			Addr:    ":" + strconv.Itoa(opts.Port),
			Handler: serverHandler,
		},
		cleanups:       cleanups,
		shutdownCancel: shutdownCancel,
	}
}

func startPprof() {
	runtime.SetBlockProfileRate(1)
	runtime.SetMutexProfileFraction(1)

	var mux = http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
	mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/block", pprof.Handler("block"))
	mux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	mux.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))

	var server = &http.Server{
		Addr:    "localhost:6060",
		Handler: mux,
	}
	log.Println("pprof listening on", server.Addr)
	log.Println(server.ListenAndServe())
}

func (s *Server) AppendCleanup(cleanup func(ctx context.Context)) {
	s.cleanups = append(s.cleanups, cleanup)
}

func (s *Server) Serve() {
	var errChan = make(chan error, 1)
	go func() {
		fmt.Println("Starting Server on " + s.instance.Addr)
		errChan <- s.instance.ListenAndServe()
	}()
	var signalCtx, stopSignal = signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignal()

	select {
	case err, ok := <-errChan:
		if !ok {
			s.Logger.Error("error channel closed")
		} else {
			s.Logger.Error("server error", "error", err.Error())
		}
	case <-signalCtx.Done():
		s.Logger.Info("signal for shutdown received")
	}
	s.close()
}

func (s *Server) close() {
	s.Logger.Info("shutdown server started")
	var ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var err error
	if err = s.instance.Shutdown(ctx); err != nil {
		s.Logger.Error("shutdown server failed", "error", err.Error())
		s.instance.Close()
	}
	s.shutdownCancel()
	s.ShutdownWait.Wait()
	for i := len(s.cleanups) - 1; i >= 0; i-- {
		s.cleanups[i](ctx)
	}
	s.Logger.Info("shutdown server finished")
}
