package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/KrainovSD/go-packages/helpers"
)

type Config struct {
	PG_URL          string
	KAFKA_SERVERS   []string
	KAFKA_PROTOCOL  string
	KAFKA_TOPIC     string
	KAFKA_MECHANISM string
	KAFKA_USER      string
	KAFKA_PASSWORD  string

	REDIS_ADDRESSES []string
	REDIS_PASSWORD  string
	REDIS_MODE      string
	REDIS_DB        int
	REDIS_MASTER    string

	SERVICE_NAME     string
	PORT             int
	LOG_LEVEL        slog.Level
	LOG_COLOR        bool
	COMPRESS_REQUEST bool

	OTLP_EXPORTER_URL string
	OTLP_PROTOCOL     string
}

func (c *Config) Validate() error {
	if c.KAFKA_TOPIC == "" {
		return fmt.Errorf("KAFKA_TOPIC env required")
	}
	if c.KAFKA_PROTOCOL == "" {
		return fmt.Errorf("KAFKA_PROTOCOL env required")
	}

	return nil
}

func Create() *Config {
	var config Config

	_ = helpers.LoadEnvFile(".env")

	config.PG_URL = os.Getenv("PG_URL")
	config.KAFKA_SERVERS = strings.Split(os.Getenv("KAFKA_SERVERS"), ",")
	for i := range config.KAFKA_SERVERS {
		config.KAFKA_SERVERS[i] = strings.TrimSpace(config.KAFKA_SERVERS[i])
	}
	config.KAFKA_TOPIC = os.Getenv("KAFKA_TOPIC")
	config.KAFKA_MECHANISM = strings.ToLower(os.Getenv("KAFKA_MECHANISM"))
	config.KAFKA_PROTOCOL = strings.ToLower(os.Getenv("KAFKA_PROTOCOL"))
	config.KAFKA_USER = os.Getenv("KAFKA_USER")
	config.KAFKA_PASSWORD = os.Getenv("KAFKA_PASSWORD")

	config.REDIS_ADDRESSES = helpers.ParseEnvSlice(os.Getenv("REDIS_ADDRESSES"))
	config.REDIS_PASSWORD = os.Getenv("REDIS_PASSWORD")
	config.REDIS_MODE = os.Getenv("REDIS_MODE")
	var redisDb = helpers.ParseEnvInt(os.Getenv("REDIS_DB"))
	if redisDb != nil {
		config.REDIS_DB = *redisDb
	}
	config.REDIS_MASTER = os.Getenv("REDIS_MASTER")

	var port = helpers.ParseEnvInt(os.Getenv("PORT"))
	if port == nil {
		config.PORT = 3000
	} else {
		config.PORT = *port
	}
	config.SERVICE_NAME = os.Getenv("SERVICE_NAME")
	if strings.ToLower(os.Getenv("LOG_LEVEL")) == "debug" {
		config.LOG_LEVEL = slog.LevelDebug
	} else if strings.ToLower(os.Getenv("LOG_LEVEL")) == "info" {
		config.LOG_LEVEL = slog.LevelInfo
	} else if strings.ToLower(os.Getenv("LOG_LEVEL")) == "warn" {
		config.LOG_LEVEL = slog.LevelWarn
	} else if strings.ToLower(os.Getenv("LOG_LEVEL")) == "error" {
		config.LOG_LEVEL = slog.LevelError
	} else {
		config.LOG_LEVEL = slog.LevelInfo
	}
	config.LOG_COLOR = os.Getenv("LOG_COLOR") == "true"
	config.COMPRESS_REQUEST = os.Getenv("COMPRESS_REQUEST") == "true"

	config.OTLP_EXPORTER_URL = os.Getenv("OTLP_EXPORTER_URL")
	config.OTLP_PROTOCOL = os.Getenv("OTLP_PROTOCOL")

	return &config
}
