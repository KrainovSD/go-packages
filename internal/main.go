package main

import (
	"context"
	"fmt"

	"github.com/KrainovSD/go-packages/api"
	"github.com/KrainovSD/go-packages/app"
	"github.com/KrainovSD/go-packages/internal/config"
	"github.com/KrainovSD/go-packages/internal/internal/middlewares"
	"github.com/KrainovSD/go-packages/internal/internal/router"
	"github.com/KrainovSD/go-packages/internal/modules/cradle"
	"github.com/KrainovSD/go-packages/internal/modules/pg"
	"github.com/KrainovSD/go-packages/queue"
	"github.com/KrainovSD/go-packages/storage"
	"github.com/KrainovSD/go-packages/web"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"time"
)

func main() {
	var err error
	var conf *config.Config = config.Create()
	if err = conf.Validate(); err != nil {
		panic(err.Error())
	}
	var server = app.New(&app.Config{
		ServiceName:     "test-service",
		ServiceVersion:  "0.0.0",
		StartupTimeout:  30 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		Server: &app.ServerConfig{
			Port:                    conf.Default.System.Port,
			ReadTimeout:             5 * time.Second,
			WriteTimeout:            10 * time.Second,
			IdleTimeout:             120 * time.Second,
			ReadHeaderTimeout:       2 * time.Second,
			MaxHeaderBytes:          1 << 20,
			CompressResponse:        conf.Default.System.CompressResponse,
			CompressResponseLevel:   conf.Default.System.CompressResponseLevel,
			CompressResponseMinSize: conf.Default.System.CompressResponseMinSize,
			ShouldCompress:          nil,
			BodySizeLimit:           1 << 20,
		},
		Observability: &app.ObservabilityConfig{
			LogLevel:        conf.Default.System.LogLevel,
			LogColor:        conf.Default.System.LogColor,
			LogTraceIDKey:   "traceId",
			OtlpExporterURL: conf.Default.System.OtlpExporterURL,
			OtlpProtocol:    conf.Default.System.OtlpExporterProtocol,
			Pprof:           false,
		},
	})
	var mux = server.Mux().Clone()
	mux.PushMiddlewares([]app.MuxMiddleware{
		{
			ID: "auth",
			Fn: middlewares.NewAuth(middlewares.AuthOptions{Strict: true}),
		},
		{
			ID: "logger",
			Fn: middlewares.NewLogger(middlewares.LoggerOptions{Strict: true}),
		},
	})
	var staticMux = mux.CloneWith(
		mux.SelectMiddlewares([]string{
			web.WriterMiddlewareID,
			web.SizeLimitMiddlewareID,
			web.GoalkeeperMiddlewareID,
		}),
	)
	var cradle = &cradle.Cradle{
		Log:            server.Logger,
		Conf:           conf,
		Traces:         server.Traces,
		Metrics:        server.Metrics,
		Wg:             server.BgWorker,
		ShutdownSignal: server.ShutdownSignal(),
	}

	server.Hooks().OnResourceInit("postgres", 10*time.Second, func(ctx context.Context) (func(context.Context), error) {
		var db *pgxpool.Pool
		if db, err = storage.NewPostgres(ctx, &storage.PostgresOptions{Connection: conf.Default.Postgres.Connection, Tracing: server.Traces.Exist()}); err != nil {
			return nil, err
		}
		if err = pg.Init(db); err != nil {
			return nil, err
		}
		cradle.Db = db
		return func(ctx context.Context) {
			db.Close()
		}, nil
	})
	server.Hooks().OnResourceInit("kafka", 10*time.Second, func(ctx context.Context) (func(context.Context), error) {
		var kq *kafka.Producer
		if kq, err = queue.NewProducer(ctx, &queue.ProducerOptions{
			Servers: conf.Default.Kafka.Servers,
			SecurityOptions: queue.SecurityOptions{
				SecurityProtocol: conf.Default.Kafka.SecurityProtocol,
				User:             conf.Default.Kafka.User,
				Password:         conf.Default.Kafka.Password,
				Mechanism:        conf.Default.Kafka.Mechanism,
				SslCaLocation:    conf.Default.Kafka.SslCaLocation,
				SslLocation:      conf.Default.Kafka.SslLocation,
				SslKeyLocation:   conf.Default.Kafka.SslKeyLocation,
				KeytabPath:       conf.Default.Kafka.KeytabPath,
				Principal:        conf.Default.Kafka.Principal,
			},
		}); err != nil {
			return nil, err
		}
		cradle.Queue = kq
		return func(ctx context.Context) {
			kq.Close()
		}, nil
	})
	server.Hooks().OnResourceInit("redis", 5*time.Second, func(ctx context.Context) (func(context.Context), error) {
		var red redis.UniversalClient
		if red, err = storage.NewRedis(ctx, &storage.RedisOptions{
			Addresses:  conf.Default.Redis.Addresses,
			Username:   conf.Default.Redis.Username,
			Password:   conf.Default.Redis.Password,
			Mode:       conf.Default.Redis.Mode,
			MasterName: conf.Default.Redis.Master,
			DB:         conf.Default.Redis.Db,
			Tracing:    server.Traces.Exist(),
			Metrics:    server.Traces.Exist(),
		}); err != nil {
			return nil, err
		}
		cradle.Redis = red
		return func(ctx context.Context) {
			red.Close()
		}, nil
	})
	server.Hooks().OnResourceInit("api-client", 5*time.Second, func(ctx context.Context) (func(context.Context), error) {
		var fetch *api.Client
		if fetch, err = api.NewClient(&api.ClientOptions{Tracing: server.Traces.Exist()}); err != nil {
			return nil, err
		}
		cradle.Api = fetch
		return func(ctx context.Context) {
			fetch.Close()
		}, nil
	})

	server.Hooks().OnPreStartup(func() error {

		if err = router.InitRoutes(&router.RoutesOptions{
			M:      server.Mux(),
			SM:     staticMux,
			TM:     mux,
			Cradle: cradle,
		}); err != nil {
			return err
		}
		return nil
	})
	server.Hooks().OnPostStartup(func() error {
		fmt.Println("Server started on", conf.Default.System.Port)
		return nil
	})
	server.Start()
}
