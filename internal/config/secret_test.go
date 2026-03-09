package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
)

type stubBackend struct {
	name      string
	loadToken string
	loadErr   error
	saveErr   error
	clearErr  error
	clearHits int
}

func (b *stubBackend) Name() string {
	return b.name
}

func (b *stubBackend) Load() (string, error) {
	if b.loadErr != nil {
		return "", b.loadErr
	}
	return b.loadToken, nil
}

func (b *stubBackend) Save(string) error {
	return b.saveErr
}

func (b *stubBackend) Clear() error {
	b.clearHits++
	return b.clearErr
}

type stubKeyringClient struct {
	getValue  string
	getErr    error
	setErr    error
	deleteErr error

	lastSetService string
	lastSetUser    string
	lastSetValue   string
	lastDeleteUser string
}

func (c *stubKeyringClient) Get(_, _ string) (string, error) {
	return c.getValue, c.getErr
}

func (c *stubKeyringClient) Set(service, user, password string) error {
	c.lastSetService = service
	c.lastSetUser = user
	c.lastSetValue = password
	return c.setErr
}

func (c *stubKeyringClient) Delete(_, user string) error {
	c.lastDeleteUser = user
	return c.deleteErr
}

func TestChainSecretStoreLoad(t *testing.T) {
	store := &chainSecretStore{
		backends: []secretBackend{
			&stubBackend{name: "first", loadToken: "secret"},
			&stubBackend{name: "second", loadToken: "ignored"},
		},
	}
	token, backend, err := store.Load()
	if err != nil || token != "secret" || backend != "first" {
		t.Fatalf("unexpected first-backend load: token=%q backend=%q err=%v", token, backend, err)
	}

	store = &chainSecretStore{
		backends: []secretBackend{
			&stubBackend{name: "first", loadErr: errSecretNotFound},
			&stubBackend{name: "second", loadToken: "fallback"},
		},
	}
	token, backend, err = store.Load()
	if err != nil || token != "fallback" || backend != "second" {
		t.Fatalf("unexpected fallback load: token=%q backend=%q err=%v", token, backend, err)
	}

	store = &chainSecretStore{
		backends: []secretBackend{
			&stubBackend{name: "first", loadErr: errors.New("boom")},
			&stubBackend{name: "second", loadErr: errSecretNotFound},
		},
	}
	token, backend, err = store.Load()
	if err != nil || token != "" || backend != "" {
		t.Fatalf("expected no secret when at least one backend reports not found, got token=%q backend=%q err=%v", token, backend, err)
	}

	loadErr := errors.New("fatal")
	store = &chainSecretStore{
		backends: []secretBackend{
			&stubBackend{name: "first", loadErr: loadErr},
			&stubBackend{name: "second", loadErr: errors.New("later")},
		},
	}
	if _, _, err := store.Load(); !errors.Is(err, loadErr) {
		t.Fatalf("expected first error, got %v", err)
	}

	store = &chainSecretStore{}
	token, backend, err = store.Load()
	if err != nil || token != "" || backend != "" {
		t.Fatalf("expected empty store load to succeed, got token=%q backend=%q err=%v", token, backend, err)
	}
}

