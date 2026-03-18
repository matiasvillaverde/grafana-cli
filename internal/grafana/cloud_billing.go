package grafana

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// CloudBilledUsageRequest defines the request for Grafana Cloud billed usage.
type CloudBilledUsageRequest struct {
	OrgSlug string `json:"org_slug"`
	Year    int    `json:"year"`
	Month   int    `json:"month"`
}

// CloudBilledUsage fetches billed usage for one organization and billing period.
func (c *Client) CloudBilledUsage(ctx context.Context, req CloudBilledUsageRequest) (any, error) {
	if strings.TrimSpace(c.cfg.CloudURL) == "" {
		return nil, ErrMissingBaseURL
	}
	q := url.Values{}
	if req.Month > 0 {
		q.Set("month", strconv.Itoa(req.Month))
	}
	if req.Year > 0 {
		q.Set("year", strconv.Itoa(req.Year))
	}
	u, err := joinURL(c.cfg.CloudURL, "/api/orgs/"+url.PathEscape(strings.TrimSpace(req.OrgSlug))+"/billed-usage", q)
	if err != nil {
		return nil, err
	}
	return c.requestJSON(ctx, http.MethodGet, u, nil)
}
