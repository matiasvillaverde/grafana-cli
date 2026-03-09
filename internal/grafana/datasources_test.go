package grafana

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matiasvillaverde/grafana-cli/internal/config"
)

func TestDatasourceClientMethods(t *testing.T) {
	hits := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits[r.URL.RequestURI()]++
		switch r.URL.Path {
		case "/api/datasources/uid/mysql-uid":
			if r.Method != http.MethodGet {
				t.Fatalf("expected GET for datasource get, got %s", r.Method)
			}
		case "/api/datasources/uid/mysql-uid/health":
			if r.Method != http.MethodGet {
				t.Fatalf("expected GET for datasource health, got %s", r.Method)
			}
		case "/api/datasources/uid/mysql-uid/resources/schemas/public":
			if r.Method != http.MethodGet || r.URL.Query().Get("limit") != "10" {
				t.Fatalf("unexpected datasource resource get request: %s?%s", r.Method, r.URL.RawQuery)
			}
		case "/api/datasources/uid/mysql-uid/resources/validate":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST for datasource resource, got %s", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "SELECT 1") {
				t.Fatalf("expected datasource resource JSON body, got %s", string(body))
			}
		case "/api/ds/query":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST for datasource query, got %s", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), `"from":"now-1h"`) || !strings.Contains(string(body), `"queries"`) {
				t.Fatalf("unexpected datasource query body: %s", string(body))
			}
		default:
			t.Fatalf("unexpected datasource request path: %s", r.URL.RequestURI())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client := NewClient(config.Config{BaseURL: srv.URL}, srv.Client())
	if _, err := client.GetDatasource(context.Background(), "mysql-uid"); err != nil {
		t.Fatalf("get datasource failed: %v", err)
	}
	if _, err := client.DatasourceHealth(context.Background(), "mysql-uid"); err != nil {
		t.Fatalf("datasource health failed: %v", err)
	}
	if _, err := client.DatasourceResource(context.Background(), http.MethodGet, "mysql-uid", "schemas/public?limit=10", nil); err != nil {
		t.Fatalf("datasource resource get failed: %v", err)
	}
	if _, err := client.DatasourceResource(context.Background(), http.MethodPost, "mysql-uid", "/validate", map[string]any{"sql": "SELECT 1"}); err != nil {
		t.Fatalf("datasource resource post failed: %v", err)
	}
	if _, err := client.DatasourceQuery(context.Background(), DatasourceQueryRequest{
		From: "now-1h",
		To:   "now",
		Queries: []map[string]any{
			{"refId": "A", "datasource": map[string]any{"uid": "mysql-uid"}, "rawSql": "SELECT 1"},
		},
	}); err != nil {
		t.Fatalf("datasource query failed: %v", err)
	}

	if hits["/api/datasources/uid/mysql-uid"] != 1 || hits["/api/datasources/uid/mysql-uid/health"] != 1 {
		t.Fatalf("expected datasource get and health hits: %+v", hits)
	}
	if hits["/api/datasources/uid/mysql-uid/resources/schemas/public?limit=10"] != 1 || hits["/api/datasources/uid/mysql-uid/resources/validate"] != 1 {
		t.Fatalf("expected datasource resource hits: %+v", hits)
	}
	if hits["/api/ds/query"] != 1 {
		t.Fatalf("expected datasource query hit: %+v", hits)
	}
}

func TestDatasourceClientMissingBaseURLAndOptionalFields(t *testing.T) {
	client := &Client{cfg: config.Config{}, doer: doerFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, errors.New("should not call")
	})}

	if _, err := client.GetDatasource(context.Background(), "x"); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for get datasource, got %v", err)
	}
	if _, err := client.DatasourceHealth(context.Background(), "x"); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for datasource health, got %v", err)
	}
	if _, err := client.DatasourceResource(context.Background(), http.MethodGet, "x", "schemas", nil); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for datasource resource, got %v", err)
	}
	if _, err := client.DatasourceQuery(context.Background(), DatasourceQueryRequest{}); !errors.Is(err, ErrMissingBaseURL) {
		t.Fatalf("expected ErrMissingBaseURL for datasource query, got %v", err)
	}

	payloads := []string{}
	client = NewClient(config.Config{BaseURL: "https://grafana.example.com"}, doerFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		payloads = append(payloads, string(body))
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			Header:     make(http.Header),
		}, nil
	}))
	if _, err := client.DatasourceQuery(context.Background(), DatasourceQueryRequest{
		Queries: []map[string]any{{"refId": "A"}},
	}); err != nil {
		t.Fatalf("unexpected datasource query error: %v", err)
	}
	if len(payloads) != 1 || strings.Contains(payloads[0], `"from"`) || strings.Contains(payloads[0], `"to"`) {
		t.Fatalf("expected datasource query to omit empty from/to, got %+v", payloads)
	}

	invalid := &Client{cfg: config.Config{BaseURL: "://bad"}, doer: doerFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, errors.New("should not call")
	})}
	if _, err := invalid.GetDatasource(context.Background(), "x"); err == nil {
		t.Fatalf("expected invalid base URL error for get datasource")
	}
	if _, err := invalid.DatasourceHealth(context.Background(), "x"); err == nil {
		t.Fatalf("expected invalid base URL error for datasource health")
	}
	if _, err := invalid.DatasourceResource(context.Background(), http.MethodGet, "x", "schemas", nil); err == nil {
		t.Fatalf("expected invalid base URL error for datasource resource")
	}
	if _, err := invalid.DatasourceQuery(context.Background(), DatasourceQueryRequest{Queries: []map[string]any{{"refId": "A"}}}); err == nil {
		t.Fatalf("expected invalid base URL error for datasource query")
	}
}

func TestDatasourceResourceAPIPath(t *testing.T) {
	if got := datasourceResourceAPIPath("uid", ""); got != "/api/datasources/uid/uid/resources" {
		t.Fatalf("unexpected empty datasource resource path: %s", got)
	}
	if got := datasourceResourceAPIPath("uid/value", "/schemas/public tables?limit=10"); got != "/api/datasources/uid/uid%2Fvalue/resources/schemas/public%20tables?limit=10" {
		t.Fatalf("unexpected datasource resource path escaping: %s", got)
	}
}
