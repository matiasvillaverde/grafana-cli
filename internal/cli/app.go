package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/itchyny/gojq"
	"github.com/matiasvillaverde/grafana-cli/internal/agent"
	"github.com/matiasvillaverde/grafana-cli/internal/config"
	"github.com/matiasvillaverde/grafana-cli/internal/grafana"
)

// APIClient is the command-layer dependency for Grafana API operations.
type APIClient interface {
	Raw(ctx context.Context, method, path string, body any) (any, error)
	CloudStacks(ctx context.Context) (any, error)
	CloudStackDatasources(ctx context.Context, stack string) (any, error)
	CloudStackConnections(ctx context.Context, stack string) (any, error)
	CloudStackPlugins(ctx context.Context, stack string) (any, error)
	CloudStackPluginsPage(ctx context.Context, req grafana.CloudStackPluginListRequest) (any, error)
	CloudStackPlugin(ctx context.Context, stack, plugin string) (any, error)
	CloudBilledUsage(ctx context.Context, req grafana.CloudBilledUsageRequest) (any, error)
	CloudAccessPolicies(ctx context.Context, req grafana.CloudAccessPolicyListRequest) (any, error)
	CloudAccessPolicy(ctx context.Context, id, region string) (any, error)
	SearchDashboards(ctx context.Context, query, tag string, limit int) (any, error)
	GetDashboard(ctx context.Context, uid string) (any, error)
	CreateDashboard(ctx context.Context, dashboard map[string]any, folderID int64, overwrite bool) (any, error)
	DeleteDashboard(ctx context.Context, uid string) (any, error)
	DashboardVersions(ctx context.Context, uid string, limit int) (any, error)
	RenderDashboard(ctx context.Context, req grafana.DashboardRenderRequest) (grafana.RenderedDashboard, error)
	CreateShortURL(ctx context.Context, req grafana.ShortURLRequest) (any, error)
	DashboardPermissions(ctx context.Context, uid string) (any, error)
	UpdateDashboardPermissions(ctx context.Context, uid string, req grafana.PermissionUpdateRequest) (any, error)
	ListDatasources(ctx context.Context) (any, error)
	GetDatasource(ctx context.Context, uid string) (any, error)
	DatasourceHealth(ctx context.Context, uid string) (any, error)
	DatasourceResource(ctx context.Context, method, uid, resourcePath string, body any) (any, error)
	DatasourceQuery(ctx context.Context, req grafana.DatasourceQueryRequest) (any, error)
	ListFolders(ctx context.Context) (any, error)
	GetFolder(ctx context.Context, uid string) (any, error)
	FolderPermissions(ctx context.Context, uid string) (any, error)
	UpdateFolderPermissions(ctx context.Context, uid string, req grafana.PermissionUpdateRequest) (any, error)
	ServiceAccounts(ctx context.Context, req grafana.ServiceAccountListRequest) (any, error)
	ServiceAccount(ctx context.Context, id int64) (any, error)
	ListAnnotations(ctx context.Context, req grafana.AnnotationListRequest) (any, error)
	AlertingRules(ctx context.Context) (any, error)
	AlertingContactPoints(ctx context.Context) (any, error)
	AlertingPolicies(ctx context.Context) (any, error)
	AssistantChat(ctx context.Context, prompt, chatID string) (any, error)
	AssistantChatStatus(ctx context.Context, chatID string) (any, error)
	AssistantSkills(ctx context.Context) (any, error)
	SyntheticChecks(ctx context.Context, req grafana.SyntheticCheckListRequest) (any, error)
	SyntheticCheck(ctx context.Context, req grafana.SyntheticCheckGetRequest) (any, error)
	MetricsRange(ctx context.Context, expr, start, end, step string) (any, error)
	LogsRange(ctx context.Context, query, start, end string, limit int) (any, error)
	TracesSearch(ctx context.Context, query, start, end string, limit int) (any, error)
	AggregateSnapshot(ctx context.Context, req grafana.AggregateRequest) (grafana.AggregateSnapshot, error)
}

// ClientFactory creates API clients from stored config.
type ClientFactory func(config.Config) APIClient

type globalOptions struct {
	Output   string
	Fields   []string
	JQ       string
	Template string
	Agent    bool
	ReadOnly bool
	Yes      bool
}

// App coordinates command parsing and execution.
type App struct {
	Out       io.Writer
	Err       io.Writer
	Store     config.Store
	Contexts  config.ContextStore
	NewClient ClientFactory
	Now       func() time.Time
}

func NewApp(store config.Store) *App {
	app := &App{
		Out:   os.Stdout,
		Err:   os.Stderr,
		Store: store,
		NewClient: func(cfg config.Config) APIClient {
			return grafana.NewClient(cfg, nil)
		},
		Now: time.Now,
	}
	if contexts, ok := store.(config.ContextStore); ok {
		app.Contexts = contexts
	}
	return app
}

func (a *App) Run(ctx context.Context, args []string) int {
	opts, rest, err := parseGlobalOptions(args)
	if err != nil {
		a.printErr(err)
		return 1
	}

	if len(rest) == 0 || isHelpArg(rest[0]) {
		_ = a.emitHelp(opts, nil, true)
		return 0
	}
	if helpPath, ok := requestedHelpPath(rest); ok {
		_ = a.emitHelp(opts, helpPath, helpCompactForPath(helpPath))
		return 0
	}
	if !opts.Yes {
		if err := enforceConfirmation(rest); err != nil {
			a.printErr(err)
			return 1
		}
	}
	if opts.ReadOnly {
		if err := enforceReadOnly(rest); err != nil {
			a.printErr(err)
			return 1
		}
	}

	var runErr error
	switch rest[0] {
	case "schema":
		runErr = a.runSchema(opts, rest[1:])
	case "auth":
		runErr = a.runAuth(ctx, opts, rest[1:])
	case "context":
		runErr = a.runContext(opts, rest[1:])
	case "config":
		runErr = a.runConfig(opts, rest[1:])
	case "api":
		runErr = a.runAPI(ctx, opts, rest[1:])
	case "cloud":
		runErr = a.runCloud(ctx, opts, rest[1:])
	case "service-accounts":
		runErr = a.runServiceAccounts(ctx, opts, rest[1:])
	case "dashboards":
		runErr = a.runDashboards(ctx, opts, rest[1:])
	case "datasources":
		runErr = a.runDatasources(ctx, opts, rest[1:])
	case "folders":
		runErr = a.runFolders(ctx, opts, rest[1:])
	case "annotations":
		runErr = a.runAnnotations(ctx, opts, rest[1:])
	case "alerting":
		runErr = a.runAlerting(ctx, opts, rest[1:])
	case "query-history":
		runErr = a.runQueryHistory(ctx, opts, rest[1:])
	case "slo":
		runErr = a.runSLO(ctx, opts, rest[1:])
	case "assistant":
		runErr = a.runAssistant(ctx, opts, rest[1:])
	case "synthetics":
		runErr = a.runSynthetics(ctx, opts, rest[1:])
	case "runtime":
		runErr = a.runRuntime(ctx, opts, rest[1:])
	case "aggregate":
		runErr = a.runAggregate(ctx, opts, rest[1:])
	case "incident":
		runErr = a.runIncident(ctx, opts, rest[1:])
	case "irm":
		runErr = a.runIRM(ctx, opts, rest[1:])
	case "oncall":
		runErr = a.runOnCall(ctx, opts, rest[1:])
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

func (a *App) runAuth(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"auth"}, true)
	}

	switch args[0] {
	case "login":
		return a.runAuthLogin(ctx, opts, args[1:])
	case "status":
		cfg, err := a.Store.Load()
		if err != nil {
			return err
		}
		return a.emit(opts, authStatusPayload(selectedContextName(a.Contexts, ""), cfg))
	case "doctor":
		cfg, err := a.Store.Load()
		if err != nil {
			return err
		}
		return a.emit(opts, authDoctorPayload(selectedContextName(a.Contexts, ""), cfg))
	case "logout":
		if err := a.Store.Clear(); err != nil {
			return err
		}
		return a.emit(opts, map[string]any{"status": "logged_out"})
	default:
		return fmt.Errorf("unknown auth command: %s", args[0])
	}
}

