package cli

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"strings"
	"testing"

	"github.com/matiasvillaverde/grafana-cli/internal/config"
	"github.com/matiasvillaverde/grafana-cli/internal/grafana"
)

func TestRunDatasourcesCommandsAndHelp(t *testing.T) {
	store := &fakeStore{cfg: config.Config{Token: "token", BaseURL: "https://grafana.example.com"}}
	client := &fakeClient{
		listDSResult: []any{
			map[string]any{"uid": "mysql-uid", "name": "mysql", "type": "mysql"},
			map[string]any{"uid": "cloudwatch-uid", "name": "cloudwatch", "type": "cloudwatch"},
		},
		getDSResult:      map[string]any{"uid": "mysql-uid", "name": "mysql", "type": "mysql"},
		dsHealthResult:   map[string]any{"status": "OK"},
		dsResourceResult: map[string]any{"items": []any{"public"}},
		dsQueryResult:    map[string]any{"results": map[string]any{"A": map[string]any{"frames": []any{map[string]any{"schema": "x"}}}}},
	}
	app, out, errOut := newTestApp(store, client)

	for _, args := range [][]string{
		{"datasources", "--help"},
		{"datasources", "resources", "--help"},
		{"datasources", "mysql", "query", "--help"},
	} {
		out.Reset()
		errOut.Reset()
		if code := app.Run(context.Background(), args); code != 0 {
			t.Fatalf("expected help to succeed for %v: %s", args, errOut.String())
		}
	}

	out.Reset()
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"--agent", "datasources", "list", "--type", "mysql"}); code != 0 {
		t.Fatalf("datasources list failed: %s", errOut.String())
	}
	listPayload := decodeJSON(t, out.String())
	if listPayload["metadata"].(map[string]any)["command"] != "datasources list" {
		t.Fatalf("expected datasource list metadata, got %+v", listPayload)
	}

	out.Reset()
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"datasources", "get", "--name", "mysql"}); code != 0 {
		t.Fatalf("datasources get failed: %s", errOut.String())
	}
	if client.getDSUID != "mysql-uid" {
		t.Fatalf("expected resolved datasource uid, got %q", client.getDSUID)
	}

	out.Reset()
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"datasources", "health", "--uid", "mysql-uid"}); code != 0 {
		t.Fatalf("datasources health failed: %s", errOut.String())
	}
	if client.dsHealthUID != "mysql-uid" {
		t.Fatalf("expected datasource health uid capture, got %q", client.dsHealthUID)
	}

	out.Reset()
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"datasources", "resources", "get", "--name", "mysql", "--path", "schemas/public?limit=10"}); code != 0 {
		t.Fatalf("datasources resources get failed: %s", errOut.String())
	}
	if client.dsResourceMethod != "GET" || client.dsResourceUID != "mysql-uid" || client.dsResourcePath != "schemas/public?limit=10" {
		t.Fatalf("unexpected datasource resource capture: %+v", client)
	}

	out.Reset()
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"datasources", "mysql", "query", "--name", "mysql", "--sql", "SELECT 1", "--format", "table"}); code != 0 {
		t.Fatalf("datasources mysql query failed: %s", errOut.String())
	}
	if client.dsQueryReq.Queries[0]["rawSql"] != "SELECT 1" || client.dsQueryReq.Queries[0]["datasource"].(map[string]any)["uid"] != "mysql-uid" {
		t.Fatalf("unexpected datasource SQL query capture: %+v", client.dsQueryReq)
	}

	out.Reset()
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"datasources", "cloudwatch", "query", "--name", "cloudwatch", "--namespace", "AWS/EC2", "--metric-name", "CPUUtilization", "--region", "us-east-1", "--statistic", "Average", "--dimensions", "InstanceId=i-123"}); code != 0 {
		t.Fatalf("datasources cloudwatch query failed: %s", errOut.String())
	}
	query := client.dsQueryReq.Queries[0]
	if query["namespace"] != "AWS/EC2" || query["metricName"] != "CPUUtilization" {
		t.Fatalf("unexpected datasource cloudwatch query capture: %+v", query)
	}

	out.Reset()
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"--agent", "datasources", "query", "--uid", "mysql-uid", "--query-json", `{"rawSql":"SELECT 1"}`}); code != 0 {
		t.Fatalf("datasources generic query failed: %s", errOut.String())
	}
	meta := decodeJSON(t, out.String())["metadata"].(map[string]any)
	if meta["command"] != "datasources query" {
		t.Fatalf("expected datasource query metadata, got %+v", meta)
	}

	errOut.Reset()
	if code := app.Run(context.Background(), []string{"datasources", "mysql"}); code != 1 {
		t.Fatalf("expected family query usage error")
	}
	if !strings.Contains(errOut.String(), "usage: datasources mysql query") {
		t.Fatalf("unexpected family usage error: %s", errOut.String())
	}

	errOut.Reset()
	if code := app.Run(context.Background(), []string{"datasources", "missing"}); code != 1 {
		t.Fatalf("expected datasource usage error")
	}
	if !strings.Contains(errOut.String(), "usage: datasources") {
		t.Fatalf("unexpected datasource usage error: %s", errOut.String())
	}
}

