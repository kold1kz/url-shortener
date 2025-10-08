package main

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"log"
	"time"
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

func initLogger() *zap.Logger {
	logger, err := zap.NewDevelopmen()
	if err != nil {
		log.Printf("Failed to initialize zap logger: %v", err)
		return zap.NewNop()
	}

	logger = logger.WithOptions(zap.IncreaseLevel(zap.InfoLevel))
	return logger
}

func main() {
	cfg := loadConfig()

	logger := initLogger()
	defer logger.Sync()

	// Инициализация зависимостей
	repo := repository.NewInMemoryURLRepository()
	urlService := service.NewURLService(repo, cfg.BaseURL)
	handlers := handler.NewHandler(urlService)

	// Настройка маршрутов
	router := gin.Default()

	router.Use(httpLoggerMiddleware(logger))

	// Регистрируем обработчики
	router.POST("/", handlers.ShortenURL)
	router.GET("/:id", handlers.GetOriginalURL)

	// Запуск сервера
	//log.Printf("Server starting on %s %s", cfg.BaseURL, cfg.ServerAddress)
	router.Run(cfg.ServerAddress)
}

func httpLoggerMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Начало запроса - засекаем время
		start := time.Now()

		// Обрабатываем запрос
		c.Next()

		// Вычисляем затраченное время
		duration := time.Since(start)

		// Получаем размер содержимого ответа
		size := c.Writer.Size()
		if size < 0 {
			size = 0
		}

		// Логируем все на уровне Info
		logger.Info("HTTP Request",
			zap.String("url", c.Request.RequestURL),
			zap.String("method", c.Request.Method),
			zap.Duration("duration", duration),
			zap.Int("status", c.Writer.Status()),
			zap.Int("size", size),
		)
	}
}