func (a *App) runAuthLogin(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("auth login", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	token := fs.String("token", "", "Grafana token")
	contextName := fs.String("context", "", "context name")
	stack := fs.String("stack", "", "Grafana Cloud stack slug or https://<stack>.grafana.net URL")
	cloudToken := fs.String("cloud-token", "", "Grafana Cloud API token used only for endpoint discovery")
	baseURL := fs.String("base-url", "", "Grafana base URL")
	cloudURL := fs.String("cloud-url", "", "Grafana cloud API URL")
	promURL := fs.String("prom-url", "", "Prometheus query URL")
	logsURL := fs.String("logs-url", "", "Loki query URL")
	tracesURL := fs.String("traces-url", "", "Tempo query URL")
	oncallURL := fs.String("oncall-url", "", "Grafana OnCall API URL")
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
	warnings, err := a.resolveAuthEndpoints(ctx, &cfg, authLoginRequest{
		Stack:      strings.TrimSpace(*stack),
		CloudToken: strings.TrimSpace(*cloudToken),
		BaseURL:    strings.TrimSpace(*baseURL),
		CloudURL:   strings.TrimSpace(*cloudURL),
		PromURL:    strings.TrimSpace(*promURL),
		LogsURL:    strings.TrimSpace(*logsURL),
		TracesURL:  strings.TrimSpace(*tracesURL),
		OnCallURL:  strings.TrimSpace(*oncallURL),
	})
	if err != nil {
		return err
	}
	if *orgID > 0 {
		cfg.OrgID = *orgID
	}

	if strings.TrimSpace(*contextName) != "" {
		if a.Contexts == nil {
			return errors.New("context support is unavailable")
		}
		if err := a.Contexts.SaveContext(*contextName, cfg); err != nil {
			return err
		}
	} else if err := a.Store.Save(cfg); err != nil {
		return err
	}

	status := authStatusPayload(selectedContextName(a.Contexts, *contextName), cfg)
	if len(warnings) > 0 {
		status["warnings"] = warnings
	}
	return a.emit(opts, status)
}

func (a *App) runContext(opts globalOptions, args []string) error {
	if a.Contexts == nil {
		return errors.New("context support is unavailable")
	}
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"context"}, true)
	}

	switch args[0] {
	case "list":
		if len(args) != 1 {
			return errors.New("usage: context list")
		}
		contexts, err := a.Contexts.ListContexts()
		if err != nil {
			return err
		}
		items := make([]any, 0, len(contexts))
		for _, item := range contexts {
			items = append(items, map[string]any{
				"name":          item.Name,
				"current":       item.Current,
				"authenticated": item.Authenticated,
				"base_url":      item.BaseURL,
				"cloud_url":     item.CloudURL,
			})
		}
		return a.emit(opts, items)
	case "use":
		if len(args) != 2 {
			return errors.New("usage: context use <NAME>")
		}
		if err := a.Contexts.UseContext(args[1]); err != nil {
			return err
		}
		cfg, err := a.Store.Load()
		if err != nil {
			return err
		}
		return a.emit(opts, configPayload(selectedContextName(a.Contexts, args[1]), cfg))
	case "view":
		if len(args) != 1 {
			return errors.New("usage: context view")
		}
		cfg, err := a.Store.Load()
		if err != nil {
			return err
		}
		return a.emit(opts, configPayload(selectedContextName(a.Contexts, ""), cfg))
	default:
		return fmt.Errorf("unknown context command: %s", args[0])
	}
}

func (a *App) runConfig(opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"config"}, true)
	}

	switch args[0] {
	case "list":
		contextName, rest, err := extractContextArg(args[1:])
		if err != nil {
			return err
		}
		if len(rest) != 0 {
			return errors.New("usage: config list [--context NAME]")
		}
		cfg, name, err := a.loadConfigForContext(contextName)
		if err != nil {
			return err
		}
		return a.emit(opts, configPayload(name, cfg))
	case "get":
		contextName, rest, err := extractContextArg(args[1:])
		if err != nil {
			return err
		}
		if len(rest) != 1 {
			return errors.New("usage: config get <KEY> [--context NAME]")
		}
		cfg, name, err := a.loadConfigForContext(contextName)
		if err != nil {
			return err
		}
		value, err := configValueForKey(cfg, rest[0])
		if err != nil {
			return err
		}
		return a.emit(opts, map[string]any{
			"context": name,
			"key":     normalizeConfigKey(rest[0]),
			"value":   value,
		})
	case "set":
		contextName, rest, err := extractContextArg(args[1:])
		if err != nil {
			return err
		}
		if len(rest) != 2 {
			return errors.New("usage: config set <KEY> <VALUE> [--context NAME]")
		}
		cfg, name, err := a.loadConfigForContext(contextName)
		if err != nil {
			return err
		}
		if err := setConfigValue(&cfg, rest[0], rest[1]); err != nil {
			return err
		}
		if strings.TrimSpace(contextName) != "" {
			if err := a.Contexts.SaveContext(contextName, cfg); err != nil {
				return err
			}
		} else if err := a.Store.Save(cfg); err != nil {
			return err
		}
		return a.emit(opts, configPayload(name, cfg))
	default:
		return fmt.Errorf("unknown config command: %s", args[0])
	}
}

func (a *App) runAPI(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"api"}, true)
	}
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
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"cloud"}, true)
	}
	switch args[0] {
	case "stacks":
		return a.runCloudStacks(ctx, opts, args[1:])
	case "billed-usage":
		return a.runCloudBilledUsage(ctx, opts, args[1:])
	case "access-policies":
		return a.runCloudAccessPolicies(ctx, opts, args[1:])
	default:
		return fmt.Errorf("unknown cloud command: %s", args[0])
	}
}

func (a *App) runDashboards(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"dashboards"}, true)
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
		return a.emitWithMetadata(opts, result, collectionMetadata("dashboards list", result, *limit, "Narrow --query or --tag, or raise --limit if you need more dashboards"))
	case "get":
		fs := flag.NewFlagSet("dashboards get", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		uid := fs.String("uid", "", "dashboard UID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*uid) == "" {
			return errors.New("--uid is required")
		}
		result, err := client.GetDashboard(ctx, *uid)
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
	case "delete":
		fs := flag.NewFlagSet("dashboards delete", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		uid := fs.String("uid", "", "dashboard UID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*uid) == "" {
			return errors.New("--uid is required")
		}
		result, err := client.DeleteDashboard(ctx, *uid)
		if err != nil {
			return err
		}
		return a.emit(opts, result)
	case "versions":
		fs := flag.NewFlagSet("dashboards versions", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		uid := fs.String("uid", "", "dashboard UID")
		limit := fs.Int("limit", 20, "limit")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*uid) == "" {
			return errors.New("--uid is required")
		}
		result, err := client.DashboardVersions(ctx, *uid, *limit)
		if err != nil {
			return err
		}
		return a.emit(opts, result)
	case "render":
		fs := flag.NewFlagSet("dashboards render", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		uid := fs.String("uid", "", "dashboard UID")
		slug := fs.String("slug", "", "dashboard slug")
		panelID := fs.Int64("panel-id", 0, "panel ID for panel render")
		width := fs.Int("width", 1600, "render width")
		height := fs.Int("height", 900, "render height")
		theme := fs.String("theme", "light", "render theme")
		from := fs.String("from", "now-6h", "time range start")
		to := fs.String("to", "now", "time range end")
		tz := fs.String("tz", "UTC", "timezone")
		out := fs.String("out", "", "output PNG path")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*uid) == "" {
			return errors.New("--uid is required")
		}
		if strings.TrimSpace(*out) == "" {
			return errors.New("--out is required")
		}
		rendered, err := client.RenderDashboard(ctx, grafana.DashboardRenderRequest{
			UID:     *uid,
			Slug:    *slug,
			PanelID: *panelID,
			Width:   *width,
			Height:  *height,
			Theme:   *theme,
			From:    *from,
			To:      *to,
			TZ:      *tz,
		})
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(*out, rendered.Data, 0o644); err != nil {
			return err
		}
		return a.emit(opts, map[string]any{
			"uid":          *uid,
			"panel_id":     *panelID,
			"path":         *out,
			"content_type": rendered.ContentType,
			"bytes":        rendered.Bytes,
			"endpoint":     rendered.Endpoint,
		})
	case "share":
		fs := flag.NewFlagSet("dashboards share", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		uid := fs.String("uid", "", "dashboard UID")
		slug := fs.String("slug", "", "dashboard slug")
		panelID := fs.Int64("panel-id", 0, "panel ID for panel share links")
		from := fs.String("from", "", "time range start")
		to := fs.String("to", "", "time range end")
		theme := fs.String("theme", "", "share theme")
		orgID := fs.Int64("org-id", 0, "organization ID override")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*uid) == "" {
			return errors.New("--uid is required")
		}

		effectiveOrgID, err := resolveDashboardShareOrgID(ctx, client, cfg, *orgID)
		if err != nil {
			return err
		}

		sharePath := buildDashboardSharePath(*uid, *slug, *panelID, *from, *to, *theme, effectiveOrgID)
		result, err := client.CreateShortURL(ctx, grafana.ShortURLRequest{
			Path:  sharePath,
			OrgID: effectiveOrgID,
		})
		if err != nil {
			return err
		}

		payload, ok := result.(map[string]any)
		if !ok {
			return a.emit(opts, map[string]any{
				"dashboard_uid": *uid,
				"panel_id":      *panelID,
				"share_path":    sharePath,
				"result":        result,
			})
		}

		enriched := map[string]any{}
		for key, value := range payload {
			enriched[key] = value
		}
		enriched["dashboard_uid"] = *uid
		enriched["panel_id"] = *panelID
		enriched["share_path"] = sharePath
		if shortURL, ok := payload["url"].(string); ok && strings.TrimSpace(shortURL) != "" {
			if absoluteURL, ok := resolveShortURLAbsolute(cfg.BaseURL, shortURL); ok {
				enriched["absolute_url"] = absoluteURL
			}
		}
		return a.emit(opts, enriched)
	case "permissions":
		return a.runDashboardPermissions(ctx, opts, client, args[1:])
	default:
		return fmt.Errorf("unknown dashboards command: %s", args[0])
	}
}

