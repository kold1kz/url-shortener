package main

import (
	"fmt"
	"log"
	"os"
	"url-shortener/internal/audit"
	"url-shortener/internal/config"
	"url-shortener/internal/handler"
	"url-shortener/internal/middleware"
	"url-shortener/internal/pprof"
	"url-shortener/internal/repository"
	"url-shortener/internal/service"

	"github.com/gin-gonic/gin"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func na(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

func loadConfig() (*config.Config, error) {
	cfg := config.Init()

	// Проверяем корректность конфигурации
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}

func buildRepo(cfg *config.Config) (repository.URLRepository, error) {
	if cfg.DB != nil {
		postgresRepo, err := repository.NewPostgresURLRepository(cfg.DB.GetDB())
		if err != nil {
			return nil, fmt.Errorf("failed to init postgres repository: %w", err)
		}
		log.Printf("Using PostgreSQL repository")
		return postgresRepo, nil
	}

	if cfg.FileStoragePath != "" {
		fileRepo, err := repository.NewFileURLRepository(cfg.FileStoragePath)
		if err != nil {
			return nil, fmt.Errorf("init file repository (%s): %w", cfg.FileStoragePath, err)
		}
		log.Printf("Using file repository: %s", cfg.FileStoragePath)
		return fileRepo, nil
	}

	log.Printf("Using in-memory repository")
	return repository.NewInMemoryURLRepository(), nil
}

func buildAuditPublisher(cfg *config.Config) *audit.Publisher {
	pub := audit.NewPublisher()

	if cfg.AuditFile != "" {
		fs, err := audit.NewFileSink(cfg.AuditFile)
		if err != nil {
			log.Printf("Failed to init audit file sink: %v", err)
		} else {
			pub.AddSink(fs)
			log.Printf("Audit file enabled: %s", cfg.AuditFile)
		}
	}

	if cfg.AuditURL != "" {
		pub.AddSink(audit.NewHTTPSink(cfg.AuditURL))
		log.Printf("Audit remote enabled: %s", cfg.AuditURL)
	}

	// если ни одного sink нет — можно вернуть nil, чтобы хендлеры не дергали Publish
	// но тогда нужно уметь в handler.NewHandler принимать nil
	// Я бы оставил pub даже пустым, если Publish у тебя нооп при 0 sinks.
	return pub
}

func main() {
	fmt.Printf("Build version: %s\n", na(buildVersion))
	fmt.Printf("Build date: %s\n", na(buildDate))
	fmt.Printf("Build commit: %s\n", na(buildCommit))
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
		// log.Fatalf("Failed to load config: %v", err)
	}
	defer cfg.Close()

	logger := middleware.InitLogger()
	defer logger.Sync()

	repo, err := buildRepo(cfg)
	if err != nil {
		return fmt.Errorf("build repository: %w", err)
		// log.Fatalf("startup error: %v", err)
	}
	svc := service.NewURLService(repo, cfg.BaseURL)
	defer svc.Close()

	auditPub := buildAuditPublisher(cfg)
	defer auditPub.Close()

	handlers := handler.NewHandler(svc, auditPub)

	router := gin.Default()

	router.Use(middleware.GzipMiddleware())
	router.Use(middleware.HTTPLoggerMiddleware(logger))

	// Добавляем авторизацию
	authGroup := router.Group("/")
	authGroup.Use(middleware.UserAuth())

	// Регистрируем обработчики
	authGroup.POST("/", handlers.ShortenURL)
	authGroup.GET("/:id", handlers.GetOriginalURL)
	// Регистрируем обработчики JSON
	authGroup.POST("/api/shorten", handlers.ShortenJSONUrl)
	authGroup.POST("/api/shorten/batch", handlers.ShortenURLBatch)

	authGroup.GET("/api/user/urls", handlers.GetUserURLs)
	authGroup.DELETE("/api/user/urls", handlers.DeleteUserURLs)

	pingHandler := handler.NewPingHandler(cfg.DB)
	router.GET("/ping", pingHandler.Ping)
	pprof.Register(router)
	// Запуск сервера
	log.Printf("Server starting on %s", cfg.BaseURL)
	if err := router.Run(cfg.ServerAddress); err != nil {
		return fmt.Errorf("server run error: %v", err)
		// log.Fatalf("server run error: %v", err)
	}
	return nil
}
