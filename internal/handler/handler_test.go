package handler

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"url-shortener/internal/auth"
	"url-shortener/internal/model"
)

type MockService struct{}

type MockServiceWithError struct{}

func (m *MockServiceWithError) ShortenURL(ctx context.Context, original string, userID string) (*model.URL, error) {
	return nil, errors.New("service error")
}

func (m *MockServiceWithError) GetOriginalURL(ctx context.Context, id string) (string, error) {
	return "", errors.New("service error")
}

func (m *MockServiceWithError) ShortenURLBatch(ctx context.Context, batch []model.BatchRequest, userID string) ([]model.BatchResponse, error) {
	return nil, errors.New("batch processing failed")
}

func (m *MockServiceWithError) FindByUserID(ctx context.Context, userID string) ([]*model.URL, error) {
	return nil, errors.New("service error")
}

type MockServiceEmptyUserURLs struct{}

func (m *MockServiceEmptyUserURLs) ShortenURL(ctx context.Context, original string, userID string) (*model.URL, error) {
	return nil, nil
}

func (m *MockServiceEmptyUserURLs) GetOriginalURL(ctx context.Context, id string) (string, error) {
	return "", nil
}

func (m *MockServiceEmptyUserURLs) ShortenURLBatch(ctx context.Context, batch []model.BatchRequest, userID string) ([]model.BatchResponse, error) {
	return nil, nil
}

func (m *MockServiceEmptyUserURLs) FindByUserID(ctx context.Context, userID string) ([]*model.URL, error) {
	return []*model.URL{}, nil
}
func (m *MockService) ShortenURL(ctx context.Context, original string, userID string) (*model.URL, error) {
	return &model.URL{
		ID:       "abc123",
		Original: original,
		Short:    "http://localhost:8080/abc123",
		UserID:   userID,
	}, nil
}

func (m *MockService) GetOriginalURL(ctx context.Context, id string) (string, error) {
	if id == "nonexistent" {
		return "", errors.New("not found")
	}
	return "https://example.com", nil
}

func (m *MockService) ShortenURLBatch(ctx context.Context, batch []model.BatchRequest, userID string) ([]model.BatchResponse, error) {
	responses := make([]model.BatchResponse, 0, len(batch))

	for _, item := range batch {
		responses = append(responses, model.BatchResponse{
			CorrelationID: item.CorrelationID,
			ShortURL:      "http://localhost:8080/" + item.CorrelationID + "_short",
		})
	}

	return responses, nil
}

func (m *MockService) FindByUserID(ctx context.Context, userID string) ([]*model.URL, error) {
	return []*model.URL{
		{
			ID:       "1",
			Original: "https://example.com/1",
			Short:    "http://localhost:8080/1",
			UserID:   userID,
		},
	}, nil
}

func setupGinRouter(handler *Handlers) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.POST("/", handler.ShortenURL)
	router.GET("/:id", handler.GetOriginalURL)
	router.POST("/api/shorten", handler.ShortenJSONUrl)
	router.POST("/api/shorten/batch", handler.ShortenURLBatch)
	router.GET("/api/user/urls", handler.GetUserURLs)

	return router
}

func TestShortenURL(t *testing.T) {
	type want struct {
		contentType string
		statusCode  int
		body        string
	}

	tests := []struct {
		name    string
		method  string
		body    string
		headers map[string]string
		want    want
	}{
		{
			name:   "invalid content type",
			method: "POST",
			body:   "https://example.com",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			want: want{
				statusCode: http.StatusBadRequest,
				body:       `{"error":"Invalid content type"}`,
			},
		},
		{
			name:   "empty url",
			method: "POST",
			body:   "",
			headers: map[string]string{
				"Content-Type": "text/plain",
			},
			want: want{
				statusCode: http.StatusBadRequest,
				body:       `{"error":"URL cannot be empty"}`,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockService := &MockService{}
			handler := NewHandler(mockService)

			router := setupGinRouter(handler)

			req := httptest.NewRequest(test.method, "/", strings.NewReader(test.body))
			for key, value := range test.headers {
				req.Header.Set(key, value)
			}

			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			result := w.Result()
			defer result.Body.Close()

			assert.Equal(t, test.want.statusCode, result.StatusCode)

			bodyResult, err := io.ReadAll(result.Body)
			assert.NoError(t, err)

			bodyStr := strings.TrimSpace(string(bodyResult))
			assert.Equal(t, test.want.body, bodyStr)
		})
	}
}

