package main

import (
	"testing"

	"go.uber.org/zap"

	"url-shortener/internal/config"
)

// "тихий" логгер, чтобы не засорять вывод тестов
func newTestLogger() *zap.SugaredLogger {
	return zap.NewNop().Sugar()
}

// Тестируем успешный путь loadConfig:
// - возвращает не nil
// - конфиг прошёл Validate (иначе был бы log.Fatalf и тест не дошёл бы сюда)
// ВАЖНО: тест опирается на реальные дефолты config.Init(),
// поэтому не трогаем env-переменные здесь.
func TestLoadConfig_Success(t *testing.T) {
	cfg := loadConfig()
	if cfg == nil {
		t.Fatal("expected non-nil config from loadConfig, got nil")
	}

	// минимальная sanity-проверка, чтобы избежать "пустого" объекта
	if cfg.ServerAddress == "" {
		t.Errorf("expected ServerAddress to be non-empty")
	}
	if cfg.URLService == nil {
		t.Errorf("expected URLService to be initialized")
	}
}

// Проверяем, что при пустом DSN база не инициализируется
// и setupDatabase просто возвращает nil, как задумано.
func TestSetupDatabase_EmptyDSN(t *testing.T) {
	cfg := &config.Config{
		DatabaseDSN: "",
	}

	logger := newTestLogger()

	db := setupDatabase(cfg, logger)
	if db != nil {
		t.Fatalf("expected nil DB when DatabaseDSN is empty, got: %#v", db)
	}
}

// Проверяем ветку, когда DSN задан, но до БД не удаётся достучаться (или миграции падают).
// NewDB в этом случае вернёт ошибку, а setupDatabase должен:
// - залогировать ошибку
// - вернуть nil, но не паникнуть и не вызвать os.Exit().
func TestSetupDatabase_InvalidDSN(t *testing.T) {
	cfg := &config.Config{
		// заведомо нерабочий DSN
		DatabaseDSN: "postgres://invalid:invalid@127.0.0.1:5432/invaliddb?sslmode=disable",
	}

	logger := newTestLogger()

	db := setupDatabase(cfg, logger)
	if db != nil {
		t.Fatalf("expected nil DB when NewDB fails with invalid DSN, got: %#v", db)
	}
}
