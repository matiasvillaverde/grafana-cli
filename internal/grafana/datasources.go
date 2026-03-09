package grafana

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// DatasourceQueryRequest defines a Grafana datasource query request sent via /api/ds/query.
type DatasourceQueryRequest struct {
	From    string           `json:"from"`
	To      string           `json:"to"`
	Queries []map[string]any `json:"queries"`
}

func (c *Client) GetDatasource(ctx context.Context, uid string) (any, error) {
	if strings.TrimSpace(c.cfg.BaseURL) == "" {
		return nil, ErrMissingBaseURL
	}
	u, err := joinURL(c.cfg.BaseURL, "/api/datasources/uid/"+url.PathEscape(uid), nil)
	if err != nil {
		return nil, err
	}
	return c.requestJSON(ctx, http.MethodGet, u, nil)
}

func (c *Client) DatasourceHealth(ctx context.Context, uid string) (any, error) {
	if strings.TrimSpace(c.cfg.BaseURL) == "" {
		return nil, ErrMissingBaseURL
	}
	u, err := joinURL(c.cfg.BaseURL, "/api/datasources/uid/"+url.PathEscape(uid)+"/health", nil)
	if err != nil {
		return nil, err
	}
	return c.requestJSON(ctx, http.MethodGet, u, nil)
}

func (c *Client) DatasourceResource(ctx context.Context, method, uid, resourcePath string, body any) (any, error) {
	if strings.TrimSpace(c.cfg.BaseURL) == "" {
		return nil, ErrMissingBaseURL
	}
	u, err := joinURL(c.cfg.BaseURL, datasourceResourceAPIPath(uid, resourcePath), nil)
	if err != nil {
		return nil, err
	}
	return c.requestJSON(ctx, method, u, body)
}

func (c *Client) DatasourceQuery(ctx context.Context, req DatasourceQueryRequest) (any, error) {
	if strings.TrimSpace(c.cfg.BaseURL) == "" {
		return nil, ErrMissingBaseURL
	}
	u, err := joinURL(c.cfg.BaseURL, "/api/ds/query", nil)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"queries": req.Queries,
	}
	if strings.TrimSpace(req.From) != "" {
		payload["from"] = req.From
	}
	if strings.TrimSpace(req.To) != "" {
		payload["to"] = req.To
	}
	return c.requestJSON(ctx, http.MethodPost, u, payload)
}

func datasourceResourceAPIPath(uid, resourcePath string) string {
	trimmed := strings.TrimSpace(resourcePath)
	query := ""
	if index := strings.Index(trimmed, "?"); index >= 0 {
		query = trimmed[index:]
		trimmed = trimmed[:index]
	}
	segments := strings.Split(strings.Trim(trimmed, "/"), "/")
	encoded := make([]string, 0, len(segments))
	for _, segment := range segments {
		if strings.TrimSpace(segment) == "" {
			continue
		}
		encoded = append(encoded, url.PathEscape(segment))
	}
	path := "/api/datasources/uid/" + url.PathEscape(uid) + "/resources"
	if len(encoded) > 0 {
		path += "/" + strings.Join(encoded, "/")
	}
	return path + query
}
