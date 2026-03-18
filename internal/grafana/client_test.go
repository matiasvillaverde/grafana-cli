package grafana

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/matiasvillaverde/grafana-cli/internal/config"
)

type errReader struct{}

func (e errReader) Read(_ []byte) (int, error) { return 0, errors.New("read failure") }
func (e errReader) Close() error               { return nil }

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestClientRawSetsHeadersAndBody(t *testing.T) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/test" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("missing auth header")
		}
		if r.Header.Get("X-Grafana-Org-Id") != "12" {
			t.Fatalf("missing org header")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("missing content type")
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "key") {
			t.Fatalf("expected JSON body")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client := NewClient(config.Config{BaseURL: srv.URL, Token: "token", OrgID: 12}, srv.Client())
	resp, err := client.Raw(context.Background(), http.MethodPost, "/api/test", map[string]any{"key": "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result, ok := resp.(map[string]any)
	if !ok {
		t.Fatalf("expected map response")
	}
	if result["ok"] != true {
		t.Fatalf("unexpected response: %+v", result)
	}
}

func TestHTTPErrorErrorString(t *testing.T) {
	err := (&HTTPError{StatusCode: 500, Body: "boom"}).Error()
	if !strings.Contains(err, "status=500") || !strings.Contains(err, "boom") {
		t.Fatalf("unexpected error string: %s", err)
	}
}

func TestNewClientDefaultsAndCustomDoer(t *testing.T) {
	byDefault := NewClient(config.Config{}, nil)
	if byDefault.doer == nil {
		t.Fatalf("expected default doer")
	}
	if byDefault.cfg.BaseURL == "" || byDefault.cfg.CloudURL == "" {
		t.Fatalf("expected defaults to be applied")
	}

	custom := doerFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})
	withCustom := NewClient(config.Config{BaseURL: "https://x", CloudURL: "https://y"}, custom)
	if withCustom.doer == nil || withCustom.cfg.BaseURL != "https://x" || withCustom.cfg.CloudURL != "https://y" {
		t.Fatalf("expected provided config and doer")
	}
}

