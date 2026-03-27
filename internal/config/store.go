package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	defaultBaseURL  = "https://grafana.com"
	defaultCloudURL = "https://grafana.com"
)

// Config stores authentication and endpoint information for grafana-cli.
type Config struct {
	BaseURL        string `json:"base_url"`
	CloudURL       string `json:"cloud_url"`
	PrometheusURL  string `json:"prometheus_url"`
	LogsURL        string `json:"logs_url"`
	TracesURL      string `json:"traces_url"`
	OnCallURL      string `json:"oncall_url"`
	Token          string `json:"token,omitempty"`
	SessionCookie  string `json:"session_cookie,omitempty"`
	TokenBackend   string `json:"-"`
	OrgID          int64  `json:"org_id"`
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
	return strings.TrimSpace(c.Token) != "" || strings.TrimSpace(c.SessionCookie) != ""
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
	path   string
	secret SecretStore
}

func NewFileStore(path string) *FileStore {
	return NewFileStoreWithSecretStore(path, newDefaultSecretStore(path))
}

func NewFileStoreWithSecretStore(path string, secret SecretStore) *FileStore {
	return &FileStore{path: path, secret: secret}
}

func DefaultPath() (string, error) {
	dir, err := defaultConfigDir(runtime.GOOS, os.Getenv)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "grafana-cli", "config.json"), nil
}

func (s *FileStore) Path() string {
	return s.path
}

func (s *FileStore) Load() (Config, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return Config{}, err
	}
	s.loadSessionCookie(&cfg)
	return cfg, nil
}

func (s *FileStore) loadConfig() (Config, error) {
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
	legacyToken := strings.TrimSpace(cfg.Token)
	cfg.ApplyDefaults()
	if s.secret == nil {
		return cfg, nil
	}
	token, backend, err := s.secret.Load()
	if err != nil {
		if legacyToken == "" {
			return Config{}, err
		}
		cfg.Token = legacyToken
		return cfg, nil
	}
	if token != "" {
		cfg.Token = token
		cfg.TokenBackend = backend
		return cfg, nil
	}
	if legacyToken == "" {
		return cfg, nil
	}
	cfg.Token = legacyToken
	backend, err = s.secret.Save(legacyToken)
	if err != nil {
		return cfg, nil
	}
	cfg.TokenBackend = backend
	if err := s.writeConfig(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (s *FileStore) loadSessionCookie(cfg *Config) {
	cookiePath := filepath.Join(filepath.Dir(s.path), "session_cookie")
	data, err := os.ReadFile(cookiePath)
	if err != nil {
		return
	}
	if v := strings.TrimSpace(string(data)); v != "" {
		cfg.SessionCookie = v
	}
}

func (s *FileStore) Save(cfg Config) error {
	cfg.ApplyDefaults()
	if s.secret != nil && strings.TrimSpace(cfg.Token) != "" {
		if _, err := s.secret.Save(cfg.Token); err != nil {
			return err
		}
	}
	cookiePath := filepath.Join(filepath.Dir(s.path), "session_cookie")
	if strings.TrimSpace(cfg.SessionCookie) != "" {
		if err := os.MkdirAll(filepath.Dir(cookiePath), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(cookiePath, []byte(cfg.SessionCookie), 0o600); err != nil {
			return err
		}
	} else {
		_ = os.Remove(cookiePath)
	}
	return s.writeConfig(cfg)
}

func (s *FileStore) Clear() error {
	err := os.Remove(s.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	cookiePath := filepath.Join(filepath.Dir(s.path), "session_cookie")
	_ = os.Remove(cookiePath)
	if s.secret != nil {
		return s.secret.Clear()
	}
	return nil
}

func (s *FileStore) writeConfig(cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	stored := cfg
	stored.Token = ""
	stored.SessionCookie = ""
	stored.TokenBackend = ""
	data, _ := json.MarshalIndent(stored, "", "  ")
	return os.WriteFile(s.path, data, 0o600)
}

func defaultConfigDir(goos string, getenv func(string) string) (string, error) {
	switch goos {
	case "windows":
		if dir := strings.TrimSpace(getenv("APPDATA")); dir != "" {
			return dir, nil
		}
		home := strings.TrimSpace(getenv("USERPROFILE"))
		if home == "" {
			home = strings.TrimSpace(getenv("HOME"))
		}
		if home == "" {
			return "", errors.New("HOME is not set")
		}
		return filepath.Join(home, "AppData", "Roaming"), nil
	case "darwin":
		home := strings.TrimSpace(getenv("HOME"))
		if home == "" {
			return "", errors.New("HOME is not set")
		}
		return filepath.Join(home, "Library", "Application Support"), nil
	default:
		if dir := strings.TrimSpace(getenv("XDG_CONFIG_HOME")); dir != "" {
			return dir, nil
		}
		home := strings.TrimSpace(getenv("HOME"))
		if home == "" {
			return "", errors.New("HOME is not set")
		}
		return filepath.Join(home, ".config"), nil
	}
}
