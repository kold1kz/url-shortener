package middleware

import (
	"compress/gzip"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
	"url-shortener/internal/auth"
)

var gzipWriterPool = sync.Pool{
	New: func() any {
		// BestSpeed почти всегда достаточно и дешевле по CPU/памяти
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return w
	},
}

func HTTPLoggerMiddleware(logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Начало запроса - засекаем время
		start := time.Now()

		// Обрабатываем запрос
		c.Next()

		// Вычисляем затраченное время
		duration := time.Since(start)

		// Читаем размер содержимого ответа
		size := c.Writer.Size()

		logger.Infow("HTTP Request",
			zap.String("url", c.Request.RequestURI),
			zap.String("method", c.Request.Method),
			zap.Duration("duration", duration),
		)
		logger.Infow("HTTP Response",
			zap.Int("status", c.Writer.Status()),
			zap.Int("size", size),
		)
	}
}

func InitLogger() *zap.SugaredLogger {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Printf("Failed to initialize zap logger: %v", err)
		return zap.NewNop().Sugar()
	}

	logger = logger.WithOptions(zap.IncreaseLevel(zap.InfoLevel))
	return logger.Sugar()
}

type compressWriter struct {
	gin.ResponseWriter
	zw *gzip.Writer
}

func newCompressWriter(w gin.ResponseWriter) *compressWriter {
	zw := gzipWriterPool.Get().(*gzip.Writer)
	zw.Reset(w)
	return &compressWriter{
		ResponseWriter: w,
		zw:             zw,
	}
}

func (c *compressWriter) Write(p []byte) (int, error) {
	if c.Header().Get("Content-Type") == "" {
		c.Header().Set("Content-Type", "application/octet-stream")
	}
	return c.zw.Write(p)
}

func (c *compressWriter) WriteString(s string) (int, error) {
	return c.zw.Write([]byte(s))
}

func (c *compressWriter) Close() error {
	err := c.zw.Close()
	c.zw.Reset(io.Discard)
	gzipWriterPool.Put(c.zw)
	c.zw = nil

	return err
}

type compressReader struct {
	io.ReadCloser
	zr *gzip.Reader
}

func newCompressReader(r io.ReadCloser) (*compressReader, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	return &compressReader{
		ReadCloser: r,
		zr:         zr,
	}, nil
}

func (c *compressReader) Read(p []byte) (n int, err error) {
	return c.zr.Read(p)
}

func (c *compressReader) Close() error {
	if err := c.ReadCloser.Close(); err != nil {
		return err
	}
	return c.zr.Close()
}

func GzipMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		acceptEncoding := c.GetHeader("Accept-Encoding")
		supportsGzip := strings.Contains(acceptEncoding, "gzip")

		contentEncoding := c.GetHeader("Content-Encoding")
		sendsGzip := strings.Contains(contentEncoding, "gzip")

		if supportsGzip {
			cw := newCompressWriter(c.Writer)
			defer cw.Close()

			c.Writer = cw
			c.Header("Content-Encoding", "gzip")
			c.Header("Vary", "Accept-Encoding")
		}

		if sendsGzip {
			cr, err := newCompressReader(c.Request.Body)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decompress request"})
				c.Abort()
				return
			}
			defer cr.Close()
			c.Request.Body = cr
		}
		c.Next()
	}
}

//func shouldCompress(contentType string) bool {
//	return strings.Contains(contentType, "application/json") ||
//		strings.Contains(contentType, "text/html") ||
//		strings.Contains(contentType, "text/plain")
//}

func UserAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		rawCookie, err := c.Cookie(auth.CookieName())

		if err != nil || rawCookie == "" {
			userID, newToken, err := auth.GetOrCreateUserIDFromCookie("")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "auth error"})
				c.Abort()
				return
			}
			if newToken != "" {
				c.SetCookie(auth.CookieName(), newToken, 0, "/", "", false, true)
			}
			c.Set(auth.ContextUserIDKey, userID)
			c.Next()
			return
		}

		userID, err := auth.GetUserIDFromCookieStrict(rawCookie)
		if err != nil || userID == "" {
			c.Status(http.StatusUnauthorized)
			c.Abort()
			return
		}

		c.Set(auth.ContextUserIDKey, userID)
		c.Next()
	}
}