func TestClientDomainMethods(t *testing.T) {
	hits := make(map[string]int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits[r.URL.Path]++
		if strings.HasPrefix(r.URL.Path, "/api/v1/check") && r.Header.Get("Authorization") != "Bearer sm-token" {
			t.Fatalf("missing synthetic monitoring auth header")
		}
		if r.URL.Path == "/api/serviceaccounts/search" && r.URL.Query().Get("perpage") != "20" {
			t.Fatalf("missing service account perpage query: %s", r.URL.RawQuery)
		}
		if r.URL.Path == "/api/v1/accesspolicies" && r.URL.Query().Get("region") != "us" {
			t.Fatalf("missing access policy region query: %s", r.URL.RawQuery)
		}
		if r.URL.Path == "/api/orgs/local-org/billed-usage" {
			if r.URL.Query().Get("year") != "2024" || r.URL.Query().Get("month") != "9" {
				t.Fatalf("missing billed usage query params: %s", r.URL.RawQuery)
			}
		}
		if r.URL.Path == "/api/instances/local-stack/plugins" && r.URL.Query().Get("pageCursor") == "cursor-1" {
			if r.URL.Query().Get("pageSize") != "50" {
				t.Fatalf("missing plugin page size query params: %s", r.URL.RawQuery)
			}
		}
		if r.URL.Path == "/api/search" {
			if r.URL.Query().Get("q") == "" && r.URL.Query().Get("type") != "dash-db" {
				t.Fatalf("missing type query")
			}
		}
		if r.URL.Path == "/api/prom/api/v1/query_range" && r.URL.Query().Get("query") != "up" {
			t.Fatalf("missing metrics query")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	cfg := config.Config{
		BaseURL:       srv.URL,
		CloudURL:      srv.URL,
		PrometheusURL: srv.URL,
		LogsURL:       srv.URL,
		TracesURL:     srv.URL,
	}
	client := NewClient(cfg, srv.Client())

	if _, err := client.CloudStacks(context.Background()); err != nil {
		t.Fatalf("cloud stacks failed: %v", err)
	}
	if _, err := client.CloudStackDatasources(context.Background(), "local-stack"); err != nil {
		t.Fatalf("cloud stack datasources failed: %v", err)
	}
	if _, err := client.CloudStackConnections(context.Background(), "local-stack"); err != nil {
		t.Fatalf("cloud stack connections failed: %v", err)
	}
	if _, err := client.CloudStackPlugins(context.Background(), "local-stack"); err != nil {
		t.Fatalf("cloud stack plugins failed: %v", err)
	}
	if _, err := client.CloudStackPluginsPage(context.Background(), CloudStackPluginListRequest{Stack: "local-stack", PageSize: 50, PageCursor: "cursor-1"}); err != nil {
		t.Fatalf("cloud stack plugins page failed: %v", err)
	}
	if _, err := client.CloudStackPlugin(context.Background(), "local-stack", "grafana-oncall-app"); err != nil {
		t.Fatalf("cloud stack plugin failed: %v", err)
	}
	if _, err := client.CloudBilledUsage(context.Background(), CloudBilledUsageRequest{OrgSlug: "local-org", Year: 2024, Month: 9}); err != nil {
		t.Fatalf("cloud billed usage failed: %v", err)
	}
	if _, err := client.CloudAccessPolicies(context.Background(), CloudAccessPolicyListRequest{Region: "us", PageSize: 10}); err != nil {
		t.Fatalf("cloud access policies failed: %v", err)
	}
	if _, err := client.CloudAccessPolicy(context.Background(), "ap-1", "us"); err != nil {
		t.Fatalf("cloud access policy failed: %v", err)
	}
	if _, err := client.SearchDashboards(context.Background(), "errors", "prod", 10); err != nil {
		t.Fatalf("search dashboards failed: %v", err)
	}
	if _, err := client.CreateDashboard(context.Background(), map[string]any{"title": "x"}, 7, true); err != nil {
		t.Fatalf("create dashboard failed: %v", err)
	}
	if _, err := client.ListDatasources(context.Background()); err != nil {
		t.Fatalf("list datasources failed: %v", err)
	}
	if _, err := client.ServiceAccounts(context.Background(), ServiceAccountListRequest{Query: "graf", Page: 2, Limit: 20}); err != nil {
		t.Fatalf("service accounts failed: %v", err)
	}
	if _, err := client.ServiceAccount(context.Background(), 1); err != nil {
		t.Fatalf("service account failed: %v", err)
	}
	if _, err := client.AssistantChat(context.Background(), "Investigate spike", "chat-1"); err != nil {
		t.Fatalf("assistant chat failed: %v", err)
	}
	if _, err := client.AssistantChatStatus(context.Background(), "chat/1"); err != nil {
		t.Fatalf("assistant chat status failed: %v", err)
	}
	if _, err := client.AssistantSkills(context.Background()); err != nil {
		t.Fatalf("assistant skills failed: %v", err)
	}
	if _, err := client.MetricsRange(context.Background(), "up", "", "", "30s"); err != nil {
		t.Fatalf("metrics failed: %v", err)
	}
	if _, err := client.LogsRange(context.Background(), "{job=\"x\"}", "", "", 5); err != nil {
		t.Fatalf("logs failed: %v", err)
	}
	if _, err := client.TracesSearch(context.Background(), "{ status = error }", "", "", 5); err != nil {
		t.Fatalf("traces failed: %v", err)
	}
	if _, err := client.SyntheticChecks(context.Background(), SyntheticCheckListRequest{BackendURL: srv.URL, Token: "sm-token", IncludeAlerts: true}); err != nil {
		t.Fatalf("synthetic checks failed: %v", err)
	}
	if _, err := client.SyntheticCheck(context.Background(), SyntheticCheckGetRequest{BackendURL: srv.URL, Token: "sm-token", ID: 123}); err != nil {
		t.Fatalf("synthetic check failed: %v", err)
	}

	if hits["/api/v1/stacks"] != 1 || hits["/api/instances/local-stack/datasources"] != 1 || hits["/api/instances/local-stack/connections"] != 1 || hits["/api/instances/local-stack/plugins"] != 2 || hits["/api/instances/local-stack/plugins/grafana-oncall-app"] != 1 || hits["/api/orgs/local-org/billed-usage"] != 1 || hits["/api/v1/accesspolicies"] != 1 || hits["/api/v1/accesspolicies/ap-1"] != 1 || hits["/api/search"] != 2 || hits["/api/dashboards/db"] != 1 || hits["/api/datasources"] != 1 {
		t.Fatalf("unexpected hit counts: %+v", hits)
	}
	if hits["/api/serviceaccounts/search"] != 1 || hits["/api/serviceaccounts/1"] != 1 {
		t.Fatalf("expected service account endpoints hit: %+v", hits)
	}
	if hits["/api/plugins/grafana-assistant-app/resources/api/v1/assistant/chats"] != 1 {
		t.Fatalf("expected assistant chat endpoint hit")
	}
	statusHits := 0
	for path, count := range hits {
		if strings.Contains(path, "/api/plugins/grafana-assistant-app/resources/api/v1/chats/chat") {
			statusHits += count
		}
	}
	if statusHits != 1 {
		t.Fatalf("expected assistant status endpoint hit, got %+v", hits)
	}
	if hits["/api/plugins/grafana-assistant-app/resources/api/v1/assistant/skills"] != 1 {
		t.Fatalf("expected assistant skills endpoint hit")
	}
	if hits["/api/v1/check"] != 1 || hits["/api/v1/check/123"] != 1 {
		t.Fatalf("expected synthetic check endpoints hit: %+v", hits)
	}
	if hits["/api/prom/api/v1/query_range"] != 1 || hits["/loki/api/v1/query_range"] != 1 || hits["/api/search"] < 2 {
		t.Fatalf("runtime endpoints not hit: %+v", hits)
	}
}

func TestClientMissingRuntimeURLs(t *testing.T) {
	client := NewClient(config.Config{BaseURL: "https://grafana.com"}, doerFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, errors.New("should not call")
	}))

	if _, err := client.MetricsRange(context.Background(), "up", "", "", ""); !errors.Is(err, ErrMissingPrometheusURL) {
		t.Fatalf("expected ErrMissingPrometheusURL, got %v", err)
	}
	if _, err := client.LogsRange(context.Background(), "{}", "", "", 0); !errors.Is(err, ErrMissingLogsURL) {
		t.Fatalf("expected ErrMissingLogsURL, got %v", err)
	}
	if _, err := client.TracesSearch(context.Background(), "{}", "", "", 0); !errors.Is(err, ErrMissingTracesURL) {
		t.Fatalf("expected ErrMissingTracesURL, got %v", err)
	}
}

