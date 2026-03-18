//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rogpeppe/go-internal/testscript"
)

type integrationHarness struct {
	baseURL         string
	promURL         string
	logsURL         string
	tracesURL       string
	token           string
	serviceAccount  string
	syntheticsToken string
	baseProxy       *httptest.Server
	cloudProxy      *httptest.Server
	oncallProxy     *httptest.Server
	promProxy       *httptest.Server
	tracesProxy     *httptest.Server
	syntheticsProxy *httptest.Server
}

var harness *integrationHarness

func TestMain(m *testing.M) {
	if filepath.Base(os.Args[0]) == "grafana" {
		os.Exit(grafanaMain())
	}

	var err error
	harness, err = newIntegrationHarness()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(2)
	}

	// testscript.Main copies this binary to a temp $PATH directory as "grafana"
	// so scripts can call `exec grafana`. It then calls os.Exit(m.Run()), bypassing
	// any deferred harness.Close(). The OS reclaims httptest.Server resources on
	// process exit, so this is a cosmetic resource leak only.
	testscript.Main(m, map[string]func(){
		"grafana": func() {
			os.Exit(grafanaMain())
		},
	})
}

func grafanaMain() int {
	return run(os.Args[1:])
}

func TestAuthConfig(t *testing.T) {
	testscript.Run(t, harness.params("auth-config"))
}

func TestSchemaGlobalFlags(t *testing.T) {
	testscript.Run(t, harness.params("schema-global-flags"))
}

func TestDashboardsDatasources(t *testing.T) {
	testscript.Run(t, harness.params("dashboards-datasources"))
}

func TestFoldersAnnotationsAlerting(t *testing.T) {
	testscript.Run(t, harness.params("folders-annotations-alerting"))
}

func TestRuntimeObservability(t *testing.T) {
	testscript.Run(t, harness.params("runtime-observability"))
}

func TestInvestigationIncidents(t *testing.T) {
	testscript.Run(t, harness.params("investigation-incidents"))
}

func TestAssistantAccessCloud(t *testing.T) {
	testscript.Run(t, harness.params("assistant-access-cloud"))
}

func TestAgentWorkflows(t *testing.T) {
	testscript.Run(t, harness.params("agent-workflows"))
}

func newIntegrationHarness() (*integrationHarness, error) {
	baseURL := strings.TrimSpace(os.Getenv("GRAFANA_CLI_INTEGRATION_GRAFANA_URL"))
	promURL := strings.TrimSpace(os.Getenv("GRAFANA_CLI_INTEGRATION_PROM_URL"))
	logsURL := strings.TrimSpace(os.Getenv("GRAFANA_CLI_INTEGRATION_LOGS_URL"))
	tracesURL := strings.TrimSpace(os.Getenv("GRAFANA_CLI_INTEGRATION_TRACES_URL"))
	token := strings.TrimSpace(os.Getenv("GRAFANA_CLI_INTEGRATION_TOKEN"))
	serviceAccount := strings.TrimSpace(os.Getenv("GRAFANA_CLI_INTEGRATION_SERVICE_ACCOUNT_ID"))
	syntheticsToken := strings.TrimSpace(os.Getenv("GRAFANA_CLI_INTEGRATION_SYNTHETICS_TOKEN"))

	missing := make([]string, 0, 6)
	for key, value := range map[string]string{
		"GRAFANA_CLI_INTEGRATION_GRAFANA_URL":        baseURL,
		"GRAFANA_CLI_INTEGRATION_PROM_URL":           promURL,
		"GRAFANA_CLI_INTEGRATION_LOGS_URL":           logsURL,
		"GRAFANA_CLI_INTEGRATION_TRACES_URL":         tracesURL,
		"GRAFANA_CLI_INTEGRATION_TOKEN":              token,
		"GRAFANA_CLI_INTEGRATION_SERVICE_ACCOUNT_ID": serviceAccount,
	} {
		if value == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing integration environment: %s", strings.Join(missing, ", "))
	}
	if syntheticsToken == "" {
		syntheticsToken = "synthetics-integration-token"
	}

	baseProxy, err := newGrafanaProxy(baseURL)
	if err != nil {
		return nil, err
	}
	cloudProxy := newCloudProxy()
	oncallProxy := newOnCallProxy()
	promProxy, err := newPrometheusProxy(promURL)
	if err != nil {
		baseProxy.Close()
		cloudProxy.Close()
		oncallProxy.Close()
		return nil, err
	}
	tracesProxy, err := newTracesProxy(tracesURL)
	if err != nil {
		baseProxy.Close()
		cloudProxy.Close()
		oncallProxy.Close()
		promProxy.Close()
		return nil, err
	}
	syntheticsProxy := newSyntheticsProxy()

	return &integrationHarness{
		baseURL:         baseURL,
		promURL:         promURL,
		logsURL:         logsURL,
		tracesURL:       tracesURL,
		token:           token,
		serviceAccount:  serviceAccount,
		syntheticsToken: syntheticsToken,
		baseProxy:       baseProxy,
		cloudProxy:      cloudProxy,
		oncallProxy:     oncallProxy,
		promProxy:       promProxy,
		tracesProxy:     tracesProxy,
		syntheticsProxy: syntheticsProxy,
	}, nil
}

