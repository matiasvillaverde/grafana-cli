package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/matiasvillaverde/grafana-cli/internal/config"
	"github.com/matiasvillaverde/grafana-cli/internal/grafana"
)

func (a *App) runCloudStacks(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"cloud", "stacks"}, true)
	}

	cfg, err := a.requireAuthConfig()
	if err != nil {
		return err
	}
	client := a.NewClient(cfg)

	switch args[0] {
	case "list":
		return a.runCloudStacksList(ctx, opts, client, args[1:])
	case "inspect":
		return a.runCloudStacksInspect(ctx, opts, client, args[1:])
	case "plugins":
		return a.runCloudStackPlugins(ctx, opts, client, args[1:])
	default:
		return fmt.Errorf("unknown cloud stacks command: %s", args[0])
	}
}

func (a *App) runCloudStacksList(ctx context.Context, opts globalOptions, client APIClient, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: cloud stacks list")
	}
	result, err := client.CloudStacks(ctx)
	if err != nil {
		return err
	}
	return a.emitWithMetadata(opts, result, collectionMetadata("cloud stacks list", result, 0, ""))
}

func (a *App) runCloudStacksInspect(ctx context.Context, opts globalOptions, client APIClient, args []string) error {
	fs, target := newCloudStackFlagSet("cloud stacks inspect")
	includeRaw := fs.Bool("include-raw", false, "include raw datasource and connection payloads")
	if err := fs.Parse(args); err != nil {
		return err
	}
	stackTarget, err := target.required()
	if err != nil {
		return err
	}

	stacks, err := client.CloudStacks(ctx)
	if err != nil {
		return err
	}
	stackRecord, ok := cloudStackBySlug(stacks, stackTarget.Slug)
	if !ok {
		return fmt.Errorf("stack not found in cloud API response: %s", stackTarget.Slug)
	}

	var (
		datasources   any
		datasourceErr error
		connections   any
		connectionErr error
		wg            sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		datasources, datasourceErr = client.CloudStackDatasources(ctx, stackTarget.Slug)
	}()
	go func() {
		defer wg.Done()
		connections, connectionErr = client.CloudStackConnections(ctx, stackTarget.Slug)
	}()
	wg.Wait()

	payload := buildCloudStackInspectPayload(stackRecord, stackTarget.BaseURL, datasources, connections, *includeRaw)
	meta := &responseMetadata{
		Command:    "cloud stacks inspect",
		NextAction: "Use `grafana auth login --token \"$GRAFANA_TOKEN\" --stack " + stackTarget.Slug + "` to persist these endpoints locally",
	}
	warnings := make([]string, 0, 2)
	if datasourceErr != nil {
		warnings = append(warnings, "cloud stack datasource discovery failed: "+datasourceErr.Error())
	}
	if connectionErr != nil {
		warnings = append(warnings, "cloud stack connection discovery failed: "+connectionErr.Error())
	}
	if len(warnings) > 0 && !opts.Agent {
		return fmt.Errorf("cloud stacks inspect incomplete: %s", strings.Join(warnings, "; "))
	}
	if len(warnings) > 0 {
		meta.Warnings = warnings
	}
	return a.emitWithMetadata(opts, payload, meta)
}

func (a *App) runCloudStackPlugins(ctx context.Context, opts globalOptions, client APIClient, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"cloud", "stacks", "plugins"}, true)
	}

	switch args[0] {
	case "list":
		return a.runCloudStackPluginsList(ctx, opts, client, args[1:])
	case "get":
		return a.runCloudStackPluginsGet(ctx, opts, client, args[1:])
	default:
		return errors.New("usage: cloud stacks plugins <list|get> [flags]")
	}
}

