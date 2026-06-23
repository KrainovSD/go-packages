package app

import (
	"log/slog"
	"os"
	"strings"

	"github.com/KrainovSD/go-packages/helpers"
)

type EnvConfig struct {
	Kafka     *EnvKafkaConfig
	Redis     *EnvRedisConfig
	Postgres  *EnvPostgresConfig
	ClickHouse *EnvClickHouseConfig
	System    *EnvSystemConfig
}

type EnvKafkaConfig struct {
	Servers          []string
	SecurityProtocol string
	Mechanism        string
	User             string
	Password         string
	SslCaLocation    string
	SslLocation      string
	SslKeyLocation   string
	KeytabPath       string
	Principal        string
}

type EnvRedisConfig struct {
	Addresses []string
	Username  string
	Password  string
	Mode      string
	Db        int
	Master    string
}

type EnvPostgresConfig struct {
	Connection string
}

type EnvClickHouseConfig struct {
	Connection string
}

type EnvSystemConfig struct {
	Port                 int
	LogLevel             slog.Level
	LogColor             bool
	CompressRequest      bool
	OtlpExporterURL      string
	OtlpExporterProtocol string
}

func NewEnvConfig(prefix string) *EnvConfig {
	var config = &EnvConfig{
		Kafka:     &EnvKafkaConfig{},
		Redis:     &EnvRedisConfig{},
		Postgres:  &EnvPostgresConfig{},
		ClickHouse: &EnvClickHouseConfig{},
		System:    &EnvSystemConfig{},
	}
	var prefixer = newEnvPrefixer(prefix)

	config.Kafka.Servers = helpers.ParseEnvSlice(prefixer("KAFKA_SERVERS"))
	config.Kafka.SecurityProtocol = prefixer("KAFKA_SECURITY_PROTOCOL")
	config.Kafka.Mechanism = prefixer("KAFKA_MECHANISM")
	config.Kafka.User = prefixer("KAFKA_USER")
	config.Kafka.Password = prefixer("KAFKA_PASSWORD")
	config.Kafka.SslCaLocation = prefixer("KAFKA_SSL_CA_LOCATION")
	config.Kafka.SslLocation = prefixer("KAFKA_SSL_LOCATION")
	config.Kafka.SslKeyLocation = prefixer("KAFKA_SSL_KEY_LOCATION")
	config.Kafka.KeytabPath = prefixer("KAFKA_KEYTAB_PATH")
	config.Kafka.Principal = prefixer("KAFKA_PRINCIPAL")

	config.Redis.Addresses = helpers.ParseEnvSlice(prefixer("REDIS_ADDRESSES"))
	config.Redis.Username = prefixer("REDIS_USERNAME")
	config.Redis.Password = prefixer("REDIS_PASSWORD")
	config.Redis.Mode = prefixer("REDIS_MODE")
	config.Redis.Db = 0
	var redisDb = helpers.ParseEnvInt(prefixer("REDIS_DB"))
	if redisDb != nil {
		config.Redis.Db = *redisDb
	}
	config.Redis.Master = prefixer("REDIS_MASTER")

	config.Postgres.Connection = prefixer("POSTGRES_CONNECTION")
	config.ClickHouse.Connection = prefixer("CLICKHOUSE_CONNECTION")

	config.System.Port = 3000
	var port = helpers.ParseEnvInt(prefixer("PORT"))
	if port != nil {
		config.System.Port = *port
	}
	var logLevel = strings.ToLower(prefixer("LOG_LEVEL"))
	switch logLevel {
	case "debug":
		config.System.LogLevel = slog.LevelDebug
	case "info":
		config.System.LogLevel = slog.LevelInfo
	case "warn":
		config.System.LogLevel = slog.LevelWarn
	case "error":
		config.System.LogLevel = slog.LevelError
	default:
		config.System.LogLevel = slog.LevelInfo

	}
	config.System.LogColor = helpers.ParseEnvBool(prefixer("LOG_COLOR"))
	config.System.CompressRequest = helpers.ParseEnvBool(prefixer("COMPRESS_REQUEST"))
	config.System.OtlpExporterURL = prefixer("OTLP_EXPORTER_URL")
	config.System.OtlpExporterProtocol = prefixer("OTLP_PROTOCOL")
	return config
}

func newEnvPrefixer(prefix string) func(name string) string {
	return func(name string) string {
		return os.Getenv(prefix + name)
	}
}
