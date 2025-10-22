package middleware

import (
	"compress/gzip"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

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
	return &compressWriter{
		ResponseWriter: w,
		zw:             gzip.NewWriter(w),
	}
}

func (c *compressWriter) Write(p []byte) (int, error) {
	return c.zw.Write(p)
}

func (c *compressWriter) WriteString(s string) (int, error) {
	return c.zw.Write([]byte(s))
}

func (c *compressWriter) Close() error {
	return c.zw.Close()
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
			contentType := c.GetHeader("Content-Type")
			if shouldCompress(contentType) {
				cw := newCompressWriter(c.Writer)
				defer cw.Close()
				c.Writer = cw
				c.Header("Content-Encoding", "gzip")
			}
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

func shouldCompress(contentType string) bool {
	return strings.Contains(contentType, "application/json") ||
		strings.Contains(contentType, "text/html") ||
		strings.Contains(contentType, "text/plain")
}
