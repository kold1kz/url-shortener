package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"url-shortener/internal/auth"
	"url-shortener/internal/middleware"
	"url-shortener/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type MockService struct {
	deletedUserID string
	deletedIDs    []string
}

type MockServiceWithError struct{}

type MockServiceEmptyUserURLs struct{}

type MockStatsService struct {
	stats *model.StatsResponse
	err   error
}

func mustParseCIDR(t *testing.T, cidr string) *net.IPNet {
	t.Helper()

	_, subnet, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("parse cidr: %v", err)
	}
	return subnet
}

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

func (m *MockServiceWithError) DeleteUserURLs(ctx context.Context, userID string, ids []string) error {
	return nil
}

func (m *MockServiceWithError) GetStats(ctx context.Context) (*model.StatsResponse, error) {
	return nil, errors.New("service error")
}

func (m *MockServiceWithError) Close() error {
	return nil
}

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

func (m *MockServiceEmptyUserURLs) DeleteUserURLs(ctx context.Context, userID string, ids []string) error {
	return nil
}

func (m *MockServiceEmptyUserURLs) GetStats(ctx context.Context) (*model.StatsResponse, error) {
	return &model.StatsResponse{
		URLs:  0,
		Users: 0,
	}, nil
}

func (m *MockServiceEmptyUserURLs) Close() error {
	return nil
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

func (m *MockService) DeleteUserURLs(ctx context.Context, userID string, ids []string) error {
	m.deletedUserID = userID
	m.deletedIDs = append([]string(nil), ids...)
	return nil
}

func (m *MockService) GetStats(ctx context.Context) (*model.StatsResponse, error) {
	return &model.StatsResponse{
		URLs:  1,
		Users: 1,
	}, nil
}

func (m *MockService) Close() error {
	return nil
}

func (m *MockStatsService) ShortenURL(ctx context.Context, original string, userID string) (*model.URL, error) {
	return nil, nil
}

func (m *MockStatsService) GetOriginalURL(ctx context.Context, id string) (string, error) {
	return "", nil
}

func (m *MockStatsService) ShortenURLBatch(ctx context.Context, batch []model.BatchRequest, userID string) ([]model.BatchResponse, error) {
	return nil, nil
}

func (m *MockStatsService) FindByUserID(ctx context.Context, userID string) ([]*model.URL, error) {
	return nil, nil
}

func (m *MockStatsService) DeleteUserURLs(ctx context.Context, userID string, ids []string) error {
	return nil
}

func (m *MockStatsService) GetStats(ctx context.Context) (*model.StatsResponse, error) {
	return m.stats, m.err
}

func (m *MockStatsService) Close() error {
	return nil
}

func setupGinRouter(handler *Handlers) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	authGroup := router.Group("/")
	authGroup.Use(middleware.UserAuth())

	authGroup.POST("/", handler.ShortenURL)
	authGroup.GET("/:id", handler.GetOriginalURL)
	authGroup.POST("/api/shorten", handler.ShortenJSONUrl)
	authGroup.POST("/api/shorten/batch", handler.ShortenURLBatch)
	authGroup.GET("/api/user/urls", handler.GetUserURLs)
	authGroup.DELETE("/api/user/urls", handler.DeleteUserURLs)

	router.GET("/api/internal/stats", handler.GetInternalStats)

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
			handler := NewHandler(mockService, nil, nil)

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
			h := NewHandler(mockService, nil, nil)

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
			h := NewHandler(mockService, nil, nil)

			router := setupGinRouter(h)

			req := httptest.NewRequest(test.method, test.url, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

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
			handler := NewHandler(mockService, nil, nil)

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
			h := NewHandler(mockService, nil, nil)

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
			h := NewHandler(mockService, nil, nil)

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

			var expected, actual []interface{}
			_ = json.Unmarshal([]byte(test.want.body), &expected)
			_ = json.Unmarshal([]byte(bodyStr), &actual)

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
			handler := NewHandler(mockService, nil, nil)

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
	tests := []struct {
		name string
	}{
		{
			name: "no_cookie_new_user_no_urls",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockService := &MockServiceEmptyUserURLs{}
			h := NewHandler(mockService, nil, nil)
			router := setupGinRouter(h)

			req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, http.StatusNoContent, res.StatusCode)

			setCookie := res.Header.Get("Set-Cookie")
			assert.NotEmpty(t, setCookie)
			assert.Contains(t, setCookie, auth.CookieName()+"=")
		})
	}
}

