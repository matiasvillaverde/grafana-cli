package grafana

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/matiasvillaverde/grafana-cli/internal/config"
)

var (
	ErrMissingBaseURL       = errors.New("missing base URL")
	ErrMissingPrometheusURL = errors.New("missing prometheus URL")
	ErrMissingLogsURL       = errors.New("missing logs URL")
	ErrMissingTracesURL     = errors.New("missing traces URL")
)

// HTTPDoer abstracts HTTP execution for testing.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// HTTPError is returned for non-2xx responses.
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("grafana API request failed: status=%d body=%s", e.StatusCode, e.Body)
}

// AggregateRequest defines cross-signal aggregation query arguments.
type AggregateRequest struct {
	MetricExpr string `json:"metric_expr"`
	LogQuery   string `json:"log_query"`
	TraceQuery string `json:"trace_query"`
	Start      string `json:"start"`
	End        string `json:"end"`
	Step       string `json:"step"`
	Limit      int    `json:"limit"`
}

// AggregateSnapshot groups multi-signal runtime payloads.
type AggregateSnapshot struct {
	Metrics any `json:"metrics"`
	Logs    any `json:"logs"`
	Traces  any `json:"traces"`
}

// Client provides typed access to Grafana API domains.
type Client struct {
	cfg       config.Config
	doer      HTTPDoer
	userAgent string
}

