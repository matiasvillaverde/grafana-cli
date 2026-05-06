package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type stubSecretStore struct {
	loadToken   string
	loadBackend string
	loadErr     error
	saveBackend string
	saveErr     error
	saveHook    func()
	clearHook   func()
	clearErr    error
}

func (s *stubSecretStore) Load() (string, string, error) {
	return s.loadToken, s.loadBackend, s.loadErr
}

func (s *stubSecretStore) Save(string) (string, error) {
	if s.saveHook != nil {
		s.saveHook()
	}
	return s.saveBackend, s.saveErr
}

func (s *stubSecretStore) Clear() error {
	if s.clearHook != nil {
		s.clearHook()
	}
	return s.clearErr
}

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

func TestDefaultConfigDir(t *testing.T) {
	getenv := func(values map[string]string) func(string) string {
		return func(key string) string {
			return values[key]
		}
	}

	linuxDir, err := defaultConfigDir("linux", getenv(map[string]string{
		"XDG_CONFIG_HOME": "/tmp/xdg",
		"HOME":            "/tmp/home",
	}))
	if err != nil || linuxDir != "/tmp/xdg" {
		t.Fatalf("unexpected linux XDG dir: %q err=%v", linuxDir, err)
	}

	linuxDir, err = defaultConfigDir("linux", getenv(map[string]string{"HOME": "/tmp/home"}))
	if err != nil || linuxDir != filepath.Join("/tmp/home", ".config") {
		t.Fatalf("unexpected linux HOME dir: %q err=%v", linuxDir, err)
	}
	if _, err := defaultConfigDir("linux", getenv(map[string]string{})); err == nil {
		t.Fatalf("expected linux HOME error")
	}

	darwinDir, err := defaultConfigDir("darwin", getenv(map[string]string{"HOME": "/tmp/home"}))
	if err != nil || darwinDir != filepath.Join("/tmp/home", "Library", "Application Support") {
		t.Fatalf("unexpected darwin dir: %q err=%v", darwinDir, err)
	}
	if _, err := defaultConfigDir("darwin", getenv(map[string]string{})); err == nil {
		t.Fatalf("expected darwin HOME error")
	}

	windowsDir, err := defaultConfigDir("windows", getenv(map[string]string{"APPDATA": `C:\Users\mati\AppData\Roaming`}))
	if err != nil || windowsDir != `C:\Users\mati\AppData\Roaming` {
		t.Fatalf("unexpected windows APPDATA dir: %q err=%v", windowsDir, err)
	}

	windowsDir, err = defaultConfigDir("windows", getenv(map[string]string{"USERPROFILE": `C:\Users\mati`}))
	if err != nil || windowsDir != filepath.Join(`C:\Users\mati`, "AppData", "Roaming") {
		t.Fatalf("unexpected windows USERPROFILE dir: %q err=%v", windowsDir, err)
	}
	if _, err := defaultConfigDir("windows", getenv(map[string]string{})); err == nil {
		t.Fatalf("expected windows HOME error")
	}
}

func TestDefaultPath(t *testing.T) {
	tmp := t.TempDir()

	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("APPDATA", "")
	t.Setenv("USERPROFILE", "")

	var expected string
	switch runtime.GOOS {
	case "windows":
		appData := filepath.Join(tmp, "Roaming")
		t.Setenv("APPDATA", appData)
		expected = filepath.Join(appData, "grafana-cli", "config.json")
	case "darwin":
		home := filepath.Join(tmp, "home")
		t.Setenv("HOME", home)
		expected = filepath.Join(home, "Library", "Application Support", "grafana-cli", "config.json")
	default:
		xdg := filepath.Join(tmp, "xdg")
		t.Setenv("HOME", filepath.Join(tmp, "home"))
		t.Setenv("XDG_CONFIG_HOME", xdg)
		expected = filepath.Join(xdg, "grafana-cli", "config.json")
	}

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("unexpected DefaultPath error: %v", err)
	}
	if path != expected {
		t.Fatalf("expected %q, got %q", expected, path)
	}

	switch runtime.GOOS {
	case "windows":
		t.Setenv("APPDATA", "")
		t.Setenv("USERPROFILE", "")
		t.Setenv("HOME", "")
	default:
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "")
	}
	if _, err := DefaultPath(); err == nil {
		t.Fatalf("expected DefaultPath error when home config is unavailable")
	}
}

