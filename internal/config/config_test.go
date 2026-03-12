package config

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func resetFlags() {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(os.Stdout)
}

func TestDetectConfigFile_FromEnv(t *testing.T) {
	t.Setenv("CONFIG", "config_example.json")
	os.Args = []string{"app"}

	got := detectConfigFile()
	expected := "config_example.json"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestDetectConfigFile_FromArgs(t *testing.T) {
	os.Args = []string{"app", "-c", "file.json"}

	got := detectConfigFile()
	expected := "file.json"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestDetectConfigFile_Empty(t *testing.T) {
	os.Args = []string{"app"}

	got := detectConfigFile()
	if got != "" {
		t.Fatalf("expected empty config file path, got %q", got)
	}
}

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

	err := os.WriteFile(path, []byte("{invalid"), 0o644)
	if err != nil {
		t.Fatalf("write bad config: %v", err)
	}

	_, err = readFileConfig(path)
	if err == nil || !strings.Contains(err.Error(), "parse json") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestReadFileConfig_OK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "good.json")

	https := true
	expectedConfig := FileConfig{
		ServerAddress: "addr",
		BaseURL:       "base",
		EnableHTTPS:   &https,
		TrustedSubnet: "10.0.0.0/8",
	}

	data, err := json.Marshal(expectedConfig)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	err = os.WriteFile(path, data, 0o644)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	got, err := readFileConfig(path)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if got.ServerAddress != expectedConfig.ServerAddress {
		t.Fatalf("unexpected server address: got %q, expected %q", got.ServerAddress, expectedConfig.ServerAddress)
	}
	if got.BaseURL != expectedConfig.BaseURL {
		t.Fatalf("unexpected base url: got %q, expected %q", got.BaseURL, expectedConfig.BaseURL)
	}
	if got.TrustedSubnet != expectedConfig.TrustedSubnet {
		t.Fatalf("unexpected trusted subnet: got %q, expected %q", got.TrustedSubnet, expectedConfig.TrustedSubnet)
	}
	if got.EnableHTTPS == nil || !*got.EnableHTTPS {
		t.Fatalf("https not parsed")
	}
}

func TestApplyEnv_All(t *testing.T) {
	cfg := &Config{}

	t.Setenv("SERVER_ADDRESS", "env_addr")
	t.Setenv("BASE_URL", "env_base")
	t.Setenv("FILE_STORAGE_PATH", "env_file")
	t.Setenv("DATABASE_DSN", "env_dsn")
	t.Setenv("AUDIT_URL", "env_audit_url")
	t.Setenv("AUDIT_FILE", "env_audit_file")
	t.Setenv("TRUSTED_SUBNET", "192.168.0.0/24")
	t.Setenv("ENABLE_HTTPS", "1")

	applyEnv(cfg)

	if cfg.ServerAddress != "env_addr" ||
		cfg.BaseURL != "env_base" ||
		cfg.FileStoragePath != "env_file" ||
		cfg.DatabaseDSN != "env_dsn" ||
		cfg.AuditURL != "env_audit_url" ||
		cfg.AuditFile != "env_audit_file" ||
		cfg.TrustedSubnet != "192.168.0.0/24" ||
		!cfg.EnableHTTPS {
		t.Fatalf("env not applied correctly: %+v", cfg)
	}
}

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

func TestClose_NoDB(t *testing.T) {
	cfg := &Config{}
	if err := cfg.Close(); err != nil {
		t.Fatalf("unexpected err")
	}
}

func TestInit_WithJSONAndEnvPriority(t *testing.T) {
	resetFlags()

	dir := t.TempDir()
	path := filepath.Join(dir, "config_example.json")

	https := false
	fileCfg := FileConfig{
		ServerAddress: "json_addr",
		BaseURL:       "json_base",
		EnableHTTPS:   &https,
	}

	data, err := json.Marshal(fileCfg)
	if err != nil {
		t.Fatalf("marshal file config: %v", err)
	}

	err = os.WriteFile(path, data, 0o644)
	if err != nil {
		t.Fatalf("write file config: %v", err)
	}

	os.Args = []string{"app", "-c", path}
	t.Setenv("SERVER_ADDRESS", "env_override")
	t.Setenv("CONFIG", "config_example.json")

	cfg := Init()

	if cfg.ServerAddress != "env_override" {
		t.Fatalf("env must override json, got %q", cfg.ServerAddress)
	}
	if cfg.BaseURL != "json_base" {
		t.Fatalf("json must apply if no env/flag override, got %q", cfg.BaseURL)
	}
}

func TestInit_WithJSONTrustedSubnet(t *testing.T) {
	resetFlags()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	https := false
	fileCfg := FileConfig{
		ServerAddress: "json_addr",
		BaseURL:       "json_base",
		EnableHTTPS:   &https,
		TrustedSubnet: "172.16.0.0/16",
	}

	data, err := json.Marshal(fileCfg)
	if err != nil {
		t.Fatalf("marshal file config: %v", err)
	}

	err = os.WriteFile(path, data, 0o644)
	if err != nil {
		t.Fatalf("write file config: %v", err)
	}

	os.Args = []string{"app", "-c", path}

	cfg := Init()

	if cfg.TrustedSubnet != "172.16.0.0/16" {
		t.Fatalf("expected trusted subnet from json, got %q", cfg.TrustedSubnet)
	}
}

func TestInit_TrustedSubnet_EnvOverridesJSON(t *testing.T) {
	resetFlags()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	https := false
	fileCfg := FileConfig{
		EnableHTTPS:   &https,
		TrustedSubnet: "172.16.0.0/16",
	}

	data, err := json.Marshal(fileCfg)
	if err != nil {
		t.Fatalf("marshal file config: %v", err)
	}

	err = os.WriteFile(path, data, 0o644)
	if err != nil {
		t.Fatalf("write file config: %v", err)
	}

	os.Args = []string{"app", "-c", path}
	t.Setenv("TRUSTED_SUBNET", "10.10.0.0/24")

	cfg := Init()

	if cfg.TrustedSubnet != "10.10.0.0/24" {
		t.Fatalf("expected env to override json, got %q", cfg.TrustedSubnet)
	}
}