func NewClient(cfg config.Config, doer HTTPDoer) *Client {
	cfg.ApplyDefaults()
	if doer == nil {
		doer = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{cfg: cfg, doer: doer, userAgent: "grafana-cli/0.1.0"}
}

func (c *Client) Raw(ctx context.Context, method, path string, body any) (any, error) {
	if strings.TrimSpace(c.cfg.BaseURL) == "" {
		return nil, ErrMissingBaseURL
	}
	u, err := joinURL(c.cfg.BaseURL, path, nil)
	if err != nil {
		return nil, err
	}
	return c.requestJSON(ctx, method, u, body)
}

func (c *Client) CloudStacks(ctx context.Context) (any, error) {
	if strings.TrimSpace(c.cfg.CloudURL) == "" {
		return nil, ErrMissingBaseURL
	}
	u, err := joinURL(c.cfg.CloudURL, "/api/v1/stacks", nil)
	if err != nil {
		return nil, err
	}
	return c.requestJSON(ctx, http.MethodGet, u, nil)
}

func (c *Client) SearchDashboards(ctx context.Context, query, tag string, limit int) (any, error) {
	if strings.TrimSpace(c.cfg.BaseURL) == "" {
		return nil, ErrMissingBaseURL
	}
	q := url.Values{}
	q.Set("type", "dash-db")
	if strings.TrimSpace(query) != "" {
		q.Set("query", query)
	}
	if strings.TrimSpace(tag) != "" {
		q.Set("tag", tag)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	u, err := joinURL(c.cfg.BaseURL, "/api/search", q)
	if err != nil {
		return nil, err
	}
	return c.requestJSON(ctx, http.MethodGet, u, nil)
}

func (c *Client) CreateDashboard(ctx context.Context, dashboard map[string]any, folderID int64, overwrite bool) (any, error) {
	if strings.TrimSpace(c.cfg.BaseURL) == "" {
		return nil, ErrMissingBaseURL
	}
	u, err := joinURL(c.cfg.BaseURL, "/api/dashboards/db", nil)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"dashboard": dashboard,
		"overwrite": overwrite,
	}
	if folderID > 0 {
		payload["folderId"] = folderID
	}
	return c.requestJSON(ctx, http.MethodPost, u, payload)
}

func (c *Client) ListDatasources(ctx context.Context) (any, error) {
	if strings.TrimSpace(c.cfg.BaseURL) == "" {
		return nil, ErrMissingBaseURL
	}
	u, err := joinURL(c.cfg.BaseURL, "/api/datasources", nil)
	if err != nil {
		return nil, err
	}
	return c.requestJSON(ctx, http.MethodGet, u, nil)
}

func (c *Client) MetricsRange(ctx context.Context, expr, start, end, step string) (any, error) {
	if strings.TrimSpace(c.cfg.PrometheusURL) == "" {
		return nil, ErrMissingPrometheusURL
	}
	q := url.Values{}
	q.Set("query", expr)
	if strings.TrimSpace(start) != "" {
		q.Set("start", start)
	}
	if strings.TrimSpace(end) != "" {
		q.Set("end", end)
	}
	if strings.TrimSpace(step) != "" {
		q.Set("step", step)
	}
	u, err := joinURL(c.cfg.PrometheusURL, "/api/prom/api/v1/query_range", q)
	if err != nil {
		return nil, err
	}
	return c.requestJSON(ctx, http.MethodGet, u, nil)
}

func (c *Client) LogsRange(ctx context.Context, query, start, end string, limit int) (any, error) {
	if strings.TrimSpace(c.cfg.LogsURL) == "" {
		return nil, ErrMissingLogsURL
	}
	q := url.Values{}
	q.Set("query", query)
	if strings.TrimSpace(start) != "" {
		q.Set("start", start)
	}
	if strings.TrimSpace(end) != "" {
		q.Set("end", end)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	u, err := joinURL(c.cfg.LogsURL, "/loki/api/v1/query_range", q)
	if err != nil {
		return nil, err
	}
	return c.requestJSON(ctx, http.MethodGet, u, nil)
}

func (c *Client) TracesSearch(ctx context.Context, query, start, end string, limit int) (any, error) {
	if strings.TrimSpace(c.cfg.TracesURL) == "" {
		return nil, ErrMissingTracesURL
	}
	q := url.Values{}
	q.Set("q", query)
	if strings.TrimSpace(start) != "" {
		q.Set("start", start)
	}
	if strings.TrimSpace(end) != "" {
		q.Set("end", end)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	u, err := joinURL(c.cfg.TracesURL, "/api/search", q)
	if err != nil {
		return nil, err
	}
	return c.requestJSON(ctx, http.MethodGet, u, nil)
}

func (c *Client) AggregateSnapshot(ctx context.Context, req AggregateRequest) (AggregateSnapshot, error) {
	metrics, err := c.MetricsRange(ctx, req.MetricExpr, req.Start, req.End, req.Step)
	if err != nil {
		return AggregateSnapshot{}, err
	}
	logs, err := c.LogsRange(ctx, req.LogQuery, req.Start, req.End, req.Limit)
	if err != nil {
		return AggregateSnapshot{}, err
	}
	traces, err := c.TracesSearch(ctx, req.TraceQuery, req.Start, req.End, req.Limit)
	if err != nil {
		return AggregateSnapshot{}, err
	}
	return AggregateSnapshot{Metrics: metrics, Logs: logs, Traces: traces}, nil
}

func (c *Client) requestJSON(ctx context.Context, method, endpoint string, body any) (any, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(c.cfg.Token) != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}
	if c.cfg.OrgID > 0 {
		req.Header.Set("X-Grafana-Org-Id", strconv.FormatInt(c.cfg.OrgID, 10))
	}

	resp, err := c.doer.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(data))}
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}

	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func joinURL(base, path string, query url.Values) (string, error) {
	if strings.TrimSpace(base) == "" {
		return "", ErrMissingBaseURL
	}
	if absolute, err := url.Parse(path); err == nil && absolute.Scheme != "" && absolute.Host != "" {
		q := absolute.Query()
		for key, values := range query {
			for _, value := range values {
				q.Add(key, value)
			}
		}
		absolute.RawQuery = q.Encode()
		return absolute.String(), nil
	}

	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	final := baseURL.ResolveReference(ref)
	q := final.Query()
	for key, values := range query {
		for _, value := range values {
			q.Add(key, value)
		}
	}
	final.RawQuery = q.Encode()
	return final.String(), nil
}