func TestClientMissingBaseURLPaths(t *testing.T) {
	client := &Client{cfg: config.Config{}, doer: doerFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, errors.New("should not call")
	})}

	if _, err := client.Raw(context.Background(), http.MethodGet, "/x", nil); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL, got %v", err)
	}
	if _, err := client.CloudStacks(context.Background()); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for cloud stacks, got %v", err)
	}
	if _, err := client.CloudStackDatasources(context.Background(), "local-stack"); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for cloud stack datasources, got %v", err)
	}
	if _, err := client.CloudStackConnections(context.Background(), "local-stack"); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for cloud stack connections, got %v", err)
	}
	if _, err := client.CloudStackPlugins(context.Background(), "local-stack"); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for cloud stack plugins, got %v", err)
	}
	if _, err := client.CloudStackPluginsPage(context.Background(), CloudStackPluginListRequest{Stack: "local-stack", PageSize: 50, PageCursor: "cursor-1"}); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for cloud stack plugin page, got %v", err)
	}
	if _, err := client.CloudStackPlugin(context.Background(), "local-stack", "grafana-oncall-app"); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for cloud stack plugin, got %v", err)
	}
	if _, err := client.CloudBilledUsage(context.Background(), CloudBilledUsageRequest{OrgSlug: "local-org", Year: 2024, Month: 9}); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for cloud billed usage, got %v", err)
	}
	if _, err := client.CloudAccessPolicies(context.Background(), CloudAccessPolicyListRequest{Region: "us"}); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for cloud access policies, got %v", err)
	}
	if _, err := client.CloudAccessPolicy(context.Background(), "ap-1", "us"); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for cloud access policy, got %v", err)
	}
	if _, err := client.SearchDashboards(context.Background(), "", "", 0); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for search dashboards, got %v", err)
	}
	if _, err := client.CreateDashboard(context.Background(), map[string]any{"title": "x"}, 0, false); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for create dashboard, got %v", err)
	}
	if _, err := client.ListDatasources(context.Background()); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for list datasources, got %v", err)
	}
	if _, err := client.GetDashboard(context.Background(), "x"); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for get dashboard, got %v", err)
	}
	if _, err := client.DeleteDashboard(context.Background(), "x"); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for delete dashboard, got %v", err)
	}
	if _, err := client.DashboardVersions(context.Background(), "x", 1); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for dashboard versions, got %v", err)
	}
	if _, err := client.RenderDashboard(context.Background(), DashboardRenderRequest{UID: "x"}); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for render dashboard, got %v", err)
	}
	if _, err := client.ListFolders(context.Background()); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for list folders, got %v", err)
	}
	if _, err := client.GetFolder(context.Background(), "x"); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for get folder, got %v", err)
	}
	if _, err := client.ServiceAccounts(context.Background(), ServiceAccountListRequest{}); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for service accounts, got %v", err)
	}
	if _, err := client.ServiceAccount(context.Background(), 1); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for service account, got %v", err)
	}
	if _, err := client.ListAnnotations(context.Background(), AnnotationListRequest{}); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for list annotations, got %v", err)
	}
	if _, err := client.AlertingRules(context.Background()); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for alerting rules, got %v", err)
	}
	if _, err := client.AlertingContactPoints(context.Background()); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for alerting contact points, got %v", err)
	}
	if _, err := client.AlertingPolicies(context.Background()); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for alerting policies, got %v", err)
	}
	if _, err := client.AssistantChat(context.Background(), "x", ""); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for assistant chat, got %v", err)
	}
	if _, err := client.AssistantChatStatus(context.Background(), "c1"); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for assistant status, got %v", err)
	}
	if _, err := client.AssistantSkills(context.Background()); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for assistant skills, got %v", err)
	}
	if _, err := client.SyntheticChecks(context.Background(), SyntheticCheckListRequest{}); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for synthetic checks, got %v", err)
	}
	if _, err := client.SyntheticCheck(context.Background(), SyntheticCheckGetRequest{ID: 1}); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for synthetic check, got %v", err)
	}
}

func TestCloudStackMethodsInvalidCloudURL(t *testing.T) {
	client := NewClient(config.Config{CloudURL: "://bad"}, doerFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, errors.New("should not call")
	}))

	if _, err := client.CloudStackDatasources(context.Background(), "local-stack"); err == nil {
		t.Fatalf("expected invalid cloud url error for stack datasources")
	}
	if _, err := client.CloudStackConnections(context.Background(), "local-stack"); err == nil {
		t.Fatalf("expected invalid cloud url error for stack connections")
	}
	if _, err := client.CloudStackPlugins(context.Background(), "local-stack"); err == nil {
		t.Fatalf("expected invalid cloud url error for stack plugins")
	}
	if _, err := client.CloudStackPluginsPage(context.Background(), CloudStackPluginListRequest{Stack: "local-stack", PageSize: 50, PageCursor: "cursor-1"}); err == nil {
		t.Fatalf("expected invalid cloud url error for stack plugin page")
	}
	if _, err := client.CloudStackPlugin(context.Background(), "local-stack", "grafana-oncall-app"); err == nil {
		t.Fatalf("expected invalid cloud url error for stack plugin")
	}
	if _, err := client.CloudBilledUsage(context.Background(), CloudBilledUsageRequest{OrgSlug: "local-org", Year: 2024, Month: 9}); err == nil {
		t.Fatalf("expected invalid cloud url error for billed usage")
	}
}

