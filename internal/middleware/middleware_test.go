package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"url-shortener/internal/auth"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestHTTPLoggerMiddleware_EmitsRequestAndResponseLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core).Sugar()

	r := gin.New()
	r.Use(HTTPLoggerMiddleware(logger))
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	entries := logs.All()
	require.Len(t, entries, 2)

	require.Equal(t, "HTTP Request", entries[0].Message)
	require.Equal(t, "HTTP Response", entries[1].Message)

	// Проверим ключевые поля
	fieldsReq := entries[0].ContextMap()
	require.Equal(t, "/ping", fieldsReq["url"])
	require.Equal(t, "GET", fieldsReq["method"])
	require.Contains(t, fieldsReq, "duration")

	fieldsResp := entries[1].ContextMap()
	require.Equal(t, int64(http.StatusOK), fieldsResp["status"])
	require.Contains(t, fieldsResp, "size")
}

func TestInitLogger_ReturnsNonNil(t *testing.T) {
	l := InitLogger()
	require.NotNil(t, l)
}

func TestGzipMiddleware_ResponseCompressedWhenClientAcceptsGzip(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(GzipMiddleware())
	r.GET("/data", func(c *gin.Context) {
		// намеренно не выставляем Content-Type, чтобы проверить дефолт
		c.String(http.StatusOK, "hello")
	})

	req := httptest.NewRequest(http.MethodGet, "/data", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	require.Equal(t, "Accept-Encoding", rec.Header().Get("Vary"))

	// тело должно быть gzip
	got := ungzipBody(t, rec.Body.Bytes())
	require.Equal(t, "hello", got)
}

func TestGzipMiddleware_RequestDecompressedWhenContentEncodingGzip(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(GzipMiddleware())
	r.POST("/echo", func(c *gin.Context) {
		b, err := c.GetRawData()
		require.NoError(t, err)
		c.String(http.StatusOK, string(b))
	})

	orig := []byte("payload")
	gz := gzipBytes(t, orig)

	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(gz))
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Content-Type", "text/plain")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "payload", rec.Body.String())
}

func TestGzipMiddleware_InvalidGzipBody_Returns500AndAborts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(GzipMiddleware())
	r.POST("/echo", func(c *gin.Context) {
		// если middleware не abort'нул, тест провалится
		c.String(http.StatusOK, "should-not-happen")
	})

	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader([]byte("not-gzip")))
	req.Header.Set("Content-Encoding", "gzip")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, rec.Body.String(), "Failed to decompress request")
}

func TestUserAuth_NoCookie_SetsUserIDAndCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(UserAuth())
	r.GET("/me", func(c *gin.Context) {
		uid := c.GetString(auth.ContextUserIDKey)
		require.NotEmpty(t, uid)
		c.String(http.StatusOK, uid)
	})

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	// middleware должен выставить Set-Cookie (новый токен)
	setCookie := rec.Header().Get("Set-Cookie")
	require.NotEmpty(t, setCookie)
}

func TestUserAuth_InvalidCookie_Returns401(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(UserAuth())
	r.GET("/me", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	// заведомо мусорный токен
	req.AddCookie(&http.Cookie{Name: auth.CookieName(), Value: "invalid-token"})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUserAuth_ValidCookie_AllowsRequestAndSetsContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// получаем валидный токен через твой же auth пакет
	wantUserID, token, err := func() (string, string, error) {
		uid, tok, e := auth.GetOrCreateUserIDFromCookie("")
		return uid, tok, e
	}()
	require.NoError(t, err)
	require.NotEmpty(t, wantUserID)
	require.NotEmpty(t, token)

	r := gin.New()
	r.Use(UserAuth())
	r.GET("/me", func(c *gin.Context) {
		got := c.GetString(auth.ContextUserIDKey)
		require.Equal(t, wantUserID, got)
		c.String(http.StatusOK, got)
	})

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName(), Value: token})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, wantUserID, rec.Body.String())
}

func gzipBytes(t *testing.T, b []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write(b)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func ungzipBody(t *testing.T, b []byte) string {
	t.Helper()

	zr, err := gzip.NewReader(bytes.NewReader(b))
	require.NoError(t, err)
	defer zr.Close()

	out, err := io.ReadAll(zr)
	require.NoError(t, err)
	return string(out)
}