func (h *integrationHarness) Close() {
	if h == nil {
		return
	}
	if h.baseProxy != nil {
		h.baseProxy.Close()
	}
	if h.cloudProxy != nil {
		h.cloudProxy.Close()
	}
	if h.oncallProxy != nil {
		h.oncallProxy.Close()
	}
	if h.promProxy != nil {
		h.promProxy.Close()
	}
	if h.tracesProxy != nil {
		h.tracesProxy.Close()
	}
	if h.syntheticsProxy != nil {
		h.syntheticsProxy.Close()
	}
}

func (h *integrationHarness) params(group string) testscript.Params {
	return testscript.Params{
		Dir:                 filepath.Join("testdata", "integration", group),
		RequireExplicitExec: true,
		RequireUniqueNames:  true,
		Setup: func(env *testscript.Env) error {
			configHome := filepath.Join(env.Cd, ".config")
			runID := integrationRunID()
			env.Setenv("HOME", env.Cd)
			env.Setenv("XDG_CONFIG_HOME", configHome)
			env.Setenv("GRAFANA_CLI_DISABLE_KEYRING", "1")
			env.Setenv("GRAFANA_ITEST_RUN_ID", runID)
			env.Setenv("GRAFANA_TOKEN", h.token)
			env.Setenv("GRAFANA_BASE_URL", h.baseProxy.URL)
			env.Setenv("GRAFANA_CLOUD_URL", h.cloudProxy.URL)
			env.Setenv("GRAFANA_PROM_URL", h.promProxy.URL)
			// GRAFANA_LOGS_URL points directly to Loki (no intercepting proxy).
			// Unlike Grafana, Prometheus, and Tempo, Loki has no stub layer here,
			// so a Loki outage will fail the runtime-observability shard without
			// a clear error message.
			env.Setenv("GRAFANA_LOGS_URL", h.logsURL)
			env.Setenv("GRAFANA_TRACES_URL", h.tracesProxy.URL)
			env.Setenv("GRAFANA_ONCALL_URL", h.oncallProxy.URL)
			env.Setenv("GRAFANA_SYNTHETICS_BACKEND_URL", h.syntheticsProxy.URL)
			env.Setenv("GRAFANA_SYNTHETICS_TOKEN", h.syntheticsToken)
			env.Setenv("GRAFANA_SERVICE_ACCOUNT_ID", h.serviceAccount)
			return nil
		},
	}
}