func TestClientInvalidURLBuildErrors(t *testing.T) {
	client := &Client{cfg: config.Config{BaseURL: "://bad", CloudURL: "://bad"}, doer: doerFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("should not call")
	})}
	if _, err := client.Raw(context.Background(), http.MethodGet, "/x", nil); err == nil {
		t.Fatalf("expected raw URL parse error")
	}
	if _, err := client.CloudStacks(context.Background()); err == nil {
		t.Fatalf("expected cloud URL parse error")
	}
	if _, err := client.ListDatasources(context.Background()); err == nil {
		t.Fatalf("expected datasource URL parse error")
	}
}

func TestMethodInvalidURLBuildErrors(t *testing.T) {
	client := &Client{cfg: config.Config{
		BaseURL:       "://bad",
		PrometheusURL: "://bad",
		LogsURL:       "://bad",
		TracesURL:     "://bad",
	}, doer: doerFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("should not call")
	})}

	if _, err := client.SearchDashboards(context.Background(), "x", "y", 1); err == nil {
		t.Fatalf("expected search dashboards URL error")
	}
	if _, err := client.CreateDashboard(context.Background(), map[string]any{"title": "x"}, 1, true); err == nil {
		t.Fatalf("expected create dashboard URL error")
	}
	if _, err := client.GetDashboard(context.Background(), "x"); err == nil {
		t.Fatalf("expected get dashboard URL error")
	}
	if _, err := client.DeleteDashboard(context.Background(), "x"); err == nil {
		t.Fatalf("expected delete dashboard URL error")
	}
	if _, err := client.DashboardVersions(context.Background(), "x", 1); err == nil {
		t.Fatalf("expected dashboard versions URL error")
	}
	if _, err := client.RenderDashboard(context.Background(), DashboardRenderRequest{UID: "x"}); err == nil {
		t.Fatalf("expected render dashboard URL error")
	}
	if _, err := client.ListFolders(context.Background()); err == nil {
		t.Fatalf("expected folders URL error")
	}
	if _, err := client.GetFolder(context.Background(), "x"); err == nil {
		t.Fatalf("expected folder URL error")
	}
	if _, err := client.ListAnnotations(context.Background(), AnnotationListRequest{}); err == nil {
		t.Fatalf("expected annotations URL error")
	}
	if _, err := client.AlertingRules(context.Background()); err == nil {
		t.Fatalf("expected alerting rules URL error")
	}
	if _, err := client.AlertingContactPoints(context.Background()); err == nil {
		t.Fatalf("expected alerting contact points URL error")
	}
	if _, err := client.AlertingPolicies(context.Background()); err == nil {
		t.Fatalf("expected alerting policies URL error")
	}
	if _, err := client.AssistantChat(context.Background(), "x", ""); err == nil {
		t.Fatalf("expected assistant chat URL error")
	}
	if _, err := client.AssistantChatStatus(context.Background(), "x"); err == nil {
		t.Fatalf("expected assistant status URL error")
	}
	if _, err := client.AssistantSkills(context.Background()); err == nil {
		t.Fatalf("expected assistant skills URL error")
	}
	if _, err := client.MetricsRange(context.Background(), "up", "", "", ""); err == nil {
		t.Fatalf("expected metrics URL error")
	}
	if _, err := client.LogsRange(context.Background(), "{}", "", "", 1); err == nil {
		t.Fatalf("expected logs URL error")
	}
	if _, err := client.TracesSearch(context.Background(), "{}", "", "", 1); err == nil {
		t.Fatalf("expected traces URL error")
	}
}

func TestAggregateSnapshot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "prom") {
			_, _ = w.Write([]byte(`{"data":{"result":[1,2]}}`))
			return
		}
		if strings.Contains(r.URL.Path, "loki") {
			_, _ = w.Write([]byte(`{"data":{"result":[1]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"traces":[1,2,3]}`))
	}))
	defer srv.Close()

	client := NewClient(config.Config{
		PrometheusURL: srv.URL,
		LogsURL:       srv.URL,
		TracesURL:     srv.URL,
	}, srv.Client())

	snapshot, err := client.AggregateSnapshot(context.Background(), AggregateRequest{
		MetricExpr: "up",
		LogQuery:   "{}",
		TraceQuery: "{ status = error }",
		Step:       "30s",
		Limit:      5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snapshot.Metrics == nil || snapshot.Logs == nil || snapshot.Traces == nil {
		t.Fatalf("expected full snapshot")
	}
}

func TestAggregateSnapshotErrorPaths(t *testing.T) {
	client := NewClient(config.Config{}, doerFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("doer should not be called")
	}))
	if _, err := client.AggregateSnapshot(context.Background(), AggregateRequest{}); !errors.Is(err, ErrMissingPrometheusURL) {
		t.Fatalf("expected prometheus URL error, got %v", err)
	}

	client = NewClient(config.Config{PrometheusURL: "https://prom"}, doerFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	}))
	if _, err := client.AggregateSnapshot(context.Background(), AggregateRequest{}); !errors.Is(err, ErrMissingLogsURL) {
		t.Fatalf("expected logs URL error, got %v", err)
	}

	client = NewClient(config.Config{PrometheusURL: "https://prom", LogsURL: "https://logs"}, doerFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	}))
	if _, err := client.AggregateSnapshot(context.Background(), AggregateRequest{}); !errors.Is(err, ErrMissingTracesURL) {
		t.Fatalf("expected traces URL error, got %v", err)
	}
}

