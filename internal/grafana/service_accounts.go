package grafana

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ServiceAccountListRequest defines filters for service-account search.
type ServiceAccountListRequest struct {
	Query string `json:"query"`
	Page  int    `json:"page"`
	Limit int    `json:"limit"`
}

// ServiceAccounts searches service accounts with paging.
func (c *Client) ServiceAccounts(ctx context.Context, req ServiceAccountListRequest) (any, error) {
	if strings.TrimSpace(c.cfg.BaseURL) == "" {
		return nil, ErrMissingBaseURL
	}
	q := url.Values{}
	if strings.TrimSpace(req.Query) != "" {
		q.Set("query", req.Query)
	}
	if req.Page > 0 {
		q.Set("page", strconv.Itoa(req.Page))
	}
	if req.Limit > 0 {
		q.Set("perpage", strconv.Itoa(req.Limit))
	}
	u, err := joinURL(c.cfg.BaseURL, "/api/serviceaccounts/search", q)
	if err != nil {
		return nil, err
	}
	return c.requestJSON(ctx, http.MethodGet, u, nil)
}

// ServiceAccount fetches one service account by numeric ID.
func (c *Client) ServiceAccount(ctx context.Context, id int64) (any, error) {
	if strings.TrimSpace(c.cfg.BaseURL) == "" {
		return nil, ErrMissingBaseURL
	}
	u, err := joinURL(c.cfg.BaseURL, "/api/serviceaccounts/"+strconv.FormatInt(id, 10), nil)
	if err != nil {
		return nil, err
	}
	return c.requestJSON(ctx, http.MethodGet, u, nil)
}
