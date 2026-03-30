package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Channel.ListenAddr != ":1491" {
		t.Errorf("expected :1491, got %s", cfg.Channel.ListenAddr)
	}
	if cfg.Store.RetainWordObjects != 1000 {
		t.Errorf("expected 1000, got %d", cfg.Store.RetainWordObjects)
	}
}

func TestValidateDefault(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("default config should be valid: %v", err)
	}
}

func TestValidateEmptyDataDir(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Store.DataDir = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty data dir")
	}
}

func TestValidateEmptyListenAddr(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Channel.ListenAddr = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty listen addr")
	}
}

func TestValidateRetainTooLow(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Store.RetainWordObjects = 0
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for retain < 1")
	}
}

func TestValidateBufferTooSmall(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Channel.MaxBufferSize = 10
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for buffer < 100")
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
server:
  log_level: debug
channel:
  listen_addr: ":2491"
  auth_password: "mypassword"
store:
  data_dir: "/tmp/sonic-test"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Server.LogLevel != "debug" {
		t.Errorf("expected debug, got %s", cfg.Server.LogLevel)
	}
	if cfg.Channel.ListenAddr != ":2491" {
		t.Errorf("expected :2491, got %s", cfg.Channel.ListenAddr)
	}
	if cfg.Channel.AuthPassword != "mypassword" {
		t.Errorf("expected mypassword, got %s", cfg.Channel.AuthPassword)
	}
	if cfg.Store.DataDir != "/tmp/sonic-test" {
		t.Errorf("expected /tmp/sonic-test, got %s", cfg.Store.DataDir)
	}
}

func TestLoadFromFileMissing(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadFromFileInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("{{invalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFromFile(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}