func TestMethodQueryParamBranches(t *testing.T) {
	requests := make([]*http.Request, 0)
	client := NewClient(config.Config{
		BaseURL:       "https://base",
		PrometheusURL: "https://prom",
		LogsURL:       "https://logs",
		TracesURL:     "https://traces",
	}, doerFunc(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{}`)),
			Header:     make(http.Header),
		}, nil
	}))

	if _, err := client.SearchDashboards(context.Background(), "", "", 0); err != nil {
		t.Fatalf("search dashboard failed: %v", err)
	}
	if _, err := client.CreateDashboard(context.Background(), map[string]any{"title": "x"}, 0, true); err != nil {
		t.Fatalf("create dashboard without folder failed: %v", err)
	}
	if _, err := client.MetricsRange(context.Background(), "up", "s", "e", "1m"); err != nil {
		t.Fatalf("metrics branch failed: %v", err)
	}
	if _, err := client.LogsRange(context.Background(), "{}", "s", "e", 0); err != nil {
		t.Fatalf("logs branch failed: %v", err)
	}
	if _, err := client.TracesSearch(context.Background(), "{}", "s", "e", 0); err != nil {
		t.Fatalf("traces branch failed: %v", err)
	}
	if _, err := client.DashboardVersions(context.Background(), "ops", 0); err != nil {
		t.Fatalf("dashboard versions branch failed: %v", err)
	}
	if _, err := client.ListAnnotations(context.Background(), AnnotationListRequest{
		DashboardUID: "ops",
		PanelID:      7,
		Limit:        15,
		From:         "s",
		To:           "e",
		Tags:         []string{"prod", "checkout"},
		Type:         "annotation",
	}); err != nil {
		t.Fatalf("annotations branch failed: %v", err)
	}
	if _, err := client.RenderDashboard(context.Background(), DashboardRenderRequest{
		UID:    "ops",
		Width:  1200,
		Height: 800,
		Theme:  "light",
		From:   "s",
		To:     "e",
		TZ:     "UTC",
	}); err != nil {
		t.Fatalf("render branch failed: %v", err)
	}

	foundMetricsParams := false
	foundLogsNoLimit := false
	foundTracesNoLimit := false
	foundAnnotationsParams := false
	foundRenderParams := false
	for _, req := range requests {
		switch req.URL.Host {
		case "prom":
			if req.URL.Query().Get("start") == "s" && req.URL.Query().Get("end") == "e" && req.URL.Query().Get("step") == "1m" {
				foundMetricsParams = true
			}
		case "logs":
			if req.URL.Query().Get("limit") == "" {
				foundLogsNoLimit = true
			}
		case "traces":
			if req.URL.Query().Get("limit") == "" {
				foundTracesNoLimit = true
			}
		case "base":
			switch req.URL.Path {
			case "/api/annotations":
				if req.URL.Query().Get("dashboardUID") == "ops" && req.URL.Query().Get("panelId") == "7" && req.URL.Query().Get("type") == "annotation" {
					foundAnnotationsParams = len(req.URL.Query()["tags"]) == 2
				}
			case "/render/d/ops/render":
				if req.URL.Query().Get("width") == "1200" && req.URL.Query().Get("height") == "800" && req.URL.Query().Get("theme") == "light" && req.URL.Query().Get("from") == "s" && req.URL.Query().Get("to") == "e" && req.URL.Query().Get("tz") == "UTC" {
					foundRenderParams = true
				}
			}
		}
	}
	if !foundMetricsParams || !foundLogsNoLimit || !foundTracesNoLimit || !foundAnnotationsParams || !foundRenderParams {
		t.Fatalf("expected query param branches to execute")
	}
}

func TestClientDashboardFolderAnnotationAlertingMethods(t *testing.T) {
	hits := make(map[string]int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits[r.Method+" "+r.URL.Path]++
		if strings.HasPrefix(r.URL.Path, "/render/") {
			if !strings.Contains(r.Header.Get("Accept"), "image/png") {
				t.Fatalf("expected render accept header")
			}
			w.Header().Set("Content-Type", "image/png; charset=binary")
			_, _ = w.Write([]byte("png"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client := NewClient(config.Config{
		BaseURL: srv.URL,
	}, srv.Client())

	if _, err := client.GetDashboard(context.Background(), "ops"); err != nil {
		t.Fatalf("get dashboard failed: %v", err)
	}
	if _, err := client.DeleteDashboard(context.Background(), "ops"); err != nil {
		t.Fatalf("delete dashboard failed: %v", err)
	}
	if _, err := client.DashboardVersions(context.Background(), "ops", 5); err != nil {
		t.Fatalf("dashboard versions failed: %v", err)
	}
	rendered, err := client.RenderDashboard(context.Background(), DashboardRenderRequest{UID: "ops", PanelID: 12})
	if err != nil {
		t.Fatalf("render dashboard failed: %v", err)
	}
	if rendered.ContentType != "image/png" || string(rendered.Data) != "png" {
		t.Fatalf("unexpected render response: %+v", rendered)
	}
	if _, err := client.ListFolders(context.Background()); err != nil {
		t.Fatalf("list folders failed: %v", err)
	}
	if _, err := client.GetFolder(context.Background(), "ops"); err != nil {
		t.Fatalf("get folder failed: %v", err)
	}
	if _, err := client.ListAnnotations(context.Background(), AnnotationListRequest{DashboardUID: "ops"}); err != nil {
		t.Fatalf("list annotations failed: %v", err)
	}
	if _, err := client.AlertingRules(context.Background()); err != nil {
		t.Fatalf("alerting rules failed: %v", err)
	}
	if _, err := client.AlertingContactPoints(context.Background()); err != nil {
		t.Fatalf("alerting contact points failed: %v", err)
	}
	if _, err := client.AlertingPolicies(context.Background()); err != nil {
		t.Fatalf("alerting policies failed: %v", err)
	}

	if hits["GET /api/dashboards/uid/ops"] != 1 || hits["DELETE /api/dashboards/uid/ops"] != 1 {
		t.Fatalf("unexpected dashboard hits: %+v", hits)
	}
	if hits["GET /api/dashboards/uid/ops/versions"] != 1 {
		t.Fatalf("expected dashboard versions hit: %+v", hits)
	}
	if hits["GET /render/d-solo/ops/render"] != 1 {
		t.Fatalf("expected render hit: %+v", hits)
	}
	if hits["GET /api/folders"] != 1 || hits["GET /api/folders/ops"] != 1 {
		t.Fatalf("unexpected folder hits: %+v", hits)
	}
	if hits["GET /api/annotations"] != 1 {
		t.Fatalf("expected annotations hit: %+v", hits)
	}
	if hits["GET /api/v1/provisioning/alert-rules"] != 1 || hits["GET /api/v1/provisioning/contact-points"] != 1 || hits["GET /api/v1/provisioning/policies"] != 1 {
		t.Fatalf("unexpected alerting hits: %+v", hits)
	}
}

func TestAssistantMethodsBranches(t *testing.T) {
	requests := make([]*http.Request, 0)
	bodies := make([]string, 0)
	client := NewClient(config.Config{
		BaseURL: "https://base",
	}, doerFunc(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r)
		if r.Body == nil {
			bodies = append(bodies, "")
		} else {
			data, _ := io.ReadAll(r.Body)
			bodies = append(bodies, string(data))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{}`)),
			Header:     make(http.Header),
		}, nil
	}))

	if _, err := client.AssistantChat(context.Background(), "Investigate errors", ""); err != nil {
		t.Fatalf("assistant chat failed: %v", err)
	}
	if _, err := client.AssistantChat(context.Background(), "Continue", "chat-1"); err != nil {
		t.Fatalf("assistant chat continuation failed: %v", err)
	}
	if _, err := client.AssistantChatStatus(context.Background(), "chat/1"); err != nil {
		t.Fatalf("assistant status failed: %v", err)
	}
	if _, err := client.AssistantSkills(context.Background()); err != nil {
		t.Fatalf("assistant skills failed: %v", err)
	}

	if len(requests) != 4 {
		t.Fatalf("expected 4 requests, got %d", len(requests))
	}
	if !strings.Contains(bodies[0], "Investigate errors") || strings.Contains(bodies[0], "chatId") {
		t.Fatalf("expected first assistant chat body without chatId: %s", bodies[0])
	}
	if !strings.Contains(bodies[1], "Continue") || !strings.Contains(bodies[1], "chatId") {
		t.Fatalf("expected second assistant chat body with chatId: %s", bodies[1])
	}
	if !strings.Contains(requests[2].URL.Path, "/api/plugins/grafana-assistant-app/resources/api/v1/chats/chat") {
		t.Fatalf("unexpected assistant status path: %s", requests[2].URL.Path)
	}
	if requests[3].URL.Path != "/api/plugins/grafana-assistant-app/resources/api/v1/assistant/skills" {
		t.Fatalf("unexpected assistant skills path: %s", requests[3].URL.Path)
	}
}