func (a *App) runCloudStackPluginsList(ctx context.Context, opts globalOptions, client APIClient, args []string) error {
	fs, target := newCloudStackFlagSet("cloud stacks plugins list")
	query := fs.String("query", "", "plugin ID or name filter")
	limit := fs.Int("limit", 100, "maximum plugins")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *limit < 1 {
		return errors.New("--limit must be at least 1")
	}
	stackTarget, err := target.required()
	if err != nil {
		return err
	}
	result, count, truncated, err := a.listCloudStackPlugins(ctx, client, stackTarget.Slug, strings.TrimSpace(*query), *limit)
	if err != nil {
		return err
	}
	meta := &responseMetadata{Command: "cloud stacks plugins list"}
	meta.Count = &count
	if truncated {
		meta.Truncated = true
		meta.NextAction = "Raise --limit or refine --query to inspect more stack plugins"
	}
	return a.emitWithMetadata(opts, result, meta)
}

func (a *App) runCloudStackPluginsGet(ctx context.Context, opts globalOptions, client APIClient, args []string) error {
	fs, target := newCloudStackFlagSet("cloud stacks plugins get")
	plugin := fs.String("plugin", "", "plugin ID")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*plugin) == "" {
		return errors.New("--plugin is required")
	}
	stackTarget, err := target.required()
	if err != nil {
		return err
	}
	result, err := client.CloudStackPlugin(ctx, stackTarget.Slug, strings.TrimSpace(*plugin))
	if err != nil {
		return err
	}
	return a.emit(opts, result)
}

func (a *App) listCloudStackPlugins(ctx context.Context, client APIClient, stackSlug, query string, limit int) (any, int, bool, error) {
	return a.listCloudCollection(ctx, func(ctx context.Context, pageSize int, pageCursor string) (any, error) {
		return client.CloudStackPluginsPage(ctx, grafana.CloudStackPluginListRequest{
			Stack:      stackSlug,
			PageSize:   pageSize,
			PageCursor: pageCursor,
		})
	}, cloudListOptions{
		Limit: limit,
		Include: func(record map[string]any) bool {
			return query == "" || matchesAnyField(record, query, "id", "name", "slug", "version")
		},
		NonCollection: func(page any) (any, int, bool, error) {
			filtered, count, truncated := filterNamedPayload(page, query, limit, "id", "name", "slug", "version")
			return filtered, count, truncated || payloadHasNextPage(page), nil
		},
	})
}

func (a *App) applyInferredStackEndpoints(ctx context.Context, cfg *config.Config, stackSlug, cloudToken string) []string {
	token := strings.TrimSpace(cloudToken)
	if token == "" {
		token = cfg.Token
	}
	cloudClient := a.NewClient(config.Config{
		BaseURL:  cfg.CloudURL,
		CloudURL: cfg.CloudURL,
		Token:    token,
	})
	warnings := make([]string, 0, 2)

	datasources, err := cloudClient.Raw(ctx, "GET", "/api/instances/"+url.PathEscape(stackSlug)+"/datasources", nil)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("stack datasource discovery failed: %v", err))
	} else {
		endpoints := inferDatasourceEndpoints(datasources)
		if endpoints.PrometheusURL != "" {
			cfg.PrometheusURL = endpoints.PrometheusURL
		}
		if endpoints.LogsURL != "" {
			cfg.LogsURL = endpoints.LogsURL
		}
		if endpoints.TracesURL != "" {
			cfg.TracesURL = endpoints.TracesURL
		}
	}

	connections, err := cloudClient.Raw(ctx, "GET", "/api/instances/"+url.PathEscape(stackSlug)+"/connections", nil)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("stack connection discovery failed: %v", err))
	} else if onCallURL := inferOnCallURL(connections); onCallURL != "" {
		cfg.OnCallURL = onCallURL
	}
	return warnings
}

func normalizeStackIdentifier(value string) (string, string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", "", errors.New("stack identifier is required")
	}
	if strings.Contains(trimmed, "://") {
		parsed, err := url.Parse(trimmed)
		if err != nil {
			return "", "", fmt.Errorf("invalid --stack value: %w", err)
		}
		host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
		if !strings.HasSuffix(host, ".grafana.net") {
			return "", "", errors.New("--stack URL must target a *.grafana.net host")
		}
		slug := strings.TrimSuffix(host, ".grafana.net")
		return slug, parsed.Scheme + "://" + parsed.Host, nil
	}
	host := strings.ToLower(trimmed)
	if strings.Contains(host, ".") {
		if !strings.HasSuffix(host, ".grafana.net") {
			return "", "", errors.New("--stack must be a Grafana Cloud slug or *.grafana.net host")
		}
		slug := strings.TrimSuffix(host, ".grafana.net")
		return slug, "https://" + host, nil
	}
	return host, "https://" + host + ".grafana.net", nil
}