func integrationRunID() string {
	runID := strings.ToLower(strconv.FormatInt(time.Now().UnixNano(), 36))
	if len(runID) > 8 {
		runID = runID[len(runID)-8:]
	}
	return runID
}

func newGrafanaProxy(upstream string) (*httptest.Server, error) {
	target, err := url.Parse(upstream)
	if err != nil {
		return nil, fmt.Errorf("parse grafana url: %w", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		http.Error(w, err.Error(), http.StatusBadGateway)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/query-history":
			writeJSON(w, http.StatusOK, map[string]any{
				"result": map[string]any{
					"totalCount": 1,
					"queryHistory": []map[string]any{
						{
							"uid":           "qh-1",
							"datasourceUid": "prometheus",
							"queryText":     "sum(rate(checkout_requests_total[5m]))",
							"starred":       true,
							"createdAt":     "2026-03-09T10:00:00Z",
						},
					},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/plugins/grafana-slo-app/resources/v1/slo":
			writeJSON(w, http.StatusOK, map[string]any{
				"slos": []map[string]any{
					{
						"id":          "slo-1",
						"uid":         "slo-1",
						"name":        "Checkout availability",
						"description": "99.9% availability for checkout requests",
					},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/plugins/grafana-irm-app/resources/api/v1/IncidentsService.QueryIncidentPreviews":
			writeJSON(w, http.StatusOK, map[string]any{
				"items": []map[string]any{
					{
						"id":          "incident-1",
						"title":       "Checkout latency spike",
						"description": "Local integration incident preview",
						"status":      "active",
					},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/plugins/grafana-assistant-app/resources/api/v1/assistant/chats":
			writeJSON(w, http.StatusOK, map[string]any{
				"chatId":  "chat-1",
				"status":  "queued",
				"message": "Integration assistant response",
			})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/plugins/grafana-assistant-app/resources/api/v1/chats/"):
			writeJSON(w, http.StatusOK, map[string]any{
				"chatId": "chat-1",
				"status": "completed",
				"messages": []map[string]any{
					{
						"role":    "assistant",
						"content": "Investigated checkout latency.",
					},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/plugins/grafana-assistant-app/resources/api/v1/assistant/skills":
			writeJSON(w, http.StatusOK, map[string]any{
				"items": []map[string]any{
					{
						"id":   "investigate",
						"name": "Investigate incidents",
					},
				},
			})
		default:
			proxy.ServeHTTP(w, r)
		}
	}))

	return server, nil
}

func newCloudProxy() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/stacks":
			writeJSON(w, http.StatusOK, map[string]any{
				"items": []map[string]any{
					{
						"slug":   "local-stack",
						"region": "us",
					},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/instances/local-stack/datasources":
			writeJSON(w, http.StatusOK, map[string]any{
				"items": []map[string]any{
					{
						"uid":  "prometheus-local",
						"name": "metrics",
						"type": "prometheus",
						"url":  "https://prometheus.local-stack.grafana.net",
					},
					{
						"uid":  "loki-local",
						"name": "logs",
						"type": "loki",
						"url":  "https://logs.local-stack.grafana.net",
					},
					{
						"uid":  "tempo-local",
						"name": "traces",
						"type": "tempo",
						"url":  "https://tempo.local-stack.grafana.net",
					},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/instances/local-stack/connections":
			writeJSON(w, http.StatusOK, map[string]any{
				"connections": []map[string]any{
					{
						"type": "oncall",
						"details": map[string]any{
							"oncallApiUrl": "https://oncall.local-stack.grafana.net",
						},
					},
				},
				"privateConnectivityInfo": map[string]any{
					"tenants": []map[string]any{
						{"type": "prometheus"},
						{"type": "logs"},
					},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/instances/local-stack/plugins":
			if r.URL.Query().Get("pageCursor") == "cursor-2" {
				writeJSON(w, http.StatusOK, map[string]any{
					"items": []map[string]any{
						{
							"id":      "grafana-incident-app",
							"name":    "Grafana IRM",
							"version": "1.1.0",
						},
					},
				})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"items": []map[string]any{
					{
						"id":      "grafana-oncall-app",
						"name":    "Grafana OnCall",
						"version": "1.0.0",
					},
				},
				"metadata": map[string]any{
					"pagination": map[string]any{"nextPage": "/api/instances/local-stack/plugins?pageCursor=cursor-2"},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/instances/local-stack/plugins/grafana-oncall-app":
			writeJSON(w, http.StatusOK, map[string]any{
				"id":      "grafana-oncall-app",
				"name":    "Grafana OnCall",
				"version": "1.0.0",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/orgs/local-org/billed-usage":
			writeJSON(w, http.StatusOK, map[string]any{
				"items": []map[string]any{
					{
						"dimensionName": "Logs",
						"amountDue":     100.5,
						"periodStart":   "2024-09-01T00:00:00Z",
						"periodEnd":     "2024-09-30T23:59:59Z",
						"usages": []map[string]any{
							{"stackName": "local-stack.grafana.net"},
						},
					},
					{
						"dimensionName": "Metrics",
						"amountDue":     778.41,
						"periodStart":   "2024-09-01T00:00:00Z",
						"periodEnd":     "2024-09-30T23:59:59Z",
						"usages": []map[string]any{
							{"stackName": "local-stack.grafana.net"},
						},
					},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/accesspolicies":
			writeJSON(w, http.StatusOK, map[string]any{
				"items": []map[string]any{
					{
						"id":     "policy-1",
						"name":   "Local integration policy",
						"status": "active",
						"region": "us",
					},
				},
			})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/accesspolicies/"):
			writeJSON(w, http.StatusOK, map[string]any{
				"id":     "policy-1",
				"name":   "Local integration policy",
				"status": "active",
				"region": "us",
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

func newPrometheusProxy(upstream string) (*httptest.Server, error) {
	target, err := url.Parse(upstream)
	if err != nil {
		return nil, fmt.Errorf("parse prometheus url: %w", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		http.Error(w, err.Error(), http.StatusBadGateway)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rewritten := r.Clone(r.Context())
		if strings.HasPrefix(rewritten.URL.Path, "/api/prom/") {
			rewritten.URL.Path = strings.TrimPrefix(rewritten.URL.Path, "/api/prom")
			if rewritten.URL.Path == "" {
				rewritten.URL.Path = "/"
			}
		}
		proxy.ServeHTTP(w, rewritten)
	}))

	return server, nil
}

func newTracesProxy(upstream string) (*httptest.Server, error) {
	target, err := url.Parse(upstream)
	if err != nil {
		return nil, fmt.Errorf("parse traces url: %w", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		http.Error(w, err.Error(), http.StatusBadGateway)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/search" {
			writeJSON(w, http.StatusOK, map[string]any{
				"traces": []map[string]any{
					{
						"traceID":         "463ac35c9f6413ad48485a3953bb6124",
						"rootServiceName": "checkout",
						"rootTraceName":   "GET /checkout",
					},
				},
			})
			return
		}
		proxy.ServeHTTP(w, r)
	}))

	return server, nil
}

func newOnCallProxy() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/schedules/" {
			writeJSON(w, http.StatusOK, map[string]any{
				"results": []map[string]any{
					{
						"name": "Primary Operations",
						"type": "calendar",
						"team": map[string]any{
							"name": "Operations",
							"slug": "ops",
						},
					},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
}

func newSyntheticsProxy() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/check":
			writeJSON(w, http.StatusOK, map[string]any{
				"items": []map[string]any{
					{
						"id":   1,
						"name": "Checkout homepage",
						"type": "http",
					},
				},
			})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/check/"):
			writeJSON(w, http.StatusOK, map[string]any{
				"id":   1,
				"name": "Checkout homepage",
				"type": "http",
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.Copy(w, bytes.NewReader(data))
}
