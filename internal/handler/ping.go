package handler

import (
	"net/http"
	"url-shortener/internal/database"

	"github.com/gin-gonic/gin"
)

type PingHandler struct {
	db database.Pinger // используем интерфейс вместо конкретного типа
}

func NewPingHandler(db database.Pinger) *PingHandler {
	return &PingHandler{db: db}
}

func (h *PingHandler) Ping(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}
	if err := h.db.Ping(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}
	c.Status(http.StatusOK)
}
