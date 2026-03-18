package cli

import (
	"context"
	"net/url"
	"strings"
)

type cloudPageFetcher func(ctx context.Context, pageSize int, pageCursor string) (any, error)

type cloudListOptions struct {
	Limit         int
	PageSize      int
	Include       func(map[string]any) bool
	NonCollection func(any) (any, int, bool, error)
}

func (a *App) listCloudCollection(ctx context.Context, fetch cloudPageFetcher, opts cloudListOptions) (any, int, bool, error) {
	collected := make([]any, 0, opts.Limit)
	pageCursor := ""

	for {
		page, err := fetch(ctx, cloudCollectionPageSize(opts.Limit, len(collected), opts.PageSize), pageCursor)
		if err != nil {
			return nil, 0, false, err
		}

		items, _, ok := collectionPayload(page)
		if !ok {
			if opts.NonCollection != nil {
				return opts.NonCollection(page)
			}
			return page, inferCollectionCount(page), payloadHasNextPage(page), nil
		}

		pageCursor = cloudNextPageCursor(page)
		for _, item := range items {
			record, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if opts.Include != nil && !opts.Include(record) {
				continue
			}
			if opts.Limit > 0 && len(collected) >= opts.Limit {
				return map[string]any{"items": collected}, len(collected), true, nil
			}
			collected = append(collected, record)
		}
		if opts.Limit > 0 && len(collected) >= opts.Limit && pageCursor != "" {
			return map[string]any{"items": collected}, len(collected), true, nil
		}
		if pageCursor == "" {
			return map[string]any{"items": collected}, len(collected), false, nil
		}
	}
}

func cloudCollectionPageSize(limit, count, requested int) int {
	pageSize := requested
	if pageSize <= 0 {
		pageSize = 100
	}
	if limit <= 0 {
		return pageSize
	}
	remaining := limit - count
	if remaining <= 0 {
		return 1
	}
	if remaining < pageSize {
		return remaining
	}
	return pageSize
}

func cloudNextPageCursor(payload any) string {
	root, ok := payload.(map[string]any)
	if !ok {
		return ""
	}
	nextValue := strings.TrimSpace(firstNonEmptyString(root, "next", "nextPage"))
	if nextValue == "" {
		nextValue = strings.TrimSpace(firstNonEmptyString(mapValue(mapValue(root, "metadata"), "pagination"), "next", "nextPage"))
	}
	if nextValue == "" {
		return ""
	}
	return cloudPageCursorValue(nextValue)
}

func cloudPageCursorValue(nextValue string) string {
	parsed, err := url.Parse(nextValue)
	if err == nil {
		if cursor := strings.TrimSpace(parsed.Query().Get("pageCursor")); cursor != "" {
			return cursor
		}
		if cursor := strings.TrimSpace(parsed.Query().Get("cursor")); cursor != "" {
			return cursor
		}
	}
	return nextValue
}
