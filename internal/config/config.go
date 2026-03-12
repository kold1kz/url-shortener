package config

import (
	"encoding/json"
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
	DB        *database.DB

	ConfigFile    string
	TrustedSubnet string
}

type FileConfig struct {
	ServerAddress   string `json:"server_address"`
	BaseURL         string `json:"base_url"`
	FileStoragePath string `json:"file_storage_path"`
	DatabaseDSN     string `json:"database_dsn"`
	EnableHTTPS     *bool  `json:"enable_https"`
	AuditFile       string `json:"audit_file"`
	AuditURL        string `json:"audit_url"`
	TrustedSubnet   string `json:"trusted_subnet"`
}

func Init() *Config {
	cfg := &Config{}

	cfg.ConfigFile = detectConfigFile()

	fileCfg, err := readFileConfig(cfg.ConfigFile)
	if err != nil {
		log.Printf("%s", err)
	}

	defAddr := "localhost:8080"
	defBase := "http://localhost:8080"
	defFile := "./tmp/shorten_url.json"
	defDSN := "postgres://root:root@localhost:5433/db"
	defHTTPS := false
	defAuditFile := ""
	defAuditURL := ""
	defTrustedSubnet := ""

	if fileCfg != nil {
		if fileCfg.ServerAddress != "" {
			defAddr = fileCfg.ServerAddress
		}
		if fileCfg.BaseURL != "" {
			defBase = fileCfg.BaseURL
		}
		if fileCfg.FileStoragePath != "" {
			defFile = fileCfg.FileStoragePath
		}
		if fileCfg.DatabaseDSN != "" {
			defDSN = fileCfg.DatabaseDSN
		}
		if fileCfg.EnableHTTPS != nil {
			defHTTPS = *fileCfg.EnableHTTPS
		}
		if fileCfg.AuditFile != "" {
			defAuditFile = fileCfg.AuditFile
		}
		if fileCfg.AuditURL != "" {
			defAuditURL = fileCfg.AuditURL
		}
		if fileCfg.TrustedSubnet != "" {
			defTrustedSubnet = fileCfg.TrustedSubnet
		}
	}

	flag.StringVar(&cfg.ServerAddress, "a", defAddr, "HTTP server address")
	flag.StringVar(&cfg.BaseURL, "b", defBase, "Base URL for short links")
	flag.StringVar(&cfg.FileStoragePath, "f", defFile, "File storage path")
	flag.StringVar(&cfg.DatabaseDSN, "d", defDSN, "Database DSN")
	flag.BoolVar(&cfg.EnableHTTPS, "s", defHTTPS, "Start server with HTTPS")
	flag.StringVar(&cfg.AuditFile, "audit-file", defAuditFile, "Audit file path")
	flag.StringVar(&cfg.AuditURL, "audit-url", defAuditURL, "Audit remote URL")

	flag.StringVar(&cfg.ConfigFile, "c", cfg.ConfigFile, "load config from file")
	flag.StringVar(&cfg.ConfigFile, "config", cfg.ConfigFile, "load config from file")

	flag.StringVar(&cfg.TrustedSubnet, "t", defTrustedSubnet, "load config from file")

	flag.Parse()

	applyEnv(cfg)

	cfg.initDB()
	return cfg
}

func detectConfigFile() string {

	for i := 0; i < len(os.Args); i++ {
		if os.Args[i] == "-c" || os.Args[i] == "-config" {
			if i+1 < len(os.Args) {
				return os.Args[i+1]
			}
		}
	}
	if p := os.Getenv("CONFIG"); p != "" {
		return p
	}
	return ""
}

func readFileConfig(path string) (*FileConfig, error) {
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read file %q: %w", path, err)
	}
	var fc FileConfig
	if err := json.Unmarshal(b, &fc); err != nil {
		return nil, fmt.Errorf("config: parse json %q: %w", path, err)
	}
	return &fc, nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("SERVER_ADDRESS"); v != "" {
		cfg.ServerAddress = v
	}
	if v := os.Getenv("BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("FILE_STORAGE_PATH"); v != "" {
		cfg.FileStoragePath = v
	}
	if v := os.Getenv("DATABASE_DSN"); v != "" {
		cfg.DatabaseDSN = v
	}
	if v := os.Getenv("AUDIT_URL"); v != "" {
		cfg.AuditURL = v
	}
	if v := os.Getenv("AUDIT_FILE"); v != "" {
		cfg.AuditFile = v
	}
	if v := os.Getenv("ENABLE_HTTPS"); v != "" {
		cfg.EnableHTTPS = true
	}
	if v := os.Getenv("TRUSTED_SUBNET"); v != "" {
		cfg.TrustedSubnet = v
	}
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