func TestFileStoreLoadSaveClear(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "sub", "config.json")
	tokenPath := filepath.Join(tmp, "sub", "token")
	store := NewFileStoreWithSecretStore(path, &chainSecretStore{
		backends: []secretBackend{&fileSecretBackend{path: tokenPath}},
	})

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
	if cfg.Token != "" || cfg.TokenBackend != "" {
		t.Fatalf("expected empty secret state, got %+v", cfg)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
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

	configBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config failed: %v", err)
	}
	if strings.Contains(string(configBytes), "token") {
		t.Fatalf("config file should not contain auth token: %s", configBytes)
	}

	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("read token failed: %v", err)
	}
	if strings.TrimSpace(string(tokenBytes)) != "token" {
		t.Fatalf("unexpected token payload: %q", tokenBytes)
	}

	cfg, err = store.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.Token != "token" || cfg.TokenBackend != "file" || cfg.OrgID != 42 {
		t.Fatalf("unexpected roundtrip config: %+v", cfg)
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("clear failed: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected config file to be removed")
	}
	if _, err := os.Stat(tokenPath); !os.IsNotExist(err) {
		t.Fatalf("expected token file to be removed")
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("clear should ignore missing files: %v", err)
	}
}

func TestFileStoreMigratesLegacyToken(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.json")
	tokenPath := filepath.Join(tmp, "token")
	store := NewFileStoreWithSecretStore(path, &chainSecretStore{
		backends: []secretBackend{&fileSecretBackend{path: tokenPath}},
	})

	if err := os.WriteFile(path, []byte(`{"base_url":"https://stack.grafana.net","token":"legacy-token"}`), 0o600); err != nil {
		t.Fatalf("write legacy config failed: %v", err)
	}

	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("unexpected legacy load error: %v", err)
	}
	if cfg.Token != "legacy-token" || cfg.TokenBackend != "file" {
		t.Fatalf("expected migrated token, got %+v", cfg)
	}

	configBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated config failed: %v", err)
	}
	if strings.Contains(string(configBytes), "legacy-token") || strings.Contains(string(configBytes), `"token"`) {
		t.Fatalf("legacy token should be scrubbed from config file: %s", configBytes)
	}

	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("read migrated token failed: %v", err)
	}
	if strings.TrimSpace(string(tokenBytes)) != "legacy-token" {
		t.Fatalf("unexpected migrated token file payload: %q", tokenBytes)
	}
}

