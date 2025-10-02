package config

import (
	"flag"
	"os"
	"testing"
)

func setEnvVars(vars map[string]string) func() {
	// Сохраняем текущие значения
	original := make(map[string]string)
	for key := range vars {
		original[key] = os.Getenv(key)
	}

	// Устанавливаем новые значения
	for key, value := range vars {
		os.Setenv(key, value)
	}

	// Возвращаем функцию для восстановления
	return func() {
		for key, value := range original {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}
}

// 🔧 СБРАСЫВАЕМ ФЛАГИ МЕЖДУ ТЕСТАМИ
func resetFlags() {
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
}

func TestConfig_EnvironmentVariables(t *testing.T) {
	// Устанавливаем переменные окружения
	cleanup := setEnvVars(map[string]string{
		"SERVER_ADDRESS": "env-host:9090",
		"BASE_URL":       "https://env-shortener.com",
	})
	defer cleanup()

	resetFlags()

	cfg := Init()

	if cfg.ServerAddress != "env-host:9090" {
		t.Errorf("Expected ServerAddress 'env-host:9090', got '%s'", cfg.ServerAddress)
	}

	if cfg.BaseURL != "https://env-shortener.com" {
		t.Errorf("Expected BaseURL 'https://env-shortener.com', got '%s'", cfg.BaseURL)
	}
}

func TestConfig_CommandLineFlags(t *testing.T) {
	// Убеждаемся, что переменные окружения не установлены
	cleanup := setEnvVars(map[string]string{
		"SERVER_ADDRESS": "",
		"BASE_URL":       "",
	})
	defer cleanup()

	resetFlags()

	// 🔧 ЭМУЛИРУЕМ АРГУМЕНТЫ КОМАНДНОЙ СТРОКИ
	os.Args = []string{"test", "-a", "flag-host:7070", "-b", "https://flag-shortener.com"}

	cfg := Init()

	if cfg.ServerAddress != "flag-host:7070" {
		t.Errorf("Expected ServerAddress 'flag-host:7070', got '%s'", cfg.ServerAddress)
	}

	if cfg.BaseURL != "https://flag-shortener.com" {
		t.Errorf("Expected BaseURL 'https://flag-shortener.com', got '%s'", cfg.BaseURL)
	}
}

func TestConfig_EnvironmentOverridesFlags(t *testing.T) {
	// Устанавливаем переменные окружения
	cleanup := setEnvVars(map[string]string{
		"SERVER_ADDRESS": "env-host:9090", // Этот должен победить
		"BASE_URL":       "https://env-shortener.com",
	})
	defer cleanup()

	resetFlags()

	// 🔧 ПЫТАЕМСЯ ПЕРЕОПРЕДЕЛИТЬ ФЛАГАМИ (но env vars должны победить)
	os.Args = []string{"test", "-a", "flag-host:7070", "-b", "https://flag-shortener.com"}

	cfg := Init()

	// Переменные окружения должны иметь приоритет
	if cfg.ServerAddress != "env-host:9090" {
		t.Errorf("Expected env var to override flag, got ServerAddress '%s'", cfg.ServerAddress)
	}

	if cfg.BaseURL != "https://env-shortener.com" {
		t.Errorf("Expected env var to override flag, got BaseURL '%s'", cfg.BaseURL)
	}
}

func TestConfig_DefaultValues(t *testing.T) {
	// Убеждаемся, что ничего не установлено
	cleanup := setEnvVars(map[string]string{
		"SERVER_ADDRESS": "",
		"BASE_URL":       "",
	})
	defer cleanup()

	resetFlags()

	os.Args = []string{"test"} // Без флагов

	cfg := Init()

	// Должны использоваться значения по умолчанию
	expectedServer := "localhost:8080"
	expectedBaseURL := "http://localhost:8080"

	if cfg.ServerAddress != expectedServer {
		t.Errorf("Expected default ServerAddress '%s', got '%s'", expectedServer, cfg.ServerAddress)
	}

	if cfg.BaseURL != expectedBaseURL {
		t.Errorf("Expected default BaseURL '%s', got '%s'", expectedBaseURL, cfg.BaseURL)
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		serverAddr  string
		baseURL     string
		shouldError bool
	}{
		{
			name:        "valid config",
			serverAddr:  "localhost:8080",
			baseURL:     "http://example.com",
			shouldError: false,
		},
		{
			name:        "empty server address",
			serverAddr:  "",
			baseURL:     "http://example.com",
			shouldError: true,
		},
		{
			name:        "empty base URL",
			serverAddr:  "localhost:8080",
			baseURL:     "",
			shouldError: true,
		},
		{
			name:        "both empty",
			serverAddr:  "",
			baseURL:     "",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				ServerAddress: tt.serverAddr,
				BaseURL:       tt.baseURL,
			}

			err := cfg.Validate()

			if tt.shouldError && err == nil {
				t.Errorf("Expected error, but got none")
			}

			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error, but got: %v", err)
			}
		})
	}
}
