package app

import (
	"log/slog"
	"os"
	"strings"

	"github.com/KrainovSD/go-packages/helpers"
	"github.com/KrainovSD/go-packages/queue"
)

type EnvConfig struct {
	Kafka      *EnvKafkaConfig
	Redis      *EnvRedisConfig
	Postgres   *EnvPostgresConfig
	ClickHouse *EnvClickHouseConfig
	Minio      *EnvMinioConfig
	System     *EnvSystemConfig
}

type EnvKafkaConfig struct {
	Servers         []string
	SecurityOptions queue.KafkaSecurityOptions
}

type EnvPulsarConfig struct {
	URL             string
	SecurityOptions queue.PulsarSecurityOptions
}

type EnvRedisConfig struct {
	Addresses []string
	Username  string
	Password  string
	Mode      string
	DB        int
	Master    string
}

type EnvPostgresConfig struct {
	Connection string
}

type EnvClickHouseConfig struct {
	Connection string
}

type EnvMinioConfig struct {
	Address   string
	AccessKey string
	SecretKey string
	Secure    bool
	Location  string
}

type EnvSystemConfig struct {
	Port                    int
	LogLevel                slog.Level
	LogColor                bool
	CompressResponse        bool
	CompressResponseLevel   int
	CompressResponseMinSize int
	OtlpExporterURL         string
	OtlpExporterProtocol    string
	Pprof                   bool
	PprofPort               int
}

func NewEnvConfig(prefix string) *EnvConfig {
	var config = &EnvConfig{
		Kafka:      NewEnvKafkaConfig(prefix),
		Redis:      NewEnvRedisConfig(prefix),
		Postgres:   NewEnvPostgresConfig(prefix),
		ClickHouse: NewEnvClickHouseConfig(prefix),
		Minio:      NewEnvMinioConfig(prefix),
		System:     NewEnvSystemConfig(prefix),
	}
	return config
}

func NewEnvPulsarConfig(prefix string) *EnvPulsarConfig {
	var prefixer = newEnvPrefixer(prefix)
	var config = &EnvPulsarConfig{
		URL: prefixer("PULSAR_URL"),
		SecurityOptions: queue.PulsarSecurityOptions{
			Token:               prefixer("PULSAR_TOKEN"),
			TokenFile:           prefixer("PULSAR_TOKEN_FILE"),
			User:                prefixer("PULSAR_USER"),
			Password:            prefixer("PULSAR_PASSWORD"),
			TLSLocation:         prefixer("PULSAR_TLS_LOCATION"),
			TLSKeyLocation:      prefixer("PULSAR_TLS_KEY_LOCATION"),
			TLSCaLocation:       prefixer("PULSAR_TLS_CA_LOCATION"),
			TLSAllowInsecure:    helpers.ParseEnvBool(prefixer("PULSAR_TLS_ALLOW_INSECURE")),
			TLSValidateHostname: helpers.ParseEnvBool(prefixer("PULSAR_TLS_VALIDATE_HOSTNAME")),
		},
	}
	return config
}

func NewEnvKafkaConfig(prefix string) *EnvKafkaConfig {
	var prefixer = newEnvPrefixer(prefix)
	var config = &EnvKafkaConfig{
		Servers: helpers.ParseEnvSlice(prefixer("KAFKA_SERVERS")),
		SecurityOptions: queue.KafkaSecurityOptions{
			SecurityProtocol: prefixer("KAFKA_SECURITY_PROTOCOL"),
			TLSCaLocation:    prefixer("KAFKA_TLS_CA_LOCATION"),
			TLSLocation:      prefixer("KAFKA_TLS_LOCATION"),
			TLSKeyLocation:   prefixer("KAFKA_TLS_KEY_LOCATION"),
			SASLMechanism:    prefixer("KAFKA_SASL_MECHANISM"),
			SASLKeytabPath:   prefixer("KAFKA_SASL_KEYTAB_PATH"),
			SASLPrincipal:    prefixer("KAFKA_SASL_PRINCIPAL"),
			User:             prefixer("KAFKA_USER"),
			Password:         prefixer("KAFKA_PASSWORD"),
		},
	}
	return config
}

func NewEnvRedisConfig(prefix string) *EnvRedisConfig {
	var prefixer = newEnvPrefixer(prefix)
	var config = &EnvRedisConfig{
		Addresses: helpers.ParseEnvSlice(prefixer("REDIS_ADDRESSES")),
		Username:  prefixer("REDIS_USERNAME"),
		Password:  prefixer("REDIS_PASSWORD"),
		Mode:      prefixer("REDIS_MODE"),
		DB:        helpers.IntFromPtr(helpers.ParseEnvInt(prefixer("REDIS_DB"))),
		Master:    prefixer("REDIS_MASTER"),
	}
	return config
}

func NewEnvPostgresConfig(prefix string) *EnvPostgresConfig {
	var prefixer = newEnvPrefixer(prefix)
	var config = &EnvPostgresConfig{
		Connection: prefixer("POSTGRES_CONNECTION"),
	}
	return config
}

func NewEnvClickHouseConfig(prefix string) *EnvClickHouseConfig {
	var prefixer = newEnvPrefixer(prefix)
	var config = &EnvClickHouseConfig{
		Connection: prefixer("CLICKHOUSE_CONNECTION"),
	}
	return config
}

func NewEnvMinioConfig(prefix string) *EnvMinioConfig {
	var prefixer = newEnvPrefixer(prefix)
	var config = &EnvMinioConfig{
		Address:   prefixer("MINIO_ADDRESS"),
		AccessKey: prefixer("MINIO_ACCESS_KEY"),
		SecretKey: prefixer("MINIO_SECRET_KEY"),
		Location:  prefixer("MINIO_LOCATION"),
		Secure:    helpers.ParseEnvBool(prefixer("MINIO_SECURE")),
	}

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
	config.CompressResponse = helpers.ParseEnvBool(prefixer("COMPRESS_RESPONSE"))
	var compressLevel = helpers.ParseEnvInt(prefixer("COMPRESS_RESPONSE_LEVEL"))
	if compressLevel != nil {
		config.CompressResponseLevel = *compressLevel
	}
	var compressMinSize = helpers.ParseEnvInt(prefixer("COMPRESS_RESPONSE_MIN_SIZE"))
	if compressMinSize != nil {
		config.CompressResponseMinSize = *compressMinSize
	}
	config.OtlpExporterURL = prefixer("OTLP_EXPORTER_URL")
	config.OtlpExporterProtocol = prefixer("OTLP_PROTOCOL")
	config.Pprof = helpers.ParseEnvBool(prefixer("PPROF"))
	var pprofPort = helpers.ParseEnvInt(prefixer("PPROF_PORT"))
	if pprofPort == nil {
		config.PprofPort = 9000
	} else {
		config.PprofPort = *pprofPort
	}

	return config
}

func newEnvPrefixer(prefix string) func(name string) string {
	return func(name string) string {
		return os.Getenv(prefix + name)
	}
}
