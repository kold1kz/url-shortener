package config

import (
	"flag"
	"fmt"
	"log"
	"os"
	"url-shortener/internal/database"
)

type Config struct {
	ServerAddress   string
	BaseURL         string
	FileStoragePath string
	DatabaseDSN     string
	EnableHTTPS     bool

	AuditFile string
	AuditURL  string

	DB *database.DB
}

func Init() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "HTTP server address")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "Base URL for short links")
	flag.StringVar(&cfg.FileStoragePath, "f", "./tmp/shorten_url.json", "File storage path")
	flag.StringVar(&cfg.DatabaseDSN, "d", "postgres://root:root@localhost:5433/db", "Database DSN")
	flag.StringVar(&cfg.AuditFile, "audit-file", "", "Audit file path")
	flag.StringVar(&cfg.AuditURL, "audit-url", "", "Audit remote URL")
	flag.BoolVar(&cfg.EnableHTTPS, "s", false, "Start server with HTTPS")
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
	if envAuditURL := os.Getenv("AUDIT_URL"); envAuditURL != "" {
		cfg.AuditURL = envAuditURL
	}
	if envAuditFile := os.Getenv("AUDIT_FILE"); envAuditFile != "" {
		cfg.AuditFile = envAuditFile
	}
	if envEnableHTTPS := os.Getenv("ENABLE_HTTPS"); envEnableHTTPS != "" {
		cfg.EnableHTTPS = true
	}

	cfg.initDB()
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

func (c *Config) initDB() {
	if c.DatabaseDSN == "" {
		return
	}

	db, err := database.NewDB(c.DatabaseDSN)
	if err != nil {
		log.Printf("Failed to connect to PostgreSQL: %v", err)
		return
	}

	c.DB = db
	log.Printf("Connected to PostgreSQL")
}

func (c *Config) Close() error {
	if c.DB != nil {
		_ = c.DB.Close()
	}
	return nil
}
