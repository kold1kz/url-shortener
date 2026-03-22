package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"url-shortener/cmd/certutil"
	"url-shortener/internal/audit"
	"url-shortener/internal/config"
	"url-shortener/internal/grpcserver"
	"url-shortener/internal/handler"
	"url-shortener/internal/middleware"
	"url-shortener/internal/pprof"
	"url-shortener/internal/repository"
	"url-shortener/internal/service"
	pb "url-shortener/proto"

	"google.golang.org/grpc/credentials"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc"
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

	urlSvc := service.NewURLService(repo, cfg.BaseURL)
	defer urlSvc.Close()

	auditPub := buildAuditPublisher(cfg, logger)
	defer auditPub.Close()

	var trustedSubnet *net.IPNet
	if cfg.TrustedSubnet != "" {
		_, subnet, err := net.ParseCIDR(cfg.TrustedSubnet)
		if err != nil {
			return fmt.Errorf("parse trusted subnet: %w", err)
		}
		trustedSubnet = subnet
	}

	handlers := handler.NewHandler(urlSvc, auditPub, trustedSubnet)

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
	router.GET("/api/internal/stats", handlers.GetInternalStats)

	pprof.Register(router)

	httpSrv := &http.Server{
		Addr:    cfg.ServerAddress,
		Handler: router,
	}

	httpErrCh := make(chan error, 1)

	// Общие TLS-файлы для HTTP и gRPC
	var certPath, keyPath string
	if cfg.EnableHTTPS {
		certPath, keyPath, err = certutil.EnsureCertFiles(certutil.EnsureOptions{
			CertPath:    "./cert/cert.pem",
			KeyPath:     "./cert/key.pem",
			ValidFor:    30 * 24 * time.Hour,
			RenewBefore: 24 * time.Hour,
			Hosts:       []string{"localhost", "127.0.0.1", "::1"},
		})
		if err != nil {
			return fmt.Errorf("ensure tls certs: %w", err)
		}
	}

	if cfg.EnableHTTPS {
		logger.Infow("http server starting with HTTPS", "base_url", cfg.BaseURL, "address", cfg.ServerAddress)
		go func() {
			httpErrCh <- httpSrv.ListenAndServeTLS(certPath, keyPath)
		}()
	} else {
		logger.Infow("http server starting", "base_url", cfg.BaseURL, "address", cfg.ServerAddress)
		go func() {
			httpErrCh <- httpSrv.ListenAndServe()
		}()
	}

	var grpcSrv *grpc.Server
	var grpcErrCh chan error

	if cfg.GRPCServerAddress != "" {
		if cfg.DB == nil {
			return fmt.Errorf("grpc login requires database connection")
		}

		userRepo := repository.NewPostgresUserRepository(cfg.DB.GetDB())
		loginSvc := service.NewLoginService(userRepo)

		grpcLis, err := net.Listen("tcp", cfg.GRPCServerAddress)
		if err != nil {
			return fmt.Errorf("grpc listen: %w", err)
		}

		if cfg.EnableHTTPS {
			creds, err := credentials.NewServerTLSFromFile(certPath, keyPath)
			if err != nil {
				return fmt.Errorf("grpc tls creds: %w", err)
			}

			grpcSrv = grpc.NewServer(
				grpc.Creds(creds),
				grpc.UnaryInterceptor(grpcserver.AuthInterceptor),
			)
			logger.Infow("grpc server starting with TLS", "address", cfg.GRPCServerAddress)
		} else {
			grpcSrv = grpc.NewServer(
				grpc.UnaryInterceptor(grpcserver.AuthInterceptor),
			)
			logger.Infow("grpc server starting without TLS", "address", cfg.GRPCServerAddress)
		}

		pb.RegisterShortenerServiceServer(
			grpcSrv,
			grpcserver.NewServer(urlSvc, loginSvc),
		)

		grpcErrCh = make(chan error, 1)

		go func() {
			grpcErrCh <- grpcSrv.Serve(grpcLis)
		}()
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGQUIT,
	)
	defer stop()

	for {
		select {
		case <-ctx.Done():
			logger.Infow("shutdown signal received")

			if grpcSrv != nil {
				grpcSrv.GracefulStop()
				logger.Infow("grpc server stopped gracefully")
			}

			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := httpSrv.Shutdown(shutdownCtx); err != nil {
				closeErr := httpSrv.Close()
				if closeErr != nil {
					logger.Errorw("forced http server close failed", "error", closeErr)
				}
				return fmt.Errorf("http server shutdown: %w", err)
			}

			logger.Infow("http server stopped gracefully")
			return nil

		case err := <-httpErrCh:
			if err == nil || err == http.ErrServerClosed {
				return nil
			}
			return fmt.Errorf("http server run error: %w", err)

		case err := <-grpcErrCh:
			if err == nil {
				return nil
			}
			return fmt.Errorf("grpc server run error: %w", err)
		}
	}
}