func TestRequestJSONErrorPaths(t *testing.T) {
	client := NewClient(config.Config{BaseURL: "https://grafana.com"}, doerFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network")
	}))
	if _, err := client.requestJSON(context.Background(), http.MethodGet, "https://grafana.com", nil); err == nil {
		t.Fatalf("expected network error")
	}

	client = NewClient(config.Config{BaseURL: "https://grafana.com"}, doerFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(strings.NewReader("bad")),
			Header:     make(http.Header),
		}, nil
	}))
	if _, err := client.requestJSON(context.Background(), http.MethodGet, "https://grafana.com", nil); err == nil {
		t.Fatalf("expected HTTP error")
	}

	client = NewClient(config.Config{BaseURL: "https://grafana.com"}, doerFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	}))
	resp, err := client.requestJSON(context.Background(), http.MethodGet, "https://grafana.com", nil)
	if err != nil {
		t.Fatalf("unexpected error for empty body: %v", err)
	}
	if _, ok := resp.(map[string]any); !ok {
		t.Fatalf("expected empty object")
	}

	client = NewClient(config.Config{BaseURL: "https://grafana.com"}, doerFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("{")),
			Header:     make(http.Header),
		}, nil
	}))
	if _, err := client.requestJSON(context.Background(), http.MethodGet, "https://grafana.com", nil); err == nil {
		t.Fatalf("expected JSON unmarshal error")
	}

	if _, err := client.requestJSON(context.Background(), http.MethodGet, "https://grafana.com", map[string]any{"bad": func() {}}); err == nil {
		t.Fatalf("expected marshal error")
	}
	if _, err := client.requestJSON(context.Background(), "GET\n", "https://grafana.com", nil); err == nil {
		t.Fatalf("expected request construction error")
	}

	client = NewClient(config.Config{BaseURL: "https://grafana.com"}, doerFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       errReader{},
			Header:     make(http.Header),
		}, nil
	}))
	if _, err := client.requestJSON(context.Background(), http.MethodGet, "https://grafana.com", nil); err == nil {
		t.Fatalf("expected read error")
	}
}