func TestShortenUrlMoke(t *testing.T) {
	type want struct {
		contentType string
		statusCode  int
		body        string
	}

	tests := []struct {
		name    string
		method  string
		body    string
		headers map[string]string
		want    want
	}{
		{
			name:   "success work shorten url",
			method: "POST",
			body:   "https://example.com",
			headers: map[string]string{
				"Content-Type": "text/plain",
			},
			want: want{
				contentType: "text/plain",
				statusCode:  http.StatusCreated,
				body:        "http://localhost:8080/abc123",
			},
		},
		{
			name:   "check content type 2",
			method: "POST",
			body:   "https://example.com",
			headers: map[string]string{
				"Content-Type": "text/plain; application/json",
			},
			want: want{
				contentType: "text/plain",
				statusCode:  http.StatusCreated,
				body:        "http://localhost:8080/abc123",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockService := &MockService{}
			h := NewHandler(mockService)

			router := setupGinRouter(h)

			req := httptest.NewRequest(test.method, "/", strings.NewReader(test.body))
			for k, v := range test.headers {
				req.Header.Set(k, v)
			}

			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, test.want.statusCode, res.StatusCode)

			assert.Equal(t, test.want.contentType, res.Header.Get("Content-Type"))

			bodyBytes, _ := io.ReadAll(res.Body)
			bodyStr := strings.TrimSpace(string(bodyBytes))
			assert.Equal(t, test.want.body, bodyStr)
		})
	}
}

func TestGetOriginalURL(t *testing.T) {
	type want struct {
		statusCode int
		location   string
		body       string
	}

	tests := []struct {
		name   string
		method string
		url    string
		want   want
	}{
		{
			name:   "successful redirect",
			method: "GET",
			url:    "/abc123",
			want: want{
				statusCode: http.StatusTemporaryRedirect,
				location:   "https://example.com",
				body:       "https://example.com",
			},
		},
		{
			name:   "url not found",
			method: "GET",
			url:    "/nonexistent",
			want: want{
				statusCode: http.StatusInternalServerError,
				body:       `{"error":"Internal Server Error"}`,
			},
		},
		{
			name:   "invalid method POST",
			method: "POST",
			url:    "/abc123",
			want: want{
				statusCode: http.StatusNotFound,
				body:       "404 page not found",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockService := &MockService{}
			h := NewHandler(mockService)

			router := setupGinRouter(h)

			req := httptest.NewRequest(test.method, test.url, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

			// Проверка статус кода
			assert.Equal(t, test.want.statusCode, res.StatusCode)

			if test.want.location != "" {
				assert.Equal(t, test.want.location, res.Header.Get("Location"))
			}

			bodyBytes, _ := io.ReadAll(res.Body)
			bodyStr := strings.TrimSpace(string(bodyBytes))
			if test.want.body != "" {
				assert.Equal(t, test.want.body, bodyStr)
			}
		})
	}
}

func TestShortenJsonURL(t *testing.T) {
	type want struct {
		contentType string
		statusCode  int
		body        string
	}

	tests := []struct {
		name    string
		method  string
		body    string
		headers map[string]string
		want    want
	}{
		{
			name:   "invalid content type",
			method: "POST",
			body:   `{"url": "https://example.com"}`,
			headers: map[string]string{
				"Content-Type": "text/plain",
			},
			want: want{
				statusCode: http.StatusBadRequest,
				body:       `{"error":"Invalid content type"}`,
			},
		},
		{
			name:   "empty json url",
			method: "POST",
			body:   `{"url": ""}`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			want: want{
				statusCode: http.StatusBadRequest,
				body:       `{"error":"Invalid JSON format"}`,
			},
		},
		{
			name:   "invalid url key",
			method: "POST",
			body:   `{"urlss": "https://practicum.yandex.ru"}`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			want: want{
				statusCode: http.StatusBadRequest,
				body:       `{"error":"Invalid JSON format"}`,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockService := &MockService{}
			handler := NewHandler(mockService)

			router := setupGinRouter(handler)

			req := httptest.NewRequest(test.method, "/api/shorten", strings.NewReader(test.body))
			for key, value := range test.headers {
				req.Header.Set(key, value)
			}

			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			result := w.Result()
			defer result.Body.Close()

			assert.Equal(t, test.want.statusCode, result.StatusCode)

			bodyResult, err := io.ReadAll(result.Body)
			assert.NoError(t, err)

			bodyStr := strings.TrimSpace(string(bodyResult))
			assert.Equal(t, test.want.body, bodyStr)
		})
	}
}

func TestShortenJsonURLMoke(t *testing.T) {
	type want struct {
		contentType string
		statusCode  int
		body        string
	}

	tests := []struct {
		name    string
		method  string
		body    string
		headers map[string]string
		want    want
	}{
		{
			name:   "success work shorten url",
			method: "POST",
			body:   `{"url": "https://example.com"}`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			want: want{
				contentType: "application/json",
				statusCode:  http.StatusCreated,
				body:        `{"result":"http://localhost:8080/abc123"}`,
			},
		},
		{
			name:   "check content type 2",
			method: "POST",
			body:   `{"url": "https://example.com"}`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			want: want{
				contentType: "application/json",
				statusCode:  http.StatusCreated,
				body:        `{"result":"http://localhost:8080/abc123"}`,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockService := &MockService{}
			h := NewHandler(mockService)

			router := setupGinRouter(h)

			req := httptest.NewRequest(test.method, "/api/shorten", strings.NewReader(test.body))
			for k, v := range test.headers {
				req.Header.Set(k, v)
			}

			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, test.want.statusCode, res.StatusCode)

			assert.Equal(t, test.want.contentType, res.Header.Get("Content-Type"))

			bodyBytes, _ := io.ReadAll(res.Body)
			bodyStr := strings.TrimSpace(string(bodyBytes))
			assert.Equal(t, test.want.body, bodyStr)
		})
	}
}

