package handler

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"url-shortener/internal/database"
)

type MockPinger struct {
	pingError error
}

func (m *MockPinger) Ping() error {
	return m.pingError
}

func TestPingHandler(t *testing.T) {
	type want struct {
		statusCode int
		body       string
	}

	tests := []struct {
		name        string
		db          database.Pinger
		want        want
		description string
	}{
		{
			name: "successful ping",
			db:   &MockPinger{pingError: nil},
			want: want{
				statusCode: http.StatusOK,
				body:       "",
			},
		},
		{
			name: "database connection failed",
			db:   &MockPinger{pingError: errors.New("connection failed")},
			want: want{
				statusCode: http.StatusInternalServerError,
				body:       `{"error":"Database connection failed"}`,
			},
		},
		{
			name: "database not configured",
			db:   nil,
			want: want{
				statusCode: http.StatusInternalServerError,
				body:       `{"error":"Database connection failed"}`,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewPingHandler(test.db)

			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.GET("/ping", handler.Ping)

			req := httptest.NewRequest("GET", "/ping", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			result := w.Result()
			defer result.Body.Close()

			assert.Equal(t, test.want.statusCode, result.StatusCode, test.description)

			bodyResult, err := io.ReadAll(result.Body)
			require.NoError(t, err)

			bodyStr := strings.TrimSpace(string(bodyResult))
			if test.want.body != "" {
				assert.Equal(t, test.want.body, bodyStr, test.description)
			} else {
				assert.Empty(t, bodyStr, test.description)
			}
		})
	}
}