func TestRequestJSONWithAuthBranches(t *testing.T) {
	call := 0
	client := NewClient(config.Config{BaseURL: "https://grafana.com"}, doerFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer explicit-token" {
			t.Fatalf("expected explicit auth token")
		}
		call++
		switch call {
		case 1:
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		default:
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("{")),
				Header:     make(http.Header),
			}, nil
		}
	}))

	resp, err := client.requestJSONWithAuth(context.Background(), http.MethodGet, "https://grafana.com", nil, "explicit-token", 0)
	if err != nil {
		t.Fatalf("expected empty-body success, got %v", err)
	}
	if _, ok := resp.(map[string]any); !ok {
		t.Fatalf("expected empty object response")
	}
	if _, err := client.requestJSONWithAuth(context.Background(), http.MethodGet, "https://grafana.com", nil, "explicit-token", 0); err == nil {
		t.Fatalf("expected invalid JSON error")
	}

	client = NewClient(config.Config{BaseURL: "https://grafana.com"}, doerFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network")
	}))
	if _, err := client.requestJSONWithAuth(context.Background(), http.MethodGet, "https://grafana.com", nil, "explicit-token", 0); err == nil {
		t.Fatalf("expected requestJSONWithAuth network error")
	}
}

func TestRequestBytesDefaultAcceptAndRenderSlugBranch(t *testing.T) {
	requests := make([]*http.Request, 0)
	client := NewClient(config.Config{BaseURL: "https://base"}, doerFunc(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r)
		if strings.Contains(r.URL.Path, "/render/") {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("png")),
				Header:     http.Header{"Content-Type": []string{"image/png"}},
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			Header:     make(http.Header),
		}, nil
	}))

	if _, _, err := client.requestBytes(context.Background(), http.MethodPost, "https://base/api/test", map[string]any{"ok": true}, ""); err != nil {
		t.Fatalf("requestBytes should succeed: %v", err)
	}
	rendered, err := client.RenderDashboard(context.Background(), DashboardRenderRequest{UID: "ops", Slug: "overview"})
	if err != nil {
		t.Fatalf("render dashboard should succeed: %v", err)
	}
	if rendered.ContentType != "image/png" || rendered.Endpoint != "https://base/render/d/ops/overview" {
		t.Fatalf("unexpected rendered dashboard: %+v", rendered)
	}

	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(requests))
	}
	if requests[0].Header.Get("Accept") != "*/*" {
		t.Fatalf("expected default accept header, got %q", requests[0].Header.Get("Accept"))
	}
	if requests[0].Header.Get("Content-Type") != "application/json" {
		t.Fatalf("expected JSON content type for body request")
	}
	if requests[1].URL.Path != "/render/d/ops/overview" {
		t.Fatalf("unexpected render path: %s", requests[1].URL.Path)
	}
}

func TestRenderDashboardErrorBranch(t *testing.T) {
	client := NewClient(config.Config{BaseURL: "https://base"}, doerFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader("boom")),
			Header:     make(http.Header),
		}, nil
	}))
	if _, err := client.RenderDashboard(context.Background(), DashboardRenderRequest{UID: "ops"}); err == nil {
		t.Fatalf("expected render dashboard error")
	}
}

func TestJoinURL(t *testing.T) {
	if _, err := joinURL("", "/x", nil); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected missing base error")
	}
	if _, err := joinURL("://bad", "/x", nil); err == nil {
		t.Fatalf("expected invalid base URL error")
	}

	joined, err := joinURL("https://grafana.com/root", "/api/test", url.Values{"a": {"1"}})
	if err != nil {
		t.Fatalf("unexpected join error: %v", err)
	}
	if !strings.Contains(joined, "/api/test") || !strings.Contains(joined, "a=1") {
		t.Fatalf("unexpected joined URL: %s", joined)
	}

	absolute, err := joinURL("https://grafana.com", "https://example.com/path", url.Values{"q": {"x"}})
	if err != nil {
		t.Fatalf("unexpected absolute join error: %v", err)
	}
	if absolute != "https://example.com/path?q=x" {
		t.Fatalf("unexpected absolute URL: %s", absolute)
	}

	if _, err := joinURL("https://grafana.com", "%", nil); err == nil {
		t.Fatalf("expected ref parse error")
	}
	if joined, err := joinURL("https://grafana.com", "/x", nil); err != nil || joined != "https://grafana.com/x" {
		t.Fatalf("expected join without query")
	}
	if normalizeExternalBaseURL("synthetic-monitoring-api-us-east-0.grafana.net") != "https://synthetic-monitoring-api-us-east-0.grafana.net" {
		t.Fatalf("expected normalizeExternalBaseURL to add https scheme")
	}
	if normalizeExternalBaseURL("https://synthetic-monitoring-api-us-east-0.grafana.net/") != "https://synthetic-monitoring-api-us-east-0.grafana.net" {
		t.Fatalf("expected normalizeExternalBaseURL to trim trailing slash")
	}
	if normalizeExternalBaseURL("") != "" {
		t.Fatalf("expected empty normalizeExternalBaseURL result")
	}
}