func (a *App) runDashboardPermissions(ctx context.Context, opts globalOptions, client APIClient, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"dashboards", "permissions"}, true)
	}

	switch args[0] {
	case "get":
		fs := flag.NewFlagSet("dashboards permissions get", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		uid := fs.String("uid", "", "dashboard UID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*uid) == "" {
			return errors.New("--uid is required")
		}
		result, err := client.DashboardPermissions(ctx, *uid)
		if err != nil {
			return err
		}
		return a.emit(opts, result)
	case "update":
		fs := flag.NewFlagSet("dashboards permissions update", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		uid := fs.String("uid", "", "dashboard UID")
		itemsJSON := fs.String("items-json", "", "permissions items JSON array")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*uid) == "" {
			return errors.New("--uid is required")
		}
		items, err := parsePermissionItemsJSON(*itemsJSON)
		if err != nil {
			return err
		}
		result, err := client.UpdateDashboardPermissions(ctx, *uid, grafana.PermissionUpdateRequest{Items: items})
		if err != nil {
			return err
		}
		return a.emit(opts, result)
	default:
		return fmt.Errorf("unknown dashboards permissions command: %s", args[0])
	}
}

func (a *App) runFolders(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"folders"}, true)
	}

	cfg, err := a.requireAuthConfig()
	if err != nil {
		return err
	}
	client := a.NewClient(cfg)

	switch args[0] {
	case "list":
		if len(args) != 1 {
			return errors.New("usage: folders list")
		}
		result, err := client.ListFolders(ctx)
		if err != nil {
			return err
		}
		return a.emitWithMetadata(opts, result, collectionMetadata("folders list", result, 0, ""))
	case "get":
		fs := flag.NewFlagSet("folders get", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		uid := fs.String("uid", "", "folder UID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*uid) == "" {
			return errors.New("--uid is required")
		}
		result, err := client.GetFolder(ctx, *uid)
		if err != nil {
			return err
		}
		return a.emit(opts, result)
	case "permissions":
		return a.runFolderPermissions(ctx, opts, client, args[1:])
	default:
		return fmt.Errorf("unknown folders command: %s", args[0])
	}
}

func (a *App) runFolderPermissions(ctx context.Context, opts globalOptions, client APIClient, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"folders", "permissions"}, true)
	}

	switch args[0] {
	case "get":
		fs := flag.NewFlagSet("folders permissions get", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		uid := fs.String("uid", "", "folder UID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*uid) == "" {
			return errors.New("--uid is required")
		}
		result, err := client.FolderPermissions(ctx, *uid)
		if err != nil {
			return err
		}
		return a.emit(opts, result)
	case "update":
		fs := flag.NewFlagSet("folders permissions update", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		uid := fs.String("uid", "", "folder UID")
		itemsJSON := fs.String("items-json", "", "permissions items JSON array")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*uid) == "" {
			return errors.New("--uid is required")
		}
		items, err := parsePermissionItemsJSON(*itemsJSON)
		if err != nil {
			return err
		}
		result, err := client.UpdateFolderPermissions(ctx, *uid, grafana.PermissionUpdateRequest{Items: items})
		if err != nil {
			return err
		}
		return a.emit(opts, result)
	default:
		return fmt.Errorf("unknown folders permissions command: %s", args[0])
	}
}

func (a *App) runAnnotations(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"annotations"}, true)
	}
	if len(args) == 0 || args[0] != "list" {
		return errors.New("usage: annotations list [--dashboard-uid UID] [--panel-id ID] [--limit 100] [--from VALUE] [--to VALUE] [--tags a,b] [--type annotation]")
	}

	fs := flag.NewFlagSet("annotations list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dashboardUID := fs.String("dashboard-uid", "", "dashboard UID")
	panelID := fs.Int64("panel-id", 0, "panel ID")
	limit := fs.Int("limit", 100, "result limit")
	from := fs.String("from", "", "from time")
	to := fs.String("to", "", "to time")
	tags := fs.String("tags", "", "comma separated tags")
	annotationType := fs.String("type", "", "annotation type")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	cfg, err := a.requireAuthConfig()
	if err != nil {
		return err
	}
	result, err := a.NewClient(cfg).ListAnnotations(ctx, grafana.AnnotationListRequest{
		DashboardUID: *dashboardUID,
		PanelID:      *panelID,
		Limit:        *limit,
		From:         *from,
		To:           *to,
		Tags:         splitCSV(*tags),
		Type:         *annotationType,
	})
	if err != nil {
		return err
	}
	return a.emitWithMetadata(opts, result, collectionMetadata("annotations list", result, *limit, "Add --dashboard-uid, --panel-id, or --tags to narrow the annotation set"))
}

func (a *App) runAlerting(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"alerting"}, true)
	}
	if len(args) < 2 {
		return errors.New("usage: alerting <rules|contact-points|policies> <list|get>")
	}

	cfg, err := a.requireAuthConfig()
	if err != nil {
		return err
	}
	client := a.NewClient(cfg)

	switch args[0] {
	case "rules":
		if args[1] != "list" || len(args) != 2 {
			return errors.New("usage: alerting rules list")
		}
		result, err := client.AlertingRules(ctx)
		if err != nil {
			return err
		}
		return a.emitWithMetadata(opts, result, collectionMetadata("alerting rules list", result, 0, ""))
	case "contact-points":
		if args[1] != "list" || len(args) != 2 {
			return errors.New("usage: alerting contact-points list")
		}
		result, err := client.AlertingContactPoints(ctx)
		if err != nil {
			return err
		}
		return a.emitWithMetadata(opts, result, collectionMetadata("alerting contact-points list", result, 0, ""))
	case "policies":
		if args[1] != "get" || len(args) != 2 {
			return errors.New("usage: alerting policies get")
		}
		result, err := client.AlertingPolicies(ctx)
		if err != nil {
			return err
		}
		return a.emit(opts, result)
	default:
		return fmt.Errorf("unknown alerting command: %s", args[0])
	}
}

