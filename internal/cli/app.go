package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/matiasvillaverde/grafana-cli/internal/agent"
	"github.com/matiasvillaverde/grafana-cli/internal/config"
	"github.com/matiasvillaverde/grafana-cli/internal/grafana"
)

// APIClient is the command-layer dependency for Grafana API operations.
type APIClient interface {
	Raw(ctx context.Context, method, path string, body any) (any, error)
	CloudStacks(ctx context.Context) (any, error)
	SearchDashboards(ctx context.Context, query, tag string, limit int) (any, error)
	CreateDashboard(ctx context.Context, dashboard map[string]any, folderID int64, overwrite bool) (any, error)
	ListDatasources(ctx context.Context) (any, error)
	AssistantChat(ctx context.Context, prompt, chatID string) (any, error)
	AssistantChatStatus(ctx context.Context, chatID string) (any, error)
	AssistantSkills(ctx context.Context) (any, error)
	MetricsRange(ctx context.Context, expr, start, end, step string) (any, error)
	LogsRange(ctx context.Context, query, start, end string, limit int) (any, error)
	TracesSearch(ctx context.Context, query, start, end string, limit int) (any, error)
	AggregateSnapshot(ctx context.Context, req grafana.AggregateRequest) (grafana.AggregateSnapshot, error)
}

// ClientFactory creates API clients from stored config.
type ClientFactory func(config.Config) APIClient

type globalOptions struct {
	Output string
	Fields []string
}

// App coordinates command parsing and execution.
type App struct {
	Out       io.Writer
	Err       io.Writer
	Store     config.Store
	NewClient ClientFactory
	Now       func() time.Time
}

func NewApp(store config.Store) *App {
	return &App{
		Out:   os.Stdout,
		Err:   os.Stderr,
		Store: store,
		NewClient: func(cfg config.Config) APIClient {
			return grafana.NewClient(cfg, nil)
		},
		Now: time.Now,
	}
}

func (a *App) Run(ctx context.Context, args []string) int {
	opts, rest, err := parseGlobalOptions(args)
	if err != nil {
		a.printErr(err)
		return 1
	}

	if len(rest) == 0 || rest[0] == "help" || rest[0] == "--help" || rest[0] == "-h" {
		_ = a.emit(opts, map[string]any{
			"commands": []string{"auth", "api", "cloud", "dashboards", "datasources", "assistant", "runtime", "aggregate", "incident", "agent"},
		})
		return 0
	}

	var runErr error
	switch rest[0] {
	case "auth":
		runErr = a.runAuth(opts, rest[1:])
	case "api":
		runErr = a.runAPI(ctx, opts, rest[1:])
	case "cloud":
		runErr = a.runCloud(ctx, opts, rest[1:])
	case "dashboards":
		runErr = a.runDashboards(ctx, opts, rest[1:])
	case "datasources":
		runErr = a.runDatasources(ctx, opts, rest[1:])
	case "assistant":
		runErr = a.runAssistant(ctx, opts, rest[1:])
	case "runtime":
		runErr = a.runRuntime(ctx, opts, rest[1:])
	case "aggregate":
		runErr = a.runAggregate(ctx, opts, rest[1:])
	case "incident":
		runErr = a.runIncident(ctx, opts, rest[1:])
	case "agent":
		runErr = a.runAgent(ctx, opts, rest[1:])
	default:
		runErr = fmt.Errorf("unknown command: %s", rest[0])
	}

	if runErr != nil {
		a.printErr(runErr)
		return 1
	}
	return 0
}

