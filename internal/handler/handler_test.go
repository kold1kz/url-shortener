package handler

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"url-shortener/internal/model"
	"url-shortener/internal/repository"
	"url-shortener/internal/service"

	"github.com/stretchr/testify/assert"
)

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
				body:       "Invalid content type\n",
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
				body:       "URL cannot be empty\n",
			},
		},
		{
			name:   "invalid method GET",
			method: "GET",
			body:   "https://example.com",
			headers: map[string]string{
				"Content-Type": "text/plain",
			},
			want: want{
				statusCode: http.StatusBadRequest,
				body:       "Method not allowed\n",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Используем реальные зависимости (проще для начала)
			repo := repository.NewInMemoryURLRepository()
			service := service.NewURLService(repo, "http://localhost:8080")
			handler := NewHandler(service)

			req := httptest.NewRequest(test.method, "/", strings.NewReader(test.body))
			for key, value := range test.headers {
				req.Header.Set(key, value)
			}

			w := httptest.NewRecorder()
			handler.ShortenURL(w, req)

			result := w.Result()
			defer result.Body.Close()

			// Проверка статус кода
			assert.Equal(t, test.want.statusCode, result.StatusCode)

			// Проверка Content-Type
			assert.NotContains(t, test.want.contentType, result.Header.Get("Content-Type"))

			// Проверка тела ответа
			bodyResult, err := io.ReadAll(result.Body)
			if err != nil {
				t.Fatalf("error reading response body: %v", err)
			}
			bodyStr := string(bodyResult)
			assert.Contains(t, test.want.body, bodyStr)
		})
	}
}

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
			// Мокаем сервис
			mockService := &MockService{}
			h := NewHandler(mockService)

			// Создаём запрос
			req := httptest.NewRequest(test.method, "/", strings.NewReader(test.body))
			for k, v := range test.headers {
				req.Header.Set(k, v)
			}

			// Записываем ответ
			w := httptest.NewRecorder()
			h.ShortenURL(w, req)

			res := w.Result()
			defer res.Body.Close()

			// Проверка статус кода
			assert.Equal(t, test.want.statusCode, res.StatusCode)

			// Проверка Content-Type (если ожидается)
			assert.Equal(t, test.want.contentType, res.Header.Get("Content-Type"))

			// Проверка тела
			bodyBytes, _ := io.ReadAll(res.Body)
			bodyStr := strings.TrimSpace(string(bodyBytes))
			assert.Contains(t, test.want.body, bodyStr)
		})
	}
}

func TestGetOriginalURL(t *testing.T) {
	type want struct {
		statusCode int
		location   string
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
			},
		},
		{
			name:   "url not found",
			method: "GET",
			url:    "/nonexistent",
			want: want{
				statusCode: http.StatusBadRequest,
				location:   "",
			},
		},
		{
			name:   "invalid method POST",
			method: "POST",
			url:    "/abc123",
			want: want{
				statusCode: http.StatusBadRequest,
				location:   "",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Мокаем сервис
			mockService := &MockService{}
			h := NewHandler(mockService)

			mux := http.NewServeMux()
			mux.HandleFunc("/{id}", h.GetOriginalURL)

			// Создаём запрос
			req := httptest.NewRequest(test.method, test.url, nil)

			// Записываем ответ
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

			// Проверка статус кода
			assert.Equal(t, test.want.statusCode, res.StatusCode)

			// Проверка ответа
			assert.Equal(t, test.want.location, res.Header.Get("Location"))
		})
	}
}