func TestDatasourceGetHealthResourcesErrors(t *testing.T) {
	authStore := &fakeStore{cfg: config.Config{Token: "token", BaseURL: "https://grafana.example.com"}}
	noAuthStore := &fakeStore{}
	client := &fakeClient{
		getDSErr:      staticError("get failed"),
		dsHealthErr:   staticError("health failed"),
		dsResourceErr: staticError("resource failed"),
		listDSResult:  []any{map[string]any{"uid": "mysql-uid", "name": "mysql", "type": "mysql"}},
	}
	app, _, _ := newTestApp(authStore, client)
	noAuthApp, _, _ := newTestApp(noAuthStore, client)

	if err := app.runDatasourceGet(context.Background(), globalOptions{}, "datasources get", []string{}); err == nil || err.Error() != "--uid or --name is required" {
		t.Fatalf("expected datasource selector error, got %v", err)
	}
	if err := app.runDatasourceGet(context.Background(), globalOptions{}, "datasources get", []string{"--uid", "a", "--name", "b"}); err == nil || err.Error() != "use either --uid or --name, not both" {
		t.Fatalf("expected selector exclusivity error, got %v", err)
	}
	if err := app.runDatasourceGet(context.Background(), globalOptions{}, "datasources get", []string{"--bad"}); err == nil {
		t.Fatalf("expected datasource get parse error")
	}
	if err := noAuthApp.runDatasourceGet(context.Background(), globalOptions{}, "datasources get", []string{"--uid", "x"}); err == nil {
		t.Fatalf("expected datasource get auth error")
	}
	if err := app.runDatasourceGet(context.Background(), globalOptions{}, "datasources get", []string{"--name", "mysql"}); err == nil || err.Error() != "get failed" {
		t.Fatalf("expected datasource get client error, got %v", err)
	}
	client.getDSErr = nil
	client.listDSResult = []any{map[string]any{"uid": "cloudwatch-uid", "name": "cloudwatch", "type": "cloudwatch"}}
	if err := app.runDatasourceGet(context.Background(), globalOptions{}, "datasources get", []string{"--name", "missing"}); err == nil || err.Error() != `no datasource matched --name "missing"` {
		t.Fatalf("expected datasource get resolver error, got %v", err)
	}

	client.listDSResult = []any{map[string]any{"uid": "mysql-uid", "name": "mysql", "type": "mysql"}}
	if err := app.runDatasourceHealth(context.Background(), globalOptions{}, "datasources health", []string{"--bad"}); err == nil {
		t.Fatalf("expected datasource health parse error")
	}
	if err := noAuthApp.runDatasourceHealth(context.Background(), globalOptions{}, "datasources health", []string{"--uid", "x"}); err == nil {
		t.Fatalf("expected datasource health auth error")
	}
	if err := app.runDatasourceHealth(context.Background(), globalOptions{}, "datasources health", []string{"--name", "mysql"}); err == nil || err.Error() != "health failed" {
		t.Fatalf("expected datasource health client error, got %v", err)
	}
	client.dsHealthErr = nil
	client.listDSResult = []any{
		map[string]any{"uid": "mysql-uid", "name": "mysql", "type": "mysql"},
		map[string]any{"uid": "mysql-shadow", "name": "mysql", "type": "mysql"},
	}
	if err := app.runDatasourceHealth(context.Background(), globalOptions{}, "datasources health", []string{"--name", "mysql"}); err == nil || !strings.Contains(err.Error(), `ambiguous datasource selection for --name "mysql"`) {
		t.Fatalf("expected datasource health resolver ambiguity, got %v", err)
	}

	if err := app.runDatasourceResources(context.Background(), globalOptions{}, nil); err != nil {
		t.Fatalf("expected datasource resources help to succeed, got %v", err)
	}
	if err := app.runDatasourceResources(context.Background(), globalOptions{}, []string{"bad"}); err == nil {
		t.Fatalf("expected datasource resources usage error")
	}
	if err := app.runDatasourceResources(context.Background(), globalOptions{}, []string{"get", "--bad"}); err == nil {
		t.Fatalf("expected datasource resources parse error")
	}
	if err := app.runDatasourceResources(context.Background(), globalOptions{}, []string{"get", "--uid", "a", "--name", "b", "--path", "schemas"}); err == nil || err.Error() != "use either --uid or --name, not both" {
		t.Fatalf("expected datasource resources selector validation error, got %v", err)
	}
	if err := app.runDatasourceResources(context.Background(), globalOptions{}, []string{"get", "--name", "mysql"}); err == nil || err.Error() != "--path is required" {
		t.Fatalf("expected datasource resources path error, got %v", err)
	}
	if err := app.runDatasourceResources(context.Background(), globalOptions{}, []string{"post", "--name", "mysql", "--path", "validate", "--body", "{"}); err == nil || !strings.Contains(err.Error(), "invalid --body JSON") {
		t.Fatalf("expected datasource resources body error, got %v", err)
	}
	if err := noAuthApp.runDatasourceResources(context.Background(), globalOptions{}, []string{"get", "--name", "mysql", "--path", "schemas"}); err == nil {
		t.Fatalf("expected datasource resources auth error")
	}
	client.listDSResult = []any{
		map[string]any{"uid": "mysql-uid", "name": "mysql", "type": "mysql"},
		map[string]any{"uid": "mysql-shadow", "name": "mysql", "type": "mysql"},
	}
	if err := app.runDatasourceResources(context.Background(), globalOptions{}, []string{"get", "--name", "mysql", "--path", "schemas"}); err == nil || !strings.Contains(err.Error(), `ambiguous datasource selection for --name "mysql"`) {
		t.Fatalf("expected datasource resources resolver error, got %v", err)
	}
	client.listDSResult = []any{map[string]any{"uid": "mysql-uid", "name": "mysql", "type": "mysql"}}
	if err := app.runDatasourceResources(context.Background(), globalOptions{}, []string{"get", "--name", "mysql", "--path", "schemas"}); err == nil || err.Error() != "resource failed" {
		t.Fatalf("expected datasource resources client error, got %v", err)
	}
	client.dsResourceErr = nil
	client.getDSResult = map[string]any{"uid": "mysql-uid", "name": "mysql", "type": "mysql"}
	client.listDSResult = []any{map[string]any{"uid": "mysql-uid", "name": "mysql", "type": "mysql"}}
	if err := app.runDatasourceResources(context.Background(), globalOptions{}, []string{"post", "--uid", "mysql-uid", "--path", "validate", "--body", `{"rawSql":"SELECT 1"}`}); err != nil {
		t.Fatalf("expected datasource resources post success, got %v", err)
	}
	if client.dsResourceMethod != "POST" || client.dsResourceUID != "mysql-uid" {
		t.Fatalf("unexpected datasource resource post capture: %+v", client)
	}
}

