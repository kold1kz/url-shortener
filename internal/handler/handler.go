package handler

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
	"url-shortener/internal/auth"
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
	rawCookie, _ := c.Cookie(auth.CookieName())
	userID, newToken, err := auth.GetOrCreateUserIDFromCookie(rawCookie)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "auth error"})
		return
	}
	if newToken != "" {
		c.SetCookie(auth.CookieName(), newToken, 0, "/", "", false, true)
	}

	url, err := h.service.ShortenURL(c.Request.Context(), originalURL, userID)
	if err != nil {
		if errors.Is(err, service.ErrURLAlreadyExists) {
			c.Header("Content-Type", "text/plain")
			c.String(http.StatusConflict, url.Short)
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": http.StatusText(http.StatusInternalServerError),
		})
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

	originalURL, err := h.service.GetOriginalURL(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrURLDeleted) {
			c.Status(http.StatusGone)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": http.StatusText(http.StatusInternalServerError),
		})
		return
	}

	if originalURL == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Url not found"})
		return
	}

	c.Header("Location", originalURL)
	c.String(http.StatusTemporaryRedirect, originalURL)
}

func (h *Handlers) ShortenJSONUrl(c *gin.Context) {
	if c.ContentType() != "application/json" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid content type"})
		return
	}

	var req model.ShortenRequest
	//if err := c.ShouldBindJSON(&req); err != nil {
	//	c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
	//	return
	//}

	dec := json.NewDecoder(c.Request.Body)
	if err := dec.Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}
	// перенести в service
	if req.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	rawCookie, _ := c.Cookie(auth.CookieName())
	userID, newToken, err := auth.GetOrCreateUserIDFromCookie(rawCookie)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "auth error"})
		return
	}
	if newToken != "" {
		c.SetCookie(auth.CookieName(), newToken, 0, "/", "", false, true)
	}

	url, err := h.service.ShortenURL(c.Request.Context(), req.URL, userID)
	if err != nil {
		resp := model.ShortenResponse{Result: ""}
		if url != nil {
			resp.Result = url.Short
		}

		if errors.Is(err, service.ErrURLAlreadyExists) {
			c.JSON(http.StatusConflict, resp)
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": http.StatusText(http.StatusInternalServerError),
		})
		return
	}

	resp := model.ShortenResponse{
		Result: url.Short,
	}
	c.Header("Content-Type", "application/json")
	c.Status(http.StatusCreated)

	enc := json.NewEncoder(c.Writer)
	if err := enc.Encode(resp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode response"})
		return
	}

	//c.JSON(http.StatusCreated, resp)
}

func (h *Handlers) ShortenURLBatch(c *gin.Context) {
	if c.ContentType() != "application/json" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid content type"})
		return
	}

	var batch []model.BatchRequest
	if err := c.ShouldBindJSON(&batch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	if len(batch) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Empty batch"})
		return
	}

	for _, item := range batch {
		if item.CorrelationID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
			return
		}
		if item.OriginalURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
			return
		}
	}

	rawCookie, _ := c.Cookie(auth.CookieName())
	userID, newToken, err := auth.GetOrCreateUserIDFromCookie(rawCookie)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "auth error"})
		return
	}
	if newToken != "" {
		c.SetCookie(auth.CookieName(), newToken, 0, "/", "", false, true)
	}

	responses, err := h.service.ShortenURLBatch(c.Request.Context(), batch, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": http.StatusText(http.StatusInternalServerError),
		})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Status(http.StatusCreated)

	enc := json.NewEncoder(c.Writer)
	if err := enc.Encode(responses); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode response"})
		return
	}
}

func (h *Handlers) GetUserURLs(c *gin.Context) {
	rawCookie, err := c.Cookie(auth.CookieName())

	var userID string
	var newToken string

	if err != nil || rawCookie == "" {
		userID, newToken, err = auth.GetOrCreateUserIDFromCookie("")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "auth error"})
			return
		}
		if newToken != "" {
			c.SetCookie(auth.CookieName(), newToken, 0, "/", "", false, true)
		}
	} else {
		userID, err = auth.GetUserIDFromCookieStrict(rawCookie)
		if err != nil || userID == "" {
			c.Status(http.StatusUnauthorized)
			return
		}
	}

	urls, err := h.service.FindByUserID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": http.StatusText(http.StatusInternalServerError)})
		return
	}

	if len(urls) == 0 {
		c.Status(http.StatusNoContent)
		return
	}

	resp := make([]model.UserURLResponse, 0, len(urls))
	for _, u := range urls {
		resp = append(resp, model.UserURLResponse{
			ShortURL:    u.Short,
			OriginalURL: u.Original,
		})
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handlers) DeleteUserURLs(c *gin.Context) {
	if c.ContentType() != "application/json" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid content type"})
		return
	}

	rawCookie, err := c.Cookie(auth.CookieName())
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return
	}

	userID, err := auth.GetUserIDFromCookieStrict(rawCookie)
	if err != nil || userID == "" {
		c.Status(http.StatusUnauthorized)
		return
	}

	var ids []string
	if err := c.ShouldBindJSON(&ids); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}
	if len(ids) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Empty batch"})
		return
	}

	if err := h.service.DeleteUserURLs(c.Request.Context(), userID, ids); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": http.StatusText(http.StatusInternalServerError)})
		return
	}

	c.Status(http.StatusAccepted)
}
