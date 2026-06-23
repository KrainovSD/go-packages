package config

import (
	"fmt"
	"os"

	"github.com/KrainovSD/go-packages/app"
	"github.com/KrainovSD/go-packages/helpers"
)

type Config struct {
	KafkaTopic string
	Default    *app.EnvConfig
}

func (c *Config) Validate() error {
	if c.KafkaTopic == "" {
		return fmt.Errorf("KAFKA_TOPIC env required")
	}

	return nil
}

func Create() *Config {
	var config Config
	_ = helpers.LoadEnvFile(".env")
	config.KafkaTopic = os.Getenv("KAFKA_TOPIC")
	config.Default = app.NewEnvConfig("")
	return &config
}