func (a *App) runQueryHistory(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"query-history"}, true)
	}
	if args[0] != "list" {
		return errors.New("usage: query-history list [--datasource-uid UID[,UID...]] [--search TEXT] [--starred] [--sort time-desc|time-asc] [--page 1] [--limit 100] [--from 24h] [--to now]")
	}

	fs := flag.NewFlagSet("query-history list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	datasourceUID := fs.String("datasource-uid", "", "comma-separated datasource UIDs")
	search := fs.String("search", "", "search string")
	starred := fs.Bool("starred", false, "only starred queries")
	sortOrder := fs.String("sort", "time-desc", "time-desc or time-asc")
	page := fs.Int("page", 1, "page number")
	limit := fs.Int("limit", 100, "page size")
	from := fs.String("from", "", "time range start")
	to := fs.String("to", "", "time range end")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *page < 1 {
		return errors.New("--page must be at least 1")
	}
	if *limit < 1 {
		return errors.New("--limit must be at least 1")
	}
	if *sortOrder != "time-desc" && *sortOrder != "time-asc" {
		return errors.New("--sort must be time-desc or time-asc")
	}

	cfg, err := a.requireAuthConfig()
	if err != nil {
		return err
	}
	query := url.Values{}
	for _, uid := range splitCSV(*datasourceUID) {
		query.Add("datasourceUid", uid)
	}
	if strings.TrimSpace(*search) != "" {
		query.Set("searchString", strings.TrimSpace(*search))
	}
	if *starred {
		query.Set("onlyStarred", "true")
	}
	query.Set("sort", *sortOrder)
	query.Set("page", strconv.Itoa(*page))
	query.Set("limit", strconv.Itoa(*limit))
	if fromValue := normalizeQueryHistoryBound(a.Now(), *from); fromValue != "" {
		query.Set("from", fromValue)
	}
	if toValue := normalizeQueryHistoryBound(a.Now(), *to); toValue != "" {
		query.Set("to", toValue)
	}

	result, err := a.NewClient(cfg).Raw(ctx, "GET", appendQuery("/api/query-history", query), nil)
	if err != nil {
		return err
	}
	return a.emitWithMetadata(opts, result, queryHistoryMetadata(result, *limit))
}

func (a *App) runSLO(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"slo"}, true)
	}
	if args[0] != "list" {
		return errors.New("usage: slo list [--query TEXT] [--limit 100]")
	}

	fs := flag.NewFlagSet("slo list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	query := fs.String("query", "", "name or description filter")
	limit := fs.Int("limit", 100, "maximum SLOs to return")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *limit < 1 {
		return errors.New("--limit must be at least 1")
	}

	cfg, err := a.requireAuthConfig()
	if err != nil {
		return err
	}
	result, err := a.NewClient(cfg).Raw(ctx, "GET", "/api/plugins/grafana-slo-app/resources/v1/slo", nil)
	if err != nil {
		return err
	}
	filtered, count, truncated := filterNamedPayload(result, strings.TrimSpace(*query), *limit, "name", "description", "uid", "id")
	meta := &responseMetadata{Command: "slo list"}
	meta.Count = &count
	if truncated {
		meta.Truncated = true
		meta.NextAction = "Narrow --query or raise --limit to inspect more SLO definitions"
	}
	return a.emitWithMetadata(opts, filtered, meta)
}

func (a *App) runAssistant(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"assistant"}, true)
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
		return a.emitWithMetadata(opts, result, collectionMetadata("assistant skills", result, 0, ""))
	case "investigate":
		fs := flag.NewFlagSet("assistant investigate", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		goal := fs.String("goal", "", "investigation goal")
		chatID := fs.String("chat-id", "", "existing chat ID to continue")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*goal) == "" {
			return errors.New("--goal is required")
		}
		result, err := client.AssistantChat(ctx, investigationPrompt(*goal), *chatID)
		if err != nil {
			return err
		}
		payload := map[string]any{
			"goal": strings.TrimSpace(*goal),
			"chat": result,
		}
		meta := &responseMetadata{Command: "assistant investigate"}
		if chatIdentifier := firstNonEmptyString(mapValue(payload, "chat"), "chatId", "chatID", "id"); chatIdentifier != "" {
			meta.NextAction = "Use `grafana assistant status --chat-id " + chatIdentifier + "` to poll the investigation"
		}
		return a.emitWithMetadata(opts, payload, meta)
	default:
		return fmt.Errorf("unknown assistant command: %s", args[0])
	}
}

func (a *App) runRuntime(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"runtime"}, true)
	}
	if len(args) < 2 {
		return errors.New("usage: runtime <metrics|logs|traces> <query|search|aggregate> [flags]")
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
		startValue, endValue := normalizeTimeRange(a.Now(), *start, *end, time.Hour)
		result, err := client.MetricsRange(ctx, *expr, startValue, endValue, *step)
		if err != nil {
			return err
		}
		return a.emitWithMetadata(opts, result, withCommandMetadata(collectionMetadata("runtime metrics query", result, 0, ""), "runtime metrics query"))
	case "logs":
		if args[1] != "query" && args[1] != "aggregate" {
			return errors.New("usage: runtime logs <query|aggregate> --query QUERY [--start RFC3339] [--end RFC3339] [--limit 200]")
		}
		fs := flag.NewFlagSet("runtime logs "+args[1], flag.ContinueOnError)
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
		startValue, endValue := normalizeTimeRange(a.Now(), *start, *end, time.Hour)
		result, err := client.LogsRange(ctx, *query, startValue, endValue, *limit)
		if err != nil {
			return err
		}
		if args[1] == "aggregate" {
			summary := summarizeLogsResult(result)
			return a.emitWithMetadata(opts, summary, runtimeAggregateMetadata("runtime logs aggregate", summary["streams"]))
		}
		return a.emitWithMetadata(opts, result, collectionMetadata("runtime logs query", result, *limit, "Narrow the LogQL query or reduce the time window if you only need representative log streams"))
	case "traces":
		if args[1] != "search" && args[1] != "aggregate" {
			return errors.New("usage: runtime traces <search|aggregate> --query QUERY [--start RFC3339] [--end RFC3339] [--limit 200]")
		}
		fs := flag.NewFlagSet("runtime traces "+args[1], flag.ContinueOnError)
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
		startValue, endValue := normalizeTimeRange(a.Now(), *start, *end, time.Hour)
		result, err := client.TracesSearch(ctx, *query, startValue, endValue, *limit)
		if err != nil {
			return err
		}
		if args[1] == "aggregate" {
			summary := summarizeTracesResult(result)
			return a.emitWithMetadata(opts, summary, runtimeAggregateMetadata("runtime traces aggregate", summary["trace_matches"]))
		}
		return a.emitWithMetadata(opts, result, collectionMetadata("runtime traces search", result, *limit, "Narrow the TraceQL expression or shrink the time range for a smaller result set"))
	default:
		return fmt.Errorf("unknown runtime command: %s", args[0])
	}
}

func (a *App) runAggregate(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"aggregate"}, true)
	}
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
	startValue, endValue := normalizeTimeRange(a.Now(), *start, *end, time.Hour)

	cfg, err := a.requireAuthConfig()
	if err != nil {
		return err
	}
	result, err := a.NewClient(cfg).AggregateSnapshot(ctx, grafana.AggregateRequest{
		MetricExpr: *metricExpr,
		LogQuery:   *logQuery,
		TraceQuery: *traceQuery,
		Start:      startValue,
		End:        endValue,
		Step:       *step,
		Limit:      *limit,
	})
	if err != nil {
		return err
	}
	return a.emit(opts, result)
}

func (a *App) runIncident(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"incident"}, true)
	}
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
	datasources, datasourceErr := client.ListDatasources(ctx)

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
	req.Start, req.End = normalizeTimeRange(a.Now(), req.Start, req.End, time.Hour)

	snapshot, err := client.AggregateSnapshot(ctx, req)
	if err != nil {
		return err
	}

	result := map[string]any{
		"goal":               *goal,
		"playbook":           plan.Playbook,
		"request":            req,
		"summary":            summarizeSnapshot(snapshot),
		"generated":          a.Now().UTC(),
		"datasource_summary": datasourceInventorySummary(datasources),
		"query_hints":        datasourceQueryHints(req, plan.Playbook, datasources),
	}
	if datasourceErr != nil {
		result["warnings"] = []string{"datasource inventory failed: " + datasourceErr.Error()}
	}
	if *includeRaw {
		result["snapshot"] = snapshot
		result["datasources"] = normalizeDatasourceCollection(datasources)
	}

	return a.emit(opts, result)
}

