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

func initLogger() *zap.SugaredLogger {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Printf("Failed to initialize zap logger: %v", err)
		return zap.NewNop().Sugar()
	}

	logger = logger.WithOptions(zap.IncreaseLevel(zap.InfoLevel))
	return logger.Sugar()
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
	// Регистрируем обработчики JSON
	router.POST("/api/shorten", handlers.ShortenJSONUrl)

	// Запуск сервера
	//log.Printf("Server starting on %s %s", cfg.BaseURL, cfg.ServerAddress)
	router.Run(cfg.ServerAddress)
}

func httpLoggerMiddleware(logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Начало запроса - засекаем время
		start := time.Now()

		// Обрабатываем запрос
		c.Next()

		// Вычисляем затраченное время
		duration := time.Since(start)

		// Читаем размер содержимого ответа
		size := c.Writer.Size()

		logger.Infow("HTTP Request",
			zap.String("url", c.Request.RequestURI),
			zap.String("method", c.Request.Method),
			zap.Duration("duration", duration),
		)
		logger.Infow("HTTP Response",
			zap.Int("status", c.Writer.Status()),
			zap.Int("size", size),
		)
	}
}
