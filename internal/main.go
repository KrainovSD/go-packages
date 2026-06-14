package main

import (
	"context"

	"github.com/KrainovSD/go-packages/api"
	"github.com/KrainovSD/go-packages/internal/config"
	"github.com/KrainovSD/go-packages/internal/internal/router"
	"github.com/KrainovSD/go-packages/internal/modules/cradle"
	"github.com/KrainovSD/go-packages/internal/modules/pg"
	"github.com/KrainovSD/go-packages/queue"
	"github.com/KrainovSD/go-packages/server"
	"github.com/KrainovSD/go-packages/storage"

	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {

	var err error
	var conf *config.Config = config.Create()
	if err = conf.Validate(); err != nil {
		panic(err.Error())
	}
	var app = server.Create(&server.ServerOptions{
		Port:            conf.PORT,
		Pprof:           false,
		StartupTime:     1 * time.Minute,
		ApiPrefix:       []string{"/api/"},
		StaticPrefix:    []string{"/"},
		LogLevel:        conf.LOG_LEVEL,
		LogColor:        conf.LOG_COLOR,
		LogTraceIdKey:   "TraceId",
		ServiceName:     conf.SERVICE_NAME,
		OtlpExporterUrl: conf.OTLP_EXPORTER_URL,
		OtlpProtocol:    conf.OTLP_PROTOCOL,
		CompressRequest: conf.COMPRESS_REQUEST,
	})

	var db *pgxpool.Pool
	if db, err = storage.CreatePg(context.Background(), &storage.CreatePgOptions{Connection: conf.PG_URL, Tracing: app.Traces.Exist()}); err != nil {
		panic(err.Error())
	}
	if err = pg.Init(db); err != nil {
		panic(err.Error())
	}
	app.AppendCleanup(func(ctx context.Context) {
		db.Close()
	})
	var q *kafka.Producer
	if q, err = queue.CreateProducer(context.Background(), &queue.CreateProducerOptions{
		Servers: conf.KAFKA_SERVERS,
		SecurityOptions: queue.SecurityOptions{
			SecurityProtocol: conf.KAFKA_PROTOCOL,
			User:             conf.KAFKA_USER,
			Password:         conf.KAFKA_PASSWORD,
			Mechanism:        conf.KAFKA_MECHANISM,
		},
	}); err != nil {
		panic(err.Error())
	}
	app.AppendCleanup(func(ctx context.Context) {
		q.Close()
	})
	var redisClient redis.UniversalClient
	if redisClient, err = storage.CreateRedis(context.Background(), &storage.CreateRedisOptions{
		Addresses: conf.REDIS_ADDRESSES,
		Password:  conf.REDIS_PASSWORD,
		Tracing:   app.Traces.Exist(),
		Metrics:   app.Traces.Exist(),
	}); err != nil {
		panic(err.Error())
	}
	app.AppendCleanup(func(ctx context.Context) {
		redisClient.Close()
	})
	var apiClient *api.Client
	if apiClient, err = api.CreateClient(api.ClientOptions{Tracing: app.Traces.Exist()}); err != nil {
		panic(err.Error())
	}
	var c *cradle.Cradle = &cradle.Cradle{
		Api:     apiClient,
		Log:     app.Logger,
		Conf:    conf,
		Traces:  app.Traces,
		Metrics: app.Metrics,
		Db:      db,
		Redis:   redisClient,
		Queue:   q,
	}
	err = router.InitRoutes(&router.RoutesOptions{
		M:      app.ApiMux,
		SM:     app.StaticMux,
		Cradle: c,
	})
	if err != nil {
		panic(err)
	}
	app.Serve()
}
