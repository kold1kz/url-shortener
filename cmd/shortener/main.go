package main

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"log"
	"url-shortener/internal/config"
	"url-shortener/internal/database"
	"url-shortener/internal/handler"
	"url-shortener/internal/middleware"
)

func loadConfig() *config.Config {
	cfg := config.Init()

	// Проверяем корректность конфигурации
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}
	log.Printf("Configuration loaded: %v", cfg)

	return cfg
}

func setupDatabase(cfg *config.Config, logger *zap.SugaredLogger) *database.DB {
	if cfg.DatabaseDSN == "" {
		logger.Warn("DATABASE_DSN is empty, database will not be initialized")
		return nil
	}

	db, err := database.NewDB(cfg.DatabaseDSN)
	if err != nil {
		logger.Errorf("Failed to connect to database: %v", err)
		return nil
	}

	logger.Info("Connected to PostgreSQL")
	return db
}

func main() {
	cfg := loadConfig()
	defer cfg.Close()

	logger := middleware.InitLogger()
	defer logger.Sync()

	db := setupDatabase(cfg, logger)
	if db != nil {
		defer db.Close()
	}

	handlers := handler.NewHandler(cfg.URLService)

	// Настройка маршрутов
	router := gin.Default()

	router.Use(middleware.GzipMiddleware())
	router.Use(middleware.HTTPLoggerMiddleware(logger))

	// Регистрируем обработчики
	router.POST("/", handlers.ShortenURL)
	router.GET("/:id", handlers.GetOriginalURL)
	// Регистрируем обработчики JSON
	router.POST("/api/shorten", handlers.ShortenJSONUrl)
	router.POST("/api/shorten/batch", handlers.ShortenURLBatch)

	router.GET("/api/user/urls", handlers.GetUserURLs)

	// Регистрируем обработчик для бд
	pingHandler := handler.NewPingHandler(db)
	router.GET("/ping", pingHandler.Ping)

	// Запуск сервера
	//log.Printf("Server starting on %s %s", cfg.BaseURL, cfg.ServerAddress)
	router.Run(cfg.ServerAddress)
}