func TestGetUserURLs_Success(t *testing.T) {
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
			h := NewHandler(mockService, nil, nil)
			router := setupGinRouter(h)

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
		useService string
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
				h = NewHandler(&MockServiceWithError{}, nil, nil)
			default:
				h = NewHandler(&MockService{}, nil, nil)
			}

			router := setupGinRouter(h)

			req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)

			if test.useService == "error" {
				_, token, err := auth.GetOrCreateUserIDFromCookie("")
				assert.NoError(t, err)
				req.AddCookie(&http.Cookie{
					Name:  auth.CookieName(),
					Value: token,
				})
			}

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

func TestDeleteUserURLs_Success(t *testing.T) {
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
			name:   "valid request with ids",
			method: "DELETE",
			body:   `["id1","id2","id3"]`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			want: want{
				statusCode: http.StatusAccepted,
				body:       "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockService{}
			h := NewHandler(mockService, nil, nil)
			router := setupGinRouter(h)

			req := httptest.NewRequest(tt.method, "/api/user/urls", strings.NewReader(tt.body))
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			_, token, err := auth.GetOrCreateUserIDFromCookie("")
			assert.NoError(t, err)

			req.AddCookie(&http.Cookie{
				Name:  auth.CookieName(),
				Value: token,
			})

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.want.statusCode, res.StatusCode)

			bodyBytes, err := io.ReadAll(res.Body)
			assert.NoError(t, err)
			bodyStr := strings.TrimSpace(string(bodyBytes))
			assert.Equal(t, tt.want.body, bodyStr)

			assert.NotEmpty(t, mockService.deletedUserID)
			assert.Equal(t, []string{"id1", "id2", "id3"}, mockService.deletedIDs)
		})
	}
}

func TestDeleteUserURLs_ValidationErrors(t *testing.T) {
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
			method: "DELETE",
			body:   `["id1","id2"]`,
			headers: map[string]string{
				"Content-Type": "text/plain",
			},
			want: want{
				statusCode: http.StatusBadRequest,
				body:       `{"error":"Invalid content type"}`,
			},
		},
		{
			name:   "invalid json format",
			method: "DELETE",
			body:   `not json`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			want: want{
				statusCode: http.StatusBadRequest,
				body:       `{"error":"Invalid JSON format"}`,
			},
		},
		{
			name:   "empty ids array",
			method: "DELETE",
			body:   `[]`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			want: want{
				statusCode: http.StatusBadRequest,
				body:       `{"error":"Empty batch"}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockService{}
			h := NewHandler(mockService, nil, nil)
			router := setupGinRouter(h)

			req := httptest.NewRequest(tt.method, "/api/user/urls", strings.NewReader(tt.body))
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			_, token, err := auth.GetOrCreateUserIDFromCookie("")
			assert.NoError(t, err)
			req.AddCookie(&http.Cookie{
				Name:  auth.CookieName(),
				Value: token,
			})

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.want.statusCode, res.StatusCode)

			bodyBytes, err := io.ReadAll(res.Body)
			assert.NoError(t, err)
			bodyStr := strings.TrimSpace(string(bodyBytes))
			assert.Equal(t, tt.want.body, bodyStr)
		})
	}
}

func TestGetInternalStats_ForbiddenWhenTrustedSubnetIsNil(t *testing.T) {
	h := NewHandler(&MockStatsService{}, nil, nil)
	router := setupGinRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
	req.Header.Set("X-Real-IP", "127.0.0.1")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetInternalStats_ForbiddenWhenHeaderIPInvalid(t *testing.T) {
	h := NewHandler(&MockStatsService{}, nil, mustParseCIDR(t, "127.0.0.0/8"))
	router := setupGinRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
	req.Header.Set("X-Real-IP", "not-an-ip")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetInternalStats_ForbiddenWhenIPOutsideTrustedSubnet(t *testing.T) {
	h := NewHandler(&MockStatsService{}, nil, mustParseCIDR(t, "127.0.0.0/8"))
	router := setupGinRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
	req.Header.Set("X-Real-IP", "10.0.0.1")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetInternalStats_ServiceError(t *testing.T) {
	h := NewHandler(&MockStatsService{
		err: errors.New("stats error"),
	}, nil, mustParseCIDR(t, "127.0.0.0/8"))
	router := setupGinRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
	req.Header.Set("X-Real-IP", "127.0.0.1")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.JSONEq(t, `{"error":"Internal Server Error"}`, w.Body.String())
}

func TestGetInternalStats_Success(t *testing.T) {
	h := NewHandler(&MockStatsService{
		stats: &model.StatsResponse{
			URLs:  7,
			Users: 3,
		},
	}, nil, mustParseCIDR(t, "127.0.0.0/8"))
	router := setupGinRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
	req.Header.Set("X-Real-IP", "127.0.0.1")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"urls":7,"users":3}`, w.Body.String())
}
