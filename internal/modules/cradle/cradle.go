package cradle

import (
	"log/slog"

	"github.com/KrainovSD/go-packages/api"
	"github.com/KrainovSD/go-packages/internal/config"
	"github.com/KrainovSD/go-packages/metrics"
	"github.com/KrainovSD/go-packages/traces"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Cradle struct {
	Api     *api.Client
	Log     *slog.Logger
	Conf    *config.Config
	Traces  *traces.Provider
	Metrics *metrics.MetricsProvider
	Db      *pgxpool.Pool
	Redis   redis.UniversalClient
	Queue   *kafka.Producer
}
