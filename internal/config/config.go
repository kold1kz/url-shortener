package config

import (
	"flag"
	"fmt"
	"os"
	"url-shortener/internal/repository"
	"url-shortener/internal/service"
)

type Config struct {
	ServerAddress   string
	BaseURL         string
	FileStoragePath string
	DatabaseDSN     string
	URLService      service.URLService // переделать на interface
}

func Init() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "HTTP server address")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "Base URL for short links")
	flag.StringVar(&cfg.FileStoragePath, "f", "./tmp/shorten_url.json", "File storage path")
	flag.StringVar(&cfg.DatabaseDSN, "d", "postgres://username:password@localhost:5432/database_name",
		"Database address")
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

	if envDatabaseDSN := os.Getenv("DATABASE_DSN"); envDatabaseDSN != "" {
		cfg.DatabaseDSN = envDatabaseDSN
	}
	cfg.initService()
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

func (c *Config) initService() {
	var repo repository.URLRepository

	if c.FileStoragePath != "" {
		fileRepo, err := repository.NewFileURLRepository(c.FileStoragePath)
		if err != nil {
			repo = repository.NewInMemoryURLRepository()
		} else {
			repo = fileRepo
		}
	} else {
		repo = repository.NewInMemoryURLRepository()
	}

	// 2. Создаем сервис с выбранным репозиторием
	c.URLService = service.NewURLService(repo, c.BaseURL)
}

func (c *Config) Close() error {
	// Закрываем репозиторий через сервис (если возможно)
	if urlService, ok := c.URLService.(interface{ Close() error }); ok {
		return urlService.Close()
	}
	return nil
}