func TestShortenURLBatch_Success(t *testing.T) {
	type want struct {
		contentType string
		statusCode  int
		body        string
	}

	tests := []struct {
		name    string
		method  string
		body    string
		headers map[string]string
		want    want
	}{
		{
			name:   "successful batch shortening",
			method: "POST",
			body: `[
				{
					"correlation_id": "1",
					"original_url": "https://example.com/page1"
				},
				{
					"correlation_id": "2", 
					"original_url": "https://example.com/page2"
				},
				{
					"correlation_id": "3",
					"original_url": "https://example.com/page3"
				}
			]`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			want: want{
				contentType: "application/json",
				statusCode:  http.StatusCreated,
				body: `[
					{
						"correlation_id": "1",
						"short_url": "http://localhost:8080/1_short"
					},
					{
						"correlation_id": "2",
						"short_url": "http://localhost:8080/2_short"
					},
					{
						"correlation_id": "3", 
						"short_url": "http://localhost:8080/3_short"
					}
				]`,
			},
		},
		{
			name:   "single item batch",
			method: "POST",
			body: `[
				{
					"correlation_id": "single",
					"original_url": "https://example.com/single"
				}
			]`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			want: want{
				contentType: "application/json",
				statusCode:  http.StatusCreated,
				body: `[
					{
						"correlation_id": "single",
						"short_url": "http://localhost:8080/single_short"
					}
				]`,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockService := &MockService{}
			h := NewHandler(mockService)

			router := setupGinRouter(h)

			req := httptest.NewRequest(test.method, "/api/shorten/batch", strings.NewReader(test.body))
			for k, v := range test.headers {
				req.Header.Set(k, v)
			}

			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, test.want.statusCode, res.StatusCode)
			assert.Equal(t, test.want.contentType, res.Header.Get("Content-Type"))

			bodyBytes, _ := io.ReadAll(res.Body)
			bodyStr := strings.TrimSpace(string(bodyBytes))

			// Для JSON сравниваем нормализованные версии
			var expected, actual []interface{}
			json.Unmarshal([]byte(test.want.body), &expected)
			json.Unmarshal([]byte(bodyStr), &actual)

			assert.Equal(t, expected, actual)
		})
	}
}

func TestShortenURLBatch_ValidationErrors(t *testing.T) {
	type want struct {
		statusCode int
		body       string
	}

	tests := []struct {
		name    string
		method  string
		body    string
		headers map[string]string
		want    want
	}{
		{
			name:   "invalid content type",
			method: "POST",
			body: `[
				{
					"correlation_id": "1",
					"original_url": "https://example.com/page1"
				}
			]`,
			headers: map[string]string{
				"Content-Type": "text/plain",
			},
			want: want{
				statusCode: http.StatusBadRequest,
				body:       `{"error":"Invalid content type"}`,
			},
		},
		{
			name:   "empty batch",
			method: "POST",
			body:   `[]`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			want: want{
				statusCode: http.StatusBadRequest,
				body:       `{"error":"Empty batch"}`,
			},
		},
		{
			name:   "empty correlation_id",
			method: "POST",
			body: `[
				{
					"correlation_id": "",
					"original_url": "https://example.com/page1"
				}
			]`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			want: want{
				statusCode: http.StatusBadRequest,
				body:       `{"error":"Invalid JSON format"}`,
			},
		},
		{
			name:   "empty original_url",
			method: "POST",
			body: `[
				{
					"correlation_id": "1",
					"original_url": ""
				}
			]`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			want: want{
				statusCode: http.StatusBadRequest,
				body:       `{"error":"Invalid JSON format"}`,
			},
		},
		{
			name:   "invalid JSON format",
			method: "POST",
			body:   `invalid json`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			want: want{
				statusCode: http.StatusBadRequest,
				body:       `{"error":"Invalid JSON format"}`,
			},
		},
		{
			name:   "missing correlation_id field",
			method: "POST",
			body: `[
				{
					"original_url": "https://example.com/page1"
				}
			]`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			want: want{
				statusCode: http.StatusBadRequest,
				body:       `{"error":"Invalid JSON format"}`,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockService := &MockService{}
			handler := NewHandler(mockService)

			router := setupGinRouter(handler)

			req := httptest.NewRequest(test.method, "/api/shorten/batch", strings.NewReader(test.body))
			for key, value := range test.headers {
				req.Header.Set(key, value)
			}

			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			result := w.Result()
			defer result.Body.Close()

			assert.Equal(t, test.want.statusCode, result.StatusCode)

			bodyResult, err := io.ReadAll(result.Body)
			assert.NoError(t, err)

			bodyStr := strings.TrimSpace(string(bodyResult))
			assert.Equal(t, test.want.body, bodyStr)
		})
	}
}

