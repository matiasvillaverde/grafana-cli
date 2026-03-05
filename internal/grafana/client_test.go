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
	if _, err := client.SearchDashboards(context.Background(), "errors", "prod", 10); err != nil {
		t.Fatalf("search dashboards failed: %v", err)
	}
	if _, err := client.CreateDashboard(context.Background(), map[string]any{"title": "x"}, 7, true); err != nil {
		t.Fatalf("create dashboard failed: %v", err)
	}
	if _, err := client.ListDatasources(context.Background()); err != nil {
		t.Fatalf("list datasources failed: %v", err)
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

	if hits["/api/v1/stacks"] != 1 || hits["/api/search"] != 2 || hits["/api/dashboards/db"] != 1 || hits["/api/datasources"] != 1 {
		t.Fatalf("unexpected hit counts: %+v", hits)
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
	if _, err := client.SearchDashboards(context.Background(), "", "", 0); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for search dashboards, got %v", err)
	}
	if _, err := client.CreateDashboard(context.Background(), map[string]any{"title": "x"}, 0, false); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for create dashboard, got %v", err)
	}
	if _, err := client.ListDatasources(context.Background()); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for list datasources, got %v", err)
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

	foundMetricsParams := false
	foundLogsNoLimit := false
	foundTracesNoLimit := false
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
		}
	}
	if !foundMetricsParams || !foundLogsNoLimit || !foundTracesNoLimit {
		t.Fatalf("expected query param branches to execute")
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
}
