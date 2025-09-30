package main

import (
	"github.com/gin-gonic/gin"
	"log"
	"url-shortener/internal/config"
	"url-shortener/internal/handler"
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

func main() {
	cfg := loadConfig()
	// Инициализация зависимостей
	repo := repository.NewInMemoryURLRepository()
	urlService := service.NewURLService(repo, cfg.BaseURL)
	handlers := handler.NewHandler(urlService)

	// Настройка маршрутов
	router := gin.Default()

	// Регистрируем обработчики
	router.POST("/", handlers.ShortenURL)       // Изменим сигнатуру
	router.GET("/:id", handlers.GetOriginalURL) // Изменим сигнатуру

	// Запуск сервера
	// log.Printf("Server starting on %s", cfg.ServerAddress)
	router.Run(cfg.ServerAddress)
}