func TestAdditionalClientBranchCoverage(t *testing.T) {
	requests := make([]*http.Request, 0)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewClient(config.Config{BaseURL: srv.URL, CloudURL: srv.URL}, srv.Client())

	if _, err := client.ServiceAccounts(context.Background(), ServiceAccountListRequest{}); err != nil {
		t.Fatalf("expected minimal service accounts request to succeed: %v", err)
	}
	if _, err := client.CloudAccessPolicies(context.Background(), CloudAccessPolicyListRequest{
		Name:            "writers",
		RealmType:       "stack",
		RealmIdentifier: "123",
		PageSize:        10,
		PageCursor:      "cursor-1",
		Region:          "us",
		Status:          "active",
	}); err != nil {
		t.Fatalf("expected full cloud access policy request to succeed: %v", err)
	}
	if _, err := client.CloudAccessPolicy(context.Background(), "ap-2", ""); err != nil {
		t.Fatalf("expected cloud access policy request without region to succeed: %v", err)
	}
	backendHost := strings.TrimPrefix(srv.URL, "https://")
	if _, err := client.SyntheticChecks(context.Background(), SyntheticCheckListRequest{BackendURL: backendHost, Token: "sm-token"}); err != nil {
		t.Fatalf("expected synthetic checks host-only backend request to succeed: %v", err)
	}
	if _, err := client.SyntheticCheck(context.Background(), SyntheticCheckGetRequest{BackendURL: backendHost, Token: "sm-token", ID: 7}); err != nil {
		t.Fatalf("expected synthetic check host-only backend request to succeed: %v", err)
	}

	if len(requests) != 5 {
		t.Fatalf("expected 5 requests, got %d", len(requests))
	}
	if requests[0].URL.RawQuery != "" {
		t.Fatalf("expected minimal service account query to stay empty, got %s", requests[0].URL.RawQuery)
	}
	if requests[1].URL.Query().Get("name") != "writers" || requests[1].URL.Query().Get("realmIdentifier") != "123" || requests[1].URL.Query().Get("pageCursor") != "cursor-1" || requests[1].URL.Query().Get("status") != "active" {
		t.Fatalf("unexpected cloud access query: %s", requests[1].URL.RawQuery)
	}
	if requests[2].URL.RawQuery != "" {
		t.Fatalf("expected cloud access get without region to omit query, got %s", requests[2].URL.RawQuery)
	}
	if requests[3].URL.Query().Get("includeAlerts") != "" || requests[3].Header.Get("Authorization") != "Bearer sm-token" {
		t.Fatalf("unexpected synthetic checks request: query=%s auth=%s", requests[3].URL.RawQuery, requests[3].Header.Get("Authorization"))
	}
	if requests[4].URL.Path != "/api/v1/check/7" {
		t.Fatalf("unexpected synthetic check path: %s", requests[4].URL.Path)
	}

	noCall := doerFunc(func(*http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP call")
		return nil, nil
	})
	if _, err := NewClient(config.Config{CloudURL: "://bad"}, noCall).CloudAccessPolicies(context.Background(), CloudAccessPolicyListRequest{Region: "us"}); err == nil {
		t.Fatalf("expected invalid cloud URL error")
	}
	if _, err := NewClient(config.Config{CloudURL: "://bad"}, noCall).CloudAccessPolicy(context.Background(), "ap-1", "us"); err == nil {
		t.Fatalf("expected invalid cloud URL error for single access policy")
	}
	if _, err := NewClient(config.Config{BaseURL: "://bad"}, noCall).ServiceAccounts(context.Background(), ServiceAccountListRequest{}); err == nil {
		t.Fatalf("expected invalid base URL error for service accounts")
	}
	if _, err := NewClient(config.Config{BaseURL: "://bad"}, noCall).ServiceAccount(context.Background(), 1); err == nil {
		t.Fatalf("expected invalid base URL error for service account")
	}
	if _, err := NewClient(config.Config{}, noCall).SyntheticChecks(context.Background(), SyntheticCheckListRequest{BackendURL: "%", Token: "sm-token"}); err == nil {
		t.Fatalf("expected invalid synthetic backend URL error")
	}
	if _, err := NewClient(config.Config{}, noCall).SyntheticCheck(context.Background(), SyntheticCheckGetRequest{BackendURL: "%", Token: "sm-token", ID: 1}); err == nil {
		t.Fatalf("expected invalid synthetic backend URL error for get")
	}
}
