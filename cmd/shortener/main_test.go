package main

import (
	"os"
	"testing"
	"url-shortener/internal/config"

	"github.com/stretchr/testify/require"
)

func TestConfigValidate_OK(t *testing.T) {
	cfg := &config.Config{
		ServerAddress: "localhost:8080",
		BaseURL:       "http://localhost:8080",
	}
	require.NoError(t, cfg.Validate())
}

func TestConfigValidate_Errors(t *testing.T) {
	cfg := &config.Config{
		ServerAddress: "",
		BaseURL:       "http://localhost:8080",
	}
	require.Error(t, cfg.Validate())

	cfg = &config.Config{
		ServerAddress: "localhost:8080",
		BaseURL:       "",
	}
	require.Error(t, cfg.Validate())
}

func TestConfigInit_ReadsEnv(t *testing.T) {
	t.Setenv("SERVER_ADDRESS", "127.0.0.1:9999")
	t.Setenv("BASE_URL", "http://example.com")
	t.Setenv("AUDIT_FILE", "/tmp/audit.log")
	t.Setenv("AUDIT_URL", "http://audit.local/ingest")

	// чтобы не мешали флаги от go test, просто вызываем Init.
	cfg := config.Init()

	require.Equal(t, "127.0.0.1:9999", cfg.ServerAddress)
	require.Equal(t, "http://example.com", cfg.BaseURL)
	require.Equal(t, "/tmp/audit.log", cfg.AuditFile)
	require.Equal(t, "http://audit.local/ingest", cfg.AuditURL)

	// DB мы не обязаны поднимать в юнит-тесте.
	_ = os.Stdout
}
