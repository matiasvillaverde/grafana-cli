//go:build integrationcloud

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/matiasvillaverde/grafana-cli/internal/cli"
	"github.com/matiasvillaverde/grafana-cli/internal/config"
)

type liveCloudConfig struct {
	cloudURL     string
	token        string
	stack        string
	accessRegion string
	orgSlug      string
	usageYear    int
	usageMonth   int
}

func TestRealCloudCommands(t *testing.T) {
	cfg := loadLiveCloudConfig(t)

	t.Run("stacks list", func(t *testing.T) {
		payload := runLiveCloudCommand(t, cfg, "cloud", "stacks", "list")
		items, ok := payload["items"].([]any)
		if !ok || len(items) == 0 {
			t.Fatalf("expected cloud stacks list items, got %+v", payload)
		}
		if !containsCloudStack(items, cfg.stack) {
			t.Fatalf("expected stack %q in cloud stacks list, got %+v", cfg.stack, items)
		}
	})

	t.Run("stacks inspect", func(t *testing.T) {
		payload := runLiveCloudCommand(t, cfg, "cloud", "stacks", "inspect", "--stack", cfg.stack)
		stack, ok := payload["stack"].(map[string]any)
		if !ok || strings.TrimSpace(stringValue(stack, "slug")) != cfg.stack {
			t.Fatalf("expected inspect payload for stack %q, got %+v", cfg.stack, payload)
		}
		if _, ok := payload["inferred_endpoints"].(map[string]any); !ok {
			t.Fatalf("expected inferred_endpoints in inspect payload, got %+v", payload)
		}
	})

	t.Run("stack plugins list and get", func(t *testing.T) {
		payload := runLiveCloudCommand(t, cfg, "cloud", "stacks", "plugins", "list", "--stack", cfg.stack, "--limit", "5")
		items, ok := payload["items"].([]any)
		if !ok {
			t.Fatalf("expected plugin list items, got %+v", payload)
		}
		if len(items) == 0 {
			t.Skip("stack has no installed plugins to fetch individually")
		}
		first, ok := items[0].(map[string]any)
		if !ok {
			t.Fatalf("expected first plugin record, got %+v", items[0])
		}
		pluginID := strings.TrimSpace(stringValue(first, "id"))
		if pluginID == "" {
			t.Fatalf("expected plugin id in %+v", first)
		}

		plugin := runLiveCloudCommand(t, cfg, "cloud", "stacks", "plugins", "get", "--stack", cfg.stack, "--plugin", pluginID)
		if strings.TrimSpace(stringValue(plugin, "id")) != pluginID {
			t.Fatalf("expected plugin %q, got %+v", pluginID, plugin)
		}
	})

	if cfg.accessRegion != "" {
		t.Run("access policies list", func(t *testing.T) {
			payload := runLiveCloudCommand(t, cfg, "cloud", "access-policies", "list", "--region", cfg.accessRegion, "--limit", "20")
			if _, ok := payload["items"].([]any); !ok {
				t.Fatalf("expected access policy items, got %+v", payload)
			}
		})
	}

	if cfg.orgSlug != "" && cfg.usageYear > 0 && cfg.usageMonth > 0 {
		t.Run("billed usage get", func(t *testing.T) {
			payload := runLiveCloudCommand(
				t,
				cfg,
				"cloud", "billed-usage", "get",
				"--org-slug", cfg.orgSlug,
				"--year", strconv.Itoa(cfg.usageYear),
				"--month", strconv.Itoa(cfg.usageMonth),
			)
			if strings.TrimSpace(stringValue(payload, "org_slug")) != cfg.orgSlug {
				t.Fatalf("expected org slug %q, got %+v", cfg.orgSlug, payload)
			}
			if _, ok := payload["items"].([]any); !ok {
				t.Fatalf("expected billed usage items, got %+v", payload)
			}
		})
	}
}

func loadLiveCloudConfig(t *testing.T) liveCloudConfig {
	t.Helper()

	cfg := liveCloudConfig{
		cloudURL:     strings.TrimSpace(os.Getenv("GRAFANA_CLI_INTEGRATION_CLOUD_URL")),
		token:        strings.TrimSpace(os.Getenv("GRAFANA_CLI_INTEGRATION_CLOUD_TOKEN")),
		stack:        strings.TrimSpace(os.Getenv("GRAFANA_CLI_INTEGRATION_CLOUD_STACK")),
		accessRegion: strings.TrimSpace(os.Getenv("GRAFANA_CLI_INTEGRATION_CLOUD_ACCESS_REGION")),
		orgSlug:      strings.TrimSpace(os.Getenv("GRAFANA_CLI_INTEGRATION_CLOUD_ORG_SLUG")),
		usageYear:    intEnv("GRAFANA_CLI_INTEGRATION_CLOUD_BILLED_USAGE_YEAR"),
		usageMonth:   intEnv("GRAFANA_CLI_INTEGRATION_CLOUD_BILLED_USAGE_MONTH"),
	}

	missing := make([]string, 0, 3)
	for key, value := range map[string]string{
		"GRAFANA_CLI_INTEGRATION_CLOUD_URL":   cfg.cloudURL,
		"GRAFANA_CLI_INTEGRATION_CLOUD_TOKEN": cfg.token,
		"GRAFANA_CLI_INTEGRATION_CLOUD_STACK": cfg.stack,
	} {
		if value == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		t.Skipf("real cloud contract tests disabled: missing %s", strings.Join(missing, ", "))
	}
	return cfg
}

func runLiveCloudCommand(t *testing.T, cfg liveCloudConfig, args ...string) map[string]any {
	t.Helper()

	t.Setenv("GRAFANA_CLI_DISABLE_KEYRING", "1")

	storePath := filepath.Join(t.TempDir(), "config.json")
	store := config.NewProfileStore(storePath)
	if err := store.Save(config.Config{
		Token:    cfg.token,
		CloudURL: cfg.cloudURL,
	}); err != nil {
		t.Fatalf("save live cloud config: %v", err)
	}

	app := cli.NewApp(store)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app.Out = &stdout
	app.Err = &stderr
	if code := app.Run(context.Background(), args); code != 0 {
		t.Fatalf("command failed: grafana %s\nstderr: %s", strings.Join(args, " "), stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode command output: %v\nstdout: %s", err, stdout.String())
	}
	return payload
}

func containsCloudStack(items []any, stack string) bool {
	for _, item := range items {
		record, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(stringValue(record, "slug")) == stack {
			return true
		}
	}
	return false
}

func stringValue(record map[string]any, key string) string {
	value, ok := record[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

func intEnv(name string) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}
