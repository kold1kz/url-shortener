package config

import (
	"flag"
	"fmt"
	"os"
	"url-shortener/internal/repository"
)

type Config struct {
	ServerAddress   string
	BaseURL         string
	FileStoragePath string
	URLRepository   repository.URLRepository
}

func Init() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "HTTP server address")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "Base URL for short links")
	flag.StringVar(&cfg.FileStoragePath, "f", "./tmp/shorten_url.json", "File storage path")
	flag.Parse()

	if envServer := os.Getenv("SERVER_ADDRESS"); envServer != "" {
		cfg.ServerAddress = envServer
	}

	if envBaseURL := os.Getenv("BASE_URL"); envBaseURL != "" {
		cfg.BaseURL = envBaseURL
	}

	if envFileStorage := os.Getenv("FILE_STORAGE_PATH"); envFileStorage != "" {
		cfg.FileStoragePath = envFileStorage
	}
	cfg.initRepository()
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

func (c *Config) initRepository() {
	if c.FileStoragePath != "" {
		fileRepo, err := repository.NewFileURLRepository(c.FileStoragePath)
		if err != nil {
			c.URLRepository = repository.NewInMemoryURLRepository()
		} else {
			c.URLRepository = fileRepo
		}
	} else {
		c.URLRepository = repository.NewInMemoryURLRepository()
	}
}

func (c *Config) Close() error {
	if fileRepo, ok := c.URLRepository.(*repository.FileURLRepository); ok {
		return fileRepo.Close()
	}
	return nil
}
