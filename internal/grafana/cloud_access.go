package grafana

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// CloudAccessPolicyListRequest defines list filters for Grafana Cloud access policies.
type CloudAccessPolicyListRequest struct {
	Name            string `json:"name"`
	RealmType       string `json:"realm_type"`
	RealmIdentifier string `json:"realm_identifier"`
	PageSize        int    `json:"page_size"`
	PageCursor      string `json:"page_cursor"`
	Region          string `json:"region"`
	Status          string `json:"status"`
}

// CloudAccessPolicies lists Grafana Cloud access policies for a region.
func (c *Client) CloudAccessPolicies(ctx context.Context, req CloudAccessPolicyListRequest) (any, error) {
	if strings.TrimSpace(c.cfg.CloudURL) == "" {
		return nil, ErrMissingBaseURL
	}
	q := url.Values{}
	if strings.TrimSpace(req.Name) != "" {
		q.Set("name", req.Name)
	}
	if strings.TrimSpace(req.RealmType) != "" {
		q.Set("realmType", req.RealmType)
	}
	if strings.TrimSpace(req.RealmIdentifier) != "" {
		q.Set("realmIdentifier", req.RealmIdentifier)
	}
	if req.PageSize > 0 {
		q.Set("pageSize", strconv.Itoa(req.PageSize))
	}
	if strings.TrimSpace(req.PageCursor) != "" {
		q.Set("pageCursor", req.PageCursor)
	}
	if strings.TrimSpace(req.Region) != "" {
		q.Set("region", req.Region)
	}
	if strings.TrimSpace(req.Status) != "" {
		q.Set("status", req.Status)
	}
	u, err := joinURL(c.cfg.CloudURL, "/api/v1/accesspolicies", q)
	if err != nil {
		return nil, err
	}
	return c.requestJSON(ctx, http.MethodGet, u, nil)
}

// CloudAccessPolicy fetches one Grafana Cloud access policy by UUID and region.
func (c *Client) CloudAccessPolicy(ctx context.Context, id, region string) (any, error) {
	if strings.TrimSpace(c.cfg.CloudURL) == "" {
		return nil, ErrMissingBaseURL
	}
	q := url.Values{}
	if strings.TrimSpace(region) != "" {
		q.Set("region", region)
	}
	u, err := joinURL(c.cfg.CloudURL, "/api/v1/accesspolicies/"+url.PathEscape(id), q)
	if err != nil {
		return nil, err
	}
	return c.requestJSON(ctx, http.MethodGet, u, nil)
}
