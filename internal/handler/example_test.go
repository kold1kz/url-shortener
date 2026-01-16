package handler_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"url-shortener/internal/handler"
	"url-shortener/internal/model"

	"github.com/gin-gonic/gin"
)

type fakeService struct{}

func (fakeService) ShortenURL(ctx context.Context, original, userID string) (*model.URL, error) {
	return &model.URL{
		ID:       "abc",
		Original: original,
		Short:    "http://localhost:8080/abc",
		UserID:   userID,
	}, nil
}
func (fakeService) GetOriginalURL(ctx context.Context, id string) (string, error) {
	if id == "abc" {
		return "https://example.com", nil
	}
	return "", nil
}
func (fakeService) ShortenURLBatch(ctx context.Context, batch []model.BatchRequest, userID string) ([]model.BatchResponse, error) {
	out := make([]model.BatchResponse, 0, len(batch))
	for _, it := range batch {
		out = append(out, model.BatchResponse{
			CorrelationID: it.CorrelationID,
			ShortURL:      "http://localhost:8080/" + it.CorrelationID,
		})
	}
	return out, nil
}
func (fakeService) FindByUserID(ctx context.Context, userID string) ([]*model.URL, error) {
	return []*model.URL{
		{ID: "abc", Original: "https://example.com", Short: "http://localhost:8080/abc", UserID: userID},
	}, nil
}
func (fakeService) DeleteUserURLs(ctx context.Context, userID string, ids []string) error { return nil }
func (fakeService) Close() error                                                          { return nil }

func ExampleHandlers_ShortenJSONUrl() {
	gin.SetMode(gin.TestMode)

	h := handler.NewHandler(fakeService{}, nil)

	r := gin.New()
	r.POST("/api/shorten", h.ShortenJSONUrl)

	req := httptest.NewRequest(http.MethodPost, "/api/shorten",
		bytes.NewBufferString(`{"url":"https://example.com"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	fmt.Println(w.Code)
	fmt.Println(strings.TrimSpace(w.Body.String()))

	// Output:
	// 201
	// {"result":"http://localhost:8080/abc"}
}

func ExampleHandlers_GetOriginalURL() {
	gin.SetMode(gin.TestMode)

	h := handler.NewHandler(fakeService{}, nil)

	r := gin.New()
	r.GET("/:id", h.GetOriginalURL)

	req := httptest.NewRequest(http.MethodGet, "/abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	fmt.Println(w.Code)
	fmt.Println(w.Header().Get("Location"))

	// Output:
	// 307
	// https://example.com
}

func ExampleHandlers_GetUserURLs() {
	gin.SetMode(gin.TestMode)

	h := handler.NewHandler(fakeService{}, nil)

	r := gin.New()
	r.GET("/api/user/urls", h.GetUserURLs)

	req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	fmt.Println(w.Code)
	fmt.Println(strings.TrimSpace(w.Body.String()))

	// Output:
	// 200
	// [{"short_url":"http://localhost:8080/abc","original_url":"https://example.com"}]
}
