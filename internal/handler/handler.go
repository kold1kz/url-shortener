package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"url-shortener/internal/audit"
	"url-shortener/internal/auth"
	"url-shortener/internal/model"
	"url-shortener/internal/service"

	"github.com/gin-gonic/gin"
)

// Handlers объединяет HTTP-хендлеры сервиса сокращения URL.
//
// Экземпляр содержит ссылку на сервисный слой и (опционально) publisher аудита.
// Если audit publisher равен nil, события аудита не публикуются.
type Handlers struct {
	service service.URLService
	audit   *audit.Publisher
}

// NewHandler создаёт набор HTTP-хендлеров.
//
// service — обязательная зависимость (бизнес-логика).
// auditPub — необязательная зависимость; если nil, аудит отключён.
func NewHandler(service service.URLService, auditPub *audit.Publisher) *Handlers {
	return &Handlers{service: service, audit: auditPub}
}

// ShortenURL обрабатывает сокращение URL в формате plain text.
//
// Endpoint: POST /
// Content-Type: text/plain
//
// Request body: исходный URL строкой.
// Response: 201 Created, тело — короткий URL (text/plain).
//
// Возможные ответы:
//   - 400 Bad Request — неверный Content-Type или пустой/битый body
//   - 409 Conflict — URL уже был сохранён ранее (возвращается short URL)
//   - 500 Internal Server Error — внутренняя ошибка
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
	userID := c.GetString(auth.ContextUserIDKey)

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

	if h.audit != nil {
		h.audit.Publish(c.Request.Context(), audit.Event{
			TS:     time.Now().Unix(),
			Action: audit.ActionShorten,
			UserID: userID,
			URL:    originalURL,
		})
	}
}

// GetOriginalURL возвращает исходный URL по короткому идентификатору.
//
// Endpoint: GET /:id
//
// Поведение:
//   - 307 Temporary Redirect + заголовок Location на original URL
//   - 404 Not Found — если id не найден
//   - 410 Gone — если URL помечен как удалённый
//   - 500 Internal Server Error — внутренняя ошибка
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

	userID := c.GetString(auth.ContextUserIDKey)

	if h.audit != nil {
		h.audit.Publish(c.Request.Context(), audit.Event{
			TS:     time.Now().Unix(),
			Action: audit.ActionFollow,
			UserID: userID,
			URL:    originalURL,
		})
	}
}

// ShortenJSONUrl сокращает URL в JSON-формате.
//
// Endpoint: POST /api/shorten
// Content-Type: application/json
//
// Request: model.ShortenRequest
// Response: 201 Created, model.ShortenResponse (JSON).
//
// Возможные ответы:
//   - 400 Bad Request — неверный Content-Type или некорректный JSON
//   - 409 Conflict — URL уже был сохранён ранее (возвращается short URL)
//   - 500 Internal Server Error — внутренняя ошибка
func (h *Handlers) ShortenJSONUrl(c *gin.Context) {
	if !strings.Contains(c.ContentType(), "application/json") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid content type"})
		return
	}

	var req model.ShortenRequest
	//  if err := c.ShouldBindJSON(&req); err != nil {
	//	c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
	//	return
	//  }

	dec := json.NewDecoder(c.Request.Body)
	if err := dec.Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}
	// TODO: перенести в service
	if req.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	userID := c.GetString(auth.ContextUserIDKey)

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
	if h.audit != nil {
		h.audit.Publish(c.Request.Context(), audit.Event{
			TS:     time.Now().Unix(),
			Action: audit.ActionShorten,
			UserID: userID,
			URL:    req.URL,
		})
	}
	// c.JSON(http.StatusCreated, resp)
}

// ShortenURLBatch сокращает набор URL одной пачкой.
//
// Endpoint: POST /api/shorten/batch
// Content-Type: application/json
//
// Request: []model.BatchRequest
// Response: 201 Created, []model.BatchResponse (JSON).
//
// Возможные ответы:
//   - 400 Bad Request — неверный Content-Type, пустой batch или некорректный JSON
//   - 500 Internal Server Error — внутренняя ошибка
func (h *Handlers) ShortenURLBatch(c *gin.Context) {
	if !strings.Contains(c.ContentType(), "application/json") {
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

	userID := c.GetString(auth.ContextUserIDKey)

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

// GetUserURLs возвращает список URL текущего пользователя.
//
// Endpoint: GET /api/user/urls
//
// Ответы:
//   - 200 OK — []model.UserURLResponse (JSON)
//   - 204 No Content — если у пользователя нет URL (или все удалены)
//   - 500 Internal Server Error — внутренняя ошибка
func (h *Handlers) GetUserURLs(c *gin.Context) {
	userID := c.GetString(auth.ContextUserIDKey)

	urls, err := h.service.FindByUserID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": http.StatusText(http.StatusInternalServerError)})
		return
	}

	resp := make([]model.UserURLResponse, 0, len(urls))
	for _, u := range urls {
		if u.IsDeleted {
			continue
		}
		resp = append(resp, model.UserURLResponse{
			ShortURL:    u.Short,
			OriginalURL: u.Original,
		})
	}

	if len(resp) == 0 {
		c.Status(http.StatusNoContent)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// DeleteUserURLs асинхронно помечает набор URL пользователя как удалённые.
//
// Endpoint: DELETE /api/user/urls
// Content-Type: application/json
//
// Request body: []string (список идентификаторов коротких URL).
//
// Ответы:
//   - 202 Accepted — запрос принят в обработку
//   - 400 Bad Request — неверный Content-Type, некорректный JSON или пустой список
//   - 500 Internal Server Error — внутренняя ошибка
func (h *Handlers) DeleteUserURLs(c *gin.Context) {
	if !strings.Contains(c.ContentType(), "application/json") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid content type"})
		return
	}

	userID := c.GetString(auth.ContextUserIDKey)

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