func inferDatasourceEndpoints(payload any) inferredStackEndpoints {
	items, _, ok := collectionPayload(payload)
	if !ok {
		return inferredStackEndpoints{}
	}
	out := inferredStackEndpoints{}
	for _, item := range items {
		record, ok := item.(map[string]any)
		if !ok {
			continue
		}
		dataType := strings.ToLower(firstNonEmptyString(record, "type", "slug", "name"))
		endpoint := firstNonEmptyString(record, "url", "endpoint", "proxyUrl", "proxy_url")
		switch {
		case endpoint == "":
			continue
		case out.PrometheusURL == "" && containsAny(dataType, "prometheus", "mimir"):
			out.PrometheusURL = endpoint
		case out.LogsURL == "" && containsAny(dataType, "loki", "logs"):
			out.LogsURL = endpoint
		case out.TracesURL == "" && containsAny(dataType, "tempo", "traces"):
			out.TracesURL = endpoint
		}
	}
	return out
}

func cloudStackBySlug(payload any, stackSlug string) (map[string]any, bool) {
	items, _, ok := collectionPayload(payload)
	if !ok {
		return nil, false
	}
	for _, item := range items {
		record, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if strings.EqualFold(firstNonEmptyString(record, "slug", "stackSlug", "name"), stackSlug) {
			return cloneAnyMap(record), true
		}
	}
	return nil, false
}

func buildCloudStackInspectPayload(stackRecord map[string]any, baseURL string, datasources, connections any, includeRaw bool) map[string]any {
	endpoints := inferDatasourceEndpoints(datasources)
	payload := map[string]any{
		"stack":                cloneAnyMap(stackRecord),
		"datasource_summary":   datasourceInventorySummary(cloudStackDatasourceItems(datasources)),
		"connectivity_summary": cloudStackConnectivitySummary(connections),
		"inferred_endpoints": map[string]any{
			"base_url":       baseURL,
			"prometheus_url": endpoints.PrometheusURL,
			"logs_url":       endpoints.LogsURL,
			"traces_url":     endpoints.TracesURL,
			"oncall_url":     inferOnCallURL(connections),
		},
	}
	if includeRaw {
		payload["datasources"] = normalizeDatasourceCollection(cloudStackDatasourceItems(datasources))
		payload["connections"] = connections
	}
	return payload
}

func cloudStackDatasourceItems(payload any) []any {
	items, _, ok := collectionPayload(payload)
	if !ok {
		return nil
	}
	return items
}

func cloudStackConnectivitySummary(payload any) map[string]any {
	summary := map[string]any{}

	connectionTypes := map[string]struct{}{}
	for _, item := range cloudStackConnectionItems(payload) {
		record, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if connectionType := strings.ToLower(firstNonEmptyString(record, "type", "kind", "name")); connectionType != "" {
			connectionTypes[connectionType] = struct{}{}
		}
	}
	if len(connectionTypes) > 0 {
		summary["connection_types"] = sortedSet(connectionTypes)
	}

	privateTenantTypes := map[string]struct{}{}
	for _, item := range collectionAtPath(payload, "privateConnectivityInfo", "tenants") {
		record, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if tenantType := strings.ToLower(firstNonEmptyString(record, "type")); tenantType != "" {
			privateTenantTypes[tenantType] = struct{}{}
		}
	}
	summary["has_private_connectivity"] = len(privateTenantTypes) > 0
	if len(privateTenantTypes) > 0 {
		summary["private_tenant_types"] = sortedSet(privateTenantTypes)
	}

	return summary
}

func cloudStackConnectionItems(payload any) []any {
	if items, _, ok := collectionPayload(payload); ok {
		return items
	}
	return collectionAtPath(payload, "connections")
}