func (a *App) runAuth(opts globalOptions, args []string) error {
	if len(args) == 0 {
		return a.emit(opts, map[string]any{"commands": []string{"login", "status", "logout"}})
	}

	switch args[0] {
	case "login":
		return a.runAuthLogin(opts, args[1:])
	case "status":
		cfg, err := a.Store.Load()
		if err != nil {
			return err
		}
		status := "unauthenticated"
		if cfg.IsAuthenticated() {
			status = "authenticated"
		}
		return a.emit(opts, map[string]any{
			"status":         status,
			"base_url":       cfg.BaseURL,
			"cloud_url":      cfg.CloudURL,
			"prometheus_url": cfg.PrometheusURL,
			"logs_url":       cfg.LogsURL,
			"traces_url":     cfg.TracesURL,
			"org_id":         cfg.OrgID,
		})
	case "logout":
		if err := a.Store.Clear(); err != nil {
			return err
		}
		return a.emit(opts, map[string]any{"status": "logged_out"})
	default:
		return fmt.Errorf("unknown auth command: %s", args[0])
	}
}

func (a *App) runAuthLogin(opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("auth login", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	token := fs.String("token", "", "Grafana token")
	baseURL := fs.String("base-url", "", "Grafana base URL")
	cloudURL := fs.String("cloud-url", "", "Grafana cloud API URL")
	promURL := fs.String("prom-url", "", "Prometheus query URL")
	logsURL := fs.String("logs-url", "", "Loki query URL")
	tracesURL := fs.String("traces-url", "", "Tempo query URL")
	orgID := fs.Int64("org-id", 0, "Grafana org ID")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*token) == "" {
		return errors.New("--token is required")
	}

	cfg, err := a.Store.Load()
	if err != nil {
		return err
	}

	cfg.Token = strings.TrimSpace(*token)
	if strings.TrimSpace(*baseURL) != "" {
		cfg.BaseURL = strings.TrimSpace(*baseURL)
	}
	if strings.TrimSpace(*cloudURL) != "" {
		cfg.CloudURL = strings.TrimSpace(*cloudURL)
	}
	if strings.TrimSpace(*promURL) != "" {
		cfg.PrometheusURL = strings.TrimSpace(*promURL)
	}
	if strings.TrimSpace(*logsURL) != "" {
		cfg.LogsURL = strings.TrimSpace(*logsURL)
	}
	if strings.TrimSpace(*tracesURL) != "" {
		cfg.TracesURL = strings.TrimSpace(*tracesURL)
	}
	if *orgID > 0 {
		cfg.OrgID = *orgID
	}

	if err := a.Store.Save(cfg); err != nil {
		return err
	}

	return a.emit(opts, map[string]any{
		"status":         "authenticated",
		"base_url":       cfg.BaseURL,
		"cloud_url":      cfg.CloudURL,
		"prometheus_url": cfg.PrometheusURL,
		"logs_url":       cfg.LogsURL,
		"traces_url":     cfg.TracesURL,
		"org_id":         cfg.OrgID,
	})
}

func (a *App) runAPI(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 2 {
		return errors.New("usage: api <METHOD> <PATH> [--body JSON]")
	}
	method := args[0]
	path := args[1]

	fs := flag.NewFlagSet("api", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	body := fs.String("body", "", "JSON body")

	if err := fs.Parse(args[2:]); err != nil {
		return err
	}

	cfg, err := a.requireAuthConfig()
	if err != nil {
		return err
	}
	client := a.NewClient(cfg)

	var parsedBody any
	if strings.TrimSpace(*body) != "" {
		if err := json.Unmarshal([]byte(*body), &parsedBody); err != nil {
			return fmt.Errorf("invalid --body JSON: %w", err)
		}
	}

	result, err := client.Raw(ctx, strings.ToUpper(method), path, parsedBody)
	if err != nil {
		return err
	}
	return a.emit(opts, result)
}

func (a *App) runCloud(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: cloud stacks list")
	}
	if args[0] != "stacks" {
		return fmt.Errorf("unknown cloud command: %s", args[0])
	}
	if len(args) < 2 || args[1] != "list" {
		return errors.New("usage: cloud stacks list")
	}
	cfg, err := a.requireAuthConfig()
	if err != nil {
		return err
	}
	result, err := a.NewClient(cfg).CloudStacks(ctx)
	if err != nil {
		return err
	}
	return a.emit(opts, result)
}

