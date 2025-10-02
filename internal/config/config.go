package config

import (
	"flag"
	"fmt"
	"os"
)

type Config struct {
	ServerAddress string
	BaseURL       string
}

func Init() *Config {
	cfg := &Config{}

	defaultServerAddress := os.Getenv("SERVER_ADDRESS")
	if defaultServerAddress == "" {
		defaultServerAddress = "localhost:8080"
	}

	defaultBaseURL := os.Getenv("BASE_URL")
	if defaultBaseURL == "" {
		defaultBaseURL = "http://localhost:8080"
	}
	flag.StringVar(&cfg.ServerAddress, "a", defaultServerAddress, "HTTP server address")
	flag.StringVar(&cfg.BaseURL, "b", defaultBaseURL, "Base URL for short links")

	flag.Parse()

	return cfg
}

func (c *Config) Validate() error {
	if c.ServerAddress == "" {
		return fmt.Errorf("server address cannot be empty")
	}
	if c.BaseURL == "" {
		return fmt.Errorf("base URL cannot be empty")
	}
	return nil
}
