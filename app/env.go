package app

import (
	"log/slog"
	"os"
	"strings"

	"github.com/KrainovSD/go-packages/helpers"
)

type EnvConfig struct {
	Kafka      *EnvKafkaConfig
	Redis      *EnvRedisConfig
	Postgres   *EnvPostgresConfig
	ClickHouse *EnvClickHouseConfig
	System     *EnvSystemConfig
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
		Kafka:      NewEnvKafkaConfig(prefix),
		Redis:      NewEnvRedisConfig(prefix),
		Postgres:   NewEnvPostgresConfig(prefix),
		ClickHouse: NewEnvClickHouseConfig(prefix),
		System:     NewEnvSystemConfig(prefix),
	}
	return config
}

func NewEnvKafkaConfig(prefix string) *EnvKafkaConfig {
	var prefixer = newEnvPrefixer(prefix)
	var config = &EnvKafkaConfig{}
	config.Servers = helpers.ParseEnvSlice(prefixer("KAFKA_SERVERS"))
	config.SecurityProtocol = prefixer("KAFKA_SECURITY_PROTOCOL")
	config.Mechanism = prefixer("KAFKA_MECHANISM")
	config.User = prefixer("KAFKA_USER")
	config.Password = prefixer("KAFKA_PASSWORD")
	config.SslCaLocation = prefixer("KAFKA_SSL_CA_LOCATION")
	config.SslLocation = prefixer("KAFKA_SSL_LOCATION")
	config.SslKeyLocation = prefixer("KAFKA_SSL_KEY_LOCATION")
	config.KeytabPath = prefixer("KAFKA_KEYTAB_PATH")
	config.Principal = prefixer("KAFKA_PRINCIPAL")
	return config
}

func NewEnvRedisConfig(prefix string) *EnvRedisConfig {
	var prefixer = newEnvPrefixer(prefix)
	var config = &EnvRedisConfig{}
	config.Addresses = helpers.ParseEnvSlice(prefixer("REDIS_ADDRESSES"))
	config.Username = prefixer("REDIS_USERNAME")
	config.Password = prefixer("REDIS_PASSWORD")
	config.Mode = prefixer("REDIS_MODE")
	config.Db = 0
	var redisDb = helpers.ParseEnvInt(prefixer("REDIS_DB"))
	if redisDb != nil {
		config.Db = *redisDb
	}
	config.Master = prefixer("REDIS_MASTER")
	return config
}

func NewEnvPostgresConfig(prefix string) *EnvPostgresConfig {
	var prefixer = newEnvPrefixer(prefix)
	var config = &EnvPostgresConfig{}
	config.Connection = prefixer("POSTGRES_CONNECTION")
	return config
}

func NewEnvClickHouseConfig(prefix string) *EnvClickHouseConfig {
	var prefixer = newEnvPrefixer(prefix)
	var config = &EnvClickHouseConfig{}
	config.Connection = prefixer("CLICKHOUSE_CONNECTION")
	return config
}

func NewEnvSystemConfig(prefix string) *EnvSystemConfig {
	var prefixer = newEnvPrefixer(prefix)
	var config = &EnvSystemConfig{}
	config.Port = 3000
	var port = helpers.ParseEnvInt(prefixer("PORT"))
	if port != nil {
		config.Port = *port
	}
	var logLevel = strings.ToLower(prefixer("LOG_LEVEL"))
	switch logLevel {
	case "debug":
		config.LogLevel = slog.LevelDebug
	case "info":
		config.LogLevel = slog.LevelInfo
	case "warn":
		config.LogLevel = slog.LevelWarn
	case "error":
		config.LogLevel = slog.LevelError
	default:
		config.LogLevel = slog.LevelInfo

	}
	config.LogColor = helpers.ParseEnvBool(prefixer("LOG_COLOR"))
	config.CompressRequest = helpers.ParseEnvBool(prefixer("COMPRESS_REQUEST"))
	config.OtlpExporterURL = prefixer("OTLP_EXPORTER_URL")
	config.OtlpExporterProtocol = prefixer("OTLP_PROTOCOL")
	return config
}

func newEnvPrefixer(prefix string) func(name string) string {
	return func(name string) string {
		return os.Getenv(prefix + name)
	}
}
