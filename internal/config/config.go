package config

import (
	"strings"

	"github.com/spf13/viper"
)

type WeaviateConfig struct {
	Host   string `mapstructure:"host"`
	Scheme string `mapstructure:"scheme"`
	APIKey string `mapstructure:"api_key"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Weaviate WeaviateConfig `mapstructure:"weaviate"`
}

func LoadConfig() (*Config, error) {
	v := viper.New()
	// Defaults
	v.SetDefault("server.port", 8081)
	v.SetDefault("weaviate.host", "localhost:8080")
	v.SetDefault("weaviate.scheme", "http")
	v.SetDefault("weaviate.api_key", "changeme")
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}
	var config Config

	err := v.Unmarshal(&config)
	return &config, err
}
