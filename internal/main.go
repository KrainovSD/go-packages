package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/KrainovSD/go-packages/api"
	"github.com/KrainovSD/go-packages/app"
	"github.com/KrainovSD/go-packages/internal/config"
	"github.com/KrainovSD/go-packages/internal/internal/middlewares"
	"github.com/KrainovSD/go-packages/internal/internal/router"
	"github.com/KrainovSD/go-packages/internal/modules/cradle"
	"github.com/KrainovSD/go-packages/internal/modules/pg"
	"github.com/KrainovSD/go-packages/queue"
	"github.com/KrainovSD/go-packages/storage"
	"github.com/confluentinc/confluent-kafka-go/kafka"
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
		ShutdownTimeout: 20 * time.Second,
		Server: &app.ServerConfig{
			Port:              conf.Default.System.Port,
			ReadTimeout:       5 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       120 * time.Second,
			ReadHeaderTimeout: 2 * time.Second,
			MaxHeaderBytes:    1 << 20,
			CompressRequest:   conf.Default.System.CompressRequest,
			ShouldCompress:    nil,
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
	var db *pgxpool.Pool
	var kq *kafka.Producer
	var red redis.UniversalClient
	var fetch *api.Client
	server.Hooks().OnPreStartup(func(ctx context.Context) (func(ctx context.Context), error) {
		if db, err = storage.NewPostgres(ctx, &storage.PostgresOptions{Connection: conf.Default.Postgres.Connection, Tracing: server.Traces.Exist()}); err != nil {
			return nil, err
		}
		if err = pg.Init(db); err != nil {
			return nil, err
		}
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
		if fetch, err = api.NewClient(&api.ClientOptions{Tracing: server.Traces.Exist()}); err != nil {
			return nil, err
		}
		return func(ctx context.Context) {
			db.Close()
			kq.Close()
			red.Close()
			fetch.Close()
		}, nil
	})
	var crad *cradle.Cradle = &cradle.Cradle{
		Api:     fetch,
		Log:     server.Logger,
		Conf:    conf,
		Traces:  server.Traces,
		Metrics: server.Metrics,
		Db:      db,
		Redis:   red,
		Queue:   kq,
	}
	server.Mux().ExtendMiddlewareGroup(
		"full",
		"test",
		[]app.MuxMiddleware{
			middlewares.NewAuth(middlewares.AuthOptions{Strict: true}),
			middlewares.NewLogger(middlewares.LoggerOptions{Strict: true}),
		},
	)
	server.Hooks().OnPreStartup(func(ctx context.Context) (func(ctx context.Context), error) {
		err = router.InitRoutes(&router.RoutesOptions{
			M:      server.Mux().FullMiddlewares(),
			SM:     server.Mux().BaseMiddlewares(),
			TM:     server.Mux().WithMiddlewares("test"),
			Cradle: crad,
		})
		if err != nil {
			return nil, err
		}
		return nil, nil
	})
	server.Hooks().OnPostStartup(func(ctx context.Context, wg *sync.WaitGroup) error {
		fmt.Println("Server started on", conf.Default.System.Port)
		return nil
	})
	server.Start()
}
