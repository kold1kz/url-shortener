package main

import (
	"github.com/gin-gonic/gin"
	"log"
	"url-shortener/internal/config"
	"url-shortener/internal/handler"
	"url-shortener/internal/middleware"
	"url-shortener/internal/service"
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

func main() {
	cfg := loadConfig()
	defer cfg.Close()

	logger := middleware.InitLogger()
	defer logger.Sync()

	// repo := repository.NewInMemoryURLRepository()
	urlService := service.NewURLService(cfg.URLRepository, cfg.BaseURL)
	handlers := handler.NewHandler(urlService)

	// Настройка маршрутов
	router := gin.Default()

	router.Use(middleware.GzipMiddleware())
	router.Use(middleware.HTTPLoggerMiddleware(logger))

	// Регистрируем обработчики
	router.POST("/", handlers.ShortenURL)
	router.GET("/:id", handlers.GetOriginalURL)
	// Регистрируем обработчики JSON
	router.POST("/api/shorten", handlers.ShortenJSONUrl)

	// Запуск сервера
	//log.Printf("Server starting on %s %s", cfg.BaseURL, cfg.ServerAddress)
	router.Run(cfg.ServerAddress)
}