func TestDatasourceQueryExecutorErrors(t *testing.T) {
	authStore := &fakeStore{cfg: config.Config{Token: "token", BaseURL: "https://grafana.example.com"}}
	noAuthStore := &fakeStore{}
	client := &fakeClient{
		dsQueryErr:  staticError("query failed"),
		getDSResult: map[string]any{"uid": "mysql-uid", "name": "mysql", "type": "mysql"},
		listDSResult: []any{
			map[string]any{"uid": "mysql-uid", "name": "mysql", "type": "mysql"},
			map[string]any{"uid": "cloudwatch-uid", "name": "cloudwatch", "type": "cloudwatch"},
		},
		dsQueryResult: map[string]any{"results": map[string]any{}},
	}
	app, _, _ := newTestApp(authStore, client)
	noAuthApp, _, _ := newTestApp(noAuthStore, client)

	executor := app.datasourceExecutor()
	if err := executor.runQuery(context.Background(), globalOptions{}, "datasources query", genericDatasourceStrategy(), []string{}); err == nil || err.Error() != "--uid or --name is required" {
		t.Fatalf("expected datasource query selector error, got %v", err)
	}
	if err := executor.runQuery(context.Background(), globalOptions{}, "datasources query", genericDatasourceStrategy(), []string{"--bad"}); err == nil {
		t.Fatalf("expected datasource query parse error")
	}
	if err := executor.runQuery(context.Background(), globalOptions{}, "datasources query", genericDatasourceStrategy(), []string{"--uid", "mysql-uid"}); err == nil || err.Error() != "--query-json or --queries-json is required" {
		t.Fatalf("expected datasource query payload error, got %v", err)
	}
	if err := noAuthApp.datasourceExecutor().runQuery(context.Background(), globalOptions{}, "datasources query", genericDatasourceStrategy(), []string{"--uid", "mysql-uid", "--query-json", `{}`}); err == nil {
		t.Fatalf("expected datasource query auth error")
	}
	if err := executor.runQuery(context.Background(), globalOptions{}, "datasources query", genericDatasourceStrategy(), []string{"--uid", "mysql-uid", "--query-json", `{}`}); err == nil || err.Error() != "query failed" {
		t.Fatalf("expected datasource query client error, got %v", err)
	}
	client.dsQueryErr = nil
	if err := executor.runQuery(context.Background(), globalOptions{}, "datasources query", genericDatasourceStrategy(), []string{"--name", "missing", "--query-json", `{}`}); err == nil || err.Error() != `no datasource matched --name "missing" for family generic` {
		t.Fatalf("expected datasource query resolver error, got %v", err)
	}
	if err := executor.runQuery(context.Background(), globalOptions{}, "datasources query", genericDatasourceStrategy(), []string{"--uid", "mysql-uid", "--queries-json", `[{"rawSql":"SELECT 1"},{"rawSql":"SELECT 2"}]`}); err != nil {
		t.Fatalf("expected datasource query success with queries-json, got %v", err)
	}
	if len(client.dsQueryReq.Queries) != 2 || client.dsQueryReq.Queries[1]["refId"] != "B" {
		t.Fatalf("unexpected datasource query request capture: %+v", client.dsQueryReq)
	}

	cloudwatch, _ := findDatasourceStrategy("cloudwatch")
	if err := executor.runQuery(context.Background(), globalOptions{}, "datasources cloudwatch query", cloudwatch, []string{"--name", "cloudwatch", "--namespace", "AWS/EC2"}); err == nil || err.Error() != "--namespace, --metric-name, and --region are required for typed cloudwatch queries" {
		t.Fatalf("expected typed cloudwatch validation error, got %v", err)
	}
}

