package main

import (
	"log"
	"net/http"
	"url-shortener/internal/handler"
	"url-shortener/internal/repository"
	"url-shortener/internal/service"
)

func main() {
	// Инициализация зависимостей
	repo := repository.NewInMemoryURLRepository()
	urlService := service.NewURLService(repo, "http://localhost:8080")
	handlers := handler.NewHandler(urlService)

	// Настройка маршрутов
	mux := http.NewServeMux()

	// Регистрируем обработчики
	mux.HandleFunc("POST /", handlers.ShortenURL)
	mux.HandleFunc("GET /{id}", handlers.GetOriginalURL)

	// Обработка несуществующих путей
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.Error(w, "Not found", http.StatusBadRequest)
			return
		}
		http.Error(w, "Method not allowed", http.StatusBadRequest)
	})

	// Настройка сервера
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Запуск сервера
	//log.Println("Server starting on http://localhost:8080")
	if err := server.ListenAndServe(); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
