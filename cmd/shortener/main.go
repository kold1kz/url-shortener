package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"url-shortener/cmd/certutil"
	"url-shortener/internal/audit"
	"url-shortener/internal/config"
	"url-shortener/internal/handler"
	"url-shortener/internal/middleware"
	"url-shortener/internal/pprof"
	"url-shortener/internal/repository"
	"url-shortener/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func na(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

func loadConfig() (*config.Config, error) {
	cfg := config.Init()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}

func buildRepo(cfg *config.Config, logger *zap.SugaredLogger) (repository.URLRepository, error) {
	if cfg.DB != nil {
		postgresRepo, err := repository.NewPostgresURLRepository(cfg.DB.GetDB())
		if err != nil {
			return nil, fmt.Errorf("failed to init postgres repository: %w", err)
		}
		logger.Infow("using PostgreSQL repository")
		return postgresRepo, nil
	}

	if cfg.FileStoragePath != "" {
		fileRepo, err := repository.NewFileURLRepository(cfg.FileStoragePath)
		if err != nil {
			return nil, fmt.Errorf("init file repository (%s): %w", cfg.FileStoragePath, err)
		}
		logger.Infow("using file repository", "path", cfg.FileStoragePath)
		return fileRepo, nil
	}

	logger.Infow("using in-memory repository")
	return repository.NewInMemoryURLRepository(), nil
}

func buildAuditPublisher(cfg *config.Config, logger *zap.SugaredLogger) *audit.Publisher {
	pub := audit.NewPublisher()

	if cfg.AuditFile != "" {
		fs, err := audit.NewFileSink(cfg.AuditFile)
		if err != nil {
			logger.Warnw("failed to init audit file sink", "error", err)
		} else {
			pub.AddSink(fs)
			logger.Infow("audit file enabled", "path", cfg.AuditFile)
		}
	}

	if cfg.AuditURL != "" {
		pub.AddSink(audit.NewHTTPSink(cfg.AuditURL))
		logger.Infow("audit remote enabled", "url", cfg.AuditURL)
	}

	return pub
}

func main() {
	fmt.Printf("Build version: %s\n", na(buildVersion))
	fmt.Printf("Build date: %s\n", na(buildDate))
	fmt.Printf("Build commit: %s\n", na(buildCommit))

	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	defer cfg.Close()

	logger := middleware.InitLogger()
	defer logger.Sync()

	repo, err := buildRepo(cfg, logger)
	if err != nil {
		return fmt.Errorf("build repository: %w", err)
	}

	svc := service.NewURLService(repo, cfg.BaseURL)
	defer svc.Close()

	auditPub := buildAuditPublisher(cfg, logger)
	defer auditPub.Close()

	handlers := handler.NewHandler(svc, auditPub)

	router := gin.Default()
	router.Use(middleware.GzipMiddleware())
	router.Use(middleware.HTTPLoggerMiddleware(logger))

	authGroup := router.Group("/")
	authGroup.Use(middleware.UserAuth())

	authGroup.POST("/", handlers.ShortenURL)
	authGroup.GET("/:id", handlers.GetOriginalURL)
	authGroup.POST("/api/shorten", handlers.ShortenJSONUrl)
	authGroup.POST("/api/shorten/batch", handlers.ShortenURLBatch)
	authGroup.GET("/api/user/urls", handlers.GetUserURLs)
	authGroup.DELETE("/api/user/urls", handlers.DeleteUserURLs)

	pingHandler := handler.NewPingHandler(cfg.DB)
	router.GET("/ping", pingHandler.Ping)

	pprof.Register(router)

	srv := &http.Server{
		Addr:    cfg.ServerAddress,
		Handler: router,
	}

	errCh := make(chan error, 1)

	if cfg.EnableHTTPS {
		logger.Infow("server starting with HTTPS", "base_url", cfg.BaseURL, "address", cfg.ServerAddress)

		certPath, keyPath, err := certutil.EnsureCertFiles(certutil.EnsureOptions{
			CertPath:    "./cert/cert.pem",
			KeyPath:     "./cert/key.pem",
			ValidFor:    30 * 24 * time.Hour,
			RenewBefore: 24 * time.Hour,
			Hosts:       []string{"localhost", "127.0.0.1", "::1"},
		})
		if err != nil {
			return fmt.Errorf("ensure tls certs: %w", err)
		}

		go func() {
			errCh <- srv.ListenAndServeTLS(certPath, keyPath)
		}()
	} else {
		logger.Infow("server starting", "base_url", cfg.BaseURL, "address", cfg.ServerAddress)

		go func() {
			errCh <- srv.ListenAndServe()
		}()
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGQUIT,
	)
	defer stop()

	select {
	case <-ctx.Done():
		logger.Infow("shutdown signal received")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			closeErr := srv.Close()
			if closeErr != nil {
				logger.Errorw("forced server close failed", "error", closeErr)
			}
			return fmt.Errorf("server shutdown: %w", err)
		}

		logger.Infow("server stopped gracefully")
		return nil

	case err := <-errCh:
		if err == nil || err == http.ErrServerClosed {
			return nil
		}
		return fmt.Errorf("server run error: %w", err)
	}
}
