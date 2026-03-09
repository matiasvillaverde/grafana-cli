package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
)

var errSecretNotFound = errors.New("secret not found")

const disableKeyringEnv = "GRAFANA_CLI_DISABLE_KEYRING"

type secretStoreFactory func(configPath string) SecretStore

// SecretStore persists the auth token outside the main config JSON.
type SecretStore interface {
	Load() (token, backend string, err error)
	Save(token string) (backend string, err error)
	Clear() error
}

type secretBackend interface {
	Name() string
	Load() (string, error)
	Save(string) error
	Clear() error
}

type chainSecretStore struct {
	backends []secretBackend
}

func newDefaultSecretStore(configPath string) SecretStore {
	backends := make([]secretBackend, 0, 2)
	if !keyringDisabled(os.Getenv(disableKeyringEnv)) {
		backends = append(backends, newKeyringBackend(configPath))
	}
	backends = append(backends, &fileSecretBackend{path: filepath.Join(filepath.Dir(configPath), "token")})

	return &chainSecretStore{
		backends: backends,
	}
}

func newDefaultSecretStoreFactory() secretStoreFactory {
	return newDefaultSecretStore
}

func keyringDisabled(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func (s *chainSecretStore) Load() (string, string, error) {
	if len(s.backends) == 0 {
		return "", "", nil
	}

	sawNotFound := false
	var firstErr error
	for _, backend := range s.backends {
		token, err := backend.Load()
		switch {
		case err == nil:
			return token, backend.Name(), nil
		case errors.Is(err, errSecretNotFound):
			sawNotFound = true
		default:
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	if sawNotFound {
		return "", "", nil
	}
	return "", "", firstErr
}

func (s *chainSecretStore) Save(token string) (string, error) {
	if len(s.backends) == 0 {
		return "", nil
	}

	var firstErr error
	for index, backend := range s.backends {
		if err := backend.Save(token); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for clearIndex, other := range s.backends {
			if clearIndex == index {
				continue
			}
			if err := other.Clear(); err != nil && !errors.Is(err, errSecretNotFound) {
				return "", err
			}
		}
		return backend.Name(), nil
	}

	return "", firstErr
}

func (s *chainSecretStore) Clear() error {
	var firstErr error
	for _, backend := range s.backends {
		if err := backend.Clear(); err != nil && !errors.Is(err, errSecretNotFound) && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

type keyringClient interface {
	Get(service, user string) (string, error)
	Set(service, user, password string) error
	Delete(service, user string) error
}

type nativeKeyringClient struct{}

func (nativeKeyringClient) Get(service, user string) (string, error) {
	return keyring.Get(service, user)
}

func (nativeKeyringClient) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}

func (nativeKeyringClient) Delete(service, user string) error {
	return keyring.Delete(service, user)
}

type keyringBackend struct {
	client  keyringClient
	service string
	user    string
}

func newKeyringBackend(configPath string) *keyringBackend {
	return &keyringBackend{
		client:  nativeKeyringClient{},
		service: "grafana-cli",
		user:    secretAccount(configPath),
	}
}

func (b *keyringBackend) Name() string {
	return "keyring"
}

func (b *keyringBackend) Load() (string, error) {
	token, err := b.client.Get(b.service, b.user)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", errSecretNotFound
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(token) == "" {
		return "", errSecretNotFound
	}
	return token, nil
}

func (b *keyringBackend) Save(token string) error {
	if strings.TrimSpace(token) == "" {
		return b.Clear()
	}
	return b.client.Set(b.service, b.user, token)
}

func (b *keyringBackend) Clear() error {
	err := b.client.Delete(b.service, b.user)
	if errors.Is(err, keyring.ErrNotFound) {
		return errSecretNotFound
	}
	return err
}

type fileSecretBackend struct {
	path string
}

func (b *fileSecretBackend) Name() string {
	return "file"
}

func (b *fileSecretBackend) Load() (string, error) {
	data, err := os.ReadFile(b.path)
	if errors.Is(err, os.ErrNotExist) {
		return "", errSecretNotFound
	}
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", errSecretNotFound
	}
	return token, nil
}

func (b *fileSecretBackend) Save(token string) error {
	if strings.TrimSpace(token) == "" {
		return b.Clear()
	}
	if err := os.MkdirAll(filepath.Dir(b.path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(b.path, []byte(token), 0o600)
}

func (b *fileSecretBackend) Clear() error {
	err := os.Remove(b.path)
	if errors.Is(err, os.ErrNotExist) {
		return errSecretNotFound
	}
	return err
}

func secretAccount(configPath string) string {
	hash := sha256.Sum256([]byte(filepath.Clean(configPath)))
	return "default-" + hex.EncodeToString(hash[:8])
}
