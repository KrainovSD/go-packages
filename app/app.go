package app

import (
	"compress/gzip"
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	"github.com/KrainovSD/go-packages/logs"
	"github.com/KrainovSD/go-packages/metrics"
	"github.com/KrainovSD/go-packages/traces"
	"github.com/KrainovSD/go-packages/web"
)

type App struct {
	Logger               *slog.Logger
	Traces               *traces.Provider
	Metrics              *metrics.Provider
	mux                  *GlobalMux
	hooks                *Hooks
	server               *http.Server
	config               *Config
	wg                   *sync.WaitGroup
	shutdownSignal       context.Context
	cancelShutdownSignal func()
}

func New(config *Config) *App {
	config.setDefaults()
	if config.Observability.Pprof {
		go startPprof()
	}
	var startupCtx, cancelStartupCtx = context.WithTimeout(context.Background(), config.StartupTimeout)
	defer cancelStartupCtx()
	var logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level:     config.Observability.LogLevel,
		AddSource: false,
	}))
	var traceProvider = traces.NewProvider(startupCtx, &traces.ProviderOptions{
		Url:      config.Observability.OtlpExporterURL,
		Protocol: config.Observability.OtlpProtocol,
		Service:  config.ServiceName,
		Logger:   logger,
	})
	var metricProvider = metrics.NewProvider(&metrics.ProviderOptions{
		Service: config.ServiceName,
		Logger:  logger,
	})
	if !config.Observability.LogColor {
		logger = slog.New(logs.NewTraceHandler(&logs.TraceHandlerOptions{
			Handler: slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level:     config.Observability.LogLevel,
				AddSource: false,
			}),
			TraceProvider: traceProvider,
			Key:           config.Observability.LogTraceIDKey,
		}))
	} else {
		logger = slog.New(logs.NewTraceHandler(&logs.TraceHandlerOptions{
			Handler: logs.NewFormatHandler(os.Stdout, &logs.FormatHandlerOptions{
				Level:  config.Observability.LogLevel,
				Colors: true,
			}),
			TraceProvider: traceProvider,
			Key:           config.Observability.LogTraceIDKey,
		}))
	}
	var writerMiddleware = web.NewWriterMiddleware(&web.WriterMiddlewareOptions{
		Compress:       config.Server.CompressRequest,
		CompressLevel:  gzip.DefaultCompression,
		ShouldCompress: config.Server.ShouldCompress,
	})
	var tracesMiddleware = traces.NewMiddleware(&traces.MiddlewareOptions{
		Traces:        traceProvider,
		ExcludeStatic: true,
	})
	var metricsMiddleware = metrics.NewMiddleware(&metrics.MiddlewareOptions{
		Metrics: metricProvider,
	})
	var loggerMiddleware = logs.NewMiddleware(&logs.MiddlewareOptions{
		Log:           logger,
		ExcludeStatic: true,
	})
	var goalkeeperMiddleware = web.NewGoalkeeperMiddleware()
	var middlewares = make(map[string][]MuxMiddleware, 2)
	middlewares["base"] = []MuxMiddleware{writerMiddleware, goalkeeperMiddleware}
	middlewares["full"] = []MuxMiddleware{writerMiddleware, tracesMiddleware, metricsMiddleware, loggerMiddleware, goalkeeperMiddleware}
	var hooks = newHooks()
	var mux = newMux(middlewares)
	var shutdownSignal, cancelShutdownSignal = context.WithCancel(context.Background())
	return &App{
		Logger:  logger,
		Traces:  traceProvider,
		Metrics: metricProvider,
		hooks:   hooks,
		config:  config,
		mux:     mux,
		server: &http.Server{
			Addr:              ":" + strconv.Itoa(config.Server.Port),
			Handler:           mux.mux,
			ReadTimeout:       config.Server.ReadTimeout,
			WriteTimeout:      config.Server.WriteTimeout,
			IdleTimeout:       config.Server.IdleTimeout,
			ReadHeaderTimeout: config.Server.ReadHeaderTimeout,
			MaxHeaderBytes:    config.Server.MaxHeaderBytes,
		},
		wg:                   &sync.WaitGroup{},
		shutdownSignal:       shutdownSignal,
		cancelShutdownSignal: cancelShutdownSignal,
	}
}

func (a *App) ShutdownWait() *sync.WaitGroup {
	return a.wg
}

func (a *App) ShutdownSignal() context.Context {
	return a.shutdownSignal
}

func (a *App) Mux() *GlobalMux {
	return a.mux
}

func (a *App) Hooks() *Hooks {
	return a.hooks
}

func (a *App) Start() {
	a.preStartup()
	var errChan = make(chan error, 1)
	go func() {
		var listener, err = net.Listen("tcp", a.server.Addr)
		defer listener.Close()
		if err != nil {
			errChan <- err
			return
		}
		a.postStartup()
		errChan <- a.server.Serve(listener)
	}()

	var signalCtx, cancelSignal = signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancelSignal()
	select {
	case err, ok := <-errChan:
		if !ok {
			a.Logger.Error("error channel closed")
		} else {
			a.Logger.Error("server error", "error", err.Error())
		}
	case <-signalCtx.Done():
		a.Logger.Info("signal for shutdown received")
	}

	a.preShutdown()
	var shutdownCtx, cancelShutdownCtx = context.WithTimeout(context.Background(), a.config.ShutdownTimeout)
	defer cancelShutdownCtx()
	var err error
	if err = a.server.Shutdown(shutdownCtx); err != nil {
		a.Logger.Error("shutdown server failed", "error", err.Error())
		a.server.Close()
	}
	a.cancelShutdownSignal()
	a.wg.Wait()
	var cleanup = a.hooks.cleanup
	for _, clean := range cleanup {
		var ctx, cancel = context.WithTimeout(context.Background(), a.config.ShutdownTimeout)
		clean(ctx)
		cancel()
	}
	a.postShutdown()
}

func (a *App) preStartup() {
	var err error
	var startupCtx, cancelStartupCtx = context.WithTimeout(context.Background(), a.config.StartupTimeout)
	defer cancelStartupCtx()
	for _, fn := range a.hooks.onPreStartup {
		var clean hooksCleanup
		if clean, err = fn(startupCtx); err != nil {
			panic(err)
		}
		if clean != nil {
			a.hooks.cleanup = append(a.hooks.cleanup, clean)
		}
	}
}

func (a *App) postStartup() {
	var err error
	for _, fn := range a.hooks.onPostStartup {
		if err = fn(a.shutdownSignal, a.wg); err != nil {
			panic(err)
		}
	}
}

func (a *App) preShutdown() {
	for _, fn := range a.hooks.onPreShutdown {
		var shutdownCtx, cancelShutdownCtx = context.WithTimeout(context.Background(), a.config.ShutdownTimeout)
		fn(shutdownCtx)
		cancelShutdownCtx()
	}
}

func (a *App) postShutdown() {
	for _, fn := range a.hooks.onPostShutdown {
		var shutdownCtx, cancelShutdownCtx = context.WithTimeout(context.Background(), a.config.ShutdownTimeout)
		fn(shutdownCtx)
		cancelShutdownCtx()
	}
}