func TestDatasourceSelectionAndResolutionHelpers(t *testing.T) {
	if _, err := parseDatasourceSelection("datasources get", []string{"--uid", "a", "--name", "b"}); err == nil {
		t.Fatalf("expected selector conflict")
	}
	if _, err := parseDatasourceSelection("datasources get", []string{"--bad"}); err == nil {
		t.Fatalf("expected selector parse error")
	}
	selector, err := parseDatasourceSelection("datasources get", []string{"--name", "mysql", "--datasource-type", "mysql"})
	if err != nil || selector.Name != "mysql" || selector.DatasourceType != "mysql" {
		t.Fatalf("unexpected selector parse result: %+v err=%v", selector, err)
	}

	resolver := listDatasourceResolver{}
	client := &fakeClient{
		getDSResult: map[string]any{"uid": "mysql-uid", "name": "mysql", "type": "mysql"},
		listDSResult: []any{
			map[string]any{"uid": "mysql-uid", "name": "mysql", "type": "mysql"},
			map[string]any{"uid": "mysql-shadow", "name": "mysql", "type": "mysql"},
			map[string]any{"uid": "cloudwatch-uid", "name": "cloudwatch", "type": "cloudwatch"},
		},
	}
	mysqlStrategy, _ := findDatasourceStrategy("mysql")
	resolved, err := resolver.Resolve(context.Background(), client, datasourceSelector{UID: "mysql-uid"}, mysqlStrategy)
	if err != nil || resolved.UID != "mysql-uid" {
		t.Fatalf("unexpected resolver uid result: %+v err=%v", resolved, err)
	}
	if _, err := resolver.Resolve(context.Background(), client, datasourceSelector{UID: "mysql-uid", DatasourceType: "cloudwatch"}, nil); err == nil {
		t.Fatalf("expected datasource type mismatch")
	}
	cloudwatchStrategy, _ := findDatasourceStrategy("cloudwatch")
	if _, err := resolver.Resolve(context.Background(), client, datasourceSelector{UID: "mysql-uid"}, cloudwatchStrategy); err == nil {
		t.Fatalf("expected strategy type mismatch")
	}
	client.getDSResult = map[string]any{"uid": "broken"}
	if _, err := resolver.Resolve(context.Background(), client, datasourceSelector{UID: "broken"}, nil); err == nil {
		t.Fatalf("expected broken datasource payload error")
	}

	client.getDSResult = map[string]any{"uid": "mysql-uid", "name": "mysql", "type": "mysql"}
	if _, err := resolver.Resolve(context.Background(), client, datasourceSelector{Name: "missing"}, nil); err == nil {
		t.Fatalf("expected missing datasource resolution error")
	}
	if _, err := resolver.Resolve(context.Background(), &fakeClient{getDSErr: staticError("get datasource failed")}, datasourceSelector{UID: "mysql-uid"}, nil); err == nil || err.Error() != "get datasource failed" {
		t.Fatalf("expected datasource get resolver error, got %v", err)
	}
	if _, err := resolver.Resolve(context.Background(), &fakeClient{listDSErr: staticError("list failed")}, datasourceSelector{Name: "mysql"}, nil); err == nil || err.Error() != "list failed" {
		t.Fatalf("expected datasource list resolver error, got %v", err)
	}
	if _, err := resolver.Resolve(context.Background(), client, datasourceSelector{}, nil); err == nil || err.Error() != "--uid or --name is required" {
		t.Fatalf("expected datasource selector validation error, got %v", err)
	}
	if _, err := resolver.Resolve(context.Background(), client, datasourceSelector{Name: "mysql"}, nil); err == nil {
		t.Fatalf("expected ambiguous datasource resolution error")
	}
	if _, err := resolver.Resolve(context.Background(), client, datasourceSelector{Name: "mysql", DatasourceType: "cloudwatch"}, nil); err == nil || err.Error() != `no datasource matched --name "mysql" and --datasource-type "cloudwatch"` {
		t.Fatalf("expected datasource name/type mismatch error, got %v", err)
	}
	resolved, err = resolver.Resolve(context.Background(), client, datasourceSelector{Name: "cloudwatch"}, cloudwatchStrategy)
	if err != nil || resolved.UID != "cloudwatch-uid" {
		t.Fatalf("unexpected resolver name result: %+v err=%v", resolved, err)
	}
}

func TestDatasourceResolverHelpers(t *testing.T) {
	if err := validateDatasourceSelector(datasourceSelector{}); err == nil {
		t.Fatalf("expected missing selector error")
	}
	if err := validateDatasourceSelector(datasourceSelector{UID: "a", Name: "b"}); err == nil {
		t.Fatalf("expected conflicting selector error")
	}
	if resolved, ok := datasourceFromPayload("bad"); ok || resolved.UID != "" {
		t.Fatalf("expected datasourceFromPayload to reject non-object payload")
	}
	if resolved, ok := datasourceFromPayload(map[string]any{"uid": "x"}); ok || resolved.UID != "" {
		t.Fatalf("expected datasourceFromPayload to reject missing type")
	}
	resolved, ok := datasourceFromPayload(map[string]any{"uid": "x", "name": "mysql", "type": "mysql"})
	if !ok || resolved.Name != "mysql" || resolved.Type != "mysql" {
		t.Fatalf("unexpected datasourceFromPayload result: %+v ok=%v", resolved, ok)
	}

	candidates := datasourceCandidates([]any{
		map[string]any{"uid": "x", "name": "mysql", "type": "mysql"},
		map[string]any{"bad": "entry"},
	})
	if len(candidates) != 1 {
		t.Fatalf("unexpected datasource candidates: %+v", candidates)
	}
	if datasourceCandidates("scalar") != nil {
		t.Fatalf("expected datasourceCandidates to reject scalar payload")
	}

	filtered := filterResolvedDatasources([]resolvedDatasource{
		{UID: "x", Name: "mysql", Type: "mysql"},
		{UID: "y", Name: "mysql", Type: "cloudwatch"},
	}, datasourceSelector{Name: "mysql", DatasourceType: "mysql"}, nil)
	if len(filtered) != 1 || filtered[0].UID != "x" {
		t.Fatalf("unexpected filtered datasources: %+v", filtered)
	}
	cloudwatchStrategy, _ := findDatasourceStrategy("cloudwatch")
	filtered = filterResolvedDatasources([]resolvedDatasource{
		{UID: "x", Name: "mysql", Type: "mysql"},
		{UID: "y", Name: "cloudwatch", Type: "cloudwatch"},
	}, datasourceSelector{}, cloudwatchStrategy)
	if len(filtered) != 1 || filtered[0].UID != "y" {
		t.Fatalf("unexpected strategy filtered datasources: %+v", filtered)
	}
	if !strings.Contains(formatDatasourceCandidates([]resolvedDatasource{{UID: "b", Name: "mysql", Type: "mysql"}, {UID: "a", Name: "cloudwatch", Type: "cloudwatch"}}), "a(cloudwatch,cloudwatch)") {
		t.Fatalf("expected formatted datasource candidates")
	}
}

