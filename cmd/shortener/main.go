package main

import (
	"github.com/gin-gonic/gin"
	"log"
	"url-shortener/internal/audit"
	"url-shortener/internal/config"
	"url-shortener/internal/handler"
	"url-shortener/internal/middleware"
	"url-shortener/internal/pprof"
	"url-shortener/internal/repository"
	"url-shortener/internal/service"
)

func loadConfig() *config.Config {
	cfg := config.Init()

	// Проверяем корректность конфигурации
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}
	return cfg
}

func buildRepo(cfg *config.Config) repository.URLRepository {

	if cfg.DB != nil {
		postgresRepo, err := repository.NewPostgresURLRepository(cfg.DB.GetDB())
		if err == nil {
			log.Printf("Using PostgreSQL repository")
			return postgresRepo
		}
		log.Printf("Failed to init PostgreSQL repo: %v. Fallback...", err)
	}

	if cfg.FileStoragePath != "" {
		fileRepo, err := repository.NewFileURLRepository(cfg.FileStoragePath)
		if err == nil {
			log.Printf("Using file repository: %s", cfg.FileStoragePath)
			return fileRepo
		}
		log.Printf("Failed to init file repo: %v. Fallback...", err)
	}

	log.Printf("Using in-memory repository")
	return repository.NewInMemoryURLRepository()
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
	cfg := loadConfig()
	defer cfg.Close()

	logger := middleware.InitLogger()
	defer logger.Sync()

	repo := buildRepo(cfg)
	svc := service.NewURLService(repo, cfg.BaseURL)
	defer svc.Close()

	auditPub := buildAuditPublisher(cfg)
	defer auditPub.Close()

	handlers := handler.NewHandler(svc, auditPub)

	router := gin.Default()

	router.Use(middleware.GzipMiddleware())
	router.Use(middleware.HTTPLoggerMiddleware(logger))

	//Добавляем авторизацию
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
	//log.Printf("Server starting on %s %s", cfg.BaseURL, cfg.ServerAddress)
	router.Run(cfg.ServerAddress)
}