func TestChainSecretStoreSave(t *testing.T) {
	first := &stubBackend{name: "first"}
	second := &stubBackend{name: "second"}
	store := &chainSecretStore{backends: []secretBackend{first, second}}

	backend, err := store.Save("secret")
	if err != nil || backend != "first" {
		t.Fatalf("expected first backend save, got backend=%q err=%v", backend, err)
	}
	if second.clearHits != 1 {
		t.Fatalf("expected stale backend to be cleared")
	}

	saveErr := errors.New("save failed")
	first = &stubBackend{name: "first", saveErr: saveErr}
	second = &stubBackend{name: "second"}
	store = &chainSecretStore{backends: []secretBackend{first, second}}
	backend, err = store.Save("secret")
	if err != nil || backend != "second" {
		t.Fatalf("expected fallback backend save, got backend=%q err=%v", backend, err)
	}
	if first.clearHits != 1 {
		t.Fatalf("expected first backend to be cleared after fallback save")
	}

	clearErr := errors.New("clear failed")
	store = &chainSecretStore{
		backends: []secretBackend{
			&stubBackend{name: "first", saveErr: errors.New("nope"), clearErr: clearErr},
			&stubBackend{name: "second"},
		},
	}
	if _, err := store.Save("secret"); !errors.Is(err, clearErr) {
		t.Fatalf("expected clear error, got %v", err)
	}

	store = &chainSecretStore{
		backends: []secretBackend{
			&stubBackend{name: "first", saveErr: saveErr},
			&stubBackend{name: "second", saveErr: errors.New("second")},
		},
	}
	if _, err := store.Save("secret"); !errors.Is(err, saveErr) {
		t.Fatalf("expected first save error, got %v", err)
	}

	store = &chainSecretStore{}
	backend, err = store.Save("secret")
	if err != nil || backend != "" {
		t.Fatalf("expected empty store save to succeed, got backend=%q err=%v", backend, err)
	}
}

func TestChainSecretStoreClear(t *testing.T) {
	firstErr := errors.New("first")
	store := &chainSecretStore{
		backends: []secretBackend{
			&stubBackend{name: "first", clearErr: errSecretNotFound},
			&stubBackend{name: "second", clearErr: firstErr},
			&stubBackend{name: "third"},
		},
	}
	if err := store.Clear(); !errors.Is(err, firstErr) {
		t.Fatalf("expected first non-not-found clear error, got %v", err)
	}
}

func TestKeyringBackend(t *testing.T) {
	client := &stubKeyringClient{}
	backend := &keyringBackend{
		client:  client,
		service: "grafana-cli",
		user:    "user",
	}

	client.getValue = "secret"
	token, err := backend.Load()
	if err != nil || token != "secret" {
		t.Fatalf("unexpected keyring load: token=%q err=%v", token, err)
	}

	client.getValue = ""
	token, err = backend.Load()
	if !errors.Is(err, errSecretNotFound) || token != "" {
		t.Fatalf("expected empty token to map to not found, got token=%q err=%v", token, err)
	}

	client.getErr = keyring.ErrNotFound
	if _, err := backend.Load(); !errors.Is(err, errSecretNotFound) {
		t.Fatalf("expected keyring not found mapping, got %v", err)
	}

	client.getErr = errors.New("boom")
	if _, err := backend.Load(); err == nil {
		t.Fatalf("expected keyring load error")
	}

	client.getErr = nil
	if err := backend.Save("secret"); err != nil {
		t.Fatalf("unexpected keyring save error: %v", err)
	}
	if client.lastSetService != "grafana-cli" || client.lastSetUser != "user" || client.lastSetValue != "secret" {
		t.Fatalf("unexpected keyring save payload: %+v", client)
	}

	if err := backend.Save(""); err != nil {
		t.Fatalf("expected empty token save to clear, got %v", err)
	}
	if client.lastDeleteUser != "user" {
		t.Fatalf("expected delete on empty save")
	}

	client.deleteErr = keyring.ErrNotFound
	if err := backend.Clear(); !errors.Is(err, errSecretNotFound) {
		t.Fatalf("expected clear not found mapping, got %v", err)
	}
}

func TestNativeKeyringClient(t *testing.T) {
	keyring.MockInit()
	client := nativeKeyringClient{}

	if err := client.Set("grafana-cli", "user", "secret"); err != nil {
		t.Fatalf("unexpected native set error: %v", err)
	}
	value, err := client.Get("grafana-cli", "user")
	if err != nil || value != "secret" {
		t.Fatalf("unexpected native get result: value=%q err=%v", value, err)
	}
	if err := client.Delete("grafana-cli", "user"); err != nil {
		t.Fatalf("unexpected native delete error: %v", err)
	}
	if _, err := client.Get("grafana-cli", "user"); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("expected native get not found after delete, got %v", err)
	}
}