func TestDatasourceInventoryHelpers(t *testing.T) {
	if family, ok := datasourceFamilyForType("prometheus"); !ok || family.Name != "prometheus" {
		t.Fatalf("expected datasource family lookup by type")
	}
	if _, ok := datasourceFamilyForType("custom-plugin"); ok {
		t.Fatalf("expected unknown datasource family type")
	}

	normalized := normalizeDatasourceRecord(map[string]any{"uid": "prom-uid", "name": "prometheus", "type": "prometheus", "url": "https://prom", "access": "proxy", "isDefault": true})
	if normalized["typed_family"] != "prometheus" || normalized["documentation_url"] == "" {
		t.Fatalf("unexpected normalized datasource record: %+v", normalized)
	}
	if normalized["raw"].(map[string]any)["name"] != "prometheus" {
		t.Fatalf("expected raw datasource payload preservation: %+v", normalized)
	}
	if normalizeDatasourceRecord(map[string]any{"uid": "broken"})["raw"] == nil {
		t.Fatalf("expected broken datasource normalization to preserve raw payload")
	}

	collection := normalizeDatasourceCollection([]any{
		map[string]any{"uid": "prom-uid", "name": "prometheus", "type": "prometheus"},
		map[string]any{"uid": "custom-uid", "name": "custom", "type": "custom-plugin"},
		"bad",
	})
	if len(collection) != 2 {
		t.Fatalf("unexpected normalized datasource collection: %+v", collection)
	}
	if normalizeDatasourceCollection("bad") != nil {
		t.Fatalf("expected invalid datasource collection payload to normalize to nil")
	}
	if flags := typedFlagNames(fakeDatasourceStrategy{flags: []discoveryFlag{{Name: ""}, {Name: "--sql"}}}); len(flags) != 1 || flags[0] != "--sql" {
		t.Fatalf("unexpected typed flag names: %+v", flags)
	}

	summary := datasourceInventorySummary([]any{
		map[string]any{"uid": "prom-uid", "name": "prometheus", "type": "prometheus"},
		map[string]any{"uid": "loki-uid", "name": "loki", "type": "loki"},
	})
	if summary["count"] != 2 || len(summary["typed_families"].([]string)) != 2 {
		t.Fatalf("unexpected datasource inventory summary: %+v", summary)
	}

	req := grafana.AggregateRequest{MetricExpr: "up", LogQuery: `{job="api"}`, TraceQuery: `{ status = error }`, Limit: 20}
	hints := datasourceQueryHints(req, "incident", []any{
		map[string]any{"uid": "prom-uid", "name": "prometheus", "type": "prometheus"},
		map[string]any{"uid": "loki-uid", "name": "loki", "type": "loki"},
		map[string]any{"uid": "tempo-uid", "name": "tempo", "type": "tempo"},
	})
	if len(hints) != 3 || !strings.Contains(hints[0]["command"].(string), "datasources prometheus query") {
		t.Fatalf("unexpected datasource query hints: %+v", hints)
	}
	if len(datasourceQueryHints(req, "incident", []any{map[string]any{"uid": "custom-uid", "name": "custom", "type": "custom-plugin"}})) != 0 {
		t.Fatalf("expected no datasource query hints for unsupported families")
	}
}