func (a *App) runIRM(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"irm"}, true)
	}
	if len(args) < 2 || args[0] != "incidents" || args[1] != "list" {
		return errors.New("usage: irm incidents list [--query TEXT] [--limit 20] [--order-field createdAt] [--order-direction desc]")
	}

	fs := flag.NewFlagSet("irm incidents list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	query := fs.String("query", "", "incident search text")
	limit := fs.Int("limit", 20, "maximum incidents")
	orderField := fs.String("order-field", "createdAt", "incident sort field")
	orderDirection := fs.String("order-direction", "desc", "asc or desc")
	if err := fs.Parse(args[2:]); err != nil {
		return err
	}
	if *limit < 1 {
		return errors.New("--limit must be at least 1")
	}
	if *orderDirection != "asc" && *orderDirection != "desc" {
		return errors.New("--order-direction must be asc or desc")
	}

	cfg, err := a.requireAuthConfig()
	if err != nil {
		return err
	}
	body := map[string]any{
		"query": map[string]any{
			"limit":       *limit,
			"queryString": strings.TrimSpace(*query),
			"orderBy": map[string]any{
				"field":     strings.TrimSpace(*orderField),
				"direction": *orderDirection,
			},
		},
	}
	result, err := a.NewClient(cfg).Raw(ctx, "POST", "/api/plugins/grafana-irm-app/resources/api/v1/IncidentsService.QueryIncidentPreviews", body)
	if err != nil {
		return err
	}
	return a.emitWithMetadata(opts, result, collectionMetadata("irm incidents list", result, *limit, "Refine --query or lower --limit to inspect a tighter incident slice"))
}

func (a *App) runOnCall(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"oncall"}, true)
	}
	if len(args) < 2 || args[0] != "schedules" || args[1] != "list" {
		return errors.New("usage: oncall schedules list [--query TEXT] [--limit 50]")
	}

	fs := flag.NewFlagSet("oncall schedules list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	query := fs.String("query", "", "schedule name or team filter")
	limit := fs.Int("limit", 50, "maximum schedules")
	if err := fs.Parse(args[2:]); err != nil {
		return err
	}
	if *limit < 1 {
		return errors.New("--limit must be at least 1")
	}

	cfg, err := a.requireOnCallConfig()
	if err != nil {
		return err
	}
	onCallCfg := cfg
	onCallCfg.BaseURL = cfg.OnCallURL
	result, err := a.NewClient(onCallCfg).Raw(ctx, "GET", "/api/v1/schedules/", nil)
	if err != nil {
		return err
	}
	filtered, count, truncated := filterNamedPayload(result, strings.TrimSpace(*query), *limit, "name", "team.name", "team.slug", "type")
	meta := &responseMetadata{Command: "oncall schedules list"}
	meta.Count = &count
	if truncated || payloadHasNextPage(result) {
		meta.Truncated = true
		meta.NextAction = "Refine --query or lower --limit to inspect specific OnCall schedules"
	}
	return a.emitWithMetadata(opts, filtered, meta)
}

func (a *App) runAgent(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"agent"}, true)
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
		datasources, datasourceErr := client.ListDatasources(ctx)
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
			"plan":               plan,
			"request":            req,
			"summary":            summarizeSnapshot(snapshot),
			"stack_count":        inferCollectionCount(stacks),
			"executed_at":        a.Now().UTC(),
			"datasource_summary": datasourceInventorySummary(datasources),
			"query_hints":        datasourceQueryHints(req, plan.Playbook, datasources),
		}
		if datasourceErr != nil {
			result["warnings"] = []string{"datasource inventory failed: " + datasourceErr.Error()}
		}
		if *includeRaw {
			result["stacks"] = stacks
			result["snapshot"] = snapshot
			result["datasources"] = normalizeDatasourceCollection(datasources)
		}
		return a.emit(opts, result)
	default:
		return fmt.Errorf("unknown agent command: %s", args[0])
	}
}

