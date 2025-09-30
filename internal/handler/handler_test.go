package handler

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"url-shortener/internal/model"
)

type MockService struct{}

func (m *MockService) ShortenURL(original string) (*model.URL, error) {
	return &model.URL{
		ID:       "abc123",
		Original: original,
		Short:    "http://localhost:8080/abc123",
	}, nil
}

func (m *MockService) GetOriginalURL(id string) (string, error) {
	if id == "nonexistent" {
		return "", errors.New("not found")
	}
	return "https://example.com", nil
}

func setupGinRouter(handler *Handlers) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.POST("/", handler.ShortenURL)
	router.GET("/:id", handler.GetOriginalURL)

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
				body:       `{"error":"Invalid server error"}`,
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