func TestFileSecretBackend(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "token")
	backend := &fileSecretBackend{path: path}

	if _, err := backend.Load(); !errors.Is(err, errSecretNotFound) {
		t.Fatalf("expected missing token to map to not found, got %v", err)
	}

	if err := backend.Save("secret"); err != nil {
		t.Fatalf("unexpected token save error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read token failed: %v", err)
	}
	if string(data) != "secret" {
		t.Fatalf("unexpected token data: %q", data)
	}

	token, err := backend.Load()
	if err != nil || token != "secret" {
		t.Fatalf("unexpected token load: token=%q err=%v", token, err)
	}

	if err := os.WriteFile(path, []byte("  "), 0o600); err != nil {
		t.Fatalf("write empty token failed: %v", err)
	}
	if _, err := backend.Load(); !errors.Is(err, errSecretNotFound) {
		t.Fatalf("expected blank token to map to not found, got %v", err)
	}

	dirPath := filepath.Join(tmp, "load-dir")
	if err := os.MkdirAll(dirPath, 0o700); err != nil {
		t.Fatalf("mkdir load dir failed: %v", err)
	}
	backend = &fileSecretBackend{path: dirPath}
	if _, err := backend.Load(); err == nil {
		t.Fatalf("expected directory load to fail")
	}

	backend = &fileSecretBackend{path: path}
	if err := backend.Save(""); err != nil {
		t.Fatalf("expected empty save to clear, got %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected token file to be removed")
	}

	if err := backend.Clear(); !errors.Is(err, errSecretNotFound) {
		t.Fatalf("expected missing clear to map to not found, got %v", err)
	}

	parentFile := filepath.Join(tmp, "parent-file")
	if err := os.WriteFile(parentFile, []byte("x"), 0o600); err != nil {
		t.Fatalf("write parent file failed: %v", err)
	}
	backend = &fileSecretBackend{path: filepath.Join(parentFile, "token")}
	if err := backend.Save("secret"); err == nil {
		t.Fatalf("expected save failure when parent is a file")
	}

	clearDirPath := filepath.Join(tmp, "dir")
	if err := os.MkdirAll(clearDirPath, 0o700); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(clearDirPath, "child"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write child failed: %v", err)
	}
	backend = &fileSecretBackend{path: clearDirPath}
	if err := backend.Clear(); err == nil {
		t.Fatalf("expected clear failure for directory path")
	}
}

func TestSecretAccountAndDefaultSecretStore(t *testing.T) {
	pathA := filepath.Join("/tmp", "a", "config.json")
	pathB := filepath.Join("/tmp", "b", "config.json")
	if secretAccount(pathA) == secretAccount(pathB) {
		t.Fatalf("expected secret account hash to vary by config path")
	}
	if account := secretAccount(pathA); account != secretAccount(pathA) {
		t.Fatalf("expected secret account hash to be stable")
	}

	t.Setenv(disableKeyringEnv, "")
	store, ok := newDefaultSecretStore(pathA).(*chainSecretStore)
	if !ok {
		t.Fatalf("expected default secret store chain")
	}
	if len(store.backends) != 2 || store.backends[0].Name() != "keyring" || store.backends[1].Name() != "file" {
		t.Fatalf("unexpected default backends: %+v", store.backends)
	}
}

func TestKeyringDisabled(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "unset", value: "", want: false},
		{name: "zero", value: "0", want: false},
		{name: "false", value: "false", want: false},
		{name: "no", value: "no", want: false},
		{name: "off", value: "off", want: false},
		{name: "one", value: "1", want: true},
		{name: "true", value: "true", want: true},
		{name: "yes", value: "yes", want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := keyringDisabled(tc.value); got != tc.want {
				t.Fatalf("unexpected disabled state for %q: got %v want %v", tc.value, got, tc.want)
			}
		})
	}

	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv(disableKeyringEnv, "1")
	store, ok := newDefaultSecretStore(path).(*chainSecretStore)
	if !ok {
		t.Fatalf("expected default secret store chain")
	}
	if len(store.backends) != 1 || store.backends[0].Name() != "file" {
		t.Fatalf("expected file-only backends when keyring is disabled, got %+v", store.backends)
	}
}