func (a *App) runSchema(opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("schema", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	compact := fs.Bool("compact", false, "return a smaller schema")
	full := fs.Bool("full", false, "return the expanded schema")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *compact && *full {
		return errors.New("--compact and --full cannot be used together")
	}
	return a.emitHelp(opts, fs.Args(), !*full)
}

type authLoginRequest struct {
	Stack      string
	CloudToken string
	BaseURL    string
	CloudURL   string
	PromURL    string
	LogsURL    string
	TracesURL  string
	OnCallURL  string
}

type inferredStackEndpoints struct {
	PrometheusURL string
	LogsURL       string
	TracesURL     string
	OnCallURL     string
}

func authStatusPayload(contextName string, cfg config.Config) map[string]any {
	capabilities, missing := authCapabilities(cfg)
	status := "unauthenticated"
	if cfg.IsAuthenticated() {
		status = "authenticated"
	}
	return map[string]any{
		"context":        contextName,
		"status":         status,
		"authenticated":  cfg.IsAuthenticated(),
		"base_url":       cfg.BaseURL,
		"cloud_url":      cfg.CloudURL,
		"prometheus_url": cfg.PrometheusURL,
		"logs_url":       cfg.LogsURL,
		"traces_url":     cfg.TracesURL,
		"oncall_url":     cfg.OnCallURL,
		"token_backend":  cfg.TokenBackend,
		"org_id":         cfg.OrgID,
		"missing":        missing,
		"capabilities":   capabilities,
	}
}

func authCapabilities(cfg config.Config) ([]map[string]any, []string) {
	capabilities := []map[string]any{
		{"name": "api", "ok": cfg.IsAuthenticated() && strings.TrimSpace(cfg.BaseURL) != "", "requires": []string{"token", "base_url"}},
		{"name": "cloud", "ok": cfg.IsAuthenticated() && strings.TrimSpace(cfg.CloudURL) != "", "requires": []string{"token", "cloud_url"}},
		{"name": "query_history", "ok": cfg.IsAuthenticated() && strings.TrimSpace(cfg.BaseURL) != "", "requires": []string{"token", "base_url"}},
		{"name": "slo", "ok": cfg.IsAuthenticated() && strings.TrimSpace(cfg.BaseURL) != "", "requires": []string{"token", "base_url"}},
		{"name": "irm", "ok": cfg.IsAuthenticated() && strings.TrimSpace(cfg.BaseURL) != "", "requires": []string{"token", "base_url"}},
		{"name": "oncall", "ok": cfg.IsAuthenticated() && strings.TrimSpace(cfg.OnCallURL) != "", "requires": []string{"token", "oncall_url"}},
		{"name": "runtime_metrics", "ok": cfg.IsAuthenticated() && strings.TrimSpace(cfg.PrometheusURL) != "", "requires": []string{"token", "prometheus_url"}},
		{"name": "runtime_logs", "ok": cfg.IsAuthenticated() && strings.TrimSpace(cfg.LogsURL) != "", "requires": []string{"token", "logs_url"}},
		{"name": "runtime_traces", "ok": cfg.IsAuthenticated() && strings.TrimSpace(cfg.TracesURL) != "", "requires": []string{"token", "traces_url"}},
	}
	missing := make([]string, 0, 6)
	if !cfg.IsAuthenticated() {
		missing = append(missing, "token")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		missing = append(missing, "base_url")
	}
	if strings.TrimSpace(cfg.CloudURL) == "" {
		missing = append(missing, "cloud_url")
	}
	if strings.TrimSpace(cfg.PrometheusURL) == "" {
		missing = append(missing, "prometheus_url")
	}
	if strings.TrimSpace(cfg.LogsURL) == "" {
		missing = append(missing, "logs_url")
	}
	if strings.TrimSpace(cfg.TracesURL) == "" {
		missing = append(missing, "traces_url")
	}
	if strings.TrimSpace(cfg.OnCallURL) == "" {
		missing = append(missing, "oncall_url")
	}
	return capabilities, missing
}

func authDoctorPayload(contextName string, cfg config.Config) map[string]any {
	payload := authStatusPayload(contextName, cfg)
	missing, _ := payload["missing"].([]string)
	suggestions := make([]string, 0, 2)
	if !cfg.IsAuthenticated() {
		suggestions = append(suggestions, `Run grafana auth login --token "$GRAFANA_TOKEN" --stack "<stack-slug>"`)
	}
	if len(missing) > 0 {
		suggestions = append(suggestions, "Resolve missing endpoints with grafana auth login --stack <stack-slug> or set them manually with grafana config set")
	}
	payload["suggestions"] = suggestions
	return payload
}

func (a *App) resolveAuthEndpoints(ctx context.Context, cfg *config.Config, req authLoginRequest) ([]string, error) {
	req.BaseURL = strings.TrimSpace(req.BaseURL)
	req.CloudURL = strings.TrimSpace(req.CloudURL)
	req.PromURL = strings.TrimSpace(req.PromURL)
	req.LogsURL = strings.TrimSpace(req.LogsURL)
	req.TracesURL = strings.TrimSpace(req.TracesURL)
	req.OnCallURL = strings.TrimSpace(req.OnCallURL)
	req.Stack = strings.TrimSpace(req.Stack)
	req.CloudToken = strings.TrimSpace(req.CloudToken)

	warnings := make([]string, 0, 2)
	if req.Stack != "" {
		stackSlug, inferredBaseURL, err := normalizeStackIdentifier(req.Stack)
		if err != nil {
			return nil, err
		}
		if req.BaseURL == "" {
			cfg.BaseURL = inferredBaseURL
		}
		if req.CloudURL == "" && strings.TrimSpace(cfg.CloudURL) == "" {
			defaults := config.Config{}
			defaults.ApplyDefaults()
			cfg.CloudURL = defaults.CloudURL
		}
		if req.PromURL == "" {
			cfg.PrometheusURL = ""
		}
		if req.LogsURL == "" {
			cfg.LogsURL = ""
		}
		if req.TracesURL == "" {
			cfg.TracesURL = ""
		}
		if req.OnCallURL == "" {
			cfg.OnCallURL = ""
		}
		discoveryWarnings := a.applyInferredStackEndpoints(ctx, cfg, stackSlug, req.CloudToken)
		warnings = append(warnings, discoveryWarnings...)
	}
	if req.BaseURL != "" {
		cfg.BaseURL = req.BaseURL
	}
	if req.CloudURL != "" {
		cfg.CloudURL = req.CloudURL
	}
	if req.PromURL != "" {
		cfg.PrometheusURL = req.PromURL
	}
	if req.LogsURL != "" {
		cfg.LogsURL = req.LogsURL
	}
	if req.TracesURL != "" {
		cfg.TracesURL = req.TracesURL
	}
	if req.OnCallURL != "" {
		cfg.OnCallURL = req.OnCallURL
	}
	return warnings, nil
}

func inferOnCallURL(payload any) string {
	if root, ok := payload.(map[string]any); ok {
		if urlValue := firstNonEmptyString(root, "oncallApiUrl"); urlValue != "" {
			return urlValue
		}
	}
	items, _, ok := collectionPayload(payload)
	if !ok {
		if nested := collectionAtPath(payload, "connections"); len(nested) > 0 {
			items = nested
			ok = true
		}
	}
	if !ok {
		return ""
	}
	for _, item := range items {
		record, ok := item.(map[string]any)
		if !ok {
			continue
		}
		hint := strings.ToLower(firstNonEmptyString(record, "type", "name", "kind"))
		if !containsAny(hint, "oncall") {
			continue
		}
		if urlValue := recursiveStringValue(record, "oncallApiUrl"); urlValue != "" {
			return urlValue
		}
		if urlValue := firstNonEmptyString(record, "url", "apiUrl", "api_url"); urlValue != "" {
			return urlValue
		}
	}
	return ""
}

func recursiveStringValue(payload any, key string) string {
	switch typed := payload.(type) {
	case map[string]any:
		if value := firstNonEmptyString(typed, key); value != "" {
			return value
		}
		for _, child := range typed {
			if value := recursiveStringValue(child, key); value != "" {
				return value
			}
		}
	case []any:
		for _, child := range typed {
			if value := recursiveStringValue(child, key); value != "" {
				return value
			}
		}
	}
	return ""
}

func containsAny(value string, parts ...string) bool {
	for _, part := range parts {
		if strings.Contains(value, part) {
			return true
		}
	}
	return false
}

func payloadHasNextPage(payload any) bool {
	root, ok := payload.(map[string]any)
	if !ok {
		return false
	}
	if strings.TrimSpace(firstNonEmptyString(root, "next", "nextPage")) != "" {
		return true
	}
	metadata := mapValue(root, "metadata")
	pagination := mapValue(metadata, "pagination")
	return strings.TrimSpace(firstNonEmptyString(pagination, "next", "nextPage")) != ""
}

func enforceConfirmation(args []string) error {
	for _, arg := range args {
		if isHelpArg(arg) {
			return nil
		}
	}
	if len(args) == 0 {
		return nil
	}
	if args[0] == "api" {
		if len(args) < 2 {
			return nil
		}
		method := strings.ToUpper(args[1])
		switch method {
		case "GET", "HEAD", "OPTIONS":
			return nil
		default:
			path := ""
			if len(args) > 2 {
				path = " " + args[2]
			}
			return fmt.Errorf("requires --yes: api %s%s mutates remote state", method, path)
		}
	}
	switch strings.Join(discoveryPathFromArgs(args), " ") {
	case "auth logout":
		return errors.New("requires --yes: auth logout clears stored credentials")
	case "dashboards delete":
		return errors.New("requires --yes: dashboards delete removes a dashboard")
	case "dashboards permissions update":
		return errors.New("requires --yes: dashboards permissions update replaces dashboard permissions")
	case "folders permissions update":
		return errors.New("requires --yes: folders permissions update replaces folder permissions")
	default:
		return nil
	}
}

func enforceReadOnly(args []string) error {
	for _, arg := range args {
		if isHelpArg(arg) {
			return nil
		}
	}
	if len(args) == 0 {
		return nil
	}
	if args[0] == "api" {
		if len(args) < 2 {
			return nil
		}
		switch strings.ToUpper(args[1]) {
		case "GET", "HEAD", "OPTIONS":
			return nil
		default:
			return errors.New("blocked by --read-only: api write methods are disabled")
		}
	}
	path := discoveryPathFromArgs(args)
	if len(path) == 0 {
		return nil
	}
	command, ok := findDiscoveryCommand(discoveryCatalog(), path)
	if ok && !command.ReadOnly {
		return fmt.Errorf("blocked by --read-only: %s mutates state", strings.Join(path, " "))
	}
	return nil
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

func (a *App) requireOnCallConfig() (config.Config, error) {
	cfg, err := a.requireAuthConfig()
	if err != nil {
		return config.Config{}, err
	}
	if strings.TrimSpace(cfg.OnCallURL) == "" {
		return config.Config{}, errors.New("oncall URL is not configured: run `grafana auth login --stack ...` or `grafana config set oncall-url ...`")
	}
	return cfg, nil
}

func parseGlobalOptions(args []string) (globalOptions, []string, error) {
	opts := globalOptions{Output: "json", Agent: isTruthyEnv("FORCE_AGENT_MODE") || isTruthyEnv("GRAFANA_CLI_AGENT_MODE")}
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
		case arg == "--json":
			if i+1 >= len(args) {
				return globalOptions{}, nil, errors.New("--json requires a value")
			}
			opts.Fields = splitCSV(args[i+1])
			opts.Output = "json"
			i++
		case strings.HasPrefix(arg, "--json="):
			opts.Fields = splitCSV(strings.TrimPrefix(arg, "--json="))
			opts.Output = "json"
		case arg == "--jq":
			if i+1 >= len(args) {
				return globalOptions{}, nil, errors.New("--jq requires a value")
			}
			opts.JQ = args[i+1]
			i++
		case strings.HasPrefix(arg, "--jq="):
			opts.JQ = strings.TrimPrefix(arg, "--jq=")
		case arg == "--template":
			if i+1 >= len(args) {
				return globalOptions{}, nil, errors.New("--template requires a value")
			}
			opts.Template = args[i+1]
			i++
		case strings.HasPrefix(arg, "--template="):
			opts.Template = strings.TrimPrefix(arg, "--template=")
		case arg == "--agent":
			opts.Agent = true
		case arg == "--read-only":
			opts.ReadOnly = true
		case arg == "--yes":
			opts.Yes = true
		default:
			rest = append(rest, arg)
		}
	}

	if opts.Output != "json" && opts.Output != "pretty" && opts.Output != "table" {
		return globalOptions{}, nil, fmt.Errorf("invalid --output value: %s", opts.Output)
	}
	if opts.JQ != "" && opts.Template != "" {
		return globalOptions{}, nil, errors.New("--jq and --template cannot be used together")
	}
	return opts, rest, nil
}