func TestFileStoreErrorPaths(t *testing.T) {
	tmp := t.TempDir()

	dirPath := filepath.Join(tmp, "as-dir")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	store := NewFileStoreWithSecretStore(dirPath, &chainSecretStore{
		backends: []secretBackend{&fileSecretBackend{path: filepath.Join(tmp, "token")}},
	})
	if _, err := store.Load(); err == nil {
		t.Fatalf("expected load error for directory path")
	}

	parentFile := filepath.Join(tmp, "parent-file")
	if err := os.WriteFile(parentFile, []byte("x"), 0o600); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	store = NewFileStoreWithSecretStore(filepath.Join(parentFile, "config.json"), &chainSecretStore{
		backends: []secretBackend{&fileSecretBackend{path: filepath.Join(tmp, "token")}},
	})
	if err := store.Save(Config{Token: "x"}); err == nil {
		t.Fatalf("expected save error when parent is a file")
	}

	dirStorePath := filepath.Join(tmp, "dir-remove")
	if err := os.MkdirAll(dirStorePath, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dirStorePath, "child"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	store = NewFileStoreWithSecretStore(dirStorePath, &chainSecretStore{
		backends: []secretBackend{&fileSecretBackend{path: filepath.Join(tmp, "token")}},
	})
	if err := store.Clear(); err == nil {
		t.Fatalf("expected clear error for directory path")
	}
}

func TestFileStoreSecretErrors(t *testing.T) {
	loadErr := errors.New("load failed")
	saveErr := errors.New("save failed")
	clearErr := errors.New("clear failed")

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"base_url":"https://stack.grafana.net"}`), 0o600); err != nil {
		t.Fatalf("write config failed: %v", err)
	}
	store := NewFileStoreWithSecretStore(path, &stubSecretStore{loadErr: loadErr})
	if _, err := store.Load(); !errors.Is(err, loadErr) {
		t.Fatalf("expected wrapped load error, got %v", err)
	}

	store = NewFileStoreWithSecretStore(filepath.Join(t.TempDir(), "config.json"), &stubSecretStore{saveErr: saveErr})
	if err := store.Save(Config{Token: "x"}); !errors.Is(err, saveErr) {
		t.Fatalf("expected wrapped save error, got %v", err)
	}

	store = NewFileStoreWithSecretStore(filepath.Join(t.TempDir(), "config.json"), &stubSecretStore{clearErr: clearErr})
	if err := store.Clear(); !errors.Is(err, clearErr) {
		t.Fatalf("expected wrapped clear error, got %v", err)
	}

	path = filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"token":"legacy-token"}`), 0o600); err != nil {
		t.Fatalf("write legacy config failed: %v", err)
	}
	store = NewFileStoreWithSecretStore(path, &stubSecretStore{loadErr: loadErr})
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("expected legacy token fallback, got %v", err)
	}
	if cfg.Token != "legacy-token" || cfg.TokenBackend != "" {
		t.Fatalf("unexpected legacy fallback config: %+v", cfg)
	}

	path = filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"token":"legacy-token"}`), 0o600); err != nil {
		t.Fatalf("write legacy config failed: %v", err)
	}
	store = NewFileStoreWithSecretStore(path, &stubSecretStore{saveErr: saveErr})
	cfg, err = store.Load()
	if err != nil {
		t.Fatalf("expected legacy migration save fallback, got %v", err)
	}
	if cfg.Token != "legacy-token" || cfg.TokenBackend != "" {
		t.Fatalf("unexpected legacy migration fallback config: %+v", cfg)
	}
}

func TestFileStoreMigrationWriteConfigErrorAndClearWithoutSecret(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "sub", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"token":"legacy-token"}`), 0o600); err != nil {
		t.Fatalf("write legacy config failed: %v", err)
	}
	store := NewFileStoreWithSecretStore(path, &stubSecretStore{
		saveBackend: "stub",
		saveHook: func() {
			parent := filepath.Dir(path)
			if err := os.Remove(path); err != nil {
				t.Fatalf("remove config failed: %v", err)
			}
			if err := os.Remove(parent); err != nil {
				t.Fatalf("remove parent failed: %v", err)
			}
			if err := os.WriteFile(parent, []byte("x"), 0o600); err != nil {
				t.Fatalf("write parent file failed: %v", err)
			}
		},
	})
	if _, err := store.Load(); err == nil {
		t.Fatalf("expected migration writeConfig failure")
	}

	clearPath := filepath.Join(tmp, "clear.json")
	if err := os.WriteFile(clearPath, []byte("{}"), 0o600); err != nil {
		t.Fatalf("write clear config failed: %v", err)
	}
	store = NewFileStoreWithSecretStore(clearPath, nil)
	if err := store.Clear(); err != nil {
		t.Fatalf("expected clear without secret to succeed, got %v", err)
	}

	loadPath := filepath.Join(tmp, "load.json")
	if err := os.WriteFile(loadPath, []byte(`{"base_url":"https://stack.grafana.net","token":"legacy-token"}`), 0o600); err != nil {
		t.Fatalf("write load config failed: %v", err)
	}
	store = NewFileStoreWithSecretStore(loadPath, nil)
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("expected load without secret store to succeed, got %v", err)
	}
	if cfg.Token != "legacy-token" {
		t.Fatalf("expected legacy token to remain when secret store is nil, got %+v", cfg)
	}

	noSecretPath := filepath.Join(tmp, "no-secret.json")
	if err := os.WriteFile(noSecretPath, []byte(`{"base_url":"https://stack.grafana.net"}`), 0o600); err != nil {
		t.Fatalf("write no-secret config failed: %v", err)
	}
	store = NewFileStoreWithSecretStore(noSecretPath, &stubSecretStore{})
	cfg, err = store.Load()
	if err != nil {
		t.Fatalf("expected empty secret load to succeed, got %v", err)
	}
	if cfg.Token != "" || cfg.TokenBackend != "" {
		t.Fatalf("expected empty secret state, got %+v", cfg)
	}
}

func TestIsAuthenticatedWithSessionCookie(t *testing.T) {
	if !(Config{SessionCookie: "abc"}).IsAuthenticated() {
		t.Fatalf("expected session cookie to authenticate")
	}
	if (Config{SessionCookie: "  "}).IsAuthenticated() {
		t.Fatalf("expected whitespace-only session cookie to be unauthenticated")
	}
}

func TestFileStoreSessionCookieRoundtrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "sub", "config.json")
	store := NewFileStoreWithSecretStore(path, nil)

	cfg := Config{
		BaseURL:       "https://stack.grafana.net",
		SessionCookie: "my-session-value",
	}
	if err := store.Save(cfg); err != nil {
		t.Fatalf("save with session cookie failed: %v", err)
	}

	// Session cookie must be stored in a separate file, not in config.json.
	configBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config failed: %v", err)
	}
	if strings.Contains(string(configBytes), "my-session-value") || strings.Contains(string(configBytes), "session_cookie") {
		t.Fatalf("config file should not contain session cookie: %s", configBytes)
	}

	cookiePath := filepath.Join(tmp, "sub", "session_cookie")
	cookieBytes, err := os.ReadFile(cookiePath)
	if err != nil {
		t.Fatalf("read session cookie file failed: %v", err)
	}
	if strings.TrimSpace(string(cookieBytes)) != "my-session-value" {
		t.Fatalf("unexpected session cookie payload: %q", cookieBytes)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.SessionCookie != "my-session-value" {
		t.Fatalf("expected session cookie roundtrip, got %q", loaded.SessionCookie)
	}

	// Saving without a session cookie should remove the file.
	cfg.SessionCookie = ""
	if err := store.Save(cfg); err != nil {
		t.Fatalf("save without session cookie failed: %v", err)
	}
	if _, err := os.Stat(cookiePath); !os.IsNotExist(err) {
		t.Fatalf("expected session cookie file to be removed")
	}

	loaded, err = store.Load()
	if err != nil {
		t.Fatalf("load after clear failed: %v", err)
	}
	if loaded.SessionCookie != "" {
		t.Fatalf("expected empty session cookie after removal, got %q", loaded.SessionCookie)
	}
}

func TestFileStoreClearRemovesSessionCookie(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.json")
	store := NewFileStoreWithSecretStore(path, nil)

	if err := store.Save(Config{SessionCookie: "sess"}); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	cookiePath := filepath.Join(tmp, "session_cookie")
	if _, err := os.Stat(cookiePath); err != nil {
		t.Fatalf("expected session cookie file to exist: %v", err)
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("clear failed: %v", err)
	}
	if _, err := os.Stat(cookiePath); !os.IsNotExist(err) {
		t.Fatalf("expected session cookie file to be removed after clear")
	}
}

func TestFileStoreSaveTokenOnlyWhenPresent(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.json")
	tokenPath := filepath.Join(tmp, "token")
	store := NewFileStoreWithSecretStore(path, &chainSecretStore{
		backends: []secretBackend{&fileSecretBackend{path: tokenPath}},
	})

	// Save with session cookie only (no token) should not call secret store save.
	if err := store.Save(Config{SessionCookie: "sess"}); err != nil {
		t.Fatalf("save with session cookie only failed: %v", err)
	}
	if _, err := os.Stat(tokenPath); !os.IsNotExist(err) {
		t.Fatalf("expected no token file when only session cookie is set")
	}
}

func TestFileStoreSessionCookieWriteErrors(t *testing.T) {
	// MkdirAll error: parent path is a file, not a directory.
	tmp := t.TempDir()
	parentFile := filepath.Join(tmp, "parent-file")
	if err := os.WriteFile(parentFile, []byte("x"), 0o600); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	store := NewFileStoreWithSecretStore(filepath.Join(parentFile, "sub", "config.json"), nil)
	if err := store.Save(Config{SessionCookie: "sess"}); err == nil {
		t.Fatalf("expected mkdir error for session cookie")
	}

	// WriteFile error: cookie path is a directory.
	tmp2 := t.TempDir()
	cookieDir := filepath.Join(tmp2, "session_cookie")
	if err := os.MkdirAll(filepath.Join(cookieDir, "child"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	store = NewFileStoreWithSecretStore(filepath.Join(tmp2, "config.json"), nil)
	if err := store.Save(Config{SessionCookie: "sess"}); err == nil {
		t.Fatalf("expected write error for session cookie")
	}
}

func TestNewFileStoreUsesDefaultSecretStore(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "config.json"))
	if store.secret == nil {
		t.Fatalf("expected default secret store")
	}
}