func TestDatasourceStrategyHelpers(t *testing.T) {
	mysqlStrategy, ok := findDatasourceStrategy("mysql")
	if !ok || !mysqlStrategy.SupportsType("mysql") || mysqlStrategy.SupportsType("cloudwatch") {
		t.Fatalf("unexpected mysql strategy support")
	}
	if _, ok := findDatasourceStrategy("missing"); ok {
		t.Fatalf("expected missing datasource strategy")
	}
	if family, ok := findDatasourceQueryFamily("cloudwatch"); !ok || family.Name != "cloudwatch" {
		t.Fatalf("expected cloudwatch family lookup")
	} else if family.DocumentationURL == "" || len(family.Notes) == 0 {
		t.Fatalf("expected cloudwatch family documentation metadata")
	}
	if docs := datasourceQuerySyntaxDocs(); docs["datasource_query"] == "" || docs["mysql"] == "" {
		t.Fatalf("expected datasource syntax docs")
	}
	if !datasourceTypeMatches("grafana-clickhouse-datasource", []string{"clickhouse"}) {
		t.Fatalf("expected datasource type alias match")
	}
	if !datasourceTypeMatches("", nil) {
		t.Fatalf("expected empty datasource type to match empty accepted set")
	}
	if datasourceTypeMatches("", []string{"cloudwatch"}) {
		t.Fatalf("expected empty datasource type mismatch")
	}
	if datasourceTypeMatches("mysql", []string{"cloudwatch"}) {
		t.Fatalf("expected datasource type mismatch")
	}
	if normalizeDefault("", "table") != "table" || normalizeDefault("time_series", "table") != "time_series" {
		t.Fatalf("unexpected normalizeDefault result")
	}
	if _, err := parseCloudWatchDimensions("bad"); err == nil {
		t.Fatalf("expected cloudwatch dimensions validation error")
	}
	dimensions, err := parseCloudWatchDimensions("InstanceId=i-123,AutoScalingGroup=asg-1")
	if err != nil || len(dimensions) != 2 {
		t.Fatalf("unexpected cloudwatch dimensions parse result: %+v err=%v", dimensions, err)
	}

	fs := flag.NewFlagSet("sql", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var sqlOpts datasourceQueryOptions
	mysqlStrategy.BindFlags(fs, &sqlOpts)
	if err := fs.Parse([]string{"--sql", "SELECT 1", "--format", "table"}); err != nil {
		t.Fatalf("unexpected sql strategy flag parse error: %v", err)
	}
	queries, err := mysqlStrategy.BuildQueries(sqlOpts, resolvedDatasource{UID: "mysql-uid", Type: "mysql"})
	if err != nil || queries[0]["rawSql"] != "SELECT 1" {
		t.Fatalf("unexpected sql strategy query build: %+v err=%v", queries, err)
	}

	prometheusStrategy, _ := findDatasourceStrategy("prometheus")
	fs = flag.NewFlagSet("prom", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var exprOpts datasourceQueryOptions
	prometheusStrategy.BindFlags(fs, &exprOpts)
	if err := fs.Parse([]string{"--expr", "up", "--instant", "--legend-format", "{{instance}}", "--min-step", "1m", "--format", "table"}); err != nil {
		t.Fatalf("unexpected prometheus strategy parse error: %v", err)
	}
	queries, err = prometheusStrategy.BuildQueries(exprOpts, resolvedDatasource{UID: "prom-uid", Type: "prometheus"})
	if err != nil || queries[0]["expr"] != "up" || queries[0]["instant"] != true || queries[0]["legendFormat"] != "{{instance}}" || queries[0]["interval"] != "1m" || queries[0]["format"] != "table" {
		t.Fatalf("unexpected prometheus strategy query build: %+v err=%v", queries, err)
	}

	lokiStrategy, _ := findDatasourceStrategy("loki")
	fs = flag.NewFlagSet("loki", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var lokiOpts datasourceQueryOptions
	lokiStrategy.BindFlags(fs, &lokiOpts)
	if err := fs.Parse([]string{"--expr", `{app="checkout"} |= "error"`, "--query-type", "instant", "--legend-format", "{{service}}"}); err != nil {
		t.Fatalf("unexpected loki strategy parse error: %v", err)
	}
	queries, err = lokiStrategy.BuildQueries(lokiOpts, resolvedDatasource{UID: "loki-uid", Type: "loki"})
	if err != nil || queries[0]["expr"] != `{app="checkout"} |= "error"` || queries[0]["queryType"] != "instant" || queries[0]["legendFormat"] != "{{service}}" {
		t.Fatalf("unexpected loki strategy query build: %+v err=%v", queries, err)
	}
	queries, err = lokiStrategy.BuildQueries(datasourceQueryOptions{QueryJSON: `{"expr":"count_over_time({app=\"checkout\"}[5m])"}`}, resolvedDatasource{UID: "loki-uid", Type: "loki"})
	if err != nil || queries[0]["expr"] != `count_over_time({app="checkout"}[5m])` {
		t.Fatalf("unexpected loki passthrough query build: %+v err=%v", queries, err)
	}

	influxStrategy, _ := findDatasourceStrategy("influxdb")
	fs = flag.NewFlagSet("influx", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var influxOpts datasourceQueryOptions
	influxStrategy.BindFlags(fs, &influxOpts)
	if err := fs.Parse([]string{"--query", "from(bucket:\"prod\")", "--query-language", "flux"}); err != nil {
		t.Fatalf("unexpected influx strategy parse error: %v", err)
	}
	queries, err = influxStrategy.BuildQueries(influxOpts, resolvedDatasource{UID: "influx-uid", Type: "influxdb"})
	if err != nil || queries[0]["queryLanguage"] != "flux" {
		t.Fatalf("unexpected influx strategy query build: %+v err=%v", queries, err)
	}

	tempoStrategy, _ := findDatasourceStrategy("tempo")
	fs = flag.NewFlagSet("tempo", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var tempoOpts datasourceQueryOptions
	tempoStrategy.BindFlags(fs, &tempoOpts)
	if err := fs.Parse([]string{"--query", `{ status = error }`, "--limit", "20"}); err != nil {
		t.Fatalf("unexpected tempo strategy parse error: %v", err)
	}
	queries, err = tempoStrategy.BuildQueries(tempoOpts, resolvedDatasource{UID: "tempo-uid", Type: "tempo"})
	if err != nil || queries[0]["query"] != `{ status = error }` || queries[0]["queryType"] != "traceqlSearch" || queries[0]["limit"] != 20 {
		t.Fatalf("unexpected tempo strategy query build: %+v err=%v", queries, err)
	}
	queries, err = tempoStrategy.BuildQueries(datasourceQueryOptions{Query: `{ status = unset }`}, resolvedDatasource{UID: "tempo-uid", Type: "tempo"})
	if err != nil || queries[0]["query"] != `{ status = unset }` {
		t.Fatalf("unexpected tempo query without limit: %+v err=%v", queries, err)
	}
	queries, err = tempoStrategy.BuildQueries(datasourceQueryOptions{QueryJSON: `{"query":"{ status = ok }"}`}, resolvedDatasource{UID: "tempo-uid", Type: "tempo"})
	if err != nil || queries[0]["query"] != `{ status = ok }` {
		t.Fatalf("unexpected tempo passthrough query build: %+v err=%v", queries, err)
	}

	graphiteStrategy, _ := findDatasourceStrategy("graphite")
	fs = flag.NewFlagSet("graphite", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var graphiteOpts datasourceQueryOptions
	graphiteStrategy.BindFlags(fs, &graphiteOpts)
	if err := fs.Parse([]string{"--expr", "sumSeries(x)", "--format", "time_series"}); err != nil {
		t.Fatalf("unexpected graphite strategy parse error: %v", err)
	}
	queries, err = graphiteStrategy.BuildQueries(graphiteOpts, resolvedDatasource{UID: "graphite-uid", Type: "graphite"})
	if err != nil || queries[0]["target"] != "sumSeries(x)" {
		t.Fatalf("unexpected graphite strategy query build: %+v err=%v", queries, err)
	}
	queries, err = graphiteStrategy.BuildQueries(datasourceQueryOptions{QueryJSON: `{"target":"sumSeries(y)"}`}, resolvedDatasource{UID: "graphite-uid", Type: "graphite"})
	if err != nil || queries[0]["target"] != "sumSeries(y)" {
		t.Fatalf("unexpected graphite passthrough query build: %+v err=%v", queries, err)
	}

	cloudwatchStrategy, _ := findDatasourceStrategy("cloudwatch")
	fs = flag.NewFlagSet("cloudwatch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var cloudwatchOpts datasourceQueryOptions
	cloudwatchStrategy.BindFlags(fs, &cloudwatchOpts)
	if err := fs.Parse([]string{"--namespace", "AWS/EC2", "--metric-name", "CPUUtilization", "--region", "us-east-1", "--statistic", "Average", "--dimensions", "InstanceId=i-123", "--match-exact"}); err != nil {
		t.Fatalf("unexpected cloudwatch strategy parse error: %v", err)
	}
	queries, err = cloudwatchStrategy.BuildQueries(cloudwatchOpts, resolvedDatasource{UID: "cloudwatch-uid", Type: "cloudwatch"})
	if err != nil || queries[0]["metricName"] != "CPUUtilization" || queries[0]["matchExact"] != true {
		t.Fatalf("unexpected cloudwatch strategy query build: %+v err=%v", queries, err)
	}
	queries, err = cloudwatchStrategy.BuildQueries(datasourceQueryOptions{QueryJSON: `{"metricName":"NetworkIn"}`}, resolvedDatasource{UID: "cloudwatch-uid", Type: "cloudwatch"})
	if err != nil || queries[0]["metricName"] != "NetworkIn" {
		t.Fatalf("unexpected cloudwatch passthrough query build: %+v err=%v", queries, err)
	}

	generic := genericDatasourceStrategy()
	queries, err = generic.BuildQueries(datasourceQueryOptions{QueryJSON: `{"rawSql":"SELECT 1"}`, RefID: "A", IntervalMS: 1000, MaxDataPoints: 10}, resolvedDatasource{UID: "generic-uid", Type: "postgres"})
	if err != nil || queries[0]["datasource"].(map[string]any)["uid"] != "generic-uid" {
		t.Fatalf("unexpected generic strategy query build: %+v err=%v", queries, err)
	}
	passthrough := passthroughDatasourceStrategy{family: datasourceQueryFamily{Name: "passthrough"}}
	fs = flag.NewFlagSet("generic", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var genericOpts datasourceQueryOptions
	passthrough.BindFlags(fs, &genericOpts)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("unexpected passthrough strategy parse error: %v", err)
	}
	queries, err = mysqlStrategy.BuildQueries(datasourceQueryOptions{QueryJSON: `{"rawSql":"SELECT 2"}`}, resolvedDatasource{UID: "mysql-uid", Type: "mysql"})
	if err != nil || queries[0]["rawSql"] != "SELECT 2" {
		t.Fatalf("unexpected sql passthrough query build: %+v err=%v", queries, err)
	}
	queries, err = prometheusStrategy.BuildQueries(datasourceQueryOptions{QueryJSON: `{"expr":"rate(http_requests_total[5m])"}`}, resolvedDatasource{UID: "prom-uid", Type: "prometheus"})
	if err != nil || queries[0]["expr"] != "rate(http_requests_total[5m])" {
		t.Fatalf("unexpected expr passthrough query build: %+v err=%v", queries, err)
	}
	queries, err = influxStrategy.BuildQueries(datasourceQueryOptions{QueryJSON: `{"query":"from(bucket:\"prod\")"}`}, resolvedDatasource{UID: "influx-uid", Type: "influxdb"})
	if err != nil || queries[0]["query"] != `from(bucket:"prod")` {
		t.Fatalf("unexpected query passthrough query build: %+v err=%v", queries, err)
	}
	if _, err := cloudwatchStrategy.BuildQueries(datasourceQueryOptions{
		CloudWatchNamespace:  "AWS/EC2",
		CloudWatchMetricName: "CPUUtilization",
		CloudWatchRegion:     "us-east-1",
		CloudWatchDimensions: "invalid",
	}, resolvedDatasource{UID: "cloudwatch-uid", Type: "cloudwatch"}); err == nil {
		t.Fatalf("expected cloudwatch typed dimensions validation error")
	}

	if len(mysqlStrategy.DiscoveryFlags()) == 0 || len(prometheusStrategy.Examples()) == 0 || len(datasourceQueryFamilies()) == 0 {
		t.Fatalf("expected datasource strategy metadata")
	}
}

func TestDatasourceCommandUtilityHelpers(t *testing.T) {
	queryFlags := datasourceQueryDiscoveryFlags(nil)
	if len(queryFlags) == 0 || queryFlags[0].Name != "--uid" {
		t.Fatalf("expected datasource query discovery flags")
	}
	if len(datasourceSelectionDiscoveryFlags()) != 3 {
		t.Fatalf("expected datasource selection discovery flags")
	}

	var opts datasourceQueryOptions
	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	bindDatasourceQueryFlags(fs, &opts)
	if err := fs.Parse([]string{"--uid", "mysql-uid", "--from", "now-30m", "--to", "now", "--ref-id", "Z", "--interval-ms", "2000", "--max-data-points", "100", "--query-json", `{}`}); err != nil {
		t.Fatalf("unexpected datasource query flag parse error: %v", err)
	}
	if opts.Selector.UID != "mysql-uid" || opts.IntervalMS != 2000 || opts.QueryJSON != "{}" {
		t.Fatalf("unexpected datasource query options: %+v", opts)
	}

	if chooseDefaultRefID("R", 0) != "R" || chooseDefaultRefID("", 2) != "C" || chooseDefaultRefID("", 26) != "Q27" {
		t.Fatalf("unexpected refId defaults")
	}
	if cloned := cloneAnyMap(nil); len(cloned) != 0 {
		t.Fatalf("expected empty clone for nil input, got %+v", cloned)
	}
	cloned := cloneAnyMap(map[string]any{"x": 1})
	cloned["x"] = 2
	if cloned["x"] != 2 {
		t.Fatalf("expected clone mutation")
	}

	queries, err := buildDatasourceQueries("uid", "mysql", "Z", 2000, 100, `{"rawSql":"SELECT 1"}`, "")
	if err != nil || queries[0]["refId"] != "Z" {
		t.Fatalf("unexpected datasource query helper result: %+v err=%v", queries, err)
	}
	if _, err := buildDatasourceQueries("uid", "", "", 1, 1, "", ""); err == nil {
		t.Fatalf("expected datasource query helper error")
	}
	if _, err := buildDatasourceQueries("uid", "", "", 1, 1, `{}`, `[]`); err == nil {
		t.Fatalf("expected conflicting datasource query helper error")
	}
	if _, err := buildDatasourceQueries("uid", "", "", 1, 1, "{", ""); err == nil {
		t.Fatalf("expected invalid query-json error")
	}
	if _, err := buildDatasourceQueries("uid", "", "", 1, 1, "", "["); err == nil {
		t.Fatalf("expected invalid queries-json error")
	}
	if _, err := buildDatasourceQueries("uid", "", "", 1, 1, "", "[]"); err == nil {
		t.Fatalf("expected empty queries-json error")
	}
	if _, err := buildDatasourceQueries("uid", "", "", 1, 1, "", `[1]`); err == nil {
		t.Fatalf("expected non-object queries-json error")
	}
	queries, err = buildDatasourceQueries("uid", "mysql", "", 1, 1, "", `[{"rawSql":"SELECT 1"}]`)
	if err != nil || queries[0]["datasource"].(map[string]any)["type"] != "mysql" {
		t.Fatalf("unexpected queries-json helper success: %+v err=%v", queries, err)
	}

	normalized := applyDatasourceQueryDefaults(map[string]any{
		"refId":         "B",
		"intervalMs":    50,
		"maxDataPoints": 10,
		"datasource":    map[string]any{"uid": "existing"},
	}, "ignored", "mysql", "A", 100, 20)
	if normalized["refId"] != "B" || normalized["datasource"].(map[string]any)["uid"] != "existing" {
		t.Fatalf("expected datasource query defaults to preserve explicit fields: %+v", normalized)
	}
}

func TestDatasourceDiscoveryPayloads(t *testing.T) {
	payload, err := buildDiscoveryPayload([]string{"datasources", "mysql", "query"}, false)
	if err != nil {
		t.Fatalf("unexpected datasource discovery payload error: %v", err)
	}
	command := payload["commands"].([]map[string]any)[0]
	if examples := command["examples"].([]string); len(examples) == 0 {
		t.Fatalf("expected datasource discovery examples")
	}
	if command["documentation_url"] == "" {
		t.Fatalf("expected datasource discovery documentation url")
	}
	if notes := command["notes"].([]string); len(notes) == 0 {
		t.Fatalf("expected datasource discovery notes")
	}

	root, err := buildDiscoveryPayload([]string{"datasources"}, false)
	if err != nil {
		t.Fatalf("unexpected datasource discovery root error: %v", err)
	}
	if _, ok := root["query_syntax"].(map[string]string)["cloudwatch"]; !ok {
		t.Fatalf("expected datasource family syntax in discovery payload")
	}
	encoded, err := json.Marshal(root)
	if err != nil || len(encoded) == 0 {
		t.Fatalf("expected datasource discovery payload to marshal: %v", err)
	}
}

type staticError string

func (e staticError) Error() string { return string(e) }

type fakeDatasourceStrategy struct {
	flags []discoveryFlag
}

func (f fakeDatasourceStrategy) Family() datasourceQueryFamily {
	return datasourceQueryFamily{Name: "fake"}
}
func (f fakeDatasourceStrategy) DiscoveryFlags() []discoveryFlag                      { return f.flags }
func (f fakeDatasourceStrategy) Examples() []string                                   { return nil }
func (f fakeDatasourceStrategy) BindFlags(_ *flag.FlagSet, _ *datasourceQueryOptions) {}
func (f fakeDatasourceStrategy) BuildQueries(_ datasourceQueryOptions, _ resolvedDatasource) ([]map[string]any, error) {
	return nil, nil
}
func (f fakeDatasourceStrategy) SupportsType(string) bool { return false }
