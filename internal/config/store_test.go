package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyDefaults(t *testing.T) {
	cfg := Config{}
	cfg.ApplyDefaults()

	if cfg.BaseURL != defaultBaseURL {
		t.Fatalf("expected default base URL, got %q", cfg.BaseURL)
	}
	if cfg.CloudURL != defaultCloudURL {
		t.Fatalf("expected default cloud URL, got %q", cfg.CloudURL)
	}

	cfg = Config{BaseURL: "https://stack.grafana.net", CloudURL: "https://grafana.example.com"}
	cfg.ApplyDefaults()
	if cfg.BaseURL != "https://stack.grafana.net" {
		t.Fatalf("base URL should not be overwritten")
	}
	if cfg.CloudURL != "https://grafana.example.com" {
		t.Fatalf("cloud URL should not be overwritten")
	}
}

func TestIsAuthenticated(t *testing.T) {
	if (Config{}).IsAuthenticated() {
		t.Fatalf("expected unauthenticated config")
	}
	if !(Config{Token: " token "}).IsAuthenticated() {
		t.Fatalf("expected authenticated config")
	}
}

func TestDefaultPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	t.Setenv("HOME", "/tmp/home")
	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join("/tmp/xdg", "grafana-cli", "config.json")
	if path != expected {
		t.Fatalf("expected %q, got %q", expected, path)
	}

	t.Setenv("XDG_CONFIG_HOME", "")
	path, err = DefaultPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected = filepath.Join("/tmp/home", ".config", "grafana-cli", "config.json")
	if path != expected {
		t.Fatalf("expected %q, got %q", expected, path)
	}

	t.Setenv("HOME", "")
	_, err = DefaultPath()
	if err == nil {
		t.Fatalf("expected error when HOME is missing")
	}
}

func TestFileStoreLoadSaveClear(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "sub", "config.json")
	store := NewFileStore(path)

	if store.Path() != path {
		t.Fatalf("unexpected path: %s", store.Path())
	}

	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("unexpected load error: %v", err)
	}
	if cfg.BaseURL == "" || cfg.CloudURL == "" {
		t.Fatalf("expected defaults to be applied")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte{}, 0o600); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	cfg, err = store.Load()
	if err != nil {
		t.Fatalf("unexpected load error for empty file: %v", err)
	}
	if cfg.BaseURL == "" || cfg.CloudURL == "" {
		t.Fatalf("expected defaults for empty file")
	}

	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatalf("expected unmarshal error")
	}

	target := Config{
		BaseURL:       "https://stack.grafana.net",
		CloudURL:      "https://grafana.com",
		PrometheusURL: "https://prom.grafana.net",
		LogsURL:       "https://logs.grafana.net",
		TracesURL:     "https://traces.grafana.net",
		Token:         "token",
		OrgID:         42,
	}
	if err := store.Save(target); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	cfg, err = store.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.Token != "token" || cfg.OrgID != 42 {
		t.Fatalf("unexpected roundtrip config: %+v", cfg)
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("clear failed: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file to be removed")
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("clear should ignore missing file: %v", err)
	}
}

func TestFileStoreErrorPaths(t *testing.T) {
	tmp := t.TempDir()

	// Load should fail when the path points to a directory.
	dirPath := filepath.Join(tmp, "as-dir")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	store := NewFileStore(dirPath)
	if _, err := store.Load(); err == nil {
		t.Fatalf("expected load error for directory path")
	}

	// Save should fail when parent path is a file.
	parentFile := filepath.Join(tmp, "parent-file")
	if err := os.WriteFile(parentFile, []byte("x"), 0o600); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	store = NewFileStore(filepath.Join(parentFile, "config.json"))
	if err := store.Save(Config{Token: "x"}); err == nil {
		t.Fatalf("expected save error when parent is a file")
	}

	// Clear should fail for invalid remove target represented as a directory.
	dirStorePath := filepath.Join(tmp, "dir-remove")
	if err := os.MkdirAll(dirStorePath, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dirStorePath, "child"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	store = NewFileStore(dirStorePath)
	if err := store.Clear(); err == nil {
		t.Fatalf("expected clear error for directory path")
	}
}
