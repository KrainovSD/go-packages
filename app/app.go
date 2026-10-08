package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/KrainovSD/go-packages/helpers"
	"github.com/KrainovSD/go-packages/logs"
	"github.com/KrainovSD/go-packages/metrics"
	"github.com/KrainovSD/go-packages/traces"
	"github.com/KrainovSD/go-packages/web"
)

type App struct {
	Logger               *slog.Logger
	Traces               *traces.Provider
	Metrics              *metrics.Provider
	BgWorker             *BackgroundWorker
	mux                  *Mux
	hooks                *Hooks
	server               *http.Server
	config               *Config
	shutdownSignal       context.Context
	cancelShutdownSignal func()
}

func New(config *Config) *App {
	config.setDefaults()
	if config.Observability.Pprof {
		go startPprof(config.Observability.PprofPort)
	}
	var startupCtx, cancelStartupCtx = context.WithTimeout(context.Background(), config.StartupTimeout)
	defer cancelStartupCtx()
	var logger *slog.Logger = config.Observability.Logger
	if config.Observability.Logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level:     config.Observability.LogLevel,
			AddSource: false,
		}))
	}
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
	if config.Observability.Logger == nil {
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
	}
	var hooks = newHooks()
	var mux = &Mux{
		mux: &http.ServeMux{},
		middlewares: []MuxMiddleware{
			{
				ID: web.WriterMiddlewareID,
				Fn: web.NewWriterMiddleware(&web.WriterMiddlewareOptions{
					Compress:        config.Server.CompressResponse,
					CompressLevel:   config.Server.CompressResponseLevel,
					CompressMinSize: config.Server.CompressResponseMinSize,
					ShouldCompress:  config.Server.ShouldCompress,
				}),
			},
			{
				ID: traces.MiddlewareID,
				Fn: traces.NewMiddleware(&traces.MiddlewareOptions{
					Traces: traceProvider,
					StaticClassifier: &web.StaticClassifierOptions{
						Enabled:           true,
						DynamicExtensions: config.Observability.DynamicExtensions,
						Custom:            config.Observability.IsStatic,
					},
				}),
			},
			{
				ID: metrics.MiddlewareID,
				Fn: metrics.NewMiddleware(&metrics.MiddlewareOptions{
					Metrics: metricProvider,
				}),
			},
			{
				ID: logs.MiddlewareID,
				Fn: logs.NewMiddleware(&logs.MiddlewareOptions{
					Log: logger,
					StaticClassifier: &web.StaticClassifierOptions{
						Enabled:           true,
						DynamicExtensions: config.Observability.DynamicExtensions,
						Custom:            config.Observability.IsStatic,
					},
				}),
			},
			{
				ID: web.SizeLimitMiddlewareID,
				Fn: web.NewSizeLimitMiddleware(config.Server.BodySizeLimit),
			},
			{
				ID: web.GoalkeeperMiddlewareID,
				Fn: web.NewGoalkeeperMiddleware(),
			},
		},
	}
	var shutdownSignal, cancelShutdownSignal = context.WithCancel(context.Background())
	return &App{
		Logger:   logger,
		Traces:   traceProvider,
		Metrics:  metricProvider,
		BgWorker: NewBackgroundWorker(config.BackgroundWorker.Capacity, config.BackgroundWorker.Workers, config.BackgroundWorker.OnPanic, logger),
		hooks:    hooks,
		config:   config,
		mux:      mux,
		server: &http.Server{
			Addr:              ":" + strconv.Itoa(config.Server.Port),
			Handler:           mux.mux,
			ReadTimeout:       config.Server.ReadTimeout,
			WriteTimeout:      config.Server.WriteTimeout,
			IdleTimeout:       config.Server.IdleTimeout,
			ReadHeaderTimeout: config.Server.ReadHeaderTimeout,
			MaxHeaderBytes:    config.Server.MaxHeaderBytes,
		},
		shutdownSignal:       shutdownSignal,
		cancelShutdownSignal: cancelShutdownSignal,
	}
}

func (a *App) ShutdownSignal() context.Context {
	return a.shutdownSignal
}

func (a *App) Mux() *Mux {
	return a.mux
}

func (a *App) Hooks() *Hooks {
	return a.hooks
}

func (a *App) Start() {
	var err = helpers.LimitWork(context.Background(), a.config.StartupTimeout, func(ctx context.Context) error {
		var err error
		for _, i := range a.hooks.onResourceInit {
			var initCtx, cancelInitCtx = context.WithTimeout(ctx, i.Duration)
			var cleanFn func(ctx context.Context)
			var start = time.Now()
			a.Logger.Info("startup resource init", "name", i.Name, "status", "started")
			if cleanFn, err = i.Fn(initCtx); err != nil {
				a.Logger.Error("startup resource init", "name", i.Name, "status", "finished", "duration", time.Since(start), "error", err)
				cancelInitCtx()
				return err
			}
			a.Logger.Info("startup resource init", "name", i.Name, "status", "finished", "duration", time.Since(start))
			cancelInitCtx()
			if cleanFn != nil {
				a.hooks.cleanup = append(a.hooks.cleanup, hooksCleanup{
					Duration: i.Duration,
					Fn:       cleanFn,
				})
			}
		}
		for _, fn := range a.hooks.onPreStartup {
			if err = fn(); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		panic(fmt.Errorf("startup: %w", err))
	}

	var errChan = make(chan error, 1)
	go func() {
		var listener, err = net.Listen("tcp", a.server.Addr)
		if err != nil {
			errChan <- err
			return
		}
		defer listener.Close()
		for _, fn := range a.hooks.onPostStartup {
			if err = fn(); err != nil {
				errChan <- err
			}
		}
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
	for _, preShutdownFn := range a.hooks.onPreShutdown {
		preShutdownFn()
	}

	if err = helpers.LimitWork(context.Background(), a.config.StartupTimeout, func(ctx context.Context) error {
		var err error
		var serverCtx, cancelServerCtx = context.WithTimeout(ctx, a.config.StartupTimeout*time.Duration(a.config.ShutdownServerBudgetRatio))
		defer cancelServerCtx()
		if err = a.server.Shutdown(serverCtx); err != nil {
			a.Logger.Error("shutdown server failed", "error", err.Error())
			a.server.Close()
		}
		a.cancelShutdownSignal()
		a.BgWorker.Stop()
		for _, c := range a.hooks.cleanup {
			var cleanCtx, cancelCleanCtx = context.WithTimeout(ctx, c.Duration)
			c.Fn(cleanCtx)
			cancelCleanCtx()
		}
		for _, postShutdownFn := range a.hooks.onPostShutdown {
			postShutdownFn()
		}
		return nil
	}); err != nil {
		panic(fmt.Errorf("shutdown: %w", err))
	}
}