func (a *App) runDashboards(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: dashboards <list|create>")
	}

	cfg, err := a.requireAuthConfig()
	if err != nil {
		return err
	}
	client := a.NewClient(cfg)

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("dashboards list", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		query := fs.String("query", "", "search query")
		tag := fs.String("tag", "", "tag filter")
		limit := fs.Int("limit", 100, "limit")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		result, err := client.SearchDashboards(ctx, *query, *tag, *limit)
		if err != nil {
			return err
		}
		return a.emit(opts, result)
	case "create":
		fs := flag.NewFlagSet("dashboards create", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		title := fs.String("title", "", "dashboard title")
		uid := fs.String("uid", "", "dashboard UID")
		schemaVersion := fs.Int("schema-version", 39, "schema version")
		folderID := fs.Int64("folder-id", 0, "folder ID")
		overwrite := fs.Bool("overwrite", true, "overwrite existing dashboard")
		tags := fs.String("tags", "", "comma separated tags")
		templateJSON := fs.String("template-json", "", "dashboard JSON object")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*title) == "" && strings.TrimSpace(*templateJSON) == "" {
			return errors.New("--title or --template-json is required")
		}

		dashboard := map[string]any{}
		if strings.TrimSpace(*templateJSON) != "" {
			if err := json.Unmarshal([]byte(*templateJSON), &dashboard); err != nil {
				return fmt.Errorf("invalid --template-json: %w", err)
			}
		} else {
			dashboard["title"] = *title
			dashboard["schemaVersion"] = *schemaVersion
			dashboard["version"] = 0
			dashboard["panels"] = []any{}
			if strings.TrimSpace(*uid) != "" {
				dashboard["uid"] = *uid
			}
			if strings.TrimSpace(*tags) != "" {
				dashboard["tags"] = splitCSV(*tags)
			}
		}

		result, err := client.CreateDashboard(ctx, dashboard, *folderID, *overwrite)
		if err != nil {
			return err
		}
		return a.emit(opts, result)
	default:
		return fmt.Errorf("unknown dashboards command: %s", args[0])
	}
}

