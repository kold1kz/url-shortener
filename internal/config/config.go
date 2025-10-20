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

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "HTTP server address")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "Base URL for short links")

	flag.Parse()

	if envServer := os.Getenv("SERVER_ADDRESS"); envServer != "" {
		cfg.ServerAddress = envServer
	}

	if envBaseURL := os.Getenv("BASE_URL"); envBaseURL != "" {
		cfg.BaseURL = envBaseURL
	}

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