func (a *App) emit(opts globalOptions, payload any) error {
	return a.emitWithMetadata(opts, payload, nil)
}

func (a *App) emitWithMetadata(opts globalOptions, payload any, meta *responseMetadata) error {
	payload = projectFields(payload, opts.Fields)
	if opts.JQ != "" {
		value, err := applyJQ(payload, opts.JQ)
		if err != nil {
			return err
		}
		payload = value
	}
	if opts.Template != "" {
		return renderTemplate(a.Out, payload, opts.Template)
	}
	if opts.Agent {
		if meta == nil {
			meta = collectionMetadata("", payload, 0, "")
		} else if meta.Count == nil {
			if count, ok := inferMetadataCount(payload); ok {
				meta.Count = &count
			}
		}
		enc := json.NewEncoder(a.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(responseEnvelope{
			Status:   "success",
			Data:     payload,
			Metadata: meta,
		})
	}
	if isScalar(payload) {
		_, err := fmt.Fprintln(a.Out, scalarString(payload))
		return err
	}
	if opts.Output == "table" {
		return renderTable(a.Out, payload)
	}
	enc := json.NewEncoder(a.Out)
	if opts.Output == "pretty" {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(payload)
}

func (a *App) emitHelp(opts globalOptions, path []string, compact bool) error {
	payload, err := buildDiscoveryPayload(path, compact)
	if err != nil {
		return err
	}
	return a.emit(opts, payload)
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

func summarizeLogsResult(payload any) map[string]any {
	streams := collectionAtPath(payload, "data", "result")
	labelKeys := map[string]struct{}{}
	topStreams := make([]any, 0, minInt(len(streams), 5))
	entryCount := 0
	validStreams := 0
	for _, stream := range streams {
		record, ok := stream.(map[string]any)
		if !ok {
			continue
		}
		validStreams++
		labels, _ := record["stream"].(map[string]any)
		values, _ := record["values"].([]any)
		entryCount += len(values)
		for key := range labels {
			labelKeys[key] = struct{}{}
		}
		if len(topStreams) < 5 {
			topStreams = append(topStreams, map[string]any{
				"labels":  labels,
				"entries": len(values),
			})
		}
	}
	sortedLabels := make([]string, 0, len(labelKeys))
	for key := range labelKeys {
		sortedLabels = append(sortedLabels, key)
	}
	sort.Strings(sortedLabels)
	return map[string]any{
		"streams":     validStreams,
		"entries":     entryCount,
		"label_keys":  sortedLabels,
		"top_streams": topStreams,
	}
}

func summarizeTracesResult(payload any) map[string]any {
	traces := collectionAtPath(payload, "traces")
	if len(traces) == 0 {
		traces = collectionAtPath(payload, "data", "traces")
	}
	services := map[string]struct{}{}
	operations := map[string]struct{}{}
	sampleTraceIDs := make([]string, 0, minInt(len(traces), 5))
	validTraces := 0
	for _, trace := range traces {
		record, ok := trace.(map[string]any)
		if !ok {
			continue
		}
		validTraces++
		if service := firstNonEmptyString(record, "rootServiceName", "serviceName", "service"); service != "" {
			services[service] = struct{}{}
		}
		if operation := firstNonEmptyString(record, "rootTraceName", "name", "traceName"); operation != "" {
			operations[operation] = struct{}{}
		}
		if len(sampleTraceIDs) < 5 {
			if traceID := firstNonEmptyString(record, "traceID", "traceId", "id"); traceID != "" {
				sampleTraceIDs = append(sampleTraceIDs, traceID)
			}
		}
	}
	return map[string]any{
		"trace_matches":    validTraces,
		"services":         sortedSet(services),
		"root_operations":  sortedSet(operations),
		"sample_trace_ids": sampleTraceIDs,
	}
}

func runtimeAggregateMetadata(command string, countValue any) *responseMetadata {
	meta := &responseMetadata{Command: command}
	if count, ok := intValue(countValue); ok {
		meta.Count = &count
	}
	return meta
}

func inferCollectionCount(payload any) int {
	switch v := payload.(type) {
	case []any:
		return len(v)
	case map[string]any:
		for _, key := range collectionKeys {
			candidate, ok := v[key]
			if !ok {
				continue
			}
			if count := inferCollectionCount(candidate); count > 0 {
				return count
			}
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func isHelpArg(value string) bool {
	switch strings.TrimSpace(value) {
	case "help", "--help", "-h", "-help":
		return true
	default:
		return false
	}
}

func selectedContextName(contexts config.ContextStore, requested string) string {
	requested = strings.TrimSpace(requested)
	if requested != "" {
		return requested
	}
	if contexts == nil {
		return defaultContextNameForCLI()
	}
	name, err := contexts.CurrentContext()
	if err != nil || strings.TrimSpace(name) == "" {
		return defaultContextNameForCLI()
	}
	return name
}

func defaultContextNameForCLI() string {
	return "default"
}

func (a *App) loadConfigForContext(name string) (config.Config, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		cfg, err := a.Store.Load()
		if err != nil {
			return config.Config{}, "", err
		}
		return cfg, selectedContextName(a.Contexts, ""), nil
	}
	if a.Contexts == nil {
		return config.Config{}, "", errors.New("context support is unavailable")
	}
	cfg, err := a.Contexts.LoadContext(name)
	if err != nil {
		return config.Config{}, "", err
	}
	return cfg, name, nil
}

func configPayload(contextName string, cfg config.Config) map[string]any {
	return map[string]any{
		"context":        contextName,
		"base_url":       cfg.BaseURL,
		"cloud_url":      cfg.CloudURL,
		"prometheus_url": cfg.PrometheusURL,
		"logs_url":       cfg.LogsURL,
		"traces_url":     cfg.TracesURL,
		"oncall_url":     cfg.OnCallURL,
		"org_id":         cfg.OrgID,
		"token_backend":  cfg.TokenBackend,
	}
}

func extractContextArg(args []string) (string, []string, error) {
	contextName := ""
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		switch {
		case arg == "--context":
			if i+1 >= len(args) {
				return "", nil, errors.New("--context requires a value")
			}
			contextName = args[i+1]
			i++
		case strings.HasPrefix(arg, "--context="):
			contextName = strings.TrimPrefix(arg, "--context=")
		default:
			rest = append(rest, args[i])
		}
	}
	return contextName, rest, nil
}

func normalizeConfigKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

func configValueForKey(cfg config.Config, key string) (any, error) {
	switch normalizeConfigKey(key) {
	case "base-url", "base_url":
		return cfg.BaseURL, nil
	case "cloud-url", "cloud_url":
		return cfg.CloudURL, nil
	case "prom-url", "prom_url", "prometheus-url", "prometheus_url":
		return cfg.PrometheusURL, nil
	case "logs-url", "logs_url":
		return cfg.LogsURL, nil
	case "traces-url", "traces_url":
		return cfg.TracesURL, nil
	case "oncall-url", "oncall_url":
		return cfg.OnCallURL, nil
	case "org-id", "org_id":
		return cfg.OrgID, nil
	case "token-backend", "token_backend":
		return cfg.TokenBackend, nil
	default:
		return nil, errors.New("unknown config key")
	}
}

func setConfigValue(cfg *config.Config, key, value string) error {
	switch normalizeConfigKey(key) {
	case "base-url", "base_url":
		cfg.BaseURL = strings.TrimSpace(value)
	case "cloud-url", "cloud_url":
		cfg.CloudURL = strings.TrimSpace(value)
	case "prom-url", "prom_url", "prometheus-url", "prometheus_url":
		cfg.PrometheusURL = strings.TrimSpace(value)
	case "logs-url", "logs_url":
		cfg.LogsURL = strings.TrimSpace(value)
	case "traces-url", "traces_url":
		cfg.TracesURL = strings.TrimSpace(value)
	case "oncall-url", "oncall_url":
		cfg.OnCallURL = strings.TrimSpace(value)
	case "org-id", "org_id":
		parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil || parsed < 0 {
			return errors.New("invalid org-id")
		}
		cfg.OrgID = parsed
	default:
		return errors.New("unknown config key")
	}
	return nil
}

func applyJQ(payload any, expr string) (any, error) {
	query, err := gojq.Parse(expr)
	if err != nil {
		return nil, err
	}
	iter := query.Run(payload)
	results := make([]any, 0, 1)
	for {
		value, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := value.(error); ok {
			return nil, err
		}
		results = append(results, value)
	}
	switch len(results) {
	case 0:
		return nil, nil
	case 1:
		return results[0], nil
	default:
		return results, nil
	}
}

func renderTemplate(out io.Writer, payload any, text string) error {
	tmpl, err := template.New("output").Funcs(template.FuncMap{
		"json": func(v any) (string, error) {
			data, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			return string(data), nil
		},
	}).Parse(text)
	if err != nil {
		return err
	}
	buf := &bytes.Buffer{}
	if err := tmpl.Execute(buf, payload); err != nil {
		return err
	}
	if !bytes.HasSuffix(buf.Bytes(), []byte("\n")) {
		buf.WriteByte('\n')
	}
	_, err = out.Write(buf.Bytes())
	return err
}

func isScalar(value any) bool {
	switch value.(type) {
	case nil, string, bool, int, int64, float64, float32, json.Number:
		return true
	default:
		return false
	}
}

func scalarString(value any) string {
	if value == nil {
		return "null"
	}
	return fmt.Sprint(value)
}

func isTruthyEnv(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func parseInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func appendQuery(path string, query url.Values) string {
	encoded := query.Encode()
	if strings.TrimSpace(encoded) == "" {
		return path
	}
	return path + "?" + encoded
}

func buildDashboardSharePath(uid, slug string, panelID int64, from, to, theme string, orgID int64) string {
	trimmedUID := strings.TrimSpace(uid)
	trimmedSlug := strings.TrimSpace(slug)
	if trimmedSlug == "" {
		trimmedSlug = "share"
	}

	path := "/d/" + url.PathEscape(trimmedUID) + "/" + url.PathEscape(trimmedSlug)
	query := url.Values{}
	if panelID > 0 {
		path = "/d-solo/" + url.PathEscape(trimmedUID) + "/" + url.PathEscape(trimmedSlug)
		query.Set("panelId", strconv.FormatInt(panelID, 10))
	}
	if strings.TrimSpace(from) != "" {
		query.Set("from", from)
	}
	if strings.TrimSpace(to) != "" {
		query.Set("to", to)
	}
	if strings.TrimSpace(theme) != "" {
		query.Set("theme", theme)
	}
	if orgID > 0 {
		query.Set("orgId", strconv.FormatInt(orgID, 10))
	}
	return appendQuery(path, query)
}

func resolveDashboardShareOrgID(ctx context.Context, client APIClient, cfg config.Config, explicitOrgID int64) (int64, error) {
	if explicitOrgID > 0 {
		return explicitOrgID, nil
	}
	if cfg.OrgID > 0 {
		return cfg.OrgID, nil
	}

	payload, err := client.Raw(ctx, http.MethodGet, "/api/org", nil)
	if err != nil {
		return 0, fmt.Errorf("org ID is not configured and current org lookup failed: %w", err)
	}
	orgID, ok := intPath(payload, "id")
	if !ok || orgID <= 0 {
		return 0, errors.New("org ID is not configured and current org lookup did not return a valid id; pass --org-id or run `grafana config set org-id <id>`")
	}
	return int64(orgID), nil
}

func parsePermissionItemsJSON(value string) ([]grafana.PermissionUpdateItem, error) {
	if strings.TrimSpace(value) == "" {
		return nil, errors.New("--items-json is required")
	}

	var items []grafana.PermissionUpdateItem
	if err := json.Unmarshal([]byte(value), &items); err != nil {
		return nil, fmt.Errorf("invalid --items-json: %w", err)
	}

	for i, item := range items {
		subjectCount := 0
		if strings.TrimSpace(item.Role) != "" {
			subjectCount++
		}
		if item.TeamID > 0 {
			subjectCount++
		}
		if item.UserID > 0 {
			subjectCount++
		}
		if subjectCount == 0 {
			return nil, fmt.Errorf("invalid --items-json: item %d must include exactly one of role, teamId, or userId", i)
		}
		if subjectCount > 1 {
			return nil, fmt.Errorf("invalid --items-json: item %d cannot include more than one of role, teamId, or userId", i)
		}
		switch item.Permission {
		case 1, 2, 4:
		default:
			return nil, fmt.Errorf("invalid --items-json: item %d permission must be one of 1, 2, or 4", i)
		}
	}

	return items, nil
}

func resolveShortURLAbsolute(baseURL, shortURL string) (string, bool) {
	parsedShortURL, err := url.Parse(strings.TrimSpace(shortURL))
	if err != nil {
		return "", false
	}
	if parsedShortURL.Scheme != "" && parsedShortURL.Host != "" {
		return parsedShortURL.String(), true
	}

	parsedBaseURL, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsedBaseURL.Scheme == "" || parsedBaseURL.Host == "" {
		return "", false
	}
	parsedBaseURL.Path = ""
	parsedBaseURL.RawPath = ""
	parsedBaseURL.RawQuery = ""
	parsedBaseURL.Fragment = ""
	return parsedBaseURL.ResolveReference(parsedShortURL).String(), true
}

func normalizeQueryHistoryBound(now time.Time, value string) string {
	normalized := normalizeTimeValue(now, value)
	if parsed, err := time.Parse(time.RFC3339, normalized); err == nil {
		return strconv.FormatInt(parsed.UnixMilli(), 10)
	}
	return normalized
}

func queryHistoryMetadata(payload any, limit int) *responseMetadata {
	meta := &responseMetadata{Command: "query-history list"}
	if count := countPath(payload, "result", "queryHistory"); count > 0 {
		meta.Count = &count
	}
	if totalCount, ok := intPath(payload, "result", "totalCount"); ok && meta.Count != nil && totalCount > *meta.Count {
		meta.Truncated = true
		meta.NextAction = "Use --page or raise --limit to inspect more query-history entries"
	}
	if meta.Count == nil && limit > 0 {
		meta.Warnings = []string{"query-history response did not expose a countable result set"}
	}
	return meta
}

func filterNamedPayload(payload any, query string, limit int, fields ...string) (any, int, bool) {
	items, wrap, ok := collectionPayload(payload)
	if !ok {
		return payload, inferCollectionCount(payload), false
	}
	filtered := items
	if strings.TrimSpace(query) != "" {
		filtered = make([]any, 0, len(items))
		for _, item := range items {
			record, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if matchesAnyField(record, query, fields...) {
				filtered = append(filtered, record)
			}
		}
	}
	count := len(filtered)
	truncated := false
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
		truncated = true
	}
	return wrap(filtered), count, truncated
}

func collectionPayload(payload any) ([]any, func([]any) any, bool) {
	switch typed := payload.(type) {
	case []any:
		return typed, func(items []any) any { return items }, true
	case map[string]any:
		for _, key := range []string{"items", "results", "slos"} {
			items, ok := typed[key].([]any)
			if !ok {
				continue
			}
			return items, func(filtered []any) any {
				cloned := map[string]any{}
				for cloneKey, cloneValue := range typed {
					cloned[cloneKey] = cloneValue
				}
				cloned[key] = filtered
				return cloned
			}, true
		}
	}
	return nil, nil, false
}

func matchesAnyField(record map[string]any, query string, fields ...string) bool {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return true
	}
	for _, field := range fields {
		value, ok := lookupPath(record, strings.Split(field, "."))
		if !ok {
			continue
		}
		if strings.Contains(strings.ToLower(fmt.Sprint(value)), needle) {
			return true
		}
	}
	return false
}

func collectionAtPath(payload any, path ...string) []any {
	root, ok := payload.(map[string]any)
	if !ok {
		return nil
	}
	value, ok := lookupPath(root, path)
	if !ok {
		return nil
	}
	items, _ := value.([]any)
	return items
}

func intPath(payload any, path ...string) (int, bool) {
	root, ok := payload.(map[string]any)
	if !ok {
		return 0, false
	}
	value, ok := lookupPath(root, path)
	if !ok {
		return 0, false
	}
	return intValue(value)
}

func intValue(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		n, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return int(n), true
	default:
		return 0, false
	}
}

func firstNonEmptyString(record map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := record[key]
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(fmt.Sprint(value))
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func sortedSet(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
