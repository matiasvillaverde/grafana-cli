package grafana

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// CloudStackPluginListRequest defines list filters for stack plugins.
type CloudStackPluginListRequest struct {
	Stack      string `json:"stack"`
	PageSize   int    `json:"page_size"`
	PageCursor string `json:"page_cursor"`
}

func (c *Client) CloudStackDatasources(ctx context.Context, stack string) (any, error) {
	if strings.TrimSpace(c.cfg.CloudURL) == "" {
		return nil, ErrMissingBaseURL
	}
	u, err := joinURL(c.cfg.CloudURL, "/api/instances/"+url.PathEscape(strings.TrimSpace(stack))+"/datasources", nil)
	if err != nil {
		return nil, err
	}
	return c.requestJSON(ctx, http.MethodGet, u, nil)
}

func (c *Client) CloudStackConnections(ctx context.Context, stack string) (any, error) {
	if strings.TrimSpace(c.cfg.CloudURL) == "" {
		return nil, ErrMissingBaseURL
	}
	u, err := joinURL(c.cfg.CloudURL, "/api/instances/"+url.PathEscape(strings.TrimSpace(stack))+"/connections", nil)
	if err != nil {
		return nil, err
	}
	return c.requestJSON(ctx, http.MethodGet, u, nil)
}

func (c *Client) CloudStackPlugins(ctx context.Context, stack string) (any, error) {
	return c.CloudStackPluginsPage(ctx, CloudStackPluginListRequest{Stack: stack})
}

func (c *Client) CloudStackPluginsPage(ctx context.Context, req CloudStackPluginListRequest) (any, error) {
	if strings.TrimSpace(c.cfg.CloudURL) == "" {
		return nil, ErrMissingBaseURL
	}
	q := url.Values{}
	if req.PageSize > 0 {
		q.Set("pageSize", strconv.Itoa(req.PageSize))
	}
	if strings.TrimSpace(req.PageCursor) != "" {
		q.Set("pageCursor", req.PageCursor)
	}
	u, err := joinURL(c.cfg.CloudURL, "/api/instances/"+url.PathEscape(strings.TrimSpace(req.Stack))+"/plugins", q)
	if err != nil {
		return nil, err
	}
	return c.requestJSON(ctx, http.MethodGet, u, nil)
}

func (c *Client) CloudStackPlugin(ctx context.Context, stack, plugin string) (any, error) {
	if strings.TrimSpace(c.cfg.CloudURL) == "" {
		return nil, ErrMissingBaseURL
	}
	u, err := joinURL(c.cfg.CloudURL, "/api/instances/"+url.PathEscape(strings.TrimSpace(stack))+"/plugins/"+url.PathEscape(strings.TrimSpace(plugin)), nil)
	if err != nil {
		return nil, err
	}
	return c.requestJSON(ctx, http.MethodGet, u, nil)
}
