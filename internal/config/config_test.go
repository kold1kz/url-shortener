package config

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

//
// ===== helpers =====
//

func resetFlags() {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
}

func resetEnv(keys ...string) {
	for _, k := range keys {
		_ = os.Unsetenv(k)
	}
}

//
// ===== detectConfigFile =====
//

func TestDetectConfigFile_FromEnv(t *testing.T) {
	resetEnv("CONFIG")
	os.Setenv("CONFIG", "env.json")
	defer resetEnv("CONFIG")

	got := detectConfigFile()
	if got != "env.json" {
		t.Fatalf("expected env.json, got %s", got)
	}
}

func TestDetectConfigFile_FromArgs(t *testing.T) {
	resetEnv("CONFIG")
	os.Args = []string{"app", "-c", "file.json"}

	got := detectConfigFile()
	if got != "file.json" {
		t.Fatalf("expected file.json, got %s", got)
	}
}

func TestDetectConfigFile_Empty(t *testing.T) {
	resetEnv("CONFIG")
	os.Args = []string{"app"}

	got := detectConfigFile()
	if got != "" {
		t.Fatalf("expected empty, got %s", got)
	}
}

//
// ===== readFileConfig =====
//

func TestReadFileConfig_EmptyPath(t *testing.T) {
	cfg, err := readFileConfig("")
	if err != nil || cfg != nil {
		t.Fatalf("expected nil,nil got %v %v", cfg, err)
	}
}

func TestReadFileConfig_FileNotExists(t *testing.T) {
	_, err := readFileConfig("no_such_file.json")
	if err == nil || !strings.Contains(err.Error(), "read file") {
		t.Fatalf("expected read error, got %v", err)
	}
}

func TestReadFileConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(path, []byte("{invalid"), 0o644)

	_, err := readFileConfig(path)
	if err == nil || !strings.Contains(err.Error(), "parse json") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestReadFileConfig_OK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "good.json")

	https := true
	fc := FileConfig{
		ServerAddress: "addr",
		BaseURL:       "base",
		EnableHTTPS:   &https,
	}
	data, _ := json.Marshal(fc)
	_ = os.WriteFile(path, data, 0o644)

	got, err := readFileConfig(path)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !reflect.DeepEqual(got.ServerAddress, "addr") {
		t.Fatalf("unexpected value")
	}
	if got.EnableHTTPS == nil || !*got.EnableHTTPS {
		t.Fatalf("https not parsed")
	}
}

//
// ===== applyEnv =====
//

func TestApplyEnv_All(t *testing.T) {
	cfg := &Config{}

	os.Setenv("SERVER_ADDRESS", "env_addr")
	os.Setenv("BASE_URL", "env_base")
	os.Setenv("FILE_STORAGE_PATH", "env_file")
	os.Setenv("DATABASE_DSN", "env_dsn")
	os.Setenv("AUDIT_URL", "env_audit_url")
	os.Setenv("AUDIT_FILE", "env_audit_file")
	os.Setenv("ENABLE_HTTPS", "1")

	defer resetEnv(
		"SERVER_ADDRESS",
		"BASE_URL",
		"FILE_STORAGE_PATH",
		"DATABASE_DSN",
		"AUDIT_URL",
		"AUDIT_FILE",
		"ENABLE_HTTPS",
	)

	applyEnv(cfg)

	if cfg.ServerAddress != "env_addr" ||
		cfg.BaseURL != "env_base" ||
		cfg.FileStoragePath != "env_file" ||
		cfg.DatabaseDSN != "env_dsn" ||
		cfg.AuditURL != "env_audit_url" ||
		cfg.AuditFile != "env_audit_file" ||
		!cfg.EnableHTTPS {
		t.Fatalf("env not applied correctly: %+v", cfg)
	}
}

//
// ===== Validate =====
//

func TestValidate(t *testing.T) {
	cfg := &Config{
		ServerAddress: "a",
		BaseURL:       "b",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error")
	}

	cfg.ServerAddress = ""
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error")
	}

	cfg.ServerAddress = "a"
	cfg.BaseURL = ""
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error")
	}
}

//
// ===== initDB =====
//

func TestInitDB_EmptyDSN(t *testing.T) {
	cfg := &Config{}
	cfg.initDB()
	if cfg.DB != nil {
		t.Fatalf("expected nil DB")
	}
}

func TestInitDB_BadDSN(t *testing.T) {
	cfg := &Config{DatabaseDSN: "bad_dsn"}
	cfg.initDB()
	if cfg.DB != nil {
		t.Fatalf("expected nil DB on error")
	}
}

//
// ===== Close =====
//

func TestClose_NoDB(t *testing.T) {
	cfg := &Config{}
	if err := cfg.Close(); err != nil {
		t.Fatalf("unexpected err")
	}
}

//
// ===== Init (integration) =====
//

func TestInit_WithJSONAndEnvPriority(t *testing.T) {
	resetFlags()
	resetEnv(
		"SERVER_ADDRESS",
		"BASE_URL",
		"FILE_STORAGE_PATH",
		"DATABASE_DSN",
		"AUDIT_URL",
		"AUDIT_FILE",
		"ENABLE_HTTPS",
		"CONFIG",
	)

	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.json")

	https := false
	fileCfg := FileConfig{
		ServerAddress: "json_addr",
		BaseURL:       "json_base",
		EnableHTTPS:   &https,
	}
	data, _ := json.Marshal(fileCfg)
	_ = os.WriteFile(path, data, 0o644)

	os.Args = []string{"app", "-c", path}
	os.Setenv("SERVER_ADDRESS", "env_override")

	cfg := Init()

	if cfg.ServerAddress != "env_override" {
		t.Fatalf("env must override json")
	}
	if cfg.BaseURL != "json_base" {
		t.Fatalf("json must apply if no env/flag override")
	}
}