func TestGetUserURLs_NoCookieAndEmpty(t *testing.T) {
	type want struct {
		statusCode   int
		body         string
		expectCookie bool
	}

	tests := []struct {
		name string
	}{
		{
			name: "no_cookie_new_user_no_urls",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// сервис, который возвращает пустой список ссылок
			mockService := &MockServiceEmptyUserURLs{}
			h := NewHandler(mockService)
			router := setupGinRouter(h)

			req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

			// Новый пользователь без куки и без ссылок -> 204 No Content
			assert.Equal(t, http.StatusNoContent, res.StatusCode)

			// При этом сервер должен выдать auth-куку
			setCookie := res.Header.Get("Set-Cookie")
			assert.NotEmpty(t, setCookie)
			assert.Contains(t, setCookie, auth.CookieName()+"=")
		})
	}
}

func TestGetUserURLs_Success(t *testing.T) {
	type want struct {
		statusCode  int
		body        string
		contentType string
	}

	tests := []struct {
		name string
	}{
		{
			name: "valid_cookie_with_urls",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockService := &MockService{}
			h := NewHandler(mockService)
			router := setupGinRouter(h)

			// генерируем валидную куку так же, как делает auth
			_, token, err := auth.GetOrCreateUserIDFromCookie("")
			assert.NoError(t, err)

			req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
			req.AddCookie(&http.Cookie{
				Name:  auth.CookieName(),
				Value: token,
			})

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, http.StatusOK, res.StatusCode)
			assert.Equal(t, "application/json; charset=utf-8", res.Header.Get("Content-Type"))

			bodyBytes, err := io.ReadAll(res.Body)
			assert.NoError(t, err)

			bodyStr := strings.TrimSpace(string(bodyBytes))

			// ожидаемый ответ от MockService.FindByUserID
			expected := `[{"short_url":"http://localhost:8080/1","original_url":"https://example.com/1"}]`
			assert.JSONEq(t, expected, bodyStr)
		})
	}
}

func TestGetUserURLs_ErrorCases(t *testing.T) {
	type want struct {
		statusCode int
		body       string
	}

	tests := []struct {
		name       string
		cookieVal  string
		useService string // "normal" или "error"
		want       want
	}{
		{
			name:       "invalid_cookie_value",
			cookieVal:  "broken-token",
			useService: "normal",
			want: want{
				statusCode: http.StatusUnauthorized,
				body:       "",
			},
		},
		{
			name:       "internal_error_from_service",
			useService: "error",
			// сюда поставим валидную куку внутри теста
			want: want{
				statusCode: http.StatusInternalServerError,
				body:       `{"error":"Internal Server Error"}`,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var h *Handlers

			switch test.useService {
			case "error":
				h = NewHandler(&MockServiceWithError{})
			default:
				h = NewHandler(&MockService{})
			}

			router := setupGinRouter(h)

			req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)

			// для кейса internal error — генерируем валидную куку
			if test.useService == "error" {
				_, token, err := auth.GetOrCreateUserIDFromCookie("")
				assert.NoError(t, err)
				req.AddCookie(&http.Cookie{
					Name:  auth.CookieName(),
					Value: token,
				})
			}

			// для кейса invalid_cookie_value — используем битый токен
			if test.cookieVal != "" {
				req.AddCookie(&http.Cookie{
					Name:  auth.CookieName(),
					Value: test.cookieVal,
				})
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, test.want.statusCode, res.StatusCode)

			if test.want.body != "" {
				bodyBytes, err := io.ReadAll(res.Body)
				assert.NoError(t, err)
				bodyStr := strings.TrimSpace(string(bodyBytes))
				assert.Equal(t, test.want.body, bodyStr)
			}
		})
	}
}
