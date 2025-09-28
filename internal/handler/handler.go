package handler

import (
	"io"
	"net/http"
	"strings"
	"url-shortener/internal/service"
)

type Handlers struct {
	service service.URLServiceI
}

func NewHandler(service service.URLServiceI) *Handlers {
	return &Handlers{service: service}
}

func (h *Handlers) ShortenURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusBadRequest)
		return
	}

	if !strings.Contains(r.Header.Get("Content-Type"), "text/plain") {
		http.Error(w, "Invalid content type", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	originalURL := strings.TrimSpace(string(body))
	if originalURL == "" {
		http.Error(w, "URL cannot be empty", http.StatusBadRequest)
		return
	}

	url, err := h.service.ShortenURL(originalURL)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(url.Short))
}

func (h *Handlers) GetOriginalURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusBadRequest)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	originalURL, err := h.service.GetOriginalURL(id)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusBadRequest)
		return
	}

	if originalURL == "" {
		http.Error(w, "URL not found", http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}
