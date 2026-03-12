package middleware

import (
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
	"url-shortener/internal/auth"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var gzipWriterPool = sync.Pool{
	New: func() any {
		// BestSpeed почти всегда достаточно и дешевле по CPU/памяти
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return w
	},
}

// HTTPLoggerMiddleware логирует информацию о запросе и ответе.
//
// Логирование выполняется после обработки запроса (после c.Next()):
//   - url, method, duration,
//   - status и size ответа.
//
// Middleware предполагает, что writer реализует gin.ResponseWriter.
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

// InitLogger создаёт production-логгер zap и возвращает SugaredLogger.
//
// При ошибке инициализации возвращает noop-логгер, чтобы сервис мог продолжить работу.
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

	if err != nil {
		return fmt.Errorf("gzip middleware: close gzip writer: %w", err)
	}
	return nil
}

type compressReader struct {
	io.ReadCloser
	zr *gzip.Reader
}

func newCompressReader(r io.ReadCloser) (*compressReader, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip middleware: create reader: %w", err)
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
		return fmt.Errorf("gzip middleware: close request body: %w", err)
	}
	if err := c.zr.Close(); err != nil {
		return fmt.Errorf("gzip middleware: close gzip reader: %w", err)
	}
	return nil
}

// GzipMiddleware включает gzip-сжатие HTTP-ответов и поддерживает gzip-тела запросов.
//
// Поведение:
//   - если клиент прислал Accept-Encoding: gzip, ответ будет сжат и выставится
//     Content-Encoding: gzip и Vary: Accept-Encoding;
//   - если клиент прислал Content-Encoding: gzip, тело запроса будет распаковано
//     перед передачей в хендлер.
//
// Для снижения аллокаций используются переиспользуемые gzip.Writer из sync.Pool.
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

// UserAuth обеспечивает идентификацию пользователя через cookie.
//
// Если cookie отсутствует/пуста — middleware создаёт новый userID и устанавливает cookie.
// Если cookie есть — валидирует токен; при невалидном токене возвращает 401.
//
// Идентификатор пользователя сохраняется в контекст gin под ключом auth.ContextUserIDKey.
func UserAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		rawCookie, err := c.Cookie(auth.CookieName())

		if err != nil || rawCookie == "" {
			var userID, newToken string
			userID, newToken, err = auth.GetOrCreateUserIDFromCookie("")
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
