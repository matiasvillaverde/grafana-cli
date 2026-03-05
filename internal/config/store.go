package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultBaseURL  = "https://grafana.com"
	defaultCloudURL = "https://grafana.com"
)

// Config stores authentication and endpoint information for grafana-cli.
type Config struct {
	BaseURL       string `json:"base_url"`
	CloudURL      string `json:"cloud_url"`
	PrometheusURL string `json:"prometheus_url"`
	LogsURL       string `json:"logs_url"`
	TracesURL     string `json:"traces_url"`
	Token         string `json:"token"`
	OrgID         int64  `json:"org_id"`
}

func (c *Config) ApplyDefaults() {
	if strings.TrimSpace(c.BaseURL) == "" {
		c.BaseURL = defaultBaseURL
	}
	if strings.TrimSpace(c.CloudURL) == "" {
		c.CloudURL = defaultCloudURL
	}
}

func (c Config) IsAuthenticated() bool {
	return strings.TrimSpace(c.Token) != ""
}

// Store persists CLI configuration.
type Store interface {
	Load() (Config, error)
	Save(Config) error
	Clear() error
	Path() string
}

// FileStore persists config as JSON on disk.
type FileStore struct {
	path string
}

func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

func DefaultPath() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); dir != "" {
		return filepath.Join(dir, "grafana-cli", "config.json"), nil
	}
	home := strings.TrimSpace(os.Getenv("HOME"))
	if home == "" {
		return "", errors.New("HOME is not set")
	}
	return filepath.Join(home, ".config", "grafana-cli", "config.json"), nil
}

func (s *FileStore) Path() string {
	return s.path
}

func (s *FileStore) Load() (Config, error) {
	cfg := Config{}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg.ApplyDefaults()
			return cfg, nil
		}
		return Config{}, err
	}
	if len(data) == 0 {
		cfg.ApplyDefaults()
		return cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	cfg.ApplyDefaults()
	return cfg, nil
}

func (s *FileStore) Save(cfg Config) error {
	cfg.ApplyDefaults()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(s.path, data, 0o600)
}

func (s *FileStore) Clear() error {
	err := os.Remove(s.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