func (a *App) runDatasources(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || args[0] != "list" {
		return errors.New("usage: datasources list [--type TYPE] [--name NAME]")
	}

	fs := flag.NewFlagSet("datasources list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	typeFilter := fs.String("type", "", "datasource type filter")
	nameFilter := fs.String("name", "", "name substring filter")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	cfg, err := a.requireAuthConfig()
	if err != nil {
		return err
	}
	result, err := a.NewClient(cfg).ListDatasources(ctx)
	if err != nil {
		return err
	}
	result = filterDatasources(result, *typeFilter, *nameFilter)
	return a.emit(opts, result)
}

func (a *App) runAssistant(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: assistant <chat|status|skills>")
	}

	cfg, err := a.requireAuthConfig()
	if err != nil {
		return err
	}
	client := a.NewClient(cfg)

	switch args[0] {
	case "chat":
		fs := flag.NewFlagSet("assistant chat", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		prompt := fs.String("prompt", "", "assistant prompt")
		chatID := fs.String("chat-id", "", "existing chat ID to continue")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*prompt) == "" {
			return errors.New("--prompt is required")
		}
		result, err := client.AssistantChat(ctx, *prompt, *chatID)
		if err != nil {
			return err
		}
		return a.emit(opts, result)
	case "status":
		fs := flag.NewFlagSet("assistant status", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		chatID := fs.String("chat-id", "", "chat ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*chatID) == "" {
			return errors.New("--chat-id is required")
		}
		result, err := client.AssistantChatStatus(ctx, *chatID)
		if err != nil {
			return err
		}
		return a.emit(opts, result)
	case "skills":
		if len(args) != 1 {
			return errors.New("usage: assistant skills")
		}
		result, err := client.AssistantSkills(ctx)
		if err != nil {
			return err
		}
		return a.emit(opts, result)
	default:
		return fmt.Errorf("unknown assistant command: %s", args[0])
	}
}

func (a *App) runRuntime(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) < 2 {
		return errors.New("usage: runtime <metrics|logs|traces> <query|search> [flags]")
	}

	cfg, err := a.requireAuthConfig()
	if err != nil {
		return err
	}
	client := a.NewClient(cfg)

	switch args[0] {
	case "metrics":
		if args[1] != "query" {
			return errors.New("usage: runtime metrics query --expr EXPR [--start RFC3339] [--end RFC3339] [--step 30s]")
		}
		fs := flag.NewFlagSet("runtime metrics query", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		expr := fs.String("expr", "", "PromQL expression")
		start := fs.String("start", "", "RFC3339 start")
		end := fs.String("end", "", "RFC3339 end")
		step := fs.String("step", "30s", "step")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if strings.TrimSpace(*expr) == "" {
			return errors.New("--expr is required")
		}
		result, err := client.MetricsRange(ctx, *expr, *start, *end, *step)
		if err != nil {
			return err
		}
		return a.emit(opts, result)
	case "logs":
		if args[1] != "query" {
			return errors.New("usage: runtime logs query --query QUERY [--start RFC3339] [--end RFC3339] [--limit 200]")
		}
		fs := flag.NewFlagSet("runtime logs query", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		query := fs.String("query", "", "LogQL query")
		start := fs.String("start", "", "RFC3339 start")
		end := fs.String("end", "", "RFC3339 end")
		limit := fs.Int("limit", 200, "result limit")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if strings.TrimSpace(*query) == "" {
			return errors.New("--query is required")
		}
		result, err := client.LogsRange(ctx, *query, *start, *end, *limit)
		if err != nil {
			return err
		}
		return a.emit(opts, result)
	case "traces":
		if args[1] != "search" {
			return errors.New("usage: runtime traces search --query QUERY [--start RFC3339] [--end RFC3339] [--limit 200]")
		}
		fs := flag.NewFlagSet("runtime traces search", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		query := fs.String("query", "", "TraceQL query")
		start := fs.String("start", "", "RFC3339 start")
		end := fs.String("end", "", "RFC3339 end")
		limit := fs.Int("limit", 200, "result limit")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if strings.TrimSpace(*query) == "" {
			return errors.New("--query is required")
		}
		result, err := client.TracesSearch(ctx, *query, *start, *end, *limit)
		if err != nil {
			return err
		}
		return a.emit(opts, result)
	default:
		return fmt.Errorf("unknown runtime command: %s", args[0])
	}
}

func (a *App) runAggregate(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || args[0] != "snapshot" {
		return errors.New("usage: aggregate snapshot --metric-expr EXPR --log-query QUERY --trace-query QUERY [--start RFC3339] [--end RFC3339] [--step 30s] [--limit 200]")
	}

	fs := flag.NewFlagSet("aggregate snapshot", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	metricExpr := fs.String("metric-expr", "", "PromQL expression")
	logQuery := fs.String("log-query", "", "LogQL query")
	traceQuery := fs.String("trace-query", "", "TraceQL query")
	start := fs.String("start", "", "RFC3339 start")
	end := fs.String("end", "", "RFC3339 end")
	step := fs.String("step", "30s", "step")
	limit := fs.Int("limit", 200, "result limit")

	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if strings.TrimSpace(*metricExpr) == "" || strings.TrimSpace(*logQuery) == "" || strings.TrimSpace(*traceQuery) == "" {
		return errors.New("--metric-expr, --log-query, and --trace-query are required")
	}

	cfg, err := a.requireAuthConfig()
	if err != nil {
		return err
	}
	result, err := a.NewClient(cfg).AggregateSnapshot(ctx, grafana.AggregateRequest{
		MetricExpr: *metricExpr,
		LogQuery:   *logQuery,
		TraceQuery: *traceQuery,
		Start:      *start,
		End:        *end,
		Step:       *step,
		Limit:      *limit,
	})
	if err != nil {
		return err
	}
	return a.emit(opts, result)
}

func (a *App) runIncident(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || args[0] != "analyze" {
		return errors.New("usage: incident analyze --goal GOAL [--start RFC3339] [--end RFC3339] [--step 30s] [--limit 200] [--include-raw]")
	}

	fs := flag.NewFlagSet("incident analyze", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	goal := fs.String("goal", "", "incident goal")
	metricExpr := fs.String("metric-expr", "", "override metric expression")
	logQuery := fs.String("log-query", "", "override log query")
	traceQuery := fs.String("trace-query", "", "override trace query")
	start := fs.String("start", "", "RFC3339 start")
	end := fs.String("end", "", "RFC3339 end")
	step := fs.String("step", "", "step")
	limit := fs.Int("limit", 0, "result limit")
	includeRaw := fs.Bool("include-raw", false, "include full response payloads")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if strings.TrimSpace(*goal) == "" {
		return errors.New("--goal is required")
	}

	cfg, err := a.requireAuthConfig()
	if err != nil {
		return err
	}
	client := a.NewClient(cfg)

	plan := agent.BuildPlan(*goal, a.Now())
	req := plan.AggregateRequest(a.Now())
	if strings.TrimSpace(*metricExpr) != "" {
		req.MetricExpr = *metricExpr
	}
	if strings.TrimSpace(*logQuery) != "" {
		req.LogQuery = *logQuery
	}
	if strings.TrimSpace(*traceQuery) != "" {
		req.TraceQuery = *traceQuery
	}
	if strings.TrimSpace(*start) != "" {
		req.Start = *start
	}
	if strings.TrimSpace(*end) != "" {
		req.End = *end
	}
	if strings.TrimSpace(*step) != "" {
		req.Step = *step
	}
	if *limit > 0 {
		req.Limit = *limit
	}

	snapshot, err := client.AggregateSnapshot(ctx, req)
	if err != nil {
		return err
	}

	result := map[string]any{
		"goal":      *goal,
		"playbook":  plan.Playbook,
		"request":   req,
		"summary":   summarizeSnapshot(snapshot),
		"generated": a.Now().UTC(),
	}
	if *includeRaw {
		result["snapshot"] = snapshot
	}

	return a.emit(opts, result)
}

func (a *App) runAgent(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: agent <plan|run> --goal GOAL")
	}

	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	goal := fs.String("goal", "", "automation goal")
	includeRaw := fs.Bool("include-raw", false, "include full payloads")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if strings.TrimSpace(*goal) == "" {
		return errors.New("--goal is required")
	}

	plan := agent.BuildPlan(*goal, a.Now())

	switch args[0] {
	case "plan":
		return a.emit(opts, plan)
	case "run":
		cfg, err := a.requireAuthConfig()
		if err != nil {
			return err
		}
		client := a.NewClient(cfg)
		stacks, err := client.CloudStacks(ctx)
		if err != nil {
			return err
		}
		req := plan.AggregateRequest(a.Now())
		snapshot, err := client.AggregateSnapshot(ctx, req)
		if err != nil {
			return err
		}
		result := map[string]any{
			"plan":        plan,
			"request":     req,
			"summary":     summarizeSnapshot(snapshot),
			"stack_count": inferCollectionCount(stacks),
			"executed_at": a.Now().UTC(),
		}
		if *includeRaw {
			result["stacks"] = stacks
			result["snapshot"] = snapshot
		}
		return a.emit(opts, result)
	default:
		return fmt.Errorf("unknown agent command: %s", args[0])
	}
}

func (a *App) requireAuthConfig() (config.Config, error) {
	cfg, err := a.Store.Load()
	if err != nil {
		return config.Config{}, err
	}
	if !cfg.IsAuthenticated() {
		return config.Config{}, errors.New("not authenticated: run `grafana auth login --token ...`")
	}
	return cfg, nil
}

func parseGlobalOptions(args []string) (globalOptions, []string, error) {
	opts := globalOptions{Output: "json"}
	rest := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--output":
			if i+1 >= len(args) {
				return globalOptions{}, nil, errors.New("--output requires a value")
			}
			opts.Output = args[i+1]
			i++
		case strings.HasPrefix(arg, "--output="):
			opts.Output = strings.TrimPrefix(arg, "--output=")
		case arg == "--fields":
			if i+1 >= len(args) {
				return globalOptions{}, nil, errors.New("--fields requires a value")
			}
			opts.Fields = splitCSV(args[i+1])
			i++
		case strings.HasPrefix(arg, "--fields="):
			opts.Fields = splitCSV(strings.TrimPrefix(arg, "--fields="))
		default:
			rest = append(rest, arg)
		}
	}

	if opts.Output != "json" && opts.Output != "pretty" {
		return globalOptions{}, nil, fmt.Errorf("invalid --output value: %s", opts.Output)
	}
	return opts, rest, nil
}

func (a *App) emit(opts globalOptions, payload any) error {
	payload = projectFields(payload, opts.Fields)
	enc := json.NewEncoder(a.Out)
	if opts.Output == "pretty" {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(payload)
}

func (a *App) printErr(err error) {
	_, _ = fmt.Fprintln(a.Err, err.Error())
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func filterDatasources(payload any, typeFilter, nameFilter string) any {
	typeFilter = strings.ToLower(strings.TrimSpace(typeFilter))
	nameFilter = strings.ToLower(strings.TrimSpace(nameFilter))

	items, ok := payload.([]any)
	if !ok {
		return payload
	}
	if typeFilter == "" && nameFilter == "" {
		return payload
	}

	out := make([]any, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if typeFilter != "" {
			value, _ := entry["type"].(string)
			if strings.ToLower(value) != typeFilter {
				continue
			}
		}
		if nameFilter != "" {
			value, _ := entry["name"].(string)
			if !strings.Contains(strings.ToLower(value), nameFilter) {
				continue
			}
		}
		out = append(out, entry)
	}
	return out
}

func summarizeSnapshot(snapshot grafana.AggregateSnapshot) map[string]any {
	return map[string]any{
		"metrics_series": countPath(snapshot.Metrics, "data", "result"),
		"log_streams":    countPath(snapshot.Logs, "data", "result"),
		"trace_matches":  maxInt(countPath(snapshot.Traces, "traces"), countPath(snapshot.Traces, "data", "traces")),
	}
}

func inferCollectionCount(payload any) int {
	switch v := payload.(type) {
	case []any:
		return len(v)
	case map[string]any:
		if items, ok := v["items"].([]any); ok {
			return len(items)
		}
	}
	return 0
}

func countPath(payload any, path ...string) int {
	current := payload
	for _, segment := range path {
		m, ok := current.(map[string]any)
		if !ok {
			return 0
		}
		next, ok := m[segment]
		if !ok {
			return 0
		}
		current = next
	}
	items, ok := current.([]any)
	if !ok {
		return 0
	}
	return len(items)
}

func projectFields(payload any, fields []string) any {
	if len(fields) == 0 {
		return payload
	}
	switch v := payload.(type) {
	case map[string]any:
		out := map[string]any{}
		for _, field := range fields {
			if value, ok := lookupPath(v, strings.Split(field, ".")); ok {
				out[field] = value
			}
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, projectFields(item, fields))
		}
		return out
	default:
		return payload
	}
}

func lookupPath(input map[string]any, path []string) (any, bool) {
	current := any(input)
	for _, segment := range path {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := m[segment]
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func parseInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
