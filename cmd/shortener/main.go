package main

import (
	"github.com/gin-gonic/gin"
	"url-shortener/internal/handler"
	"url-shortener/internal/repository"
	"url-shortener/internal/service"
)

// так как я мало что знаю в go испольщовал deepsek для помоши и объяснений
func main() {
	// Инициализация зависимостей
	repo := repository.NewInMemoryURLRepository()
	urlService := service.NewURLService(repo, "http://localhost:8080")
	handlers := handler.NewHandler(urlService)

	// Настройка маршрутов
	router := gin.Default()

	// Регистрируем обработчики
	router.POST("/", handlers.ShortenURL)       // Изменим сигнатуру
	router.GET("/:id", handlers.GetOriginalURL) // Изменим сигнатуру

	// Запуск сервера
	//log.Println("Server starting on http://localhost:8080")
	router.Run(":8080")
}
