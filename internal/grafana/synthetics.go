package grafana

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// SyntheticCheckListRequest defines the request for listing synthetic monitoring checks.
type SyntheticCheckListRequest struct {
	BackendURL    string `json:"backend_url"`
	Token         string `json:"token"`
	IncludeAlerts bool   `json:"include_alerts"`
}

// SyntheticCheckGetRequest defines the request for one synthetic monitoring check.
type SyntheticCheckGetRequest struct {
	BackendURL string `json:"backend_url"`
	Token      string `json:"token"`
	ID         int64  `json:"id"`
}

// SyntheticChecks lists checks from the Synthetic Monitoring API.
func (c *Client) SyntheticChecks(ctx context.Context, req SyntheticCheckListRequest) (any, error) {
	baseURL := normalizeExternalBaseURL(req.BackendURL)
	if baseURL == "" {
		return nil, ErrMissingBaseURL
	}
	q := url.Values{}
	if req.IncludeAlerts {
		q.Set("includeAlerts", "true")
	}
	u, err := joinURL(baseURL, "/api/v1/check", q)
	if err != nil {
		return nil, err
	}
	return c.requestJSONWithAuth(ctx, http.MethodGet, u, nil, req.Token, 0)
}

// SyntheticCheck fetches one synthetic monitoring check by ID.
func (c *Client) SyntheticCheck(ctx context.Context, req SyntheticCheckGetRequest) (any, error) {
	baseURL := normalizeExternalBaseURL(req.BackendURL)
	if baseURL == "" {
		return nil, ErrMissingBaseURL
	}
	u, err := joinURL(baseURL, "/api/v1/check/"+strconv.FormatInt(req.ID, 10), nil)
	if err != nil {
		return nil, err
	}
	return c.requestJSONWithAuth(ctx, http.MethodGet, u, nil, req.Token, 0)
}

func normalizeExternalBaseURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}
	return strings.TrimRight(trimmed, "/")
}
