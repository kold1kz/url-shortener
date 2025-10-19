package handler

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
	"url-shortener/internal/model"
	"url-shortener/internal/service"
)

type Handlers struct {
	service service.URLService
}

func NewHandler(service service.URLService) *Handlers {
	return &Handlers{service: service}
}

func (h *Handlers) ShortenURL(c *gin.Context) {

	if !strings.Contains(c.ContentType(), "text/plain") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid content type"})
		return
	}

	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	originalURL := strings.TrimSpace(string(body))
	if originalURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL cannot be empty"})
		return
	}

	url, err := h.service.ShortenURL(originalURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.Header("Content-Type", "text/plain")
	c.String(http.StatusCreated, url.Short)
}

func (h *Handlers) GetOriginalURL(c *gin.Context) {

	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}

	originalURL, err := h.service.GetOriginalURL(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid server error"})
		return
	}

	if originalURL == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Url not found"})
		return
	}

	c.Header("Location", originalURL)
	// если я правильно понял задания и здесь не нужен c.Redirect
	c.String(http.StatusTemporaryRedirect, originalURL)
}

func (h *Handlers) ShortenJSONUrl(c *gin.Context) {
	if c.ContentType() != "application/json" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid content type"})
		return
	}

	var req model.ShortenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	url, err := h.service.ShortenURL(req.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	resp := model.ShortenResponse{
		Result: url.Short,
	}

	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusCreated, resp)
}
