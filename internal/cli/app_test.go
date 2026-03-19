package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/matiasvillaverde/grafana-cli/internal/config"
	"github.com/matiasvillaverde/grafana-cli/internal/grafana"
)

type fakeStore struct {
	cfg      config.Config
	loadErr  error
	saveErr  error
	clearErr error
}

type failWriter struct {
	failAfter int
	writes    int
}

func (f *failWriter) Write(_ []byte) (int, error) {
	if f.writes >= f.failAfter {
		return 0, errors.New("write failure")
	}
	f.writes++
	return 1, nil
}

func (f *fakeStore) Load() (config.Config, error) {
	if f.loadErr != nil {
		return config.Config{}, f.loadErr
	}
	return f.cfg, nil
}

func (f *fakeStore) Save(cfg config.Config) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.cfg = cfg
	return nil
}

func (f *fakeStore) Clear() error {
	if f.clearErr != nil {
		return f.clearErr
	}
	f.cfg = config.Config{}
	return nil
}

func (f *fakeStore) Path() string {
	return "/tmp/config.json"
}

type fakeContextStore struct {
	cfgs        map[string]config.Config
	current     string
	loadErr     error
	saveErr     error
	clearErr    error
	listErr     error
	currentErr  error
	useErr      error
	loadCtxErr  error
	saveCtxErr  error
	currentPath string
}

func (f *fakeContextStore) Load() (config.Config, error) {
	if f.loadErr != nil {
		return config.Config{}, f.loadErr
	}
	name := f.current
	if name == "" {
		name = "default"
	}
	cfg := f.cfgs[name]
	cfg.ApplyDefaults()
	return cfg, nil
}

func (f *fakeContextStore) Save(cfg config.Config) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	name := f.current
	if name == "" {
		name = "default"
	}
	if f.cfgs == nil {
		f.cfgs = map[string]config.Config{}
	}
	f.cfgs[name] = cfg
	return nil
}

func (f *fakeContextStore) Clear() error {
	if f.clearErr != nil {
		return f.clearErr
	}
	name := f.current
	if name == "" {
		name = "default"
	}
	delete(f.cfgs, name)
	return nil
}

func (f *fakeContextStore) Path() string {
	if f.currentPath != "" {
		return f.currentPath
	}
	return "/tmp/config.json"
}

func (f *fakeContextStore) CurrentContext() (string, error) {
	if f.currentErr != nil {
		return "", f.currentErr
	}
	if f.current == "" {
		return "default", nil
	}
	return f.current, nil
}

func (f *fakeContextStore) ListContexts() ([]config.ContextSummary, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	if len(f.cfgs) == 0 {
		return []config.ContextSummary{{Name: "default", Current: true}}, nil
	}
	items := make([]config.ContextSummary, 0, len(f.cfgs))
	current, _ := f.CurrentContext()
	for name, cfg := range f.cfgs {
		items = append(items, config.ContextSummary{
			Name:          name,
			Current:       name == current,
			Authenticated: cfg.IsAuthenticated(),
			BaseURL:       cfg.BaseURL,
			CloudURL:      cfg.CloudURL,
		})
	}
	return items, nil
}

func (f *fakeContextStore) UseContext(name string) error {
	if f.useErr != nil {
		return f.useErr
	}
	if _, ok := f.cfgs[name]; !ok && name != "default" {
		return errors.New("context not found")
	}
	f.current = name
	return nil
}

func (f *fakeContextStore) LoadContext(name string) (config.Config, error) {
	if f.loadCtxErr != nil {
		return config.Config{}, f.loadCtxErr
	}
	cfg := f.cfgs[name]
	cfg.ApplyDefaults()
	return cfg, nil
}

func (f *fakeContextStore) SaveContext(name string, cfg config.Config) error {
	if f.saveCtxErr != nil {
		return f.saveCtxErr
	}
	if f.cfgs == nil {
		f.cfgs = map[string]config.Config{}
	}
	f.cfgs[name] = cfg
	f.current = name
	return nil
}

type fakeClient struct {
	rawResult            any
	rawErr               error
	rawResponses         map[string]any
	rawErrors            map[string]error
	rawMethod            string
	rawPath              string
	rawBody              any
	rawCalls             []string
	createShortURLResp   any
	createShortURLErr    error
	createShortURLReq    grafana.ShortURLRequest
	cloudResult          any
	cloudErr             error
	cloudStackSlug       string
	cloudStackDSResult   any
	cloudStackDSErr      error
	cloudStackConn       any
	cloudStackConnErr    error
	cloudStackPlugins    any
	cloudStackPluginsErr error
	cloudStackPluginReq  grafana.CloudStackPluginListRequest
	cloudStackPluginPage map[string]any
	cloudStackPluginID   string
	cloudStackPlugin     any
	cloudStackPluginErr  error
	cloudBillingReq      grafana.CloudBilledUsageRequest
	cloudBillingResp     any
	cloudBillingErr      error
	cloudAccessResult    any
	cloudAccessPages     map[string]any
	cloudAccessErr       error
	cloudAccessReq       grafana.CloudAccessPolicyListRequest
	cloudAccessOne       any
	cloudAccessOneErr    error
	cloudAccessID        string
	cloudAccessRegion    string
	searchDashResult     any
	searchDashErr        error
	getDashResult        any
	getDashErr           error
	createDashResult     any
	createDashErr        error
	deleteDashResult     any
	deleteDashErr        error
	dashVersionsResult   any
	dashVersionsErr      error
	renderDashboardResp  grafana.RenderedDashboard
	renderDashboardErr   error
	renderDashboardReq   grafana.DashboardRenderRequest
	listDSResult         any
	listDSErr            error
	getDSResult          any
	getDSErr             error
	getDSUID             string
	dsHealthResult       any
	dsHealthErr          error
	dsHealthUID          string
	dsResourceResult     any
	dsResourceErr        error
	dsResourceMethod     string
	dsResourceUID        string
	dsResourcePath       string
	dsResourceBody       any
	dsQueryResult        any
	dsQueryErr           error
	dsQueryReq           grafana.DatasourceQueryRequest
	listFoldersResult    any
	listFoldersErr       error
	getFolderResult      any
	getFolderErr         error
	serviceAccountsResp  any
	serviceAccountsErr   error
	serviceAccountsReq   grafana.ServiceAccountListRequest
	serviceAccountResp   any
	serviceAccountErr    error
	serviceAccountID     int64
	annotationsResult    any
	annotationsErr       error
	annotationsReq       grafana.AnnotationListRequest
	alertRulesResult     any
	alertRulesErr        error
	alertContactResult   any
	alertContactErr      error
	alertPoliciesResult  any
	alertPoliciesErr     error
	assistantChatResult  any
	assistantChatErr     error
	assistantStatusResp  any
	assistantStatusErr   error
	assistantSkillsResp  any
	assistantSkillsErr   error
	assistantPrompt      string
	assistantChatID      string
	assistantStatusID    string
	metricsResult        any
	metricsErr           error
	metricsExpr          string
	metricsStart         string
	metricsEnd           string
	metricsStep          string
	logsResult           any
	logsErr              error
	logsQuery            string
	logsStart            string
	logsEnd              string
	logsLimit            int
	tracesResult         any
	tracesErr            error
	tracesQuery          string
	tracesStart          string
	tracesEnd            string
	tracesLimit          int
	syntheticChecksResp  any
	syntheticChecksErr   error
	syntheticChecksReq   grafana.SyntheticCheckListRequest
	syntheticCheckResp   any
	syntheticCheckErr    error
	syntheticCheckReq    grafana.SyntheticCheckGetRequest
	aggregateResult      grafana.AggregateSnapshot
	aggregateErr         error
	aggregateReq         grafana.AggregateRequest
	createDashboardArg   map[string]any
	createFolderID       int64
	createOverwrite      bool
}

func (f *fakeClient) Raw(_ context.Context, method, path string, body any) (any, error) {
	f.rawMethod = method
	f.rawPath = path
	f.rawBody = body
	f.rawCalls = append(f.rawCalls, method+" "+path)
	if err, ok := f.rawErrors[path]; ok {
		return nil, err
	}
	if result, ok := f.rawResponses[path]; ok {
		return result, nil
	}
	return f.rawResult, f.rawErr
}

func (f *fakeClient) CreateShortURL(_ context.Context, req grafana.ShortURLRequest) (any, error) {
	f.createShortURLReq = req
	if f.createShortURLErr != nil {
		return nil, f.createShortURLErr
	}
	return f.createShortURLResp, nil
}

func (f *fakeClient) CloudStacks(_ context.Context) (any, error) {
	return f.cloudResult, f.cloudErr
}

func (f *fakeClient) CloudStackDatasources(_ context.Context, stack string) (any, error) {
	f.cloudStackSlug = stack
	return f.cloudStackDSResult, f.cloudStackDSErr
}

func (f *fakeClient) CloudStackConnections(_ context.Context, stack string) (any, error) {
	f.cloudStackSlug = stack
	return f.cloudStackConn, f.cloudStackConnErr
}

func (f *fakeClient) CloudStackPlugins(_ context.Context, stack string) (any, error) {
	f.cloudStackPluginReq = grafana.CloudStackPluginListRequest{Stack: stack}
	f.cloudStackSlug = stack
	return f.cloudStackPlugins, f.cloudStackPluginsErr
}

func (f *fakeClient) CloudStackPluginsPage(_ context.Context, req grafana.CloudStackPluginListRequest) (any, error) {
	f.cloudStackPluginReq = req
	f.cloudStackSlug = req.Stack
	if f.cloudStackPluginPage != nil {
		if result, ok := f.cloudStackPluginPage[req.PageCursor]; ok {
			return result, f.cloudStackPluginsErr
		}
	}
	return f.cloudStackPlugins, f.cloudStackPluginsErr
}

func (f *fakeClient) CloudStackPlugin(_ context.Context, stack, plugin string) (any, error) {
	f.cloudStackSlug = stack
	f.cloudStackPluginID = plugin
	return f.cloudStackPlugin, f.cloudStackPluginErr
}

func (f *fakeClient) CloudBilledUsage(_ context.Context, req grafana.CloudBilledUsageRequest) (any, error) {
	f.cloudBillingReq = req
	return f.cloudBillingResp, f.cloudBillingErr
}

func (f *fakeClient) CloudAccessPolicies(_ context.Context, req grafana.CloudAccessPolicyListRequest) (any, error) {
	f.cloudAccessReq = req
	if f.cloudAccessPages != nil {
		if result, ok := f.cloudAccessPages[req.PageCursor]; ok {
			return result, f.cloudAccessErr
		}
	}
	return f.cloudAccessResult, f.cloudAccessErr
}

func (f *fakeClient) CloudAccessPolicy(_ context.Context, id, region string) (any, error) {
	f.cloudAccessID = id
	f.cloudAccessRegion = region
	return f.cloudAccessOne, f.cloudAccessOneErr
}

func (f *fakeClient) SearchDashboards(_ context.Context, _, _ string, _ int) (any, error) {
	return f.searchDashResult, f.searchDashErr
}

func (f *fakeClient) GetDashboard(_ context.Context, _ string) (any, error) {
	return f.getDashResult, f.getDashErr
}

func (f *fakeClient) CreateDashboard(_ context.Context, dashboard map[string]any, folderID int64, overwrite bool) (any, error) {
	f.createDashboardArg = dashboard
	f.createFolderID = folderID
	f.createOverwrite = overwrite
	return f.createDashResult, f.createDashErr
}

func (f *fakeClient) DeleteDashboard(_ context.Context, _ string) (any, error) {
	return f.deleteDashResult, f.deleteDashErr
}

func (f *fakeClient) DashboardVersions(_ context.Context, _ string, _ int) (any, error) {
	return f.dashVersionsResult, f.dashVersionsErr
}

func (f *fakeClient) RenderDashboard(_ context.Context, req grafana.DashboardRenderRequest) (grafana.RenderedDashboard, error) {
	f.renderDashboardReq = req
	return f.renderDashboardResp, f.renderDashboardErr
}

func (f *fakeClient) ListDatasources(_ context.Context) (any, error) {
	return f.listDSResult, f.listDSErr
}

func (f *fakeClient) GetDatasource(_ context.Context, uid string) (any, error) {
	f.getDSUID = uid
	return f.getDSResult, f.getDSErr
}

func (f *fakeClient) DatasourceHealth(_ context.Context, uid string) (any, error) {
	f.dsHealthUID = uid
	return f.dsHealthResult, f.dsHealthErr
}

func (f *fakeClient) DatasourceResource(_ context.Context, method, uid, resourcePath string, body any) (any, error) {
	f.dsResourceMethod = method
	f.dsResourceUID = uid
	f.dsResourcePath = resourcePath
	f.dsResourceBody = body
	return f.dsResourceResult, f.dsResourceErr
}

func (f *fakeClient) DatasourceQuery(_ context.Context, req grafana.DatasourceQueryRequest) (any, error) {
	f.dsQueryReq = req
	return f.dsQueryResult, f.dsQueryErr
}

func (f *fakeClient) ListFolders(_ context.Context) (any, error) {
	return f.listFoldersResult, f.listFoldersErr
}

func (f *fakeClient) GetFolder(_ context.Context, _ string) (any, error) {
	return f.getFolderResult, f.getFolderErr
}

func (f *fakeClient) ServiceAccounts(_ context.Context, req grafana.ServiceAccountListRequest) (any, error) {
	f.serviceAccountsReq = req
	return f.serviceAccountsResp, f.serviceAccountsErr
}

func (f *fakeClient) ServiceAccount(_ context.Context, id int64) (any, error) {
	f.serviceAccountID = id
	return f.serviceAccountResp, f.serviceAccountErr
}

func (f *fakeClient) ListAnnotations(_ context.Context, req grafana.AnnotationListRequest) (any, error) {
	f.annotationsReq = req
	return f.annotationsResult, f.annotationsErr
}

func (f *fakeClient) AlertingRules(_ context.Context) (any, error) {
	return f.alertRulesResult, f.alertRulesErr
}

func (f *fakeClient) AlertingContactPoints(_ context.Context) (any, error) {
	return f.alertContactResult, f.alertContactErr
}

func (f *fakeClient) AlertingPolicies(_ context.Context) (any, error) {
	return f.alertPoliciesResult, f.alertPoliciesErr
}

func (f *fakeClient) AssistantChat(_ context.Context, prompt, chatID string) (any, error) {
	f.assistantPrompt = prompt
	f.assistantChatID = chatID
	return f.assistantChatResult, f.assistantChatErr
}

func (f *fakeClient) AssistantChatStatus(_ context.Context, chatID string) (any, error) {
	f.assistantStatusID = chatID
	return f.assistantStatusResp, f.assistantStatusErr
}

func (f *fakeClient) AssistantSkills(_ context.Context) (any, error) {
	return f.assistantSkillsResp, f.assistantSkillsErr
}

func (f *fakeClient) SyntheticChecks(_ context.Context, req grafana.SyntheticCheckListRequest) (any, error) {
	f.syntheticChecksReq = req
	return f.syntheticChecksResp, f.syntheticChecksErr
}

func (f *fakeClient) SyntheticCheck(_ context.Context, req grafana.SyntheticCheckGetRequest) (any, error) {
	f.syntheticCheckReq = req
	return f.syntheticCheckResp, f.syntheticCheckErr
}

func (f *fakeClient) MetricsRange(_ context.Context, expr, start, end, step string) (any, error) {
	f.metricsExpr = expr
	f.metricsStart = start
	f.metricsEnd = end
	f.metricsStep = step
	return f.metricsResult, f.metricsErr
}

func (f *fakeClient) LogsRange(_ context.Context, query, start, end string, limit int) (any, error) {
	f.logsQuery = query
	f.logsStart = start
	f.logsEnd = end
	f.logsLimit = limit
	return f.logsResult, f.logsErr
}

func (f *fakeClient) TracesSearch(_ context.Context, query, start, end string, limit int) (any, error) {
	f.tracesQuery = query
	f.tracesStart = start
	f.tracesEnd = end
	f.tracesLimit = limit
	return f.tracesResult, f.tracesErr
}

func (f *fakeClient) AggregateSnapshot(_ context.Context, req grafana.AggregateRequest) (grafana.AggregateSnapshot, error) {
	f.aggregateReq = req
	return f.aggregateResult, f.aggregateErr
}

func newTestApp(store config.Store, client APIClient) (*App, *strings.Builder, *strings.Builder) {
	out := &strings.Builder{}
	errOut := &strings.Builder{}
	app := NewApp(store)
	app.Out = out
	app.Err = errOut
	app.NewClient = func(config.Config) APIClient { return client }
	app.Now = func() time.Time { return time.Date(2026, 3, 5, 15, 4, 0, 0, time.UTC) }
	return app, out, errOut
}

func decodeJSON(t *testing.T, value string) map[string]any {
	t.Helper()
	out := map[string]any{}
	if err := json.Unmarshal([]byte(value), &out); err != nil {
		t.Fatalf("invalid JSON output: %v, value=%s", err, value)
	}
	return out
}

func decodeJSONArray(t *testing.T, value string) []map[string]any {
	t.Helper()
	out := []map[string]any{}
	if err := json.Unmarshal([]byte(value), &out); err != nil {
		t.Fatalf("invalid JSON array output: %v, value=%s", err, value)
	}
	return out
}

type discoveryHelpCase struct {
	path    []string
	command discoveryCommand
}

func flattenDiscoveryPaths(commands []discoveryCommand, prefix []string) []discoveryHelpCase {
	cases := make([]discoveryHelpCase, 0, len(commands))
	for _, command := range commands {
		path := append(append([]string{}, prefix...), command.Name)
		cases = append(cases, discoveryHelpCase{
			path:    path,
			command: command,
		})
		cases = append(cases, flattenDiscoveryPaths(command.Subcommands, path)...)
	}
	return cases
}

func findCommandByName(t *testing.T, commands []any, name string) map[string]any {
	t.Helper()
	for _, command := range commands {
		entry, ok := command.(map[string]any)
		if !ok {
			continue
		}
		if entry["name"] == name {
			return entry
		}
	}
	t.Fatalf("command %q not found in %#v", name, commands)
	return nil
}

func TestParseGlobalOptions(t *testing.T) {
	t.Setenv("FORCE_AGENT_MODE", "")
	t.Setenv("GRAFANA_CLI_AGENT_MODE", "")

	opts, rest, err := parseGlobalOptions([]string{"--fields", "a,b", "--output", "pretty", "auth", "status"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Output != "pretty" || len(opts.Fields) != 2 {
		t.Fatalf("unexpected opts: %+v", opts)
	}
	if len(rest) != 2 || rest[0] != "auth" {
		t.Fatalf("unexpected rest: %+v", rest)
	}

	opts, _, err = parseGlobalOptions([]string{"--output=json", "--fields=x.y"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Output != "json" || len(opts.Fields) != 1 {
		t.Fatalf("unexpected opts: %+v", opts)
	}

	if _, _, err := parseGlobalOptions([]string{"--output"}); err == nil {
		t.Fatalf("expected missing output error")
	}
	if _, _, err := parseGlobalOptions([]string{"--fields"}); err == nil {
		t.Fatalf("expected missing fields error")
	}
	if _, _, err := parseGlobalOptions([]string{"--output", "bad"}); err == nil {
		t.Fatalf("expected invalid output error")
	}
	opts, _, err = parseGlobalOptions([]string{"--output", "table", "--agent", "--read-only", "--yes", "auth", "status"})
	if err != nil {
		t.Fatalf("unexpected error for table/agent/read-only/yes: %v", err)
	}
	if opts.Output != "table" || !opts.Agent || !opts.ReadOnly || !opts.Yes {
		t.Fatalf("unexpected opts for table/agent/read-only/yes: %+v", opts)
	}
}

func TestParseGlobalOptionsExtended(t *testing.T) {
	t.Setenv("FORCE_AGENT_MODE", "1")
	t.Setenv("GRAFANA_CLI_AGENT_MODE", "")

	opts, rest, err := parseGlobalOptions([]string{"--json", "a,b", "--jq", ".x", "context", "view"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Output != "json" || opts.JQ != ".x" || len(opts.Fields) != 2 || !opts.Agent {
		t.Fatalf("unexpected opts: %+v", opts)
	}
	if len(rest) != 2 || rest[0] != "context" || rest[1] != "view" {
		t.Fatalf("unexpected rest: %+v", rest)
	}

	opts, _, err = parseGlobalOptions([]string{"--template={{.context}}", "context", "view"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Template != "{{.context}}" {
		t.Fatalf("unexpected template: %+v", opts)
	}

	if _, _, err := parseGlobalOptions([]string{"--json"}); err == nil {
		t.Fatalf("expected missing json error")
	}
	if _, _, err := parseGlobalOptions([]string{"--jq"}); err == nil {
		t.Fatalf("expected missing jq error")
	}
	if _, _, err := parseGlobalOptions([]string{"--template"}); err == nil {
		t.Fatalf("expected missing template error")
	}
	if _, _, err := parseGlobalOptions([]string{"--jq", ".x", "--template", "{{.}}"}); err == nil {
		t.Fatalf("expected jq/template conflict")
	}
}

func TestNewAppDefaults(t *testing.T) {
	app := NewApp(&fakeStore{})
	if app.Out == nil || app.Err == nil || app.NewClient == nil || app.Now == nil {
		t.Fatalf("expected defaults to be initialized")
	}
	if app.NewClient(config.Config{}) == nil {
		t.Fatalf("expected default client factory to return a client")
	}
}

func TestNewAppWithContextStore(t *testing.T) {
	store := &fakeContextStore{cfgs: map[string]config.Config{"default": {}}}
	app := NewApp(store)
	if app.Contexts == nil {
		t.Fatalf("expected context store to be wired")
	}
}

func TestRunHelpAndUnknown(t *testing.T) {
	store := &fakeStore{}
	client := &fakeClient{}
	app, out, errOut := newTestApp(store, client)

	if code := app.Run(context.Background(), []string{}); code != 0 {
		t.Fatalf("expected success for help")
	}
	resp := decodeJSON(t, out.String())
	if resp["version"] != cliVersion {
		t.Fatalf("expected version in discovery help, got %+v", resp)
	}
	commands, ok := resp["commands"].([]any)
	if !ok {
		t.Fatalf("expected commands output")
	}
	assistant := findCommandByName(t, commands, "assistant")
	if assistant["description"] == "" || assistant["token_cost"] == "" {
		t.Fatalf("expected assistant command metadata, got %+v", assistant)
	}
	if findCommandByName(t, commands, "schema")["description"] == "" {
		t.Fatalf("expected schema command in root help")
	}
	if findCommandByName(t, commands, "service-accounts")["description"] == "" {
		t.Fatalf("expected service-accounts command in root help")
	}
	if findCommandByName(t, commands, "synthetics")["description"] == "" {
		t.Fatalf("expected synthetics command in root help")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"-help"}); code != 0 {
		t.Fatalf("expected root -help to succeed")
	}
	if _, ok := decodeJSON(t, out.String())["commands"]; !ok {
		t.Fatalf("expected command list for root -help")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"bad"}); code != 1 {
		t.Fatalf("expected failure for unknown command")
	}
	if !strings.Contains(errOut.String(), "unknown command") {
		t.Fatalf("expected unknown command error, got %s", errOut.String())
	}

	errOut.Reset()
	if code := app.Run(context.Background(), []string{"--output"}); code != 1 {
		t.Fatalf("expected parse failure")
	}
	if !strings.Contains(errOut.String(), "--output requires a value") {
		t.Fatalf("expected global option error, got %s", errOut.String())
	}
}

func TestAuthFlows(t *testing.T) {
	store := &fakeStore{}
	client := &fakeClient{}
	app, out, errOut := newTestApp(store, client)

	if code := app.Run(context.Background(), []string{"auth", "login", "--token", "abc", "--base-url", "https://stack"}); code != 0 {
		t.Fatalf("auth login should succeed, err=%s", errOut.String())
	}
	resp := decodeJSON(t, out.String())
	if resp["status"] != "authenticated" {
		t.Fatalf("unexpected login response: %+v", resp)
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"auth", "status"}); code != 0 {
		t.Fatalf("status should succeed")
	}
	resp = decodeJSON(t, out.String())
	if resp["status"] != "authenticated" || resp["capabilities"] == nil || resp["missing"] == nil {
		t.Fatalf("expected richer authenticated status, got %+v", resp)
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"auth", "doctor"}); code != 0 {
		t.Fatalf("doctor should succeed")
	}
	doctor := decodeJSON(t, out.String())
	if doctor["authenticated"] != true || doctor["capabilities"] == nil {
		t.Fatalf("unexpected doctor response: %+v", doctor)
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"auth", "logout"}); code != 1 {
		t.Fatalf("logout should require --yes")
	}
	if !strings.Contains(errOut.String(), "requires --yes") {
		t.Fatalf("expected confirmation error, got %s", errOut.String())
	}

	errOut.Reset()
	out.Reset()
	if code := app.Run(context.Background(), []string{"--yes", "auth", "logout"}); code != 0 {
		t.Fatalf("logout should succeed")
	}
	resp = decodeJSON(t, out.String())
	if resp["status"] != "logged_out" {
		t.Fatalf("unexpected logout response")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"auth", "status"}); code != 0 {
		t.Fatalf("status after logout should succeed")
	}
	if decodeJSON(t, out.String())["status"] != "unauthenticated" {
		t.Fatalf("expected unauthenticated status after logout")
	}

	if code := app.Run(context.Background(), []string{"auth", "login"}); code != 1 {
		t.Fatalf("missing token should fail")
	}
	if !strings.Contains(errOut.String(), "--token is required") {
		t.Fatalf("expected token error")
	}

	if code := app.Run(context.Background(), []string{"auth", "bad"}); code != 1 {
		t.Fatalf("unknown auth command should fail")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"auth"}); code != 0 {
		t.Fatalf("auth summary should succeed")
	}
	if _, ok := decodeJSON(t, out.String())["commands"]; !ok {
		t.Fatalf("expected auth command list")
	}

	if code := app.Run(context.Background(), []string{"auth", "login", "--bad"}); code != 1 {
		t.Fatalf("unknown auth login flag should fail")
	}
}

func TestAuthLoginStackInference(t *testing.T) {
	store := &fakeStore{}
	client := &fakeClient{
		rawResponses: map[string]any{
			"/api/instances/prod-observability/datasources": []any{
				map[string]any{"type": "prometheus", "url": "https://prom.grafana.net"},
				map[string]any{"type": "loki", "url": "https://logs.grafana.net"},
				map[string]any{"type": "tempo", "url": "https://traces.grafana.net"},
			},
			"/api/instances/prod-observability/connections": []any{
				map[string]any{"type": "oncall", "oncallApiUrl": "https://oncall.grafana.net"},
			},
		},
	}
	app, out, errOut := newTestApp(store, client)

	if code := app.Run(context.Background(), []string{"auth", "login", "--token", "abc", "--stack", "https://prod-observability.grafana.net"}); code != 0 {
		t.Fatalf("stack auth login should succeed: %s", errOut.String())
	}
	resp := decodeJSON(t, out.String())
	if resp["base_url"] != "https://prod-observability.grafana.net" || resp["prometheus_url"] != "https://prom.grafana.net" || resp["oncall_url"] != "https://oncall.grafana.net" {
		t.Fatalf("expected inferred endpoints in login response, got %+v", resp)
	}
	if store.cfg.BaseURL != "https://prod-observability.grafana.net" || store.cfg.PrometheusURL != "https://prom.grafana.net" || store.cfg.OnCallURL != "https://oncall.grafana.net" {
		t.Fatalf("expected inferred endpoints in stored config, got %+v", store.cfg)
	}
	if len(client.rawCalls) != 2 {
		t.Fatalf("expected two cloud discovery calls, got %+v", client.rawCalls)
	}

	out.Reset()
	client = &fakeClient{
		rawErrors: map[string]error{
			"/api/instances/prod-observability/datasources": errors.New("discovery failed"),
		},
	}
	app, out, errOut = newTestApp(&fakeStore{}, client)
	if code := app.Run(context.Background(), []string{"auth", "login", "--token", "abc", "--stack", "prod-observability", "--oncall-url", "https://manual-oncall.grafana.net"}); code != 0 {
		t.Fatalf("stack auth login with partial discovery should succeed: %s", errOut.String())
	}
	resp = decodeJSON(t, out.String())
	warnings, ok := resp["warnings"].([]any)
	if !ok || len(warnings) != 1 {
		t.Fatalf("expected discovery warnings, got %+v", resp)
	}

	if code := app.Run(context.Background(), []string{"auth", "login", "--token", "abc", "--stack", "https://example.com"}); code != 1 {
		t.Fatalf("invalid stack host should fail")
	}
}

func TestCommandErrorsFromStore(t *testing.T) {
	store := &fakeStore{loadErr: errors.New("load fail")}
	client := &fakeClient{}
	app, _, errOut := newTestApp(store, client)

	if code := app.Run(context.Background(), []string{"auth", "status"}); code != 1 {
		t.Fatalf("expected failure")
	}
	if !strings.Contains(errOut.String(), "load fail") {
		t.Fatalf("expected load fail error")
	}
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"auth", "doctor"}); code != 1 {
		t.Fatalf("expected doctor failure")
	}
	if !strings.Contains(errOut.String(), "load fail") {
		t.Fatalf("expected doctor load fail error")
	}

	store = &fakeStore{cfg: config.Config{Token: "x"}, saveErr: errors.New("save fail")}
	app, _, errOut = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"auth", "login", "--token", "x"}); code != 1 {
		t.Fatalf("expected save failure")
	}
	if !strings.Contains(errOut.String(), "save fail") {
		t.Fatalf("expected save fail error")
	}

	store = &fakeStore{cfg: config.Config{Token: "x"}, clearErr: errors.New("clear fail")}
	app, _, errOut = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"--yes", "auth", "logout"}); code != 1 {
		t.Fatalf("expected clear failure")
	}
	if !strings.Contains(errOut.String(), "clear fail") {
		t.Fatalf("expected clear fail error")
	}
}

func TestAPICloudDashboardDatasourceCommands(t *testing.T) {
	store := &fakeStore{cfg: config.Config{Token: "token"}}
	client := &fakeClient{
		rawResult: map[string]any{"ok": true},
		cloudResult: map[string]any{"items": []any{
			map[string]any{"id": 1, "slug": "local-stack", "region": "us"},
		}},
		cloudStackDSResult: map[string]any{"items": []any{
			map[string]any{"uid": "prom-uid", "name": "prom", "type": "prometheus", "url": "https://prom"},
			map[string]any{"uid": "loki-uid", "name": "loki", "type": "loki", "url": "https://logs"},
			map[string]any{"uid": "tempo-uid", "name": "tempo", "type": "tempo", "url": "https://traces"},
		}},
		cloudStackConn: map[string]any{
			"connections": []any{
				map[string]any{"type": "oncall", "details": map[string]any{"oncallApiUrl": "https://oncall"}},
			},
			"privateConnectivityInfo": map[string]any{
				"tenants": []any{map[string]any{"type": "prometheus"}},
			},
		},
		cloudStackPluginPage: map[string]any{
			"": map[string]any{
				"items": []any{
					map[string]any{"id": "grafana-oncall-app", "name": "Grafana OnCall", "version": "1.0.0"},
				},
				"metadata": map[string]any{
					"pagination": map[string]any{"nextPage": "/api/instances/local-stack/plugins?pageCursor=cursor-2"},
				},
			},
			"cursor-2": map[string]any{
				"items": []any{
					map[string]any{"id": "grafana-incident-app", "name": "Grafana IRM", "version": "1.1.0"},
				},
			},
		},
		cloudStackPlugin: map[string]any{"id": "grafana-oncall-app", "name": "Grafana OnCall", "version": "1.0.0"},
		cloudBillingResp: map[string]any{"items": []any{
			map[string]any{
				"dimensionName": "Logs",
				"amountDue":     100.5,
				"periodStart":   "2024-09-01T00:00:00Z",
				"periodEnd":     "2024-09-30T23:59:59Z",
				"usages":        []any{map[string]any{"stackName": "local-stack.grafana.net"}},
			},
			map[string]any{
				"dimensionName": "Metrics",
				"amountDue":     778.41,
				"periodStart":   "2024-09-01T00:00:00Z",
				"periodEnd":     "2024-09-30T23:59:59Z",
				"usages":        []any{map[string]any{"stackName": "local-stack.grafana.net"}},
			},
		}},
		searchDashResult: []any{map[string]any{"uid": "x"}},
		createDashResult: map[string]any{"status": "success"},
		listDSResult: []any{
			map[string]any{"uid": "prom-uid", "name": "prom", "type": "prometheus"},
			map[string]any{"uid": "loki-uid", "name": "loki", "type": "loki"},
		},
	}
	app, out, errOut := newTestApp(store, client)

	if code := app.Run(context.Background(), []string{"api", "GET", "/api/test", "--body", "{\"k\":1}"}); code != 0 {
		t.Fatalf("api should succeed: %s", errOut.String())
	}
	if decodeJSON(t, out.String())["ok"] != true {
		t.Fatalf("unexpected api output")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"--output", "pretty", "--fields", "ok", "api", "GET", "/api/test"}); code != 0 {
		t.Fatalf("api pretty output should succeed")
	}
	prettyResp := decodeJSON(t, out.String())
	if prettyResp["ok"] != true {
		t.Fatalf("unexpected pretty fields output: %+v", prettyResp)
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"api", "GET", "/api/test", "--body", "{"}); code != 1 {
		t.Fatalf("invalid body should fail")
	}
	if code := app.Run(context.Background(), []string{"api", "GET"}); code != 1 {
		t.Fatalf("missing api args should fail")
	}
	if code := app.Run(context.Background(), []string{"api", "GET", "/api/test", "--bad"}); code != 1 {
		t.Fatalf("unknown api flag should fail")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "list"}); code != 0 {
		t.Fatalf("cloud list should succeed")
	}
	if decodeJSON(t, out.String())["items"] == nil {
		t.Fatalf("unexpected cloud output")
	}
	out.Reset()
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "inspect", "--stack", "local-stack"}); code != 0 {
		t.Fatalf("cloud inspect should succeed: %s", errOut.String())
	}
	inspect := decodeJSON(t, out.String())
	inferred := inspect["inferred_endpoints"].(map[string]any)
	if client.cloudStackSlug != "local-stack" || inferred["prometheus_url"] != "https://prom" || inferred["oncall_url"] != "https://oncall" {
		t.Fatalf("unexpected cloud inspect payload: %+v", inspect)
	}
	if inspect["datasource_summary"].(map[string]any)["count"] != float64(3) {
		t.Fatalf("expected datasource summary in cloud inspect: %+v", inspect)
	}
	if inspect["connectivity_summary"].(map[string]any)["has_private_connectivity"] != true {
		t.Fatalf("expected connectivity summary in cloud inspect: %+v", inspect)
	}
	out.Reset()
	if code := app.Run(context.Background(), []string{"cloud"}); code != 0 {
		t.Fatalf("cloud help should succeed")
	}
	if _, ok := decodeJSON(t, out.String())["commands"]; !ok {
		t.Fatalf("expected cloud discovery output")
	}
	if code := app.Run(context.Background(), []string{"cloud", "bad"}); code != 1 {
		t.Fatalf("unknown cloud should fail")
	}
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "bad"}); code != 1 {
		t.Fatalf("cloud stacks bad verb should fail")
	}
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "inspect"}); code != 1 {
		t.Fatalf("cloud inspect missing stack should fail")
	}
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "inspect", "--stack", "missing"}); code != 1 {
		t.Fatalf("cloud inspect missing stack lookup should fail")
	}
	out.Reset()
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "plugins", "list", "--stack", "local-stack", "--query", "oncall", "--limit", "1"}); code != 0 {
		t.Fatalf("cloud stack plugins list should succeed: %s", errOut.String())
	}
	plugins := decodeJSON(t, out.String())
	if client.cloudStackSlug != "local-stack" || len(plugins["items"].([]any)) != 1 {
		t.Fatalf("unexpected cloud stack plugins list payload: %+v", plugins)
	}
	out.Reset()
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "plugins", "list", "--stack", "local-stack", "--limit", "2"}); code != 0 {
		t.Fatalf("cloud stack plugins paginated list should succeed: %s", errOut.String())
	}
	plugins = decodeJSON(t, out.String())
	if len(plugins["items"].([]any)) != 2 || plugins["items"].([]any)[1].(map[string]any)["id"] != "grafana-incident-app" {
		t.Fatalf("unexpected paginated cloud stack plugins payload: %+v", plugins)
	}
	if client.cloudStackPluginReq.PageCursor != "cursor-2" {
		t.Fatalf("expected second plugin page cursor to be used, got %+v", client.cloudStackPluginReq)
	}
	out.Reset()
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "plugins", "get", "--stack", "local-stack", "--plugin", "grafana-oncall-app"}); code != 0 {
		t.Fatalf("cloud stack plugin get should succeed: %s", errOut.String())
	}
	plugin := decodeJSON(t, out.String())
	if client.cloudStackPluginID != "grafana-oncall-app" || plugin["id"] != "grafana-oncall-app" {
		t.Fatalf("unexpected cloud stack plugin payload: %+v", plugin)
	}
	out.Reset()
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "plugins"}); code != 0 {
		t.Fatalf("cloud stacks plugins help should succeed")
	}
	if _, ok := decodeJSON(t, out.String())["commands"]; !ok {
		t.Fatalf("expected cloud stack plugins discovery output")
	}
	out.Reset()
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "plugins", "-help"}); code != 0 {
		t.Fatalf("cloud stacks plugins explicit help should succeed")
	}
	if _, ok := decodeJSON(t, out.String())["commands"]; !ok {
		t.Fatalf("expected explicit cloud stack plugins discovery output")
	}
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "plugins", "list", "--stack", "local-stack", "--limit", "0"}); code != 1 {
		t.Fatalf("cloud stack plugins list invalid limit should fail")
	}
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "plugins", "list"}); code != 1 {
		t.Fatalf("cloud stack plugins list missing stack should fail")
	}
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "plugins", "list", "--bad"}); code != 1 {
		t.Fatalf("cloud stack plugins list parse should fail")
	}
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "plugins", "get", "--stack", "local-stack"}); code != 1 {
		t.Fatalf("cloud stack plugin get missing plugin should fail")
	}
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "plugins", "get", "--plugin", "grafana-oncall-app"}); code != 1 {
		t.Fatalf("cloud stack plugin get missing stack should fail")
	}
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "plugins", "get", "--stack", "https://example.com", "--plugin", "grafana-oncall-app"}); code != 1 {
		t.Fatalf("cloud stack plugin get invalid stack should fail")
	}
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "plugins", "get", "--bad"}); code != 1 {
		t.Fatalf("cloud stack plugin get parse should fail")
	}
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "plugins", "bad"}); code != 1 {
		t.Fatalf("cloud stack plugins bad verb should fail")
	}
	out.Reset()
	if code := app.Run(context.Background(), []string{"--agent", "cloud", "stacks", "plugins", "list", "--stack", "local-stack", "--limit", "1"}); code != 0 {
		t.Fatalf("cloud stack plugins list envelope should succeed: %s", errOut.String())
	}
	pluginsEnvelope := decodeJSON(t, out.String())
	pluginsMeta := pluginsEnvelope["metadata"].(map[string]any)
	if pluginsMeta["command"] != "cloud stacks plugins list" || pluginsMeta["truncated"] != true {
		t.Fatalf("unexpected cloud stack plugins metadata: %+v", pluginsMeta)
	}
	client.cloudStackPluginsErr = errors.New("plugins failed")
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "plugins", "list", "--stack", "local-stack"}); code != 1 {
		t.Fatalf("cloud stack plugins list client error should fail")
	}
	client.cloudStackPluginsErr = nil
	client.cloudStackPluginErr = errors.New("plugin failed")
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "plugins", "get", "--stack", "local-stack", "--plugin", "grafana-oncall-app"}); code != 1 {
		t.Fatalf("cloud stack plugin get client error should fail")
	}
	client.cloudStackPluginErr = nil
	out.Reset()
	if code := app.Run(context.Background(), []string{"--agent", "cloud", "billed-usage", "get", "--org-slug", "local-org", "--year", "2024", "--month", "9"}); code != 0 {
		t.Fatalf("cloud billed usage should succeed: %s", errOut.String())
	}
	billingEnvelope := decodeJSON(t, out.String())
	billingData := billingEnvelope["data"].(map[string]any)
	billingMeta := billingEnvelope["metadata"].(map[string]any)
	if billingMeta["command"] != "cloud billed-usage get" || client.cloudBillingReq.OrgSlug != "local-org" || client.cloudBillingReq.Year != 2024 || client.cloudBillingReq.Month != 9 {
		t.Fatalf("unexpected cloud billed usage metadata or request: meta=%+v req=%+v", billingMeta, client.cloudBillingReq)
	}
	if billingData["org_slug"] != "local-org" || billingData["summary"].(map[string]any)["total_amount_due"] != 878.91 {
		t.Fatalf("unexpected cloud billed usage payload: %+v", billingData)
	}
	out.Reset()
	if code := app.Run(context.Background(), []string{"cloud", "billed-usage"}); code != 0 {
		t.Fatalf("cloud billed usage help should succeed")
	}
	if _, ok := decodeJSON(t, out.String())["commands"]; !ok {
		t.Fatalf("expected cloud billed usage discovery output")
	}
	out.Reset()
	if code := app.Run(context.Background(), []string{"cloud", "billed-usage", "-help"}); code != 0 {
		t.Fatalf("cloud billed usage explicit help should succeed")
	}
	if _, ok := decodeJSON(t, out.String())["commands"]; !ok {
		t.Fatalf("expected explicit cloud billed usage discovery output")
	}
	if code := app.Run(context.Background(), []string{"cloud", "billed-usage", "bad"}); code != 1 {
		t.Fatalf("cloud billed usage bad verb should fail")
	}
	if code := app.Run(context.Background(), []string{"cloud", "billed-usage", "get", "--year", "2024", "--month", "9"}); code != 1 {
		t.Fatalf("cloud billed usage missing org slug should fail")
	}
	if code := app.Run(context.Background(), []string{"cloud", "billed-usage", "get"}); code != 1 {
		t.Fatalf("cloud billed usage missing args should fail")
	}
	if code := app.Run(context.Background(), []string{"cloud", "billed-usage", "get", "--org-slug", "local-org", "--year", "2024", "--month", "13"}); code != 1 {
		t.Fatalf("cloud billed usage invalid month should fail")
	}
	if code := app.Run(context.Background(), []string{"cloud", "billed-usage", "get", "--org-slug", "local-org", "--year", "0", "--month", "9"}); code != 1 {
		t.Fatalf("cloud billed usage invalid year should fail")
	}
	if code := app.Run(context.Background(), []string{"cloud", "billed-usage", "get", "--bad"}); code != 1 {
		t.Fatalf("cloud billed usage parse should fail")
	}
	client.cloudBillingErr = errors.New("billing failed")
	if code := app.Run(context.Background(), []string{"cloud", "billed-usage", "get", "--org-slug", "local-org", "--year", "2024", "--month", "9"}); code != 1 {
		t.Fatalf("cloud billed usage client error should fail")
	}
	client.cloudBillingErr = nil

	out.Reset()
	if code := app.Run(context.Background(), []string{"dashboards", "list", "--query", "err"}); code != 0 {
		t.Fatalf("dashboard list should succeed")
	}
	if code := app.Run(context.Background(), []string{"dashboards", "list", "--bad"}); code != 1 {
		t.Fatalf("dashboard list parse should fail")
	}
	if code := app.Run(context.Background(), []string{"dashboards", "create", "--title", "Ops"}); code != 0 {
		t.Fatalf("dashboard create should succeed")
	}
	if client.createDashboardArg["title"] != "Ops" {
		t.Fatalf("expected generated dashboard payload")
	}
	if code := app.Run(context.Background(), []string{"dashboards", "create", "--template-json", "{"}); code != 1 {
		t.Fatalf("invalid dashboard template should fail")
	}
	if code := app.Run(context.Background(), []string{"dashboards", "create", "--template-json", "{\"title\":\"FromTemplate\"}"}); code != 0 {
		t.Fatalf("valid dashboard template should succeed")
	}
	if code := app.Run(context.Background(), []string{"dashboards", "create"}); code != 1 {
		t.Fatalf("missing dashboard create flags should fail")
	}
	out.Reset()
	if code := app.Run(context.Background(), []string{"dashboards"}); code != 0 {
		t.Fatalf("dashboards help should succeed")
	}
	if _, ok := decodeJSON(t, out.String())["commands"]; !ok {
		t.Fatalf("expected dashboards discovery output")
	}
	if code := app.Run(context.Background(), []string{"dashboards", "bad"}); code != 1 {
		t.Fatalf("unknown dashboard command should fail")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"datasources", "list", "--type", "loki"}); code != 0 {
		t.Fatalf("datasource list should succeed")
	}
	filtered := make([]map[string]any, 0)
	if err := json.Unmarshal([]byte(out.String()), &filtered); err != nil {
		t.Fatalf("unexpected datasource JSON: %v", err)
	}
	if len(filtered) != 1 || filtered[0]["type"] != "loki" || filtered[0]["typed_family"] != "loki" {
		t.Fatalf("unexpected datasource filter output: %+v", filtered)
	}
	if filtered[0]["raw"].(map[string]any)["type"] != "loki" {
		t.Fatalf("expected datasource raw payload preservation: %+v", filtered[0])
	}
	if code := app.Run(context.Background(), []string{"datasources", "bad"}); code != 1 {
		t.Fatalf("invalid datasources usage should fail")
	}
	if code := app.Run(context.Background(), []string{"datasources", "list", "--bad"}); code != 1 {
		t.Fatalf("datasources list parse should fail")
	}
}

func TestCloudStacksInspectWarningsAndHelp(t *testing.T) {
	store := &fakeStore{cfg: config.Config{Token: "token"}}
	client := &fakeClient{
		cloudResult: map[string]any{"items": []any{
			map[string]any{"slug": "local-stack", "region": "us"},
		}},
		cloudStackDSErr:   errors.New("datasources failed"),
		cloudStackConnErr: errors.New("connections failed"),
	}
	app, out, errOut := newTestApp(store, client)

	if code := app.Run(context.Background(), []string{"cloud", "stacks", "-help"}); code != 0 {
		t.Fatalf("cloud stacks help should succeed: %s", errOut.String())
	}
	if _, ok := decodeJSON(t, out.String())["commands"]; !ok {
		t.Fatalf("expected cloud stacks discovery output")
	}
	out.Reset()
	if code := app.Run(context.Background(), []string{"cloud", "stacks"}); code != 0 {
		t.Fatalf("cloud stacks subtree help should succeed: %s", errOut.String())
	}
	if _, ok := decodeJSON(t, out.String())["commands"]; !ok {
		t.Fatalf("expected cloud stacks subtree discovery output")
	}

	if code := app.Run(context.Background(), []string{"cloud", "stacks", "list", "extra"}); code != 1 {
		t.Fatalf("cloud stacks list extra args should fail")
	}
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "inspect", "--bad"}); code != 1 {
		t.Fatalf("cloud inspect parse error should fail")
	}
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "inspect", "--stack", "local-stack"}); code != 1 {
		t.Fatalf("cloud inspect should fail outside agent mode when discovery is incomplete")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"--agent", "cloud", "stacks", "inspect", "--stack", "local-stack", "--include-raw"}); code != 0 {
		t.Fatalf("cloud inspect with warnings should succeed: %s", errOut.String())
	}
	envelope := decodeJSON(t, out.String())
	meta := envelope["metadata"].(map[string]any)
	if meta["command"] != "cloud stacks inspect" || len(meta["warnings"].([]any)) != 2 {
		t.Fatalf("unexpected cloud inspect metadata: %+v", meta)
	}
	data := envelope["data"].(map[string]any)
	if _, ok := data["datasources"]; !ok {
		t.Fatalf("expected include-raw datasources field in inspect payload")
	}
	if _, ok := data["connections"]; !ok {
		t.Fatalf("expected include-raw connections field in inspect payload")
	}

	client.cloudErr = errors.New("cloud inspect failed")
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "inspect", "--stack", "local-stack"}); code != 1 {
		t.Fatalf("cloud inspect cloud client error should fail")
	}
}

func TestDashboardFolderAnnotationAndAlertingCommands(t *testing.T) {
	store := &fakeStore{cfg: config.Config{Token: "token", BaseURL: "https://grafana.example"}}
	client := &fakeClient{
		getDashResult:       map[string]any{"dashboard": map[string]any{"uid": "ops"}},
		deleteDashResult:    map[string]any{"status": "deleted"},
		dashVersionsResult:  []any{map[string]any{"version": 1}},
		renderDashboardResp: grafana.RenderedDashboard{Data: []byte("png-bytes"), ContentType: "image/png", Endpoint: "https://stack/render/d/ops/render", Bytes: 9},
		listFoldersResult:   []any{map[string]any{"uid": "root"}},
		getFolderResult:     map[string]any{"uid": "ops"},
		annotationsResult:   []any{map[string]any{"id": 1}},
		alertRulesResult:    []any{map[string]any{"uid": "rule-1"}},
		alertContactResult:  []any{map[string]any{"name": "pagerduty"}},
		alertPoliciesResult: map[string]any{"receiver": "default"},
	}
	app, out, _ := newTestApp(store, client)

	if code := app.Run(context.Background(), []string{"dashboards", "get", "--uid", "ops"}); code != 0 {
		t.Fatalf("dashboard get should succeed")
	}
	if decodeJSON(t, out.String())["dashboard"] == nil {
		t.Fatalf("unexpected dashboard get output")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"--yes", "dashboards", "delete", "--uid", "ops"}); code != 0 {
		t.Fatalf("dashboard delete should succeed")
	}
	if decodeJSON(t, out.String())["status"] != "deleted" {
		t.Fatalf("unexpected dashboard delete output")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"dashboards", "versions", "--uid", "ops", "--limit", "5"}); code != 0 {
		t.Fatalf("dashboard versions should succeed")
	}
	versions := make([]map[string]any, 0)
	if err := json.Unmarshal([]byte(out.String()), &versions); err != nil {
		t.Fatalf("unexpected versions JSON: %v", err)
	}
	if len(versions) != 1 || versions[0]["version"] != float64(1) {
		t.Fatalf("unexpected dashboard versions output: %+v", versions)
	}

	renderPath := filepath.Join(t.TempDir(), "renders", "ops.png")
	out.Reset()
	if code := app.Run(context.Background(), []string{"dashboards", "render", "--uid", "ops", "--panel-id", "12", "--out", renderPath}); code != 0 {
		t.Fatalf("dashboard render should succeed")
	}
	rendered := decodeJSON(t, out.String())
	if rendered["path"] != renderPath || rendered["content_type"] != "image/png" {
		t.Fatalf("unexpected render output: %+v", rendered)
	}
	data, err := os.ReadFile(renderPath)
	if err != nil {
		t.Fatalf("expected rendered file to exist: %v", err)
	}
	if string(data) != "png-bytes" {
		t.Fatalf("unexpected rendered bytes: %q", data)
	}
	if client.renderDashboardReq.PanelID != 12 || client.renderDashboardReq.UID != "ops" || client.renderDashboardReq.Theme != "light" {
		t.Fatalf("unexpected render request: %+v", client.renderDashboardReq)
	}

	client.createShortURLResp = map[string]any{"uid": "short-1", "url": "/goto/short-1"}
	out.Reset()
	if code := app.Run(context.Background(), []string{"dashboards", "share", "--uid", "ops", "--panel-id", "12", "--from", "now-6h", "--to", "now", "--theme", "light", "--org-id", "7"}); code != 0 {
		t.Fatalf("dashboard share should succeed")
	}
	shared := decodeJSON(t, out.String())
	if shared["uid"] != "ops" || shared["panel_id"] != float64(12) || shared["share_path"] != "/d-solo/ops/share?from=now-6h&orgId=7&panelId=12&theme=light&to=now" {
		t.Fatalf("unexpected share output: %+v", shared)
	}
	if shared["absolute_url"] != "https://grafana.example/goto/short-1" {
		t.Fatalf("unexpected share absolute url: %+v", shared)
	}
	if client.createShortURLReq.Path != "/d-solo/ops/share?from=now-6h&orgId=7&panelId=12&theme=light&to=now" || client.createShortURLReq.OrgID != 7 {
		t.Fatalf("unexpected share request: %+v", client.createShortURLReq)
	}

	client.createShortURLResp = map[string]any{"uid": "short-2", "url": "https://grafana.example/goto/short-2?orgId=1"}
	out.Reset()
	if code := app.Run(context.Background(), []string{"dashboards", "share", "--uid", "ops"}); code != 0 {
		t.Fatalf("dashboard share with absolute url should succeed")
	}
	shared = decodeJSON(t, out.String())
	if shared["share_path"] != "/d/ops/share" || shared["absolute_url"] != "https://grafana.example/goto/short-2?orgId=1" {
		t.Fatalf("unexpected absolute short url output: %+v", shared)
	}

	client.createShortURLResp = "short-raw"
	out.Reset()
	if code := app.Run(context.Background(), []string{"dashboards", "share", "--uid", "ops"}); code != 0 {
		t.Fatalf("dashboard share raw fallback should succeed")
	}
	shared = decodeJSON(t, out.String())
	if shared["share_path"] != "/d/ops/share" || shared["result"] != "short-raw" {
		t.Fatalf("unexpected share fallback output: %+v", shared)
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"folders", "list"}); code != 0 {
		t.Fatalf("folders list should succeed")
	}
	folders := make([]map[string]any, 0)
	if err := json.Unmarshal([]byte(out.String()), &folders); err != nil {
		t.Fatalf("unexpected folders JSON: %v", err)
	}
	if len(folders) != 1 || folders[0]["uid"] != "root" {
		t.Fatalf("unexpected folders output: %+v", folders)
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"folders", "get", "--uid", "ops"}); code != 0 {
		t.Fatalf("folders get should succeed")
	}
	if decodeJSON(t, out.String())["uid"] != "ops" {
		t.Fatalf("unexpected folder get output")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"annotations", "list", "--dashboard-uid", "ops", "--panel-id", "4", "--limit", "20", "--from", "now-1h", "--to", "now", "--tags", "prod,error", "--type", "annotation"}); code != 0 {
		t.Fatalf("annotations list should succeed")
	}
	annotations := make([]map[string]any, 0)
	if err := json.Unmarshal([]byte(out.String()), &annotations); err != nil {
		t.Fatalf("unexpected annotations JSON: %v", err)
	}
	if len(annotations) != 1 || annotations[0]["id"] != float64(1) {
		t.Fatalf("unexpected annotations output: %+v", annotations)
	}
	if client.annotationsReq.DashboardUID != "ops" || client.annotationsReq.PanelID != 4 || client.annotationsReq.Limit != 20 || len(client.annotationsReq.Tags) != 2 {
		t.Fatalf("unexpected annotations request: %+v", client.annotationsReq)
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"alerting", "rules", "list"}); code != 0 {
		t.Fatalf("alerting rules should succeed")
	}
	rules := make([]map[string]any, 0)
	if err := json.Unmarshal([]byte(out.String()), &rules); err != nil {
		t.Fatalf("unexpected alert rules JSON: %v", err)
	}
	if len(rules) != 1 || rules[0]["uid"] != "rule-1" {
		t.Fatalf("unexpected alert rules output: %+v", rules)
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"alerting", "contact-points", "list"}); code != 0 {
		t.Fatalf("alerting contact points should succeed")
	}
	contacts := make([]map[string]any, 0)
	if err := json.Unmarshal([]byte(out.String()), &contacts); err != nil {
		t.Fatalf("unexpected contact points JSON: %v", err)
	}
	if len(contacts) != 1 || contacts[0]["name"] != "pagerduty" {
		t.Fatalf("unexpected contact points output: %+v", contacts)
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"alerting", "policies", "get"}); code != 0 {
		t.Fatalf("alerting policies should succeed")
	}
	if decodeJSON(t, out.String())["receiver"] != "default" {
		t.Fatalf("unexpected alerting policies output")
	}

	if code := app.Run(context.Background(), []string{"dashboards", "get"}); code != 1 {
		t.Fatalf("dashboard get missing uid should fail")
	}
	if code := app.Run(context.Background(), []string{"--yes", "dashboards", "delete"}); code != 1 {
		t.Fatalf("dashboard delete missing uid should fail")
	}
	if code := app.Run(context.Background(), []string{"dashboards", "versions"}); code != 1 {
		t.Fatalf("dashboard versions missing uid should fail")
	}
	if code := app.Run(context.Background(), []string{"dashboards", "render", "--uid", "ops"}); code != 1 {
		t.Fatalf("dashboard render missing out should fail")
	}
	if code := app.Run(context.Background(), []string{"dashboards", "share"}); code != 1 {
		t.Fatalf("dashboard share missing uid should fail")
	}
	if code := app.Run(context.Background(), []string{"dashboards", "render", "--out", renderPath}); code != 1 {
		t.Fatalf("dashboard render missing uid should fail")
	}
	if code := app.Run(context.Background(), []string{"dashboards", "get", "--bad"}); code != 1 {
		t.Fatalf("dashboard get parse should fail")
	}
	if code := app.Run(context.Background(), []string{"--yes", "dashboards", "delete", "--bad"}); code != 1 {
		t.Fatalf("dashboard delete parse should fail")
	}
	if code := app.Run(context.Background(), []string{"dashboards", "versions", "--bad"}); code != 1 {
		t.Fatalf("dashboard versions parse should fail")
	}
	if code := app.Run(context.Background(), []string{"dashboards", "render", "--bad"}); code != 1 {
		t.Fatalf("dashboard render parse should fail")
	}
	if code := app.Run(context.Background(), []string{"dashboards", "share", "--bad"}); code != 1 {
		t.Fatalf("dashboard share parse should fail")
	}

	badOut := filepath.Join(t.TempDir(), "parent-file")
	if err := os.WriteFile(badOut, []byte("x"), 0o600); err != nil {
		t.Fatalf("write bad out parent failed: %v", err)
	}
	if code := app.Run(context.Background(), []string{"dashboards", "render", "--uid", "ops", "--out", filepath.Join(badOut, "render.png")}); code != 1 {
		t.Fatalf("dashboard render parent file should fail")
	}

	directoryOut := filepath.Join(t.TempDir(), "render-dir")
	if err := os.MkdirAll(directoryOut, 0o755); err != nil {
		t.Fatalf("mkdir render dir failed: %v", err)
	}
	if code := app.Run(context.Background(), []string{"dashboards", "render", "--uid", "ops", "--out", directoryOut}); code != 1 {
		t.Fatalf("dashboard render directory path should fail")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"folders"}); code != 0 {
		t.Fatalf("folders help should succeed")
	}
	if _, ok := decodeJSON(t, out.String())["commands"]; !ok {
		t.Fatalf("expected folders discovery output")
	}
	if code := app.Run(context.Background(), []string{"folders", "list", "extra"}); code != 1 {
		t.Fatalf("folders list usage should fail")
	}
	if code := app.Run(context.Background(), []string{"folders", "get"}); code != 1 {
		t.Fatalf("folders get missing uid should fail")
	}
	if code := app.Run(context.Background(), []string{"folders", "get", "--bad"}); code != 1 {
		t.Fatalf("folders get parse should fail")
	}
	if code := app.Run(context.Background(), []string{"folders", "bad"}); code != 1 {
		t.Fatalf("folders unknown command should fail")
	}

	if code := app.Run(context.Background(), []string{"annotations"}); code != 0 {
		t.Fatalf("annotations help should succeed")
	}
	if code := app.Run(context.Background(), []string{"annotations", "bad"}); code != 1 {
		t.Fatalf("annotations unknown command should fail")
	}
	if code := app.Run(context.Background(), []string{"annotations", "list", "--bad"}); code != 1 {
		t.Fatalf("annotations parse should fail")
	}

	if code := app.Run(context.Background(), []string{"alerting"}); code != 0 {
		t.Fatalf("alerting help should succeed")
	}
	if code := app.Run(context.Background(), []string{"alerting", "rules", "bad"}); code != 1 {
		t.Fatalf("alerting rules usage should fail")
	}
	if code := app.Run(context.Background(), []string{"alerting", "rules"}); code != 1 {
		t.Fatalf("alerting rules missing verb should fail")
	}
	if code := app.Run(context.Background(), []string{"alerting", "contact-points", "bad"}); code != 1 {
		t.Fatalf("alerting contact points usage should fail")
	}
	if code := app.Run(context.Background(), []string{"alerting", "policies", "bad"}); code != 1 {
		t.Fatalf("alerting policies usage should fail")
	}
	if code := app.Run(context.Background(), []string{"alerting", "bad", "list"}); code != 1 {
		t.Fatalf("alerting unknown command should fail")
	}
}

func TestCloudAccessServiceAccountsAndSyntheticsCommands(t *testing.T) {
	store := &fakeStore{cfg: config.Config{Token: "token"}}
	client := &fakeClient{
		cloudAccessPages: map[string]any{
			"": map[string]any{
				"items": []any{map[string]any{"id": "ap-1", "name": "stack-readers"}},
				"metadata": map[string]any{
					"pagination": map[string]any{"nextPage": "/v1/accesspolicies?pageCursor=abc"},
				},
			},
			"abc": map[string]any{
				"items": []any{map[string]any{"id": "ap-2", "name": "stack-writers"}},
			},
		},
		cloudAccessOne:      map[string]any{"id": "ap-1", "name": "stack-readers"},
		serviceAccountsResp: map[string]any{"totalCount": 3, "serviceAccounts": []any{map[string]any{"id": 1, "name": "grafana"}}, "page": 2, "perPage": 1},
		serviceAccountResp:  map[string]any{"id": 1, "name": "grafana"},
		syntheticChecksResp: []any{map[string]any{"id": 123, "job": "checkout"}},
		syntheticCheckResp:  map[string]any{"id": 123, "job": "checkout"},
	}
	app, out, errOut := newTestApp(store, client)

	if code := app.Run(context.Background(), []string{"--agent", "cloud", "access-policies", "list", "--region", "us", "--realm-type", "stack", "--realm-identifier", "123", "--status", "active", "--page-size", "50", "--limit", "2"}); code != 0 {
		t.Fatalf("cloud access-policies list should succeed: %s", errOut.String())
	}
	if client.cloudAccessReq.Region != "us" || client.cloudAccessReq.RealmType != "stack" || client.cloudAccessReq.RealmIdentifier != "123" || client.cloudAccessReq.Status != "active" || client.cloudAccessReq.PageSize != 1 || client.cloudAccessReq.PageCursor != "abc" {
		t.Fatalf("unexpected cloud access request: %+v", client.cloudAccessReq)
	}
	accessEnvelope := decodeJSON(t, out.String())
	accessMeta := accessEnvelope["metadata"].(map[string]any)
	if accessMeta["command"] != "cloud access-policies list" || accessMeta["count"] != float64(2) {
		t.Fatalf("unexpected cloud access metadata: %+v", accessMeta)
	}
	if accessMeta["truncated"] == true || accessMeta["next_action"] != nil {
		t.Fatalf("expected fully paged access policy output, got metadata %+v", accessMeta)
	}
	accessItems := accessEnvelope["data"].(map[string]any)["items"].([]any)
	if len(accessItems) != 2 {
		t.Fatalf("expected access policies from both pages, got %+v", accessItems)
	}

	out.Reset()
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"cloud", "access-policies", "get", "--id", "ap-1", "--region", "us"}); code != 0 {
		t.Fatalf("cloud access-policies get should succeed: %s", errOut.String())
	}
	if client.cloudAccessID != "ap-1" || client.cloudAccessRegion != "us" {
		t.Fatalf("unexpected cloud access get args: id=%s region=%s", client.cloudAccessID, client.cloudAccessRegion)
	}
	if decodeJSON(t, out.String())["id"] != "ap-1" {
		t.Fatalf("unexpected access policy get output")
	}

	out.Reset()
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"--agent", "service-accounts", "list", "--query", "graf", "--page", "2", "--limit", "1"}); code != 0 {
		t.Fatalf("service-accounts list should succeed: %s", errOut.String())
	}
	if client.serviceAccountsReq.Query != "graf" || client.serviceAccountsReq.Page != 2 || client.serviceAccountsReq.Limit != 1 {
		t.Fatalf("unexpected service account request: %+v", client.serviceAccountsReq)
	}
	serviceEnvelope := decodeJSON(t, out.String())
	serviceMeta := serviceEnvelope["metadata"].(map[string]any)
	if serviceMeta["command"] != "service-accounts list" || serviceMeta["count"] != float64(1) || serviceMeta["truncated"] != true {
		t.Fatalf("unexpected service account metadata: %+v", serviceMeta)
	}

	out.Reset()
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"service-accounts", "get", "--id", "1"}); code != 0 {
		t.Fatalf("service-accounts get should succeed: %s", errOut.String())
	}
	if client.serviceAccountID != 1 {
		t.Fatalf("unexpected service account id: %d", client.serviceAccountID)
	}
	if decodeJSON(t, out.String())["id"] != float64(1) {
		t.Fatalf("unexpected service account get output")
	}

	out.Reset()
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"synthetics", "checks", "list", "--backend-url", "synthetic-monitoring-api-us-east-0.grafana.net", "--token", "sm-token", "--include-alerts"}); code != 0 {
		t.Fatalf("synthetics checks list should succeed: %s", errOut.String())
	}
	if client.syntheticChecksReq.BackendURL != "synthetic-monitoring-api-us-east-0.grafana.net" || client.syntheticChecksReq.Token != "sm-token" || !client.syntheticChecksReq.IncludeAlerts {
		t.Fatalf("unexpected synthetic checks request: %+v", client.syntheticChecksReq)
	}
	if len(decodeJSONArray(t, out.String())) != 1 {
		t.Fatalf("expected one synthetic check in output")
	}

	t.Setenv("GRAFANA_SYNTHETICS_BACKEND_URL", "synthetic-monitoring-api-eu-west-0.grafana.net")
	t.Setenv("GRAFANA_SYNTHETICS_TOKEN", "env-token")
	out.Reset()
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"synthetics", "checks", "get", "--id", "123"}); code != 0 {
		t.Fatalf("synthetics checks get should succeed with env auth: %s", errOut.String())
	}
	if client.syntheticCheckReq.BackendURL != "synthetic-monitoring-api-eu-west-0.grafana.net" || client.syntheticCheckReq.Token != "env-token" || client.syntheticCheckReq.ID != 123 {
		t.Fatalf("unexpected synthetic check request: %+v", client.syntheticCheckReq)
	}
	if decodeJSON(t, out.String())["id"] != float64(123) {
		t.Fatalf("unexpected synthetic check get output")
	}
}

func TestCloudAccessServiceAccountsAndSyntheticsValidation(t *testing.T) {
	store := &fakeStore{cfg: config.Config{Token: "token"}}
	client := &fakeClient{}
	app, out, errOut := newTestApp(store, client)

	for _, args := range [][]string{
		{"cloud", "access-policies", "list"},
		{"cloud", "access-policies", "list", "--bad"},
		{"cloud", "access-policies", "list", "--region", "us", "--limit", "0"},
		{"cloud", "access-policies", "list", "--region", "us", "--page-size", "0"},
		{"cloud", "access-policies", "list", "--region", "us", "--realm-identifier", "123"},
		{"cloud", "access-policies", "list", "--region", "us", "--realm-type", "team"},
		{"cloud", "access-policies", "list", "--region", "us", "--status", "paused"},
		{"cloud", "access-policies", "get", "--bad"},
		{"cloud", "access-policies", "get", "--region", "us"},
		{"cloud", "access-policies", "get", "--id", "ap-1"},
		{"cloud", "access-policies", "bad"},
		{"service-accounts", "list", "--page", "0"},
		{"service-accounts", "list", "--bad"},
		{"service-accounts", "list", "--limit", "0"},
		{"service-accounts", "get", "--bad"},
		{"service-accounts", "get", "--id", "0"},
		{"service-accounts", "bad"},
		{"synthetics", "bad"},
		{"synthetics", "bad", "list"},
		{"synthetics", "checks"},
		{"synthetics", "checks", "list"},
		{"synthetics", "checks", "list", "--bad"},
		{"synthetics", "checks", "get", "--id", "0"},
		{"synthetics", "checks", "get", "--bad"},
		{"synthetics", "checks", "bad"},
	} {
		out.Reset()
		errOut.Reset()
		if code := app.Run(context.Background(), args); code != 1 {
			t.Fatalf("expected failure for %v", args)
		}
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"cloud", "access-policies"}); code != 0 {
		t.Fatalf("cloud access-policies root help should succeed")
	}
	if _, ok := decodeJSON(t, out.String())["commands"]; !ok {
		t.Fatalf("expected cloud access-policies root help output")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"cloud", "access-policies", "--help"}); code != 0 {
		t.Fatalf("cloud access-policies help should succeed")
	}
	if _, ok := decodeJSON(t, out.String())["commands"]; !ok {
		t.Fatalf("expected cloud access-policies help output")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"service-accounts"}); code != 0 {
		t.Fatalf("service-accounts help should succeed")
	}
	if _, ok := decodeJSON(t, out.String())["commands"]; !ok {
		t.Fatalf("expected service-accounts help output")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"synthetics"}); code != 0 {
		t.Fatalf("synthetics help should succeed")
	}
	if _, ok := decodeJSON(t, out.String())["commands"]; !ok {
		t.Fatalf("expected synthetics help output")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"synthetics", "--help"}); code != 0 {
		t.Fatalf("synthetics explicit help should succeed")
	}
	if _, ok := decodeJSON(t, out.String())["commands"]; !ok {
		t.Fatalf("expected explicit synthetics help output")
	}
	if err := app.runSynthetics(context.Background(), globalOptions{}, []string{"--help"}); err != nil {
		t.Fatalf("expected direct synthetics help to succeed: %v", err)
	}

	out.Reset()
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"synthetics", "checks", "list", "--backend-url", "synthetic-monitoring-api-us-east-0.grafana.net"}); code != 1 {
		t.Fatalf("synthetics checks list missing token should fail")
	}
	if !strings.Contains(errOut.String(), "--token is required") {
		t.Fatalf("expected synthetic token validation error, got %s", errOut.String())
	}
}

func TestGroupHelpWithoutAuth(t *testing.T) {
	store := &fakeContextStore{
		current: "default",
		cfgs:    map[string]config.Config{"default": {}},
	}
	client := &fakeClient{}
	app, out, errOut := newTestApp(store, client)

	for _, args := range [][]string{
		{"auth", "-help"},
		{"context", "-help"},
		{"config", "-help"},
		{"cloud", "-help"},
		{"service-accounts", "-help"},
		{"dashboards", "-help"},
		{"datasources", "-help"},
		{"folders", "-help"},
		{"annotations", "-help"},
		{"alerting", "-help"},
		{"assistant", "-help"},
		{"synthetics", "-help"},
		{"runtime", "-help"},
		{"aggregate", "-help"},
		{"incident", "-help"},
		{"irm", "-help"},
		{"oncall", "-help"},
		{"agent", "-help"},
	} {
		out.Reset()
		errOut.Reset()
		if code := app.Run(context.Background(), args); code != 0 {
			t.Fatalf("expected help to succeed for %v, err=%s", args, errOut.String())
		}
		if _, ok := decodeJSON(t, out.String())["commands"]; !ok {
			t.Fatalf("expected commands output for %v", args)
		}
	}

	t.Setenv("GRAFANA_SYNTHETICS_BACKEND_URL", "")
	t.Setenv("GRAFANA_SYNTHETICS_TOKEN", "")
	out.Reset()
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"synthetics", "checks", "get", "--id", "1"}); code != 1 {
		t.Fatalf("expected synthetics get missing auth failure")
	}
	if !strings.Contains(errOut.String(), "--backend-url is required") {
		t.Fatalf("expected synthetics get auth validation error, got %s", errOut.String())
	}
}

func TestSchemaCommandAndNestedHelp(t *testing.T) {
	store := &fakeStore{}
	client := &fakeClient{}
	app, out, errOut := newTestApp(store, client)

	if code := app.Run(context.Background(), []string{"schema", "--compact"}); code != 0 {
		t.Fatalf("schema compact should succeed: %s", errOut.String())
	}
	root := decodeJSON(t, out.String())
	if root["version"] != cliVersion {
		t.Fatalf("expected schema version, got %+v", root)
	}
	if _, ok := root["best_practices"]; ok {
		t.Fatalf("did not expect expanded docs in compact schema")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"schema"}); code != 0 {
		t.Fatalf("schema default should succeed: %s", errOut.String())
	}
	if _, ok := decodeJSON(t, out.String())["best_practices"]; ok {
		t.Fatalf("schema default should stay compact")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"schema", "--full", "runtime"}); code != 0 {
		t.Fatalf("schema full runtime should succeed: %s", errOut.String())
	}
	runtimeSchema := decodeJSON(t, out.String())
	if runtimeSchema["scope"] != "runtime" {
		t.Fatalf("expected runtime scope, got %+v", runtimeSchema)
	}
	if _, ok := runtimeSchema["query_syntax"].(map[string]any)["metrics"]; !ok {
		t.Fatalf("expected runtime query syntax, got %+v", runtimeSchema["query_syntax"])
	}
	if _, ok := runtimeSchema["best_practices"].([]any); !ok {
		t.Fatalf("expected expanded runtime schema docs")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"runtime", "metrics", "query", "--help"}); code != 0 {
		t.Fatalf("nested leaf help should succeed: %s", errOut.String())
	}
	leaf := decodeJSON(t, out.String())
	if leaf["scope"] != "runtime metrics query" {
		t.Fatalf("expected nested help scope, got %+v", leaf)
	}
	commands := leaf["commands"].([]any)
	if len(commands) != 1 {
		t.Fatalf("expected one leaf command, got %+v", commands)
	}
	command := commands[0].(map[string]any)
	if command["name"] != "query" {
		t.Fatalf("expected leaf command metadata, got %+v", command)
	}
	if strings.TrimSpace(command["output_shape"].(string)) == "" {
		t.Fatalf("expected leaf help to include output shape, got %+v", command)
	}
	if examples, ok := command["examples"].([]any); !ok || len(examples) == 0 {
		t.Fatalf("expected leaf help to include examples, got %+v", command)
	}
	if related, ok := command["related_commands"].([]any); !ok || len(related) == 0 {
		t.Fatalf("expected leaf help to include related commands, got %+v", command)
	}
	if _, ok := leaf["query_syntax"].(map[string]any)["metrics"]; !ok {
		t.Fatalf("expected metrics query syntax in leaf help")
	}
	if _, ok := leaf["best_practices"].([]any); !ok {
		t.Fatalf("expected leaf help to expand best practices, got %+v", leaf)
	}

	if code := app.Run(context.Background(), []string{"schema", "--bad"}); code != 1 {
		t.Fatalf("schema should fail for unknown flag")
	}
	if code := app.Run(context.Background(), []string{"schema", "--compact", "--full"}); code != 1 {
		t.Fatalf("schema should fail for conflicting compact/full flags")
	}
}

func TestDiscoveryHelpers(t *testing.T) {
	payload, err := buildDiscoveryPayload([]string{"runtime", "logs"}, false)
	if err != nil {
		t.Fatalf("unexpected discovery payload error: %v", err)
	}
	if payload["scope"] != "runtime logs" {
		t.Fatalf("expected logs scope, got %+v", payload)
	}
	if _, ok := payload["query_syntax"].(map[string]string); !ok {
		t.Fatalf("expected logs query syntax map, got %#v", payload["query_syntax"])
	}
	commands := payload["commands"].([]map[string]any)
	if commands[0]["output_shape"] == "" {
		t.Fatalf("expected full discovery payload to include output shape")
	}

	if _, err := buildDiscoveryPayload([]string{"missing"}, true); err == nil {
		t.Fatalf("expected unknown schema path error")
	}

	path, ok := requestedHelpPath([]string{"dashboards", "get", "--help"})
	if !ok || strings.Join(path, " ") != "dashboards get" {
		t.Fatalf("unexpected help path: ok=%v path=%v", ok, path)
	}
	if path, ok := requestedHelpPath([]string{"dashboards", "get"}); ok || path != nil {
		t.Fatalf("unexpected help path without help flag: ok=%v path=%v", ok, path)
	}
	if strings.Join(discoveryPathFromArgs([]string{"api", "GET", "/api/test"}), " ") != "api" {
		t.Fatalf("expected api command path")
	}
	if workflows := discoveryWorkflows([]string{"dashboards"}); workflows != nil {
		t.Fatalf("expected no workflows for unrelated discovery scope, got %+v", workflows)
	}
	if !helpCompactForPath(nil) {
		t.Fatalf("expected root help to stay compact")
	}
	if !helpCompactForPath([]string{"runtime"}) {
		t.Fatalf("expected grouped help to stay compact")
	}
	if helpCompactForPath([]string{"runtime", "metrics", "query"}) {
		t.Fatalf("expected leaf help to expand")
	}
	if helpCompactForPath([]string{"api"}) {
		t.Fatalf("expected top-level leaf help to expand")
	}
	if !helpCompactForPath([]string{"missing"}) {
		t.Fatalf("expected unknown help path to stay compact")
	}
}

func TestEveryDiscoveryHelpPathIsSelfDescribing(t *testing.T) {
	store := &fakeContextStore{}
	app := NewApp(store)
	var out, errOut strings.Builder
	app.Out = &out
	app.Err = &errOut

	for _, tc := range flattenDiscoveryPaths(discoveryCatalog(), nil) {
		t.Run(strings.Join(tc.path, " "), func(t *testing.T) {
			out.Reset()
			errOut.Reset()

			args := append(append([]string{}, tc.path...), "--help")
			if code := app.Run(context.Background(), args); code != 0 {
				t.Fatalf("expected help to succeed for %v: %s", args, errOut.String())
			}

			payload := decodeJSON(t, out.String())
			if payload["scope"] != strings.Join(tc.path, " ") {
				t.Fatalf("expected help scope %q, got %+v", strings.Join(tc.path, " "), payload)
			}

			commands, ok := payload["commands"].([]any)
			if !ok || len(commands) != 1 {
				t.Fatalf("expected one command payload for %v, got %+v", tc.path, payload["commands"])
			}

			command, ok := commands[0].(map[string]any)
			if !ok {
				t.Fatalf("expected command map for %v, got %#v", tc.path, commands[0])
			}
			if command["name"] != tc.command.Name {
				t.Fatalf("expected command name %q, got %+v", tc.command.Name, command)
			}
			if strings.TrimSpace(command["description"].(string)) == "" {
				t.Fatalf("expected description for %v, got %+v", tc.path, command)
			}

			if len(tc.command.Subcommands) == 0 {
				if strings.TrimSpace(command["output_shape"].(string)) == "" {
					t.Fatalf("expected output shape for leaf help %v, got %+v", tc.path, command)
				}
				examples, ok := command["examples"].([]any)
				if !ok || len(examples) == 0 {
					t.Fatalf("expected examples for leaf help %v, got %+v", tc.path, command)
				}
				related, ok := command["related_commands"].([]any)
				if !ok || len(related) == 0 {
					t.Fatalf("expected related commands for leaf help %v, got %+v", tc.path, command)
				}
				if _, ok := payload["best_practices"].([]any); !ok {
					t.Fatalf("expected expanded guidance for leaf help %v, got %+v", tc.path, payload)
				}
				return
			}

			if _, ok := command["subcommands"].([]any); !ok {
				t.Fatalf("expected subgroup help to expose child commands for %v, got %+v", tc.path, command)
			}
			if _, ok := payload["best_practices"]; ok {
				t.Fatalf("expected subgroup help to remain compact for %v, got %+v", tc.path, payload)
			}
		})
	}
}

func TestDiscoveryPayloadBudgets(t *testing.T) {
	compactRoot, err := buildDiscoveryPayload(nil, true)
	if err != nil {
		t.Fatalf("unexpected compact root payload error: %v", err)
	}
	fullRoot, err := buildDiscoveryPayload(nil, false)
	if err != nil {
		t.Fatalf("unexpected full root payload error: %v", err)
	}
	compactBytes, err := json.Marshal(compactRoot)
	if err != nil {
		t.Fatalf("unexpected compact root marshal error: %v", err)
	}
	fullBytes, err := json.Marshal(fullRoot)
	if err != nil {
		t.Fatalf("unexpected full root marshal error: %v", err)
	}
	if len(compactBytes) >= len(fullBytes) {
		t.Fatalf("expected compact discovery payload to be smaller than full: compact=%d full=%d", len(compactBytes), len(fullBytes))
	}
	if len(compactBytes) > 20000 {
		t.Fatalf("compact discovery payload budget exceeded: %d bytes", len(compactBytes))
	}
	if _, ok := compactRoot["workflows"]; ok {
		t.Fatalf("compact discovery payload should omit workflows")
	}
	workflows, ok := fullRoot["workflows"].([]discoveryWorkflow)
	if !ok || len(workflows) == 0 || workflows[0].TokenCost == "" {
		t.Fatalf("expected full discovery workflows with token cost, got %+v", fullRoot["workflows"])
	}
}

func TestAuthDoctorPayloadAndReadOnly(t *testing.T) {
	doctor := authDoctorPayload("prod", config.Config{
		BaseURL:      "https://stack.grafana.net",
		Token:        "token",
		TokenBackend: "keyring",
	})
	if doctor["authenticated"] != true {
		t.Fatalf("expected authenticated doctor payload, got %+v", doctor)
	}
	if len(doctor["missing"].([]string)) == 0 {
		t.Fatalf("expected missing capabilities for partial config")
	}
	if len(doctor["capabilities"].([]map[string]any)) != 9 {
		t.Fatalf("expected capability matrix, got %+v", doctor["capabilities"])
	}
	if doctor["oncall_url"] != "" {
		t.Fatalf("expected empty oncall url in partial doctor payload, got %+v", doctor)
	}

	if err := enforceReadOnly([]string{"dashboards", "create", "--title", "Ops"}); err == nil {
		t.Fatalf("expected read-only enforcement for dashboard create")
	}
	if err := enforceReadOnly([]string{"dashboards", "share", "--uid", "ops"}); err == nil {
		t.Fatalf("expected read-only enforcement for dashboard share")
	}
	if err := enforceConfirmation([]string{"auth", "logout"}); err == nil {
		t.Fatalf("expected confirmation enforcement for auth logout")
	}
	if err := enforceConfirmation([]string{"dashboards", "delete", "--uid", "ops"}); err == nil {
		t.Fatalf("expected confirmation enforcement for dashboard delete")
	}
	if err := enforceConfirmation([]string{"api", "POST", "/api/test"}); err == nil {
		t.Fatalf("expected confirmation enforcement for api POST")
	}
	if err := enforceConfirmation([]string{"api", "GET", "/api/test"}); err != nil {
		t.Fatalf("expected api GET to bypass confirmation: %v", err)
	}
	if err := enforceConfirmation([]string{"dashboards", "delete", "--help"}); err != nil {
		t.Fatalf("expected help to bypass confirmation: %v", err)
	}
	if err := enforceReadOnly([]string{"api", "POST", "/api/test"}); err == nil {
		t.Fatalf("expected read-only enforcement for api POST")
	}
	if err := enforceReadOnly([]string{"api", "GET", "/api/test"}); err != nil {
		t.Fatalf("expected api GET to be allowed in read-only mode: %v", err)
	}
	if err := enforceReadOnly([]string{"dashboards", "get", "--help"}); err != nil {
		t.Fatalf("expected help to bypass read-only enforcement: %v", err)
	}

	store := &fakeStore{cfg: config.Config{Token: "token"}}
	client := &fakeClient{rawResult: map[string]any{"ok": true}}
	app, _, errOut := newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"--read-only", "dashboards", "create", "--title", "Ops"}); code != 1 {
		t.Fatalf("expected read-only dashboards create failure")
	}
	if !strings.Contains(errOut.String(), "blocked by --read-only") {
		t.Fatalf("expected read-only error, got %q", errOut.String())
	}
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"--read-only", "dashboards", "share", "--uid", "ops"}); code != 1 {
		t.Fatalf("expected read-only dashboards share failure")
	}
	if !strings.Contains(errOut.String(), "blocked by --read-only") {
		t.Fatalf("expected read-only share error, got %q", errOut.String())
	}
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"--read-only", "api", "GET", "/api/test"}); code != 0 {
		t.Fatalf("expected read-only api GET success, err=%q", errOut.String())
	}
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"dashboards", "delete", "--uid", "ops"}); code != 1 {
		t.Fatalf("expected confirmation failure for dashboard delete")
	}
	if !strings.Contains(errOut.String(), "requires --yes") {
		t.Fatalf("expected --yes error, got %q", errOut.String())
	}
}

func TestAgentEnvelopeAndTableOutput(t *testing.T) {
	store := &fakeStore{cfg: config.Config{Token: "token"}}
	client := &fakeClient{
		searchDashResult: []any{map[string]any{"uid": "ops"}},
		listDSResult:     []any{map[string]any{"name": "loki", "type": "loki"}},
	}
	app, out, errOut := newTestApp(store, client)

	if code := app.Run(context.Background(), []string{"--agent", "dashboards", "list", "--limit", "1"}); code != 0 {
		t.Fatalf("agent envelope should succeed: %s", errOut.String())
	}
	envelope := decodeJSON(t, out.String())
	if envelope["status"] != "success" {
		t.Fatalf("expected success envelope, got %+v", envelope)
	}
	metadata := envelope["metadata"].(map[string]any)
	if metadata["command"] != "dashboards list" {
		t.Fatalf("expected command metadata, got %+v", metadata)
	}
	if metadata["truncated"] != true || metadata["count"] != float64(1) {
		t.Fatalf("expected truncation metadata, got %+v", metadata)
	}
	if metadata["next_action"] == "" {
		t.Fatalf("expected next action guidance, got %+v", metadata)
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"--output", "table", "datasources", "list"}); code != 0 {
		t.Fatalf("table output should succeed: %s", errOut.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected table header and row, got %q", out.String())
	}
	if !strings.Contains(lines[0], "name") || !strings.Contains(lines[0], "type") {
		t.Fatalf("expected table header, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "loki") {
		t.Fatalf("expected datasource row, got %q", lines[1])
	}
}

func TestTimeNormalizationHelpers(t *testing.T) {
	now := time.Date(2026, 3, 6, 14, 0, 0, 0, time.UTC)
	if got := normalizeTimeValue(now, "now"); got != "2026-03-06T14:00:00Z" {
		t.Fatalf("unexpected now normalization: %s", got)
	}
	if got := normalizeTimeValue(now, "30m"); got != "2026-03-06T13:30:00Z" {
		t.Fatalf("unexpected 30m normalization: %s", got)
	}
	if got := normalizeTimeValue(now, "now-2h"); got != "2026-03-06T12:00:00Z" {
		t.Fatalf("unexpected now-2h normalization: %s", got)
	}
	if got := normalizeTimeValue(now, "2026-03-06T11:00:00Z"); got != "2026-03-06T11:00:00Z" {
		t.Fatalf("unexpected RFC3339 normalization: %s", got)
	}
	if got := normalizeTimeValue(now, "1700000000"); got != "1700000000" {
		t.Fatalf("expected unix timestamp passthrough, got %s", got)
	}
	start, end := normalizeTimeRange(now, "", "", time.Hour)
	if start != "2026-03-06T13:00:00Z" || end != "2026-03-06T14:00:00Z" {
		t.Fatalf("unexpected default range: start=%s end=%s", start, end)
	}
	start, end = normalizeTimeRange(now, "", "2026-03-06T10:00:00Z", time.Hour)
	if start != "2026-03-06T09:00:00Z" || end != "2026-03-06T10:00:00Z" {
		t.Fatalf("unexpected anchored range: start=%s end=%s", start, end)
	}

	if duration, ok := parseRelativeDuration("5 minutes"); !ok || duration != 5*time.Minute {
		t.Fatalf("expected 5 minute relative duration, got %v ok=%v", duration, ok)
	}
	if _, ok := parseRelativeDuration("garbage"); ok {
		t.Fatalf("unexpected parse success for invalid duration")
	}
}

func TestDiscoveryAndRenderHelpersCoverage(t *testing.T) {
	if syntax := discoveryQuerySyntax([]string{"runtime", "traces"}); len(syntax) != 1 || syntax["traces"] == "" {
		t.Fatalf("expected traces-only syntax, got %+v", syntax)
	}
	if syntax := discoveryQuerySyntax([]string{"dashboards"}); syntax != nil {
		t.Fatalf("expected no query syntax for dashboards, got %+v", syntax)
	}

	withOptional := fullCommandPayload(discoveryCommand{
		Name:            "x",
		Description:     "desc",
		ReadOnly:        true,
		OutputShape:     `{"ok":true}`,
		Examples:        []string{"grafana x"},
		RelatedCommands: []string{"schema"},
	}, nil)
	if withOptional["output_shape"] == "" || len(withOptional["examples"].([]string)) != 1 {
		t.Fatalf("expected optional fields in full payload, got %+v", withOptional)
	}
	withoutOptional := fullCommandPayload(discoveryCommand{Name: "y", Description: "desc", ReadOnly: true}, nil)
	if _, ok := withoutOptional["output_shape"]; ok {
		t.Fatalf("did not expect output shape in sparse full payload")
	}

	if _, ok := findDiscoveryCommand(discoveryCatalog(), []string{"runtime", "missing"}); ok {
		t.Fatalf("unexpected discovery match for missing child")
	}
	if _, ok := findDiscoveryCommand(discoveryCatalog(), nil); ok {
		t.Fatalf("unexpected discovery match for empty path")
	}
	if len(discoveryPathFromArgs(nil)) != 0 {
		t.Fatalf("expected empty discovery path for nil args")
	}
	if len(discoveryPathFromArgs([]string{"--help"})) != 0 {
		t.Fatalf("expected empty discovery path for help-only args")
	}

	if meta := withCommandMetadata(nil, "runtime logs query"); meta.Command != "runtime logs query" {
		t.Fatalf("expected command metadata on nil input, got %+v", meta)
	}
	if meta := collectionMetadata("", "scalar", 0, ""); meta != nil {
		t.Fatalf("expected nil metadata for scalar payload with no hints, got %+v", meta)
	}
	if count, ok := inferMetadataCount(map[string]any{"data": map[string]any{"results": []any{1, 2}}}); !ok || count != 2 {
		t.Fatalf("expected nested metadata count, got count=%d ok=%v", count, ok)
	}
	if _, ok := inferMetadataCount("scalar"); ok {
		t.Fatalf("unexpected metadata count for scalar payload")
	}

	var out strings.Builder
	if err := renderTable(&out, []any{}); err != nil {
		t.Fatalf("expected empty table render to succeed: %v", err)
	}
	if strings.TrimSpace(out.String()) != "No rows" {
		t.Fatalf("unexpected empty table output: %q", out.String())
	}
	if rows := rowsForTable(map[string]any{"items": []any{map[string]any{"id": 1, "attributes": map[string]any{"name": "ops"}}}}); len(rows) != 1 || rows[0]["attributes.name"] != "ops" {
		t.Fatalf("expected flattened rows, got %+v", rows)
	}
	if rows := rowsForTable(map[string]any{"data": []any{map[string]any{"id": 2}}}); len(rows) != 1 || rows[0]["id"] != 2 {
		t.Fatalf("expected nested data rows, got %+v", rows)
	}
	if rows := rowsForTable(map[string]any{"name": "ops"}); len(rows) != 1 || rows[0]["name"] != "ops" {
		t.Fatalf("expected single object row, got %+v", rows)
	}
	if rows := rowsForTable("scalar"); rows != nil {
		t.Fatalf("expected nil rows for scalar payload, got %+v", rows)
	}
	if flattened := flattenTableRow(map[string]any{"a": map[string]any{"b": "c"}}, ""); flattened["a.b"] != "c" {
		t.Fatalf("expected flattened nested row, got %+v", flattened)
	}
	if got := tableCell(nil); got != "" {
		t.Fatalf("unexpected nil cell: %q", got)
	}
	if got := tableCell("ops"); got != "ops" {
		t.Fatalf("unexpected string cell: %q", got)
	}
	if got := tableCell(true); got != "true" {
		t.Fatalf("unexpected bool cell: %q", got)
	}
	if got := tableCell(false); got != "false" {
		t.Fatalf("unexpected false cell: %q", got)
	}
	if got := tableCell(3); got != "3" {
		t.Fatalf("unexpected numeric cell: %q", got)
	}
	if got := tableCell([]any{1, 2}); got != "[2 items]" {
		t.Fatalf("unexpected array cell: %q", got)
	}
	if got := tableCell(map[string]any{"x": 1}); got != `{"x":1}` {
		t.Fatalf("unexpected object cell: %q", got)
	}
	if got := tableCell(func() {}); !strings.Contains(got, "0x") {
		t.Fatalf("expected fmt fallback for unsupported value, got %q", got)
	}

	for _, tc := range []struct {
		value string
		want  time.Duration
	}{
		{"5s", 5 * time.Second},
		{"2hours", 2 * time.Hour},
		{"3days", 72 * time.Hour},
		{"1week", 7 * 24 * time.Hour},
		{"-5m", 5 * time.Minute},
	} {
		if got, ok := parseRelativeDuration(tc.value); !ok || got != tc.want {
			t.Fatalf("unexpected duration for %s: got=%v ok=%v", tc.value, got, ok)
		}
	}
	if _, ok := parseRelativeDuration("999999999999999999999h"); ok {
		t.Fatalf("expected parse failure for overflowing duration")
	}
}

func TestAdditionalCoveragePaths(t *testing.T) {
	unauthenticatedDoctor := authDoctorPayload("default", config.Config{})
	if unauthenticatedDoctor["authenticated"] != false {
		t.Fatalf("expected unauthenticated doctor payload, got %+v", unauthenticatedDoctor)
	}
	if len(unauthenticatedDoctor["suggestions"].([]string)) == 0 {
		t.Fatalf("expected auth suggestions for unauthenticated payload")
	}

	if err := enforceReadOnly(nil); err != nil {
		t.Fatalf("expected nil args to bypass read-only enforcement: %v", err)
	}
	if err := enforceConfirmation(nil); err != nil {
		t.Fatalf("expected nil args to bypass confirmation enforcement: %v", err)
	}
	if err := enforceReadOnly([]string{"api"}); err != nil {
		t.Fatalf("expected api command without method to bypass enforcement: %v", err)
	}
	if err := enforceConfirmation([]string{"api"}); err != nil {
		t.Fatalf("expected api command without method to bypass confirmation enforcement: %v", err)
	}
	if err := enforceReadOnly([]string{"dashboards", "get", "--uid", "ops"}); err != nil {
		t.Fatalf("expected read-only command to be allowed: %v", err)
	}
	if err := enforceConfirmation([]string{"dashboards", "get", "--uid", "ops"}); err != nil {
		t.Fatalf("expected non-destructive command to bypass confirmation: %v", err)
	}
	if err := enforceReadOnly([]string{"unknown"}); err != nil {
		t.Fatalf("expected unknown command path to bypass read-only enforcement: %v", err)
	}
	if err := enforceConfirmation([]string{"unknown"}); err != nil {
		t.Fatalf("expected unknown command path to bypass confirmation enforcement: %v", err)
	}

	store := &fakeStore{}
	app, out, _ := newTestApp(store, &fakeClient{})
	if err := app.emitWithMetadata(globalOptions{Agent: true}, []any{map[string]any{"id": 1}}, nil); err != nil {
		t.Fatalf("expected emitWithMetadata to build metadata from nil meta, got %v", err)
	}
	nilMetaEnvelope := decodeJSON(t, out.String())
	if nilMetaEnvelope["metadata"].(map[string]any)["count"] != float64(1) {
		t.Fatalf("expected inferred count from nil meta, got %+v", nilMetaEnvelope)
	}

	out.Reset()
	if err := app.emitWithMetadata(globalOptions{Agent: true}, []any{map[string]any{"id": 1}}, &responseMetadata{}); err != nil {
		t.Fatalf("expected emitWithMetadata to infer counts, got %v", err)
	}
	envelope := decodeJSON(t, out.String())
	if envelope["metadata"].(map[string]any)["count"] != float64(1) {
		t.Fatalf("expected inferred count, got %+v", envelope)
	}

	out.Reset()
	if err := app.emitHelp(globalOptions{}, []string{"missing"}, true); err == nil {
		t.Fatalf("expected emitHelp to fail for missing path")
	}

	out.Reset()
	if err := renderTable(out, map[string]any{"name": "ops", "enabled": true}); err != nil {
		t.Fatalf("expected object table render to succeed: %v", err)
	}
	if !strings.Contains(out.String(), "name") || !strings.Contains(out.String(), "ops") {
		t.Fatalf("unexpected object table render: %q", out.String())
	}

	if err := renderTable(&failWriter{failAfter: 0}, []any{}); err == nil {
		t.Fatalf("expected renderTable empty write failure")
	}
	if err := renderTable(&failWriter{failAfter: 0}, map[string]any{"name": "ops"}); err == nil {
		t.Fatalf("expected renderTable header write failure")
	}
	if err := renderTable(&failWriter{failAfter: 1}, map[string]any{"name": "ops"}); err == nil {
		t.Fatalf("expected renderTable row write failure")
	}
}

func TestNoArgDiscoveryPaths(t *testing.T) {
	store := &fakeContextStore{
		current: "default",
		cfgs:    map[string]config.Config{"default": {}},
	}
	app, out, errOut := newTestApp(store, &fakeClient{})

	for _, args := range [][]string{
		{"auth"},
		{"context"},
		{"config"},
		{"api"},
		{"cloud"},
		{"dashboards"},
		{"datasources"},
		{"folders"},
		{"annotations"},
		{"alerting"},
		{"query-history"},
		{"slo"},
		{"irm"},
		{"oncall"},
		{"assistant"},
		{"runtime"},
		{"aggregate"},
		{"incident"},
		{"agent"},
	} {
		out.Reset()
		errOut.Reset()
		if code := app.Run(context.Background(), args); code != 0 {
			t.Fatalf("expected discovery help for %v, err=%s", args, errOut.String())
		}
		resp := decodeJSON(t, out.String())
		if _, ok := resp["commands"]; !ok {
			t.Fatalf("expected discovery commands for %v, got %+v", args, resp)
		}
	}
}

func TestQueryHistoryAndSLOCommands(t *testing.T) {
	store := &fakeStore{cfg: config.Config{Token: "token"}}
	client := &fakeClient{
		rawResult: map[string]any{
			"result": map[string]any{
				"queryHistory": []any{
					map[string]any{"uid": "qh-1", "datasourceUid": "loki-uid"},
					map[string]any{"uid": "qh-2", "datasourceUid": "prom-uid"},
				},
				"totalCount": 3,
			},
		},
	}
	app, out, errOut := newTestApp(store, client)

	if code := app.Run(context.Background(), []string{"--agent", "query-history", "list", "--datasource-uid", "loki-uid,prom-uid", "--search", "checkout", "--starred", "--sort", "time-asc", "--page", "2", "--limit", "2", "--from", "30m", "--to", "now"}); code != 0 {
		t.Fatalf("query-history list should succeed: %s", errOut.String())
	}
	if client.rawMethod != "GET" || !strings.HasPrefix(client.rawPath, "/api/query-history?") {
		t.Fatalf("unexpected query-history request: method=%s path=%s", client.rawMethod, client.rawPath)
	}
	for _, fragment := range []string{
		"datasourceUid=loki-uid",
		"datasourceUid=prom-uid",
		"searchString=checkout",
		"onlyStarred=true",
		"sort=time-asc",
		"page=2",
		"limit=2",
		"from=" + normalizeQueryHistoryBound(app.Now(), "30m"),
		"to=" + normalizeQueryHistoryBound(app.Now(), "now"),
	} {
		if !strings.Contains(client.rawPath, fragment) {
			t.Fatalf("expected %q in query-history path %q", fragment, client.rawPath)
		}
	}
	queryEnvelope := decodeJSON(t, out.String())
	queryMeta := queryEnvelope["metadata"].(map[string]any)
	if queryMeta["count"] != float64(2) || queryMeta["truncated"] != true {
		t.Fatalf("expected query-history metadata, got %+v", queryMeta)
	}
	if queryMeta["next_action"] == "" {
		t.Fatalf("expected query-history next action, got %+v", queryMeta)
	}

	out.Reset()
	client.rawResult = []any{
		map[string]any{"name": "Checkout Availability", "description": "Success budget"},
		map[string]any{"name": "Payments Availability", "description": "Payments budget"},
		map[string]any{"name": "Checkout Latency", "description": "Latency objective"},
	}
	if code := app.Run(context.Background(), []string{"--agent", "slo", "list", "--query", "checkout", "--limit", "1"}); code != 0 {
		t.Fatalf("slo list should succeed: %s", errOut.String())
	}
	if client.rawPath != "/api/plugins/grafana-slo-app/resources/v1/slo" {
		t.Fatalf("unexpected slo path: %s", client.rawPath)
	}
	sloEnvelope := decodeJSON(t, out.String())
	sloMeta := sloEnvelope["metadata"].(map[string]any)
	if sloMeta["count"] != float64(2) || sloMeta["truncated"] != true {
		t.Fatalf("expected slo metadata, got %+v", sloMeta)
	}
	data := sloEnvelope["data"].([]any)
	if len(data) != 1 || data[0].(map[string]any)["name"] != "Checkout Availability" {
		t.Fatalf("expected filtered slo data, got %+v", data)
	}

	if code := app.Run(context.Background(), []string{"query-history", "bad"}); code != 1 {
		t.Fatalf("query-history bad subcommand should fail")
	}
	if code := app.Run(context.Background(), []string{"query-history", "list", "--sort", "bad"}); code != 1 {
		t.Fatalf("query-history invalid sort should fail")
	}
	if code := app.Run(context.Background(), []string{"query-history", "list", "--limit", "0"}); code != 1 {
		t.Fatalf("query-history invalid limit should fail")
	}
	if code := app.Run(context.Background(), []string{"query-history", "list", "--page", "0"}); code != 1 {
		t.Fatalf("query-history invalid page should fail")
	}
	if code := app.Run(context.Background(), []string{"query-history", "list", "--bad"}); code != 1 {
		t.Fatalf("query-history parse failure should fail")
	}
	if code := app.Run(context.Background(), []string{"slo", "bad"}); code != 1 {
		t.Fatalf("slo bad subcommand should fail")
	}
	if code := app.Run(context.Background(), []string{"slo", "list", "--limit", "0"}); code != 1 {
		t.Fatalf("slo invalid limit should fail")
	}
	if code := app.Run(context.Background(), []string{"slo", "list", "--bad"}); code != 1 {
		t.Fatalf("slo parse failure should fail")
	}
}

func TestIRMAndOnCallCommands(t *testing.T) {
	store := &fakeStore{cfg: config.Config{Token: "token", BaseURL: "https://stack.grafana.net", OnCallURL: "https://oncall.grafana.net"}}
	client := &fakeClient{
		rawResponses: map[string]any{
			"/api/plugins/grafana-irm-app/resources/api/v1/IncidentsService.QueryIncidentPreviews": map[string]any{
				"results": []any{
					map[string]any{"incident": map[string]any{"title": "Checkout latency", "id": "inc-1"}},
					map[string]any{"incident": map[string]any{"title": "Payments latency", "id": "inc-2"}},
				},
			},
			"/api/v1/schedules/": map[string]any{
				"count": 2,
				"next":  "https://oncall.grafana.net/api/v1/schedules/?page=2",
				"results": []any{
					map[string]any{"name": "Primary Checkout", "team": map[string]any{"name": "Checkout"}},
					map[string]any{"name": "Primary Payments", "team": map[string]any{"name": "Payments"}},
				},
			},
		},
	}
	out := &strings.Builder{}
	errOut := &strings.Builder{}
	app := NewApp(store)
	app.Out = out
	app.Err = errOut
	app.Now = func() time.Time { return time.Date(2026, 3, 5, 15, 4, 0, 0, time.UTC) }
	var clientConfigs []config.Config
	app.NewClient = func(cfg config.Config) APIClient {
		clientConfigs = append(clientConfigs, cfg)
		return client
	}

	if code := app.Run(context.Background(), []string{"--agent", "irm", "incidents", "list", "--query", "checkout", "--limit", "2", "--order-field", "updatedAt", "--order-direction", "asc"}); code != 0 {
		t.Fatalf("irm incidents list should succeed: %s", errOut.String())
	}
	if client.rawMethod != "POST" || client.rawPath != "/api/plugins/grafana-irm-app/resources/api/v1/IncidentsService.QueryIncidentPreviews" {
		t.Fatalf("unexpected irm request: method=%s path=%s", client.rawMethod, client.rawPath)
	}
	body := client.rawBody.(map[string]any)["query"].(map[string]any)
	if body["limit"] != 2 || body["queryString"] != "checkout" {
		t.Fatalf("unexpected irm request body: %+v", body)
	}
	orderBy := body["orderBy"].(map[string]any)
	if orderBy["field"] != "updatedAt" || orderBy["direction"] != "asc" {
		t.Fatalf("unexpected irm orderBy: %+v", orderBy)
	}
	irmEnvelope := decodeJSON(t, out.String())
	irmMeta := irmEnvelope["metadata"].(map[string]any)
	if irmMeta["command"] != "irm incidents list" || irmMeta["count"] != float64(2) {
		t.Fatalf("unexpected irm metadata: %+v", irmMeta)
	}

	out.Reset()
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"--agent", "oncall", "schedules", "list", "--query", "checkout", "--limit", "1"}); code != 0 {
		t.Fatalf("oncall schedules list should succeed: %s", errOut.String())
	}
	if client.rawMethod != "GET" || client.rawPath != "/api/v1/schedules/" {
		t.Fatalf("unexpected oncall request: method=%s path=%s", client.rawMethod, client.rawPath)
	}
	if len(clientConfigs) < 2 || clientConfigs[len(clientConfigs)-1].BaseURL != "https://oncall.grafana.net" {
		t.Fatalf("expected oncall client to use oncall base URL, got %+v", clientConfigs)
	}
	oncallEnvelope := decodeJSON(t, out.String())
	oncallMeta := oncallEnvelope["metadata"].(map[string]any)
	if oncallMeta["command"] != "oncall schedules list" || oncallMeta["count"] != float64(1) || oncallMeta["truncated"] != true {
		t.Fatalf("unexpected oncall metadata: %+v", oncallMeta)
	}
	oncallData := oncallEnvelope["data"].(map[string]any)["results"].([]any)
	if len(oncallData) != 1 || oncallData[0].(map[string]any)["name"] != "Primary Checkout" {
		t.Fatalf("unexpected oncall payload: %+v", oncallData)
	}

	for _, args := range [][]string{
		{"irm", "bad"},
		{"irm", "incidents", "list", "--limit", "0"},
		{"irm", "incidents", "list", "--order-direction", "sideways"},
		{"irm", "incidents", "list", "--bad"},
		{"oncall", "bad"},
		{"oncall", "schedules", "list", "--limit", "0"},
		{"oncall", "schedules", "list", "--bad"},
	} {
		if code := app.Run(context.Background(), args); code != 1 {
			t.Fatalf("expected failure for %v", args)
		}
	}
}

func TestAssistantCommands(t *testing.T) {
	store := &fakeStore{cfg: config.Config{Token: "token"}}
	client := &fakeClient{
		assistantChatResult: map[string]any{"chatId": "c1"},
		assistantStatusResp: map[string]any{"status": "completed"},
		assistantSkillsResp: map[string]any{"items": []any{map[string]any{"name": "InvestigateIncident"}}},
	}
	app, out, errOut := newTestApp(store, client)

	if code := app.Run(context.Background(), []string{"assistant", "chat", "--prompt", "Investigate error rate"}); code != 0 {
		t.Fatalf("assistant chat should succeed: %s", errOut.String())
	}
	if client.assistantPrompt != "Investigate error rate" || client.assistantChatID != "" {
		t.Fatalf("assistant chat args not propagated")
	}
	if decodeJSON(t, out.String())["chatId"] != "c1" {
		t.Fatalf("unexpected assistant chat response")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"assistant", "chat", "--prompt", "Continue", "--chat-id", "c1"}); code != 0 {
		t.Fatalf("assistant chat continuation should succeed")
	}
	if client.assistantChatID != "c1" {
		t.Fatalf("assistant chat-id not propagated")
	}

	out.Reset()
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"--agent", "assistant", "investigate", "--goal", "Investigate checkout latency spike"}); code != 0 {
		t.Fatalf("assistant investigate should succeed: %s", errOut.String())
	}
	if !strings.Contains(client.assistantPrompt, "Goal: Investigate checkout latency spike") {
		t.Fatalf("assistant investigate prompt not shaped for investigations: %s", client.assistantPrompt)
	}
	investigateEnvelope := decodeJSON(t, out.String())
	if investigateEnvelope["metadata"].(map[string]any)["command"] != "assistant investigate" {
		t.Fatalf("unexpected assistant investigate metadata: %+v", investigateEnvelope)
	}
	if investigateEnvelope["metadata"].(map[string]any)["next_action"] == "" {
		t.Fatalf("expected assistant investigate next_action")
	}
	if investigateEnvelope["data"].(map[string]any)["goal"] != "Investigate checkout latency spike" {
		t.Fatalf("unexpected assistant investigate payload: %+v", investigateEnvelope["data"])
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"assistant", "status", "--chat-id", "c1"}); code != 0 {
		t.Fatalf("assistant status should succeed")
	}
	if client.assistantStatusID != "c1" {
		t.Fatalf("assistant status chat-id not propagated")
	}
	if decodeJSON(t, out.String())["status"] != "completed" {
		t.Fatalf("unexpected assistant status response")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"assistant", "skills"}); code != 0 {
		t.Fatalf("assistant skills should succeed")
	}
	if decodeJSON(t, out.String())["items"] == nil {
		t.Fatalf("unexpected assistant skills response")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"assistant"}); code != 0 {
		t.Fatalf("assistant help should succeed")
	}
	if _, ok := decodeJSON(t, out.String())["commands"]; !ok {
		t.Fatalf("expected assistant discovery output")
	}
	if code := app.Run(context.Background(), []string{"assistant", "chat"}); code != 1 {
		t.Fatalf("assistant chat missing prompt should fail")
	}
	if code := app.Run(context.Background(), []string{"assistant", "chat", "--bad"}); code != 1 {
		t.Fatalf("assistant chat parse should fail")
	}
	if code := app.Run(context.Background(), []string{"assistant", "investigate"}); code != 1 {
		t.Fatalf("assistant investigate missing goal should fail")
	}
	if code := app.Run(context.Background(), []string{"assistant", "investigate", "--bad"}); code != 1 {
		t.Fatalf("assistant investigate parse should fail")
	}
	if code := app.Run(context.Background(), []string{"assistant", "status"}); code != 1 {
		t.Fatalf("assistant status missing chat id should fail")
	}
	if code := app.Run(context.Background(), []string{"assistant", "status", "--bad"}); code != 1 {
		t.Fatalf("assistant status parse should fail")
	}
	if code := app.Run(context.Background(), []string{"assistant", "skills", "extra"}); code != 1 {
		t.Fatalf("assistant skills usage should fail")
	}
	if code := app.Run(context.Background(), []string{"assistant", "bad"}); code != 1 {
		t.Fatalf("assistant unknown command should fail")
	}
}

func TestRuntimeAggregateIncidentAgent(t *testing.T) {
	store := &fakeStore{cfg: config.Config{Token: "token"}}
	client := &fakeClient{
		metricsResult: map[string]any{"m": 1},
		listDSResult: []any{
			map[string]any{"uid": "prom-uid", "name": "prometheus", "type": "prometheus"},
			map[string]any{"uid": "loki-uid", "name": "loki", "type": "loki"},
			map[string]any{"uid": "tempo-uid", "name": "tempo", "type": "tempo"},
		},
		logsResult: map[string]any{
			"data": map[string]any{
				"result": []any{
					map[string]any{
						"stream": map[string]any{"app": "checkout", "level": "error"},
						"values": []any{[]any{"1", "error"}, []any{"2", "timeout"}},
					},
					map[string]any{
						"stream": map[string]any{"app": "payments"},
						"values": []any{[]any{"3", "error"}},
					},
				},
			},
		},
		tracesResult: map[string]any{
			"traces": []any{
				map[string]any{"traceID": "t-1", "rootServiceName": "checkout", "rootTraceName": "GET /checkout"},
				map[string]any{"traceID": "t-2", "rootServiceName": "payments", "rootTraceName": "POST /charge"},
			},
		},
		aggregateResult: grafana.AggregateSnapshot{
			Metrics: map[string]any{"data": map[string]any{"result": []any{1, 2}}},
			Logs:    map[string]any{"data": map[string]any{"result": []any{1}}},
			Traces:  map[string]any{"traces": []any{1, 2, 3}},
		},
		cloudResult: map[string]any{"items": []any{1, 2}},
	}
	app, out, errOut := newTestApp(store, client)

	if code := app.Run(context.Background(), []string{"runtime", "metrics", "query", "--expr", "up"}); code != 0 {
		t.Fatalf("runtime metrics failed: %s", errOut.String())
	}
	if client.metricsExpr != "up" || client.metricsStep != "30s" {
		t.Fatalf("unexpected metrics request capture: %+v", client)
	}
	if _, err := time.Parse(time.RFC3339, client.metricsStart); err != nil {
		t.Fatalf("expected normalized runtime metrics start, got %q", client.metricsStart)
	}
	if _, err := time.Parse(time.RFC3339, client.metricsEnd); err != nil {
		t.Fatalf("expected normalized runtime metrics end, got %q", client.metricsEnd)
	}
	out.Reset()
	if code := app.Run(context.Background(), []string{"runtime"}); code != 0 {
		t.Fatalf("runtime help should succeed")
	}
	if _, ok := decodeJSON(t, out.String())["commands"]; !ok {
		t.Fatalf("expected runtime discovery output")
	}
	if code := app.Run(context.Background(), []string{"runtime", "metrics", "bad"}); code != 1 {
		t.Fatalf("runtime metrics bad verb should fail")
	}
	if code := app.Run(context.Background(), []string{"runtime", "metrics"}); code != 1 {
		t.Fatalf("runtime metrics missing verb should fail")
	}
	if code := app.Run(context.Background(), []string{"runtime", "logs", "bad"}); code != 1 {
		t.Fatalf("runtime logs bad verb should fail")
	}
	if code := app.Run(context.Background(), []string{"runtime", "traces", "bad"}); code != 1 {
		t.Fatalf("runtime traces bad verb should fail")
	}
	if code := app.Run(context.Background(), []string{"runtime", "metrics", "query", "--bad"}); code != 1 {
		t.Fatalf("runtime metrics parse should fail")
	}
	if code := app.Run(context.Background(), []string{"runtime", "logs", "query", "--bad"}); code != 1 {
		t.Fatalf("runtime logs parse should fail")
	}
	if code := app.Run(context.Background(), []string{"runtime", "traces", "search", "--bad"}); code != 1 {
		t.Fatalf("runtime traces parse should fail")
	}
	if code := app.Run(context.Background(), []string{"runtime", "logs", "query", "--query", "{}"}); code != 0 {
		t.Fatalf("runtime logs failed")
	}
	if client.logsQuery != "{}" || client.logsLimit != 200 {
		t.Fatalf("unexpected logs request capture: %+v", client)
	}
	if _, err := time.Parse(time.RFC3339, client.logsStart); err != nil {
		t.Fatalf("expected normalized runtime logs start, got %q", client.logsStart)
	}
	if _, err := time.Parse(time.RFC3339, client.logsEnd); err != nil {
		t.Fatalf("expected normalized runtime logs end, got %q", client.logsEnd)
	}
	out.Reset()
	if code := app.Run(context.Background(), []string{"runtime", "logs", "aggregate", "--query", "{}"}); code != 0 {
		t.Fatalf("runtime logs aggregate failed")
	}
	logSummary := decodeJSON(t, out.String())
	if logSummary["streams"].(float64) != 2 || logSummary["entries"].(float64) != 3 {
		t.Fatalf("unexpected logs aggregate summary: %+v", logSummary)
	}
	if code := app.Run(context.Background(), []string{"runtime", "traces", "search", "--query", "{}"}); code != 0 {
		t.Fatalf("runtime traces failed")
	}
	if client.tracesQuery != "{}" || client.tracesLimit != 200 {
		t.Fatalf("unexpected traces request capture: %+v", client)
	}
	if _, err := time.Parse(time.RFC3339, client.tracesStart); err != nil {
		t.Fatalf("expected normalized runtime traces start, got %q", client.tracesStart)
	}
	if _, err := time.Parse(time.RFC3339, client.tracesEnd); err != nil {
		t.Fatalf("expected normalized runtime traces end, got %q", client.tracesEnd)
	}
	out.Reset()
	if code := app.Run(context.Background(), []string{"runtime", "traces", "aggregate", "--query", "{}"}); code != 0 {
		t.Fatalf("runtime traces aggregate failed")
	}
	traceSummary := decodeJSON(t, out.String())
	if traceSummary["trace_matches"].(float64) != 2 {
		t.Fatalf("unexpected traces aggregate summary: %+v", traceSummary)
	}
	if code := app.Run(context.Background(), []string{"runtime", "metrics", "query"}); code != 1 {
		t.Fatalf("missing expr should fail")
	}
	if code := app.Run(context.Background(), []string{"runtime", "logs", "aggregate"}); code != 1 {
		t.Fatalf("logs aggregate should require --query")
	}
	if code := app.Run(context.Background(), []string{"runtime", "traces", "aggregate"}); code != 1 {
		t.Fatalf("traces aggregate should require --query")
	}
	if code := app.Run(context.Background(), []string{"runtime", "bad", "query"}); code != 1 {
		t.Fatalf("bad runtime command should fail")
	}

	if code := app.Run(context.Background(), []string{"aggregate", "snapshot", "--metric-expr", "up", "--log-query", "{}", "--trace-query", "{}"}); code != 0 {
		t.Fatalf("aggregate should succeed")
	}
	if client.aggregateReq.MetricExpr != "up" {
		t.Fatalf("aggregate request not captured")
	}
	if code := app.Run(context.Background(), []string{"aggregate", "snapshot"}); code != 1 {
		t.Fatalf("aggregate missing flags should fail")
	}
	if code := app.Run(context.Background(), []string{"aggregate", "bad"}); code != 1 {
		t.Fatalf("aggregate bad subcommand should fail")
	}
	out.Reset()
	if code := app.Run(context.Background(), []string{"aggregate"}); code != 0 {
		t.Fatalf("aggregate help should succeed")
	}
	if _, ok := decodeJSON(t, out.String())["commands"]; !ok {
		t.Fatalf("expected aggregate discovery output")
	}
	if code := app.Run(context.Background(), []string{"aggregate", "snapshot", "--bad"}); code != 1 {
		t.Fatalf("aggregate parse should fail")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"incident", "analyze", "--goal", "error spike"}); code != 0 {
		t.Fatalf("incident analyze should succeed: %s", errOut.String())
	}
	inc := decodeJSON(t, out.String())
	summary := inc["summary"].(map[string]any)
	if summary["metrics_series"].(float64) != 2 {
		t.Fatalf("unexpected incident summary: %+v", summary)
	}
	if inc["datasource_summary"].(map[string]any)["count"].(float64) != 3 {
		t.Fatalf("expected incident datasource summary: %+v", inc)
	}
	if len(inc["query_hints"].([]any)) != 3 {
		t.Fatalf("expected incident query hints: %+v", inc)
	}
	if code := app.Run(context.Background(), []string{"incident", "analyze"}); code != 1 {
		t.Fatalf("missing goal should fail")
	}
	if code := app.Run(context.Background(), []string{"incident", "bad"}); code != 1 {
		t.Fatalf("incident usage should fail")
	}
	out.Reset()
	if code := app.Run(context.Background(), []string{"incident", "analyze", "--goal", "slow", "--metric-expr", "m", "--log-query", "l", "--trace-query", "t", "--start", "s", "--end", "e", "--step", "1m", "--limit", "10", "--include-raw"}); code != 0 {
		t.Fatalf("incident include-raw should succeed")
	}
	if len(decodeJSON(t, out.String())["datasources"].([]any)) != 3 {
		t.Fatalf("expected incident raw datasource inventory")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"agent", "plan", "--goal", "latency"}); code != 0 {
		t.Fatalf("agent plan should succeed")
	}
	plan := decodeJSON(t, out.String())
	if plan["playbook"] != "latency" {
		t.Fatalf("expected latency playbook")
	}
	if plan["actions"].([]any)[0].(map[string]any)["id"] != "datasource-inventory" {
		t.Fatalf("expected datasource inventory action in plan: %+v", plan)
	}
	out.Reset()
	if code := app.Run(context.Background(), []string{"agent", "run", "--goal", "errors"}); code != 0 {
		t.Fatalf("agent run should succeed")
	}
	agentResult := decodeJSON(t, out.String())
	if agentResult["datasource_summary"].(map[string]any)["count"].(float64) != 3 {
		t.Fatalf("expected agent datasource summary: %+v", agentResult)
	}
	if len(agentResult["query_hints"].([]any)) != 3 {
		t.Fatalf("expected agent query hints: %+v", agentResult)
	}
	out.Reset()
	if code := app.Run(context.Background(), []string{"agent", "run", "--goal", "errors", "--include-raw"}); code != 0 {
		t.Fatalf("agent include-raw should succeed")
	}
	if len(decodeJSON(t, out.String())["datasources"].([]any)) != 3 {
		t.Fatalf("expected agent raw datasource inventory")
	}
	if code := app.Run(context.Background(), []string{"agent", "bad", "--goal", "x"}); code != 1 {
		t.Fatalf("unknown agent command should fail")
	}
	if code := app.Run(context.Background(), []string{"agent", "plan"}); code != 1 {
		t.Fatalf("missing goal should fail")
	}
}

func TestRequireAuthAndClientErrors(t *testing.T) {
	store := &fakeStore{loadErr: errors.New("load failed")}
	client := &fakeClient{}
	app, _, errOut := newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "list"}); code != 1 {
		t.Fatalf("expected load failure")
	}
	if !strings.Contains(errOut.String(), "load failed") {
		t.Fatalf("unexpected load error: %s", errOut.String())
	}

	store = &fakeStore{cfg: config.Config{}}
	client = &fakeClient{}
	app, _, errOut = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "list"}); code != 1 {
		t.Fatalf("expected auth failure")
	}
	if !strings.Contains(errOut.String(), "not authenticated") {
		t.Fatalf("unexpected auth error: %s", errOut.String())
	}

	store = &fakeStore{cfg: config.Config{}}
	client = &fakeClient{}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"query-history", "list"}); code != 1 {
		t.Fatalf("expected query-history auth error")
	}
	store = &fakeStore{cfg: config.Config{Token: "x"}}
	client = &fakeClient{rawErr: errors.New("query-history fail")}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"query-history", "list"}); code != 1 {
		t.Fatalf("expected query-history client error")
	}

	store = &fakeStore{cfg: config.Config{}}
	client = &fakeClient{}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"slo", "list"}); code != 1 {
		t.Fatalf("expected slo auth error")
	}
	store = &fakeStore{cfg: config.Config{Token: "x"}}
	client = &fakeClient{rawErr: errors.New("slo fail")}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"slo", "list"}); code != 1 {
		t.Fatalf("expected slo client error")
	}

	store = &fakeStore{cfg: config.Config{}}
	client = &fakeClient{}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"irm", "incidents", "list"}); code != 1 {
		t.Fatalf("expected irm auth error")
	}
	store = &fakeStore{cfg: config.Config{Token: "x"}}
	client = &fakeClient{rawErr: errors.New("irm fail")}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"irm", "incidents", "list"}); code != 1 {
		t.Fatalf("expected irm client error")
	}

	store = &fakeStore{cfg: config.Config{Token: "x"}}
	client = &fakeClient{}
	app, _, errOut = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"oncall", "schedules", "list"}); code != 1 {
		t.Fatalf("expected oncall config error")
	}
	if !strings.Contains(errOut.String(), "oncall URL is not configured") {
		t.Fatalf("unexpected oncall config error: %s", errOut.String())
	}
	store = &fakeStore{cfg: config.Config{Token: "x", OnCallURL: "https://oncall"}}
	client = &fakeClient{rawErr: errors.New("oncall fail")}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"oncall", "schedules", "list"}); code != 1 {
		t.Fatalf("expected oncall client error")
	}
	store = &fakeStore{cfg: config.Config{}}
	client = &fakeClient{}
	app, _, _ = newTestApp(store, client)
	if _, err := app.requireOnCallConfig(); err == nil {
		t.Fatalf("expected requireOnCallConfig auth failure")
	}

	store = &fakeStore{cfg: config.Config{Token: "x"}}
	client = &fakeClient{cloudErr: errors.New("cloud fail")}
	app, _, errOut = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "list"}); code != 1 {
		t.Fatalf("expected cloud client failure")
	}
	if !strings.Contains(errOut.String(), "cloud fail") {
		t.Fatalf("unexpected cloud error: %s", errOut.String())
	}

	store = &fakeStore{cfg: config.Config{Token: "x"}}
	client = &fakeClient{
		createShortURLErr:  errors.New("share fail"),
		getDashErr:         errors.New("get dash fail"),
		deleteDashErr:      errors.New("delete dash fail"),
		dashVersionsErr:    errors.New("versions fail"),
		renderDashboardErr: errors.New("render fail"),
		listFoldersErr:     errors.New("folders fail"),
		getFolderErr:       errors.New("folder fail"),
		annotationsErr:     errors.New("annotations fail"),
		alertRulesErr:      errors.New("alert rules fail"),
		alertContactErr:    errors.New("alert contact fail"),
		alertPoliciesErr:   errors.New("alert policies fail"),
	}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"dashboards", "get", "--uid", "ops"}); code != 1 {
		t.Fatalf("expected dashboard get client failure")
	}
	if code := app.Run(context.Background(), []string{"--yes", "dashboards", "delete", "--uid", "ops"}); code != 1 {
		t.Fatalf("expected dashboard delete client failure")
	}
	if code := app.Run(context.Background(), []string{"dashboards", "versions", "--uid", "ops"}); code != 1 {
		t.Fatalf("expected dashboard versions client failure")
	}
	if code := app.Run(context.Background(), []string{"dashboards", "render", "--uid", "ops", "--out", filepath.Join(t.TempDir(), "x.png")}); code != 1 {
		t.Fatalf("expected dashboard render client failure")
	}
	if code := app.Run(context.Background(), []string{"dashboards", "share", "--uid", "ops"}); code != 1 {
		t.Fatalf("expected dashboard share client failure")
	}
	if code := app.Run(context.Background(), []string{"folders", "list"}); code != 1 {
		t.Fatalf("expected folders list client failure")
	}
	if code := app.Run(context.Background(), []string{"folders", "get", "--uid", "ops"}); code != 1 {
		t.Fatalf("expected folder get client failure")
	}
	if code := app.Run(context.Background(), []string{"annotations", "list"}); code != 1 {
		t.Fatalf("expected annotations client failure")
	}
	if code := app.Run(context.Background(), []string{"alerting", "rules", "list"}); code != 1 {
		t.Fatalf("expected alerting rules client failure")
	}
	if code := app.Run(context.Background(), []string{"alerting", "contact-points", "list"}); code != 1 {
		t.Fatalf("expected alerting contact points client failure")
	}
	if code := app.Run(context.Background(), []string{"alerting", "policies", "get"}); code != 1 {
		t.Fatalf("expected alerting policies client failure")
	}

	store = &fakeStore{cfg: config.Config{}}
	client = &fakeClient{}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"folders", "list"}); code != 1 {
		t.Fatalf("expected folders auth failure")
	}
	if code := app.Run(context.Background(), []string{"annotations", "list"}); code != 1 {
		t.Fatalf("expected annotations auth failure")
	}
	if code := app.Run(context.Background(), []string{"alerting", "rules", "list"}); code != 1 {
		t.Fatalf("expected alerting auth failure")
	}
}

func TestAdditionalCommandBranches(t *testing.T) {
	store := &fakeStore{cfg: config.Config{Token: "token"}}
	client := &fakeClient{
		rawResult:        map[string]any{"ok": true},
		createDashResult: map[string]any{"status": "ok"},
		listDSResult:     []any{map[string]any{"name": "a", "type": "x"}},
		aggregateResult: grafana.AggregateSnapshot{
			Metrics: map[string]any{},
			Logs:    map[string]any{},
			Traces:  map[string]any{},
		},
		cloudResult: map[string]any{"items": []any{}},
	}
	app, _, errOut := newTestApp(store, client)

	// auth login with all optional flags.
	if code := app.Run(context.Background(), []string{
		"auth", "login",
		"--token", "abc",
		"--base-url", "https://base",
		"--cloud-url", "https://cloud",
		"--prom-url", "https://prom",
		"--logs-url", "https://logs",
		"--traces-url", "https://traces",
		"--org-id", "7",
	}); code != 0 {
		t.Fatalf("expected full auth login to succeed: %s", errOut.String())
	}

	// API should propagate client errors.
	client.rawErr = errors.New("raw fail")
	if code := app.Run(context.Background(), []string{"api", "GET", "/x"}); code != 1 {
		t.Fatalf("expected raw failure")
	}
	client.rawErr = nil

	// Dashboards create with generated optional fields.
	if code := app.Run(context.Background(), []string{"dashboards", "create", "--title", "Ops", "--uid", "uid1", "--tags", "a,b", "--folder-id", "12", "--overwrite=false"}); code != 0 {
		t.Fatalf("dashboard create optional fields should succeed")
	}
	if client.createDashboardArg["uid"] != "uid1" || client.createFolderID != 12 || client.createOverwrite != false {
		t.Fatalf("dashboard create options not propagated")
	}

	// Datasources list without filters should pass through all entries.
	if code := app.Run(context.Background(), []string{"datasources", "list"}); code != 0 {
		t.Fatalf("datasources list without filters should succeed")
	}

	// runtime required query checks.
	if code := app.Run(context.Background(), []string{"runtime", "logs", "query"}); code != 1 {
		t.Fatalf("logs query should require --query")
	}
	if code := app.Run(context.Background(), []string{"runtime", "traces", "search"}); code != 1 {
		t.Fatalf("traces search should require --query")
	}

	// incident parse error branch.
	if code := app.Run(context.Background(), []string{"incident", "analyze", "--goal", "x", "--bad"}); code != 1 {
		t.Fatalf("incident parse error expected")
	}

	// agent with no subcommand.
	if code := app.Run(context.Background(), []string{"agent"}); code != 0 {
		t.Fatalf("agent help should succeed")
	}
}

func TestAppErrorBranches(t *testing.T) {
	// runAuthLogin load error branch.
	store := &fakeStore{loadErr: errors.New("load error")}
	client := &fakeClient{}
	app, _, _ := newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"auth", "login", "--token", "x"}); code != 1 {
		t.Fatalf("expected auth login load error")
	}

	// runAPI requireAuthConfig error branch.
	store = &fakeStore{cfg: config.Config{}}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"api", "GET", "/x"}); code != 1 {
		t.Fatalf("expected api auth error")
	}

	// runDashboards requireAuthConfig error branch.
	if code := app.Run(context.Background(), []string{"dashboards", "list"}); code != 1 {
		t.Fatalf("expected dashboards auth error")
	}
	if code := app.Run(context.Background(), []string{"service-accounts", "list"}); code != 1 {
		t.Fatalf("expected service-accounts auth error")
	}
	if code := app.Run(context.Background(), []string{"cloud", "access-policies", "list", "--region", "us"}); code != 1 {
		t.Fatalf("expected cloud access-policies auth error")
	}
	if code := app.Run(context.Background(), []string{"cloud", "billed-usage", "get", "--org-slug", "local-org", "--year", "2024", "--month", "9"}); code != 1 {
		t.Fatalf("expected cloud billed-usage auth error")
	}

	// runDashboards list and create client error branches + create parse error.
	store = &fakeStore{cfg: config.Config{Token: "x"}}
	client = &fakeClient{searchDashErr: errors.New("search fail")}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"dashboards", "list"}); code != 1 {
		t.Fatalf("expected dashboards list client error")
	}
	if code := app.Run(context.Background(), []string{"dashboards", "create", "--title", "x", "--bad"}); code != 1 {
		t.Fatalf("expected dashboards create parse error")
	}
	client.createDashErr = errors.New("create fail")
	if code := app.Run(context.Background(), []string{"dashboards", "create", "--title", "x"}); code != 1 {
		t.Fatalf("expected dashboards create client error")
	}

	// runDatasources auth + client error branches.
	store = &fakeStore{cfg: config.Config{}}
	client = &fakeClient{}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"datasources", "list"}); code != 1 {
		t.Fatalf("expected datasources auth error")
	}
	store = &fakeStore{cfg: config.Config{Token: "x"}}
	client = &fakeClient{listDSErr: errors.New("ds fail")}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"datasources", "list"}); code != 1 {
		t.Fatalf("expected datasources client error")
	}
	client = &fakeClient{serviceAccountsErr: errors.New("service accounts fail")}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"service-accounts", "list"}); code != 1 {
		t.Fatalf("expected service-accounts client error")
	}
	client = &fakeClient{serviceAccountErr: errors.New("service account fail")}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"service-accounts", "get", "--id", "1"}); code != 1 {
		t.Fatalf("expected service-account get client error")
	}
	client = &fakeClient{cloudAccessErr: errors.New("cloud access fail")}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"cloud", "access-policies", "list", "--region", "us"}); code != 1 {
		t.Fatalf("expected cloud access-policies client error")
	}
	client = &fakeClient{cloudBillingErr: errors.New("cloud billing fail")}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"cloud", "billed-usage", "get", "--org-slug", "local-org", "--year", "2024", "--month", "9"}); code != 1 {
		t.Fatalf("expected cloud billed-usage client error")
	}
	client = &fakeClient{cloudAccessOneErr: errors.New("cloud access get fail")}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"cloud", "access-policies", "get", "--id", "ap-1", "--region", "us"}); code != 1 {
		t.Fatalf("expected cloud access-policy get client error")
	}

	// runAssistant auth + client error branches.
	store = &fakeStore{cfg: config.Config{}}
	client = &fakeClient{}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"assistant", "skills"}); code != 1 {
		t.Fatalf("expected assistant auth error")
	}
	store = &fakeStore{cfg: config.Config{Token: "x"}}
	client = &fakeClient{assistantChatErr: errors.New("assistant chat fail")}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"assistant", "chat", "--prompt", "x"}); code != 1 {
		t.Fatalf("expected assistant chat client error")
	}
	client.assistantChatErr = nil
	client.assistantStatusErr = errors.New("assistant status fail")
	if code := app.Run(context.Background(), []string{"assistant", "status", "--chat-id", "c1"}); code != 1 {
		t.Fatalf("expected assistant status client error")
	}
	client.assistantStatusErr = nil
	client.assistantSkillsErr = errors.New("assistant skills fail")
	if code := app.Run(context.Background(), []string{"assistant", "skills"}); code != 1 {
		t.Fatalf("expected assistant skills client error")
	}
	client.assistantSkillsErr = nil
	client.assistantChatErr = errors.New("assistant investigate fail")
	if code := app.Run(context.Background(), []string{"assistant", "investigate", "--goal", "x"}); code != 1 {
		t.Fatalf("expected assistant investigate client error")
	}

	// runRuntime auth + client error branches.
	store = &fakeStore{cfg: config.Config{}}
	client = &fakeClient{}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"runtime", "metrics", "query", "--expr", "up"}); code != 1 {
		t.Fatalf("expected runtime auth error")
	}
	store = &fakeStore{cfg: config.Config{Token: "x"}}
	client = &fakeClient{metricsErr: errors.New("metrics fail")}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"runtime", "metrics", "query", "--expr", "up"}); code != 1 {
		t.Fatalf("expected runtime metrics client error")
	}
	client.metricsErr = nil
	client.logsErr = errors.New("logs fail")
	if code := app.Run(context.Background(), []string{"runtime", "logs", "query", "--query", "{}"}); code != 1 {
		t.Fatalf("expected runtime logs client error")
	}
	client.logsErr = nil
	client.tracesErr = errors.New("traces fail")
	if code := app.Run(context.Background(), []string{"runtime", "traces", "search", "--query", "{}"}); code != 1 {
		t.Fatalf("expected runtime traces client error")
	}

	// runSynthetics client error branches.
	client = &fakeClient{syntheticChecksErr: errors.New("synthetics fail")}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"synthetics", "checks", "list", "--backend-url", "synthetic-monitoring-api-us-east-0.grafana.net", "--token", "sm-token"}); code != 1 {
		t.Fatalf("expected synthetics list client error")
	}
	client = &fakeClient{syntheticCheckErr: errors.New("synthetic check fail")}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"synthetics", "checks", "get", "--backend-url", "synthetic-monitoring-api-us-east-0.grafana.net", "--token", "sm-token", "--id", "1"}); code != 1 {
		t.Fatalf("expected synthetic check client error")
	}

	// runAggregate auth + aggregate error branches.
	store = &fakeStore{cfg: config.Config{}}
	client = &fakeClient{}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"aggregate", "snapshot", "--metric-expr", "m", "--log-query", "l", "--trace-query", "t"}); code != 1 {
		t.Fatalf("expected aggregate auth error")
	}
	store = &fakeStore{cfg: config.Config{Token: "x"}}
	client = &fakeClient{aggregateErr: errors.New("aggregate fail")}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"aggregate", "snapshot", "--metric-expr", "m", "--log-query", "l", "--trace-query", "t"}); code != 1 {
		t.Fatalf("expected aggregate client error")
	}

	// runIncident auth + aggregate error branches.
	store = &fakeStore{cfg: config.Config{}}
	client = &fakeClient{}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"incident", "analyze", "--goal", "x"}); code != 1 {
		t.Fatalf("expected incident auth error")
	}
	store = &fakeStore{cfg: config.Config{Token: "x"}}
	client = &fakeClient{aggregateErr: errors.New("incident aggregate fail")}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"incident", "analyze", "--goal", "x"}); code != 1 {
		t.Fatalf("expected incident aggregate error")
	}

	client = &fakeClient{listDSErr: errors.New("datasource inventory fail"), aggregateResult: grafana.AggregateSnapshot{}}
	app, out, _ := newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"incident", "analyze", "--goal", "x"}); code != 0 {
		t.Fatalf("expected incident warning path success")
	}
	if len(decodeJSON(t, out.String())["warnings"].([]any)) != 1 {
		t.Fatalf("expected incident datasource warning")
	}

	// runAgent parse + auth + cloud + aggregate error branches.
	store = &fakeStore{cfg: config.Config{Token: "x"}}
	client = &fakeClient{}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"agent", "plan", "--bad"}); code != 1 {
		t.Fatalf("expected agent parse error")
	}

	store = &fakeStore{cfg: config.Config{}}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"agent", "run", "--goal", "x"}); code != 1 {
		t.Fatalf("expected agent run auth error")
	}

	store = &fakeStore{cfg: config.Config{Token: "x"}}
	client = &fakeClient{cloudErr: errors.New("cloud fail")}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"agent", "run", "--goal", "x"}); code != 1 {
		t.Fatalf("expected agent cloud error")
	}

	client = &fakeClient{cloudResult: map[string]any{"items": []any{}}, aggregateErr: errors.New("aggregate fail")}
	app, _, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"agent", "run", "--goal", "x"}); code != 1 {
		t.Fatalf("expected agent aggregate error")
	}

	client = &fakeClient{listDSErr: errors.New("datasource inventory fail"), cloudResult: map[string]any{"items": []any{}}, aggregateResult: grafana.AggregateSnapshot{}}
	app, out, _ = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"agent", "run", "--goal", "x"}); code != 0 {
		t.Fatalf("expected agent warning path success")
	}
	if len(decodeJSON(t, out.String())["warnings"].([]any)) != 1 {
		t.Fatalf("expected agent datasource warning")
	}
}

func TestHelpers(t *testing.T) {
	if got := splitCSV("a, b,,c"); len(got) != 3 {
		t.Fatalf("unexpected splitCSV result: %+v", got)
	}

	payload := []any{
		map[string]any{"name": "Prom", "type": "prometheus"},
		map[string]any{"name": "Loki", "type": "loki"},
		"bad",
	}
	filtered := filterDatasources(payload, "loki", "lo")
	items := filtered.([]any)
	if len(items) != 1 {
		t.Fatalf("unexpected filtered datasources: %+v", filtered)
	}
	if none := filterDatasources(payload, "", "nomatch").([]any); len(none) != 0 {
		t.Fatalf("expected name-filter mismatch to drop entries")
	}
	if all := filterDatasources(payload, "", "").([]any); len(all) != 3 {
		t.Fatalf("expected unfiltered payload")
	}
	if filterDatasources(map[string]any{"x": 1}, "", "") == nil {
		t.Fatalf("non-array payload should be returned unchanged")
	}

	summary := summarizeSnapshot(grafana.AggregateSnapshot{
		Metrics: map[string]any{"data": map[string]any{"result": []any{1}}},
		Logs:    map[string]any{"data": map[string]any{"result": []any{1, 2}}},
		Traces:  map[string]any{"data": map[string]any{"traces": []any{1, 2, 3}}},
	})
	if summary["trace_matches"].(int) != 3 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	if inferCollectionCount([]any{1, 2}) != 2 {
		t.Fatalf("unexpected collection count")
	}
	if inferCollectionCount(map[string]any{"items": []any{1}}) != 1 {
		t.Fatalf("unexpected map collection count")
	}
	if inferCollectionCount(map[string]any{"result": map[string]any{"queryHistory": []any{1}}}) != 1 {
		t.Fatalf("unexpected nested collection count")
	}
	if inferCollectionCount("x") != 0 {
		t.Fatalf("unexpected fallback count")
	}
	if inferCollectionCount(map[string]any{"ignored": []any{1}}) != 0 {
		t.Fatalf("unexpected collection count for ignored key")
	}

	if countPath(map[string]any{"a": map[string]any{"b": []any{1, 2}}}, "a", "b") != 2 {
		t.Fatalf("unexpected countPath result")
	}
	if countPath(map[string]any{"a": 1}, "a", "b") != 0 {
		t.Fatalf("countPath should return 0")
	}
	if countPath(map[string]any{"a": map[string]any{"b": 1}}, "a", "b") != 0 {
		t.Fatalf("countPath should return 0 for non-array leaf")
	}

	projected := projectFields(map[string]any{"a": map[string]any{"b": 1}, "x": 2}, []string{"a.b", "x"}).(map[string]any)
	if projected["a.b"] != 1 || projected["x"] != 2 {
		t.Fatalf("unexpected projection: %+v", projected)
	}
	projectedArr := projectFields([]any{map[string]any{"x": 1}}, []string{"x"}).([]any)
	if len(projectedArr) != 1 {
		t.Fatalf("unexpected projected array")
	}
	if passthrough := projectFields(map[string]any{"z": 1}, nil).(map[string]any); passthrough["z"] != 1 {
		t.Fatalf("expected passthrough projection")
	}
	if scalar := projectFields("value", []string{"x"}); scalar != "value" {
		t.Fatalf("expected scalar projection passthrough")
	}
	if _, ok := lookupPath(map[string]any{"a": map[string]any{"b": 1}}, []string{"a", "b"}); !ok {
		t.Fatalf("lookupPath should find value")
	}
	if _, ok := lookupPath(map[string]any{"a": map[string]any{"b": 1}}, []string{"a", "c"}); ok {
		t.Fatalf("lookupPath should fail for missing key")
	}
	if _, ok := lookupPath(map[string]any{"a": 1}, []string{"a", "b"}); ok {
		t.Fatalf("lookupPath should fail")
	}
	if maxInt(3, 2) != 3 || maxInt(1, 2) != 2 {
		t.Fatalf("unexpected max results")
	}
	if parseInt("10", 5) != 10 || parseInt("bad", 5) != 5 {
		t.Fatalf("unexpected parseInt result")
	}
	if minInt(3, 2) != 2 || minInt(1, 2) != 1 {
		t.Fatalf("unexpected min results")
	}
	if appendQuery("/api/test", nil) != "/api/test" {
		t.Fatalf("expected empty query append to preserve path")
	}
	if got := appendQuery("/api/test", url.Values{"q": []string{"x"}}); got != "/api/test?q=x" {
		t.Fatalf("unexpected query append: %s", got)
	}
	if got := normalizeQueryHistoryBound(time.Date(2026, 3, 5, 15, 4, 0, 0, time.UTC), "30m"); got != "1772721240000" {
		t.Fatalf("unexpected query-history bound normalization: %s", got)
	}
	if got := normalizeQueryHistoryBound(time.Date(2026, 3, 5, 15, 4, 0, 0, time.UTC), "1700000000"); got != "1700000000" {
		t.Fatalf("expected unix passthrough, got %s", got)
	}
	if meta := queryHistoryMetadata(map[string]any{"result": map[string]any{"queryHistory": []any{1, 2}, "totalCount": 4}}, 2); meta.Count == nil || *meta.Count != 2 || !meta.Truncated {
		t.Fatalf("unexpected query-history metadata: %+v", meta)
	}
	if meta := queryHistoryMetadata(map[string]any{"result": map[string]any{}}, 1); len(meta.Warnings) != 1 {
		t.Fatalf("expected query-history warnings, got %+v", meta)
	}
	filteredPayload, count, truncated := filterNamedPayload([]any{
		map[string]any{"name": "Checkout Availability", "description": "Success"},
		map[string]any{"name": "Payments Availability", "description": "Payments"},
		map[string]any{"name": "Checkout Latency", "description": "Latency"},
	}, "checkout", 1, "name", "description")
	filteredItems := filteredPayload.([]any)
	if count != 2 || !truncated || len(filteredItems) != 1 {
		t.Fatalf("unexpected filtered payload result: count=%d truncated=%v payload=%+v", count, truncated, filteredPayload)
	}
	payloadMap, count, truncated := filterNamedPayload(map[string]any{"slos": []any{
		map[string]any{"name": "Checkout", "description": "Budget"},
	}}, "", 10, "name")
	if count != 1 || truncated || payloadMap.(map[string]any)["slos"] == nil {
		t.Fatalf("unexpected wrapped filtered payload: count=%d truncated=%v payload=%+v", count, truncated, payloadMap)
	}
	resultsPayload, count, truncated := filterNamedPayload(map[string]any{"results": []any{
		map[string]any{"name": "Checkout"},
		map[string]any{"name": "Payments"},
	}}, "checkout", 10, "name")
	if count != 1 || truncated || len(resultsPayload.(map[string]any)["results"].([]any)) != 1 {
		t.Fatalf("unexpected results wrapped filtered payload: count=%d truncated=%v payload=%+v", count, truncated, resultsPayload)
	}
	if _, _, ok := collectionPayload("scalar"); ok {
		t.Fatalf("unexpected collection payload for scalar")
	}
	if passthroughPayload, passthroughCount, passthroughTruncated := filterNamedPayload(map[string]any{"other": "value"}, "checkout", 5, "name"); passthroughCount != 0 || passthroughTruncated || passthroughPayload.(map[string]any)["other"] != "value" {
		t.Fatalf("unexpected passthrough filtered payload: count=%d truncated=%v payload=%+v", passthroughCount, passthroughTruncated, passthroughPayload)
	}
	if filteredPayload, count, truncated := filterNamedPayload([]any{"bad", map[string]any{"name": "Checkout"}}, "checkout", 10, "name"); count != 1 || truncated || len(filteredPayload.([]any)) != 1 {
		t.Fatalf("unexpected mixed filtered payload: count=%d truncated=%v payload=%+v", count, truncated, filteredPayload)
	}
	if !matchesAnyField(map[string]any{"name": "Checkout"}, "check", "name") {
		t.Fatalf("expected matchesAnyField to match")
	}
	if !matchesAnyField(map[string]any{"name": "Checkout"}, "", "name") {
		t.Fatalf("expected empty query to match")
	}
	if matchesAnyField(map[string]any{"name": "Checkout"}, "payments", "name") {
		t.Fatalf("expected matchesAnyField to miss")
	}
	if got := collectionAtPath(map[string]any{"data": map[string]any{"result": []any{1, 2}}}, "data", "result"); len(got) != 2 {
		t.Fatalf("unexpected collectionAtPath result: %+v", got)
	}
	if got := collectionAtPath("scalar", "data"); got != nil {
		t.Fatalf("expected nil collectionAtPath for scalar")
	}
	if got, ok := intPath(map[string]any{"result": map[string]any{"totalCount": float64(4)}}, "result", "totalCount"); !ok || got != 4 {
		t.Fatalf("unexpected intPath result: got=%d ok=%v", got, ok)
	}
	if _, ok := intPath("scalar", "x"); ok {
		t.Fatalf("unexpected intPath success for scalar")
	}
	if got, ok := intValue(json.Number("4")); !ok || got != 4 {
		t.Fatalf("unexpected intValue result: got=%d ok=%v", got, ok)
	}
	if got, ok := intValue(int64(6)); !ok || got != 6 {
		t.Fatalf("unexpected intValue int64 result: got=%d ok=%v", got, ok)
	}
	if _, ok := intValue(json.Number("bad")); ok {
		t.Fatalf("unexpected intValue success for invalid json number")
	}
	if _, ok := intValue("bad"); ok {
		t.Fatalf("unexpected intValue success for string")
	}
	if got := firstNonEmptyString(map[string]any{"a": "", "b": "checkout"}, "a", "b"); got != "checkout" {
		t.Fatalf("unexpected firstNonEmptyString result: %s", got)
	}
	if got := firstNonEmptyString(map[string]any{"a": ""}, "missing", "a"); got != "" {
		t.Fatalf("expected empty firstNonEmptyString result, got %q", got)
	}
	if got := sortedSet(map[string]struct{}{"b": {}, "a": {}}); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("unexpected sortedSet result: %+v", got)
	}
	logSummary := summarizeLogsResult(map[string]any{"data": map[string]any{"result": []any{
		"bad",
		map[string]any{"stream": map[string]any{"app": "checkout"}, "values": []any{1, 2}},
	}}})
	if logSummary["streams"].(int) != 1 || logSummary["entries"].(int) != 2 {
		t.Fatalf("unexpected log summary: %+v", logSummary)
	}
	traceSummary := summarizeTracesResult(map[string]any{"data": map[string]any{"traces": []any{
		"bad",
		map[string]any{"traceID": "t-1", "rootServiceName": "checkout", "rootTraceName": "GET /checkout"},
	}}})
	if traceSummary["trace_matches"].(int) != 1 || traceSummary["services"].([]string)[0] != "checkout" {
		t.Fatalf("unexpected trace summary: %+v", traceSummary)
	}
	if meta := runtimeAggregateMetadata("runtime logs aggregate", 2); meta.Command != "runtime logs aggregate" || meta.Count == nil || *meta.Count != 2 {
		t.Fatalf("unexpected runtime aggregate metadata: %+v", meta)
	}
	if !payloadHasNextPage(map[string]any{"next": "https://next"}) {
		t.Fatalf("expected payloadHasNextPage to detect next page")
	}
	if !payloadHasNextPage(map[string]any{"metadata": map[string]any{"pagination": map[string]any{"nextPage": "/v1/accesspolicies?pageCursor=abc"}}}) {
		t.Fatalf("expected payloadHasNextPage to detect nested next page")
	}
	if payloadHasNextPage([]any{1}) {
		t.Fatalf("expected payloadHasNextPage to ignore non-map payloads")
	}
	if got := mapValue(map[string]any{"a": map[string]any{"b": map[string]any{"c": "d"}}}, "a", "b")["c"]; got != "d" {
		t.Fatalf("unexpected mapValue result: %+v", got)
	}
	if mapValue(map[string]any{"a": 1}, "a") != nil {
		t.Fatalf("expected nil mapValue for non-map leaf")
	}
	if mapValue("scalar", "a") != nil {
		t.Fatalf("expected nil mapValue for non-map root")
	}
	if !strings.Contains(investigationPrompt("Investigate checkout latency"), "Goal: Investigate checkout latency") {
		t.Fatalf("expected investigationPrompt to embed the goal")
	}
	t.Setenv("GRAFANA_SYNTHETICS_BACKEND_URL", "synthetic-monitoring-api-us-east-0.grafana.net")
	t.Setenv("GRAFANA_SYNTHETICS_TOKEN", "sm-token")
	if backendURL, token, err := resolveSyntheticsAuth("", ""); err != nil || backendURL != "synthetic-monitoring-api-us-east-0.grafana.net" || token != "sm-token" {
		t.Fatalf("unexpected resolved synthetics auth: backend=%s token=%s err=%v", backendURL, token, err)
	}
	if _, _, err := resolveSyntheticsAuth("", ""); err != nil {
		// keep env-backed path covered before clearing for the negative assertions below
		t.Fatalf("expected env-backed resolveSyntheticsAuth to succeed: %v", err)
	}
	t.Setenv("GRAFANA_SYNTHETICS_BACKEND_URL", "")
	t.Setenv("GRAFANA_SYNTHETICS_TOKEN", "")
	if _, _, err := resolveSyntheticsAuth("", ""); err == nil {
		t.Fatalf("expected missing synthetics auth error")
	}
	if _, err := syntheticCheckGetRequest("", "", 7); err == nil {
		t.Fatalf("expected syntheticCheckGetRequest error when auth is missing")
	}
}

func TestAuthInferenceHelpers(t *testing.T) {
	slug, baseURL, err := normalizeStackIdentifier("https://prod-observability.grafana.net")
	if err != nil || slug != "prod-observability" || baseURL != "https://prod-observability.grafana.net" {
		t.Fatalf("unexpected stack identifier parse from url: slug=%s base=%s err=%v", slug, baseURL, err)
	}
	slug, baseURL, err = normalizeStackIdentifier("prod-observability.grafana.net")
	if err != nil || slug != "prod-observability" || baseURL != "https://prod-observability.grafana.net" {
		t.Fatalf("unexpected stack identifier parse from host: slug=%s base=%s err=%v", slug, baseURL, err)
	}
	slug, baseURL, err = normalizeStackIdentifier("prod-observability")
	if err != nil || slug != "prod-observability" || baseURL != "https://prod-observability.grafana.net" {
		t.Fatalf("unexpected stack identifier parse from slug: slug=%s base=%s err=%v", slug, baseURL, err)
	}
	if _, _, err := normalizeStackIdentifier("https://example.com"); err == nil {
		t.Fatalf("expected invalid stack host error")
	}
	if _, _, err := normalizeStackIdentifier("example.com"); err == nil {
		t.Fatalf("expected invalid stack host without scheme error")
	}
	if _, _, err := normalizeStackIdentifier("https://%"); err == nil {
		t.Fatalf("expected invalid stack url parse error")
	}
	if _, _, err := normalizeStackIdentifier(""); err == nil {
		t.Fatalf("expected empty stack error")
	}
	var target cloudStackTarget
	if err := target.Set("prod-observability.grafana.net"); err != nil {
		t.Fatalf("unexpected cloud stack target parse error: %v", err)
	}
	if target.Slug != "prod-observability" || target.BaseURL != "https://prod-observability.grafana.net" {
		t.Fatalf("unexpected cloud stack target: %+v", target)
	}
	if target.String() != "https://prod-observability.grafana.net" {
		t.Fatalf("unexpected cloud stack target string: %s", target.String())
	}
	if required, err := target.required(); err != nil || required.Slug != "prod-observability" {
		t.Fatalf("unexpected required cloud stack target: %+v err=%v", required, err)
	}
	var emptyTarget cloudStackTarget
	if _, err := emptyTarget.required(); err == nil {
		t.Fatalf("expected empty required cloud stack target error")
	}
	if err := emptyTarget.Set("https://example.com"); err == nil {
		t.Fatalf("expected invalid cloud stack target error")
	}
	fs, parsedTarget := newCloudStackFlagSet("cloud stacks test")
	if err := fs.Parse([]string{"--stack", "https://prod-observability.grafana.net"}); err != nil {
		t.Fatalf("unexpected cloud stack flag parse error: %v", err)
	}
	if parsedTarget.Slug != "prod-observability" || parsedTarget.BaseURL != "https://prod-observability.grafana.net" {
		t.Fatalf("unexpected parsed cloud stack target: %+v", parsedTarget)
	}
	if size := cloudCollectionPageSize(0, 0, 0); size != 100 {
		t.Fatalf("unexpected default cloud collection page size: %d", size)
	}
	if size := cloudCollectionPageSize(1, 1, 50); size != 1 {
		t.Fatalf("unexpected exhausted cloud collection page size: %d", size)
	}
	if size := cloudCollectionPageSize(250, 100, 200); size != 150 {
		t.Fatalf("unexpected remaining cloud collection page size: %d", size)
	}
	if size := cloudCollectionPageSize(250, 100, 50); size != 50 {
		t.Fatalf("unexpected requested cloud collection page size: %d", size)
	}
	if cursor := cloudNextPageCursor("scalar"); cursor != "" {
		t.Fatalf("expected empty cloud next page cursor for scalar payload, got %q", cursor)
	}
	if cursor := cloudNextPageCursor(map[string]any{"next": "/api/instances/local-stack/plugins?pageCursor=cursor-1"}); cursor != "cursor-1" {
		t.Fatalf("unexpected cloud next page cursor from root next: %q", cursor)
	}
	if cursor := cloudNextPageCursor(map[string]any{"metadata": map[string]any{"pagination": map[string]any{"nextPage": "/api/instances/local-stack/plugins?cursor=cursor-2"}}}); cursor != "cursor-2" {
		t.Fatalf("unexpected cloud next page cursor from nested nextPage: %q", cursor)
	}
	if cursor := cloudNextPageCursor(map[string]any{"items": []any{}}); cursor != "" {
		t.Fatalf("expected empty cloud next page cursor without pagination, got %q", cursor)
	}
	if cursor := cloudPageCursorValue("cursor-raw"); cursor != "cursor-raw" {
		t.Fatalf("unexpected raw cloud page cursor value: %q", cursor)
	}
	if cursor := cloudPageCursorValue("://bad"); cursor != "://bad" {
		t.Fatalf("unexpected invalid-url cloud page cursor value: %q", cursor)
	}

	endpoints := inferDatasourceEndpoints([]any{
		map[string]any{"type": "prometheus", "url": "https://prom"},
		map[string]any{"type": "loki", "url": "https://logs"},
		map[string]any{"type": "tempo", "url": "https://traces"},
	})
	if endpoints.PrometheusURL != "https://prom" || endpoints.LogsURL != "https://logs" || endpoints.TracesURL != "https://traces" {
		t.Fatalf("unexpected datasource endpoint inference: %+v", endpoints)
	}
	if endpoints := inferDatasourceEndpoints([]any{"bad", map[string]any{"type": "prometheus"}}); endpoints.PrometheusURL != "" {
		t.Fatalf("expected datasource inference to ignore invalid entries, got %+v", endpoints)
	}
	if endpoints := inferDatasourceEndpoints("scalar"); endpoints.PrometheusURL != "" || endpoints.LogsURL != "" || endpoints.TracesURL != "" {
		t.Fatalf("expected empty datasource inference for scalar payload, got %+v", endpoints)
	}
	stackList := map[string]any{"items": []any{
		map[string]any{"slug": "prod-observability", "region": "us"},
	}}
	if stack, ok := cloudStackBySlug(stackList, "prod-observability"); !ok || stack["region"] != "us" {
		t.Fatalf("unexpected cloud stack lookup: ok=%v stack=%+v", ok, stack)
	}
	if _, ok := cloudStackBySlug(map[string]any{"items": []any{"bad"}}, "prod-observability"); ok {
		t.Fatalf("expected cloud stack lookup to ignore invalid records")
	}
	if _, ok := cloudStackBySlug("scalar", "prod-observability"); ok {
		t.Fatalf("expected cloud stack lookup to fail for scalar payload")
	}
	if oncallURL := inferOnCallURL(map[string]any{"connections": []any{
		map[string]any{"type": "oncall", "details": map[string]any{"oncallApiUrl": "https://oncall"}},
	}}); oncallURL != "https://oncall" {
		t.Fatalf("unexpected oncall url inference: %s", oncallURL)
	}
	if oncallURL := inferOnCallURL(map[string]any{"oncallApiUrl": "https://root-oncall"}); oncallURL != "https://root-oncall" {
		t.Fatalf("unexpected top-level oncall url inference: %s", oncallURL)
	}
	if oncallURL := inferOnCallURL(map[string]any{"results": []any{
		"bad",
		map[string]any{"type": "oncall", "details": map[string]any{"oncallApiUrl": "https://record-oncall"}},
	}}); oncallURL != "https://record-oncall" {
		t.Fatalf("unexpected oncall url inference from record scan: %s", oncallURL)
	}
	if oncallURL := inferOnCallURL(map[string]any{"connections": []any{
		map[string]any{"type": "oncall", "url": "https://fallback-oncall"},
	}}); oncallURL != "https://fallback-oncall" {
		t.Fatalf("unexpected oncall fallback url inference: %s", oncallURL)
	}
	if oncallURL := inferOnCallURL(map[string]any{"results": []any{
		map[string]any{"type": "pagerduty", "url": "https://pagerduty"},
	}}); oncallURL != "" {
		t.Fatalf("expected empty oncall inference for unrelated payload, got %s", oncallURL)
	}
	if oncallURL := inferOnCallURL("scalar"); oncallURL != "" {
		t.Fatalf("expected empty oncall inference for scalar payload, got %s", oncallURL)
	}
	if value := recursiveStringValue(map[string]any{"a": []any{map[string]any{"oncallApiUrl": "https://oncall"}}}, "oncallApiUrl"); value != "https://oncall" {
		t.Fatalf("unexpected recursive string value: %s", value)
	}
	if !containsAny("grafana-oncall", "oncall", "schedules") {
		t.Fatalf("expected containsAny match")
	}
	if containsAny("grafana-runtime", "oncall", "schedules") {
		t.Fatalf("expected containsAny to miss")
	}
	connectionSummary := cloudStackConnectivitySummary(map[string]any{
		"connections": []any{
			map[string]any{"type": "oncall"},
			map[string]any{"type": "pagerduty"},
		},
		"privateConnectivityInfo": map[string]any{
			"tenants": []any{
				map[string]any{"type": "prometheus"},
				map[string]any{"type": "logs"},
			},
		},
	})
	if connectionSummary["has_private_connectivity"] != true || len(connectionSummary["connection_types"].([]string)) != 2 {
		t.Fatalf("unexpected cloud connectivity summary: %+v", connectionSummary)
	}
	if summary := cloudStackConnectivitySummary(map[string]any{"connections": []any{"bad"}}); summary["has_private_connectivity"] != false {
		t.Fatalf("expected cloud connectivity summary without private tenants: %+v", summary)
	}
	if summary := cloudStackConnectivitySummary(map[string]any{
		"privateConnectivityInfo": map[string]any{"tenants": []any{"bad"}},
	}); summary["has_private_connectivity"] != false {
		t.Fatalf("expected cloud connectivity summary to ignore invalid private tenant records: %+v", summary)
	}
	if items := cloudStackDatasourceItems("scalar"); items != nil {
		t.Fatalf("expected nil cloud datasource items for scalar payload")
	}
	if items := cloudStackConnectionItems(map[string]any{"connections": []any{map[string]any{"type": "oncall"}}}); len(items) != 1 {
		t.Fatalf("unexpected cloud connection items: %+v", items)
	}
	if items := cloudStackConnectionItems(map[string]any{"items": []any{map[string]any{"type": "oncall"}}}); len(items) != 1 {
		t.Fatalf("unexpected cloud connection items from collection payload: %+v", items)
	}
	inspectPayload := buildCloudStackInspectPayload(
		map[string]any{"slug": "prod-observability"},
		"https://prod-observability.grafana.net",
		map[string]any{"items": []any{
			map[string]any{"uid": "prom", "name": "metrics", "type": "prometheus", "url": "https://prom"},
		}},
		map[string]any{"connections": []any{
			map[string]any{"type": "oncall", "details": map[string]any{"oncallApiUrl": "https://oncall"}},
		}},
		true,
	)
	if inspectPayload["inferred_endpoints"].(map[string]any)["oncall_url"] != "https://oncall" || len(inspectPayload["datasources"].([]any)) != 1 {
		t.Fatalf("unexpected cloud inspect payload: %+v", inspectPayload)
	}
	billingPayload := buildCloudBilledUsagePayload("local-org", 2024, 9, map[string]any{
		"items": []any{
			map[string]any{
				"dimensionName": "Logs",
				"amountDue":     100.5,
				"periodStart":   "2024-09-01T00:00:00Z",
				"periodEnd":     "2024-09-30T23:59:59Z",
				"usages": []any{
					map[string]any{"stackName": "stack-a"},
					"bad",
				},
			},
			"bad",
		},
	})
	if billingPayload["org_slug"] != "local-org" || billingPayload["summary"].(map[string]any)["stack_count"] != 1 {
		t.Fatalf("unexpected cloud billed usage payload: %+v", billingPayload)
	}
	if summary := cloudBilledUsageSummary(nil); summary["items"] != 0 || summary["total_amount_due"] != 0.0 {
		t.Fatalf("unexpected empty cloud billed usage summary: %+v", summary)
	}

	app := NewApp(&fakeStore{})
	fake := &fakeClient{
		rawResponses: map[string]any{
			"/api/instances/prod-observability/datasources": []any{map[string]any{"type": "prometheus", "url": "https://prom"}},
		},
		rawErrors: map[string]error{
			"/api/instances/prod-observability/connections": errors.New("connections failed"),
		},
	}
	app.NewClient = func(config.Config) APIClient { return fake }
	cfg := &config.Config{CloudURL: "https://grafana.com", Token: "token"}
	warnings := app.applyInferredStackEndpoints(context.Background(), cfg, "prod-observability", "")
	if len(warnings) != 1 || cfg.PrometheusURL != "https://prom" {
		t.Fatalf("unexpected inferred stack endpoint warnings or config: warnings=%+v cfg=%+v", warnings, cfg)
	}

	paginatedClient := &fakeClient{
		cloudStackPluginPage: map[string]any{
			"": map[string]any{
				"items": []any{
					"bad",
					map[string]any{"id": "grafana-oncall-app", "name": "Grafana OnCall"},
					map[string]any{"id": "grafana-incident-app", "name": "Grafana IRM"},
				},
			},
		},
	}
	paginatedPayload, count, truncated, err := app.listCloudStackPlugins(context.Background(), paginatedClient, "prod-observability", "", 1)
	if err != nil || count != 1 || truncated != true || len(paginatedPayload.(map[string]any)["items"].([]any)) != 1 {
		t.Fatalf("unexpected paginated plugin list result: payload=%+v count=%d truncated=%v err=%v", paginatedPayload, count, truncated, err)
	}

	nextPageClient := &fakeClient{
		cloudStackPluginPage: map[string]any{
			"": map[string]any{
				"items": []any{
					map[string]any{"id": "grafana-oncall-app", "name": "Grafana OnCall"},
				},
				"metadata": map[string]any{"pagination": map[string]any{"nextPage": "/api/instances/prod-observability/plugins?pageCursor=cursor-2"}},
			},
		},
	}
	_, count, truncated, err = app.listCloudStackPlugins(context.Background(), nextPageClient, "prod-observability", "", 1)
	if err != nil || count != 1 || truncated != true {
		t.Fatalf("unexpected next-page plugin list result: count=%d truncated=%v err=%v", count, truncated, err)
	}

	scalarClient := &fakeClient{
		cloudStackPlugins: map[string]any{
			"id":       "grafana-oncall-app",
			"metadata": map[string]any{"pagination": map[string]any{"nextPage": "/api/instances/prod-observability/plugins?pageCursor=cursor-3"}},
		},
	}
	scalarPayload, count, truncated, err := app.listCloudStackPlugins(context.Background(), scalarClient, "prod-observability", "", 1)
	if err != nil || count != 0 || truncated != true || scalarPayload.(map[string]any)["id"] != "grafana-oncall-app" {
		t.Fatalf("unexpected scalar plugin list result: payload=%+v count=%d truncated=%v err=%v", scalarPayload, count, truncated, err)
	}

	errorClient := &fakeClient{cloudStackPluginsErr: errors.New("plugins failed")}
	if _, _, _, err := app.listCloudStackPlugins(context.Background(), errorClient, "prod-observability", "", 1); err == nil {
		t.Fatalf("expected plugin list page error")
	}

	accessClient := &fakeClient{
		cloudAccessPages: map[string]any{
			"": map[string]any{
				"items": []any{
					map[string]any{"id": "ap-1", "name": "stack-readers"},
				},
				"metadata": map[string]any{"pagination": map[string]any{"nextPage": "/api/v1/accesspolicies?pageCursor=cursor-2"}},
			},
			"cursor-2": map[string]any{
				"items": []any{
					map[string]any{"id": "ap-2", "name": "stack-writers"},
				},
			},
		},
	}
	accessPayload, count, truncated, err := app.listCloudAccessPolicies(context.Background(), accessClient, grafana.CloudAccessPolicyListRequest{
		Region:   "us",
		PageSize: 50,
	}, 2)
	if err != nil || count != 2 || truncated {
		t.Fatalf("unexpected access policy list result: payload=%+v count=%d truncated=%v err=%v", accessPayload, count, truncated, err)
	}
	if len(accessPayload.(map[string]any)["items"].([]any)) != 2 {
		t.Fatalf("expected both access policy pages, got %+v", accessPayload)
	}
	if accessClient.cloudAccessReq.PageCursor != "cursor-2" || accessClient.cloudAccessReq.PageSize != 1 {
		t.Fatalf("unexpected paged access policy request: %+v", accessClient.cloudAccessReq)
	}

	accessPayload, count, truncated, err = app.listCloudAccessPolicies(context.Background(), accessClient, grafana.CloudAccessPolicyListRequest{
		Region:   "us",
		PageSize: 50,
	}, 1)
	if err != nil || count != 1 || !truncated {
		t.Fatalf("unexpected truncated access policy result: payload=%+v count=%d truncated=%v err=%v", accessPayload, count, truncated, err)
	}

	scalarPayload, count, truncated, err = app.listCloudCollection(context.Background(), func(context.Context, int, string) (any, error) {
		return map[string]any{
			"id":       "policy-1",
			"metadata": map[string]any{"pagination": map[string]any{"nextPage": "/api/v1/accesspolicies?pageCursor=cursor-3"}},
		}, nil
	}, cloudListOptions{Limit: 10})
	if err != nil || count != 0 || !truncated || scalarPayload.(map[string]any)["id"] != "policy-1" {
		t.Fatalf("unexpected scalar cloud collection result: payload=%+v count=%d truncated=%v err=%v", scalarPayload, count, truncated, err)
	}

	filteredPayload, count, truncated, err := app.listCloudCollection(context.Background(), func(context.Context, int, string) (any, error) {
		return map[string]any{
			"items": []any{
				map[string]any{"id": "skip"},
				map[string]any{"id": "keep"},
			},
		}, nil
	}, cloudListOptions{
		Limit: 10,
		Include: func(record map[string]any) bool {
			return record["id"] == "keep"
		},
	})
	if err != nil || count != 1 || truncated {
		t.Fatalf("unexpected filtered cloud collection result: payload=%+v count=%d truncated=%v err=%v", filteredPayload, count, truncated, err)
	}
	if len(filteredPayload.(map[string]any)["items"].([]any)) != 1 {
		t.Fatalf("expected one filtered cloud collection item, got %+v", filteredPayload)
	}

	meta := accessPolicyMetadata(0, false)
	if meta.Count != nil || meta.Truncated {
		t.Fatalf("unexpected non-truncated access policy metadata: %+v", meta)
	}
	meta = accessPolicyMetadata(1, true)
	if meta.Count == nil || *meta.Count != 1 || !meta.Truncated || meta.NextAction == "" {
		t.Fatalf("unexpected truncated access policy metadata: %+v", meta)
	}
}

func TestContextAndConfigCommands(t *testing.T) {
	store := &fakeContextStore{
		current: "default",
		cfgs: map[string]config.Config{
			"default": {
				Token:         "default-token",
				BaseURL:       "https://default.grafana.net",
				CloudURL:      "https://grafana.com",
				PrometheusURL: "https://prom-default.grafana.net",
				LogsURL:       "https://logs-default.grafana.net",
				TracesURL:     "https://traces-default.grafana.net",
				OnCallURL:     "https://oncall-default.grafana.net",
				OrgID:         1,
			},
			"prod": {
				Token:         "prod-token",
				BaseURL:       "https://prod.grafana.net",
				CloudURL:      "https://grafana.com",
				PrometheusURL: "https://prom-prod.grafana.net",
				LogsURL:       "https://logs-prod.grafana.net",
				TracesURL:     "https://traces-prod.grafana.net",
				OnCallURL:     "https://oncall-prod.grafana.net",
				OrgID:         2,
			},
		},
	}
	app, out, errOut := newTestApp(store, &fakeClient{})

	if code := app.Run(context.Background(), []string{"context", "-help"}); code != 0 {
		t.Fatalf("context help should succeed: %s", errOut.String())
	}
	helpResp := decodeJSON(t, out.String())
	if _, ok := helpResp["commands"]; !ok {
		t.Fatalf("expected context command list")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"context", "list"}); code != 0 {
		t.Fatalf("context list should succeed: %s", errOut.String())
	}
	items := decodeJSONArray(t, out.String())
	if len(items) != 2 {
		t.Fatalf("expected two contexts, got %+v", items)
	}
	foundDefault := false
	foundProd := false
	for _, item := range items {
		switch item["name"] {
		case "default":
			foundDefault = item["current"] == true
		case "prod":
			foundProd = item["authenticated"] == true
		}
	}
	if !foundDefault || !foundProd {
		t.Fatalf("unexpected context list payload: %+v", items)
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"context", "view"}); code != 0 {
		t.Fatalf("context view should succeed: %s", errOut.String())
	}
	view := decodeJSON(t, out.String())
	if view["context"] != "default" || view["base_url"] != "https://default.grafana.net" {
		t.Fatalf("unexpected context view payload: %+v", view)
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"context", "use", "prod"}); code != 0 {
		t.Fatalf("context use should succeed: %s", errOut.String())
	}
	view = decodeJSON(t, out.String())
	if view["context"] != "prod" || view["base_url"] != "https://prod.grafana.net" {
		t.Fatalf("unexpected context use payload: %+v", view)
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"config", "list"}); code != 0 {
		t.Fatalf("config list should succeed: %s", errOut.String())
	}
	listResp := decodeJSON(t, out.String())
	if listResp["context"] != "prod" || listResp["org_id"] != float64(2) {
		t.Fatalf("unexpected config list payload: %+v", listResp)
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"config", "get", "base-url"}); code != 0 {
		t.Fatalf("config get should succeed: %s", errOut.String())
	}
	getResp := decodeJSON(t, out.String())
	if getResp["context"] != "prod" || getResp["key"] != "base-url" || getResp["value"] != "https://prod.grafana.net" {
		t.Fatalf("unexpected config get payload: %+v", getResp)
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"config", "set", "base-url", "https://prod-updated.grafana.net"}); code != 0 {
		t.Fatalf("config set should succeed: %s", errOut.String())
	}
	setResp := decodeJSON(t, out.String())
	if setResp["base_url"] != "https://prod-updated.grafana.net" || store.cfgs["prod"].BaseURL != "https://prod-updated.grafana.net" {
		t.Fatalf("unexpected config set result: %+v store=%+v", setResp, store.cfgs["prod"])
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"config", "get", "--context", "default", "org-id"}); code != 0 {
		t.Fatalf("config get with context should succeed: %s", errOut.String())
	}
	getResp = decodeJSON(t, out.String())
	if getResp["context"] != "default" || getResp["value"] != float64(1) {
		t.Fatalf("unexpected config get with context payload: %+v", getResp)
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"config", "get", "base-url", "--context", "default"}); code != 0 {
		t.Fatalf("config get with trailing context should succeed: %s", errOut.String())
	}
	getResp = decodeJSON(t, out.String())
	if getResp["context"] != "default" || getResp["value"] != "https://default.grafana.net" {
		t.Fatalf("unexpected trailing context get payload: %+v", getResp)
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"config", "set", "--context", "default", "org-id", "9"}); code != 0 {
		t.Fatalf("config set with context should succeed: %s", errOut.String())
	}
	setResp = decodeJSON(t, out.String())
	if setResp["context"] != "default" || setResp["org_id"] != float64(9) || store.cfgs["default"].OrgID != 9 {
		t.Fatalf("unexpected context-specific config set payload: %+v store=%+v", setResp, store.cfgs["default"])
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"config", "set", "org-id", "10", "--context", "default"}); code != 0 {
		t.Fatalf("config set with trailing context should succeed: %s", errOut.String())
	}
	setResp = decodeJSON(t, out.String())
	if setResp["context"] != "default" || setResp["org_id"] != float64(10) || store.cfgs["default"].OrgID != 10 {
		t.Fatalf("unexpected trailing context set payload: %+v store=%+v", setResp, store.cfgs["default"])
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"auth", "login", "--context", "ops", "--token", "ops-token", "--base-url", "https://ops.grafana.net"}); code != 0 {
		t.Fatalf("auth login with context should succeed: %s", errOut.String())
	}
	loginResp := decodeJSON(t, out.String())
	if loginResp["context"] != "ops" || store.current != "ops" {
		t.Fatalf("unexpected context auth login payload: %+v current=%s", loginResp, store.current)
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"auth", "status"}); code != 0 {
		t.Fatalf("auth status should succeed: %s", errOut.String())
	}
	status := decodeJSON(t, out.String())
	if status["context"] != "ops" || status["status"] != "authenticated" || status["capabilities"] == nil {
		t.Fatalf("unexpected auth status: %+v", status)
	}
}

func TestContextAndConfigErrors(t *testing.T) {
	client := &fakeClient{}

	app, _, errOut := newTestApp(&fakeStore{}, client)
	if code := app.Run(context.Background(), []string{"context", "list"}); code != 1 {
		t.Fatalf("expected context support failure")
	}
	if !strings.Contains(errOut.String(), "context support is unavailable") {
		t.Fatalf("expected context support error, got %s", errOut.String())
	}

	errOut.Reset()
	if code := app.Run(context.Background(), []string{"auth", "login", "--context", "prod", "--token", "x"}); code != 1 {
		t.Fatalf("expected auth context failure")
	}
	if !strings.Contains(errOut.String(), "context support is unavailable") {
		t.Fatalf("expected auth context support error, got %s", errOut.String())
	}

	errOut.Reset()
	if code := app.Run(context.Background(), []string{"config", "get", "base-url", "--context", "prod"}); code != 1 {
		t.Fatalf("expected config context failure with trailing flag order")
	}
	if !strings.Contains(errOut.String(), "context support is unavailable") {
		t.Fatalf("expected trailing context support error, got %s", errOut.String())
	}

	errOut.Reset()
	if code := app.Run(context.Background(), []string{"config", "get", "--context", "prod", "base-url"}); code != 1 {
		t.Fatalf("expected config context failure")
	}
	if !strings.Contains(errOut.String(), "context support is unavailable") {
		t.Fatalf("expected config context support error, got %s", errOut.String())
	}

	for _, args := range [][]string{
		{"config", "list", "--context"},
		{"config", "get", "--context"},
		{"config", "set", "--context"},
	} {
		errOut.Reset()
		if code := app.Run(context.Background(), args); code != 1 {
			t.Fatalf("expected missing context value failure for %v", args)
		}
		if !strings.Contains(errOut.String(), "--context requires a value") {
			t.Fatalf("expected missing context value error for %v, got %s", args, errOut.String())
		}
	}

	store := &fakeContextStore{cfgs: map[string]config.Config{"default": {Token: "token"}}}
	app, _, errOut = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"context", "use", "prod"}); code != 1 {
		t.Fatalf("expected missing context failure")
	}
	if !strings.Contains(errOut.String(), "context not found") {
		t.Fatalf("expected missing context error, got %s", errOut.String())
	}

	errOut.Reset()
	if code := app.Run(context.Background(), []string{"context", "use"}); code != 1 {
		t.Fatalf("expected usage failure")
	}
	if !strings.Contains(errOut.String(), "usage: context use <NAME>") {
		t.Fatalf("expected context use usage error, got %s", errOut.String())
	}

	errOut.Reset()
	if code := app.Run(context.Background(), []string{"context", "view", "prod"}); code != 1 {
		t.Fatalf("expected context view usage failure")
	}
	if !strings.Contains(errOut.String(), "usage: context view") {
		t.Fatalf("expected context view usage error, got %s", errOut.String())
	}

	errOut.Reset()
	if code := app.Run(context.Background(), []string{"context", "bad"}); code != 1 {
		t.Fatalf("expected unknown context command failure")
	}
	if !strings.Contains(errOut.String(), "unknown context command") {
		t.Fatalf("expected unknown context command error, got %s", errOut.String())
	}

	store.listErr = errors.New("list fail")
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"context", "list"}); code != 1 {
		t.Fatalf("expected list error")
	}
	if !strings.Contains(errOut.String(), "list fail") {
		t.Fatalf("expected list error, got %s", errOut.String())
	}

	store.listErr = nil
	store.loadErr = errors.New("load fail")
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"context", "view"}); code != 1 {
		t.Fatalf("expected view load failure")
	}
	if !strings.Contains(errOut.String(), "load fail") {
		t.Fatalf("expected view load error, got %s", errOut.String())
	}

	store.loadErr = nil
	store.useErr = errors.New("use fail")
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"context", "use", "default"}); code != 1 {
		t.Fatalf("expected use error")
	}
	if !strings.Contains(errOut.String(), "use fail") {
		t.Fatalf("expected use error, got %s", errOut.String())
	}

	store.useErr = nil
	store.loadErr = errors.New("reload fail")
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"context", "use", "default"}); code != 1 {
		t.Fatalf("expected reload failure")
	}
	if !strings.Contains(errOut.String(), "reload fail") {
		t.Fatalf("expected reload error after context use, got %s", errOut.String())
	}

	store.loadErr = nil
	store.loadCtxErr = errors.New("load context fail")
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"config", "list", "--context", "default"}); code != 1 {
		t.Fatalf("expected config load context failure")
	}
	if !strings.Contains(errOut.String(), "load context fail") {
		t.Fatalf("expected config load context error, got %s", errOut.String())
	}

	store.loadCtxErr = nil
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"config", "list", "--context", "default", "extra"}); code != 1 {
		t.Fatalf("expected config list usage failure")
	}
	if !strings.Contains(errOut.String(), "usage: config list") {
		t.Fatalf("expected config list usage error, got %s", errOut.String())
	}

	errOut.Reset()
	if code := app.Run(context.Background(), []string{"config", "get"}); code != 1 {
		t.Fatalf("expected config get usage failure")
	}
	if !strings.Contains(errOut.String(), "usage: config get") {
		t.Fatalf("expected config get usage error, got %s", errOut.String())
	}

	errOut.Reset()
	if code := app.Run(context.Background(), []string{"config", "set"}); code != 1 {
		t.Fatalf("expected config set usage failure")
	}
	if !strings.Contains(errOut.String(), "usage: config set") {
		t.Fatalf("expected config set usage error, got %s", errOut.String())
	}

	errOut.Reset()
	if code := app.Run(context.Background(), []string{"config", "bad"}); code != 1 {
		t.Fatalf("expected unknown config command failure")
	}
	if !strings.Contains(errOut.String(), "unknown config command") {
		t.Fatalf("expected unknown config command error, got %s", errOut.String())
	}

	errOut.Reset()
	if code := app.Run(context.Background(), []string{"config", "get", "bad-key"}); code != 1 {
		t.Fatalf("expected unknown config get key failure")
	}
	if !strings.Contains(errOut.String(), "unknown config key") {
		t.Fatalf("expected unknown config get key error, got %s", errOut.String())
	}

	errOut.Reset()
	if code := app.Run(context.Background(), []string{"config", "set", "bad-key", "value"}); code != 1 {
		t.Fatalf("expected unknown config key failure")
	}
	if !strings.Contains(errOut.String(), "unknown config key") {
		t.Fatalf("expected unknown config key error, got %s", errOut.String())
	}

	errOut.Reset()
	if code := app.Run(context.Background(), []string{"config", "set", "org-id", "bad"}); code != 1 {
		t.Fatalf("expected invalid org id failure")
	}
	if !strings.Contains(errOut.String(), "invalid org-id") {
		t.Fatalf("expected invalid org-id error, got %s", errOut.String())
	}

	store.saveCtxErr = errors.New("save context fail")
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"auth", "login", "--context", "ops", "--token", "x"}); code != 1 {
		t.Fatalf("expected auth context save failure")
	}
	if !strings.Contains(errOut.String(), "save context fail") {
		t.Fatalf("expected auth context save error, got %s", errOut.String())
	}

	store.saveCtxErr = nil
	store.saveErr = errors.New("save fail")
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"config", "set", "base-url", "https://x"}); code != 1 {
		t.Fatalf("expected config save failure")
	}
	if !strings.Contains(errOut.String(), "save fail") {
		t.Fatalf("expected config save error, got %s", errOut.String())
	}

	store.saveErr = nil
	store.loadCtxErr = errors.New("load context fail for set")
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"config", "set", "--context", "default", "base-url", "https://x"}); code != 1 {
		t.Fatalf("expected config context load failure")
	}
	if !strings.Contains(errOut.String(), "load context fail for set") {
		t.Fatalf("expected config context load error, got %s", errOut.String())
	}

	store.loadCtxErr = nil
	store.saveCtxErr = errors.New("save context fail")
	errOut.Reset()
	if code := app.Run(context.Background(), []string{"config", "set", "--context", "default", "base-url", "https://x"}); code != 1 {
		t.Fatalf("expected config context save failure")
	}
	if !strings.Contains(errOut.String(), "save context fail") {
		t.Fatalf("expected config context save error, got %s", errOut.String())
	}
}

func TestOutputFormattingAndContextHelpers(t *testing.T) {
	store := &fakeContextStore{
		current: "default",
		cfgs: map[string]config.Config{
			"default": {
				Token:         "token",
				BaseURL:       "https://default.grafana.net",
				CloudURL:      "https://grafana.com",
				PrometheusURL: "https://prom.grafana.net",
				LogsURL:       "https://logs.grafana.net",
				TracesURL:     "https://traces.grafana.net",
				OnCallURL:     "https://oncall.grafana.net",
				TokenBackend:  "keyring",
				OrgID:         7,
			},
		},
	}
	app, out, errOut := newTestApp(store, &fakeClient{})

	if code := app.Run(context.Background(), []string{"--output", "pretty", "context", "view"}); code != 0 {
		t.Fatalf("pretty output should succeed: %s", errOut.String())
	}
	if !strings.Contains(out.String(), "\n  \"context\": \"default\"") {
		t.Fatalf("expected indented pretty output, got %s", out.String())
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"--json", "context,base_url", "context", "view"}); code != 0 {
		t.Fatalf("json field projection should succeed: %s", errOut.String())
	}
	projected := decodeJSON(t, out.String())
	if len(projected) != 2 || projected["context"] != "default" || projected["base_url"] != "https://default.grafana.net" {
		t.Fatalf("unexpected field projection: %+v", projected)
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"--jq", ".base_url", "context", "view"}); code != 0 {
		t.Fatalf("jq output should succeed: %s", errOut.String())
	}
	if out.String() != "https://default.grafana.net\n" {
		t.Fatalf("unexpected jq scalar output: %q", out.String())
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"--template", "{{.context}} {{.org_id}}", "context", "view"}); code != 0 {
		t.Fatalf("template output should succeed: %s", errOut.String())
	}
	if out.String() != "default 7\n" {
		t.Fatalf("unexpected template output: %q", out.String())
	}

	errOut.Reset()
	if code := app.Run(context.Background(), []string{"--jq", ".[", "context", "view"}); code != 1 {
		t.Fatalf("expected jq parse failure")
	}
	if errOut.Len() == 0 {
		t.Fatalf("expected jq parse error output")
	}

	if selectedContextName(nil, "") != "default" {
		t.Fatalf("expected default context fallback")
	}
	if selectedContextName(store, "explicit") != "explicit" {
		t.Fatalf("expected explicit context name")
	}
	if selectedContextName(&fakeContextStore{currentErr: errors.New("boom")}, "") != "default" {
		t.Fatalf("expected default context on current-context error")
	}

	cfg, name, err := app.loadConfigForContext("")
	if err != nil || name != "default" || cfg.BaseURL != "https://default.grafana.net" {
		t.Fatalf("unexpected load current context result: cfg=%+v name=%s err=%v", cfg, name, err)
	}
	cfg, name, err = app.loadConfigForContext("default")
	if err != nil || name != "default" || cfg.BaseURL != "https://default.grafana.net" {
		t.Fatalf("unexpected load explicit context result: cfg=%+v name=%s err=%v", cfg, name, err)
	}

	payload := configPayload("default", store.cfgs["default"])
	if payload["token_backend"] != "keyring" || payload["oncall_url"] != "https://oncall.grafana.net" {
		t.Fatalf("unexpected config payload: %+v", payload)
	}

	if normalizeConfigKey(" Base-URL ") != "base-url" {
		t.Fatalf("unexpected normalized key")
	}

	valueCfg := config.Config{
		BaseURL:       "base",
		CloudURL:      "cloud",
		PrometheusURL: "prom",
		LogsURL:       "logs",
		TracesURL:     "traces",
		OnCallURL:     "oncall",
		TokenBackend:  "keyring",
		OrgID:         5,
	}
	for key, want := range map[string]any{
		"base-url":       "base",
		"base_url":       "base",
		"cloud-url":      "cloud",
		"prom-url":       "prom",
		"prometheus_url": "prom",
		"logs-url":       "logs",
		"traces_url":     "traces",
		"oncall-url":     "oncall",
		"org-id":         int64(5),
		"token-backend":  "keyring",
	} {
		got, err := configValueForKey(valueCfg, key)
		if err != nil || got != want {
			t.Fatalf("unexpected config value for %s: got=%v err=%v", key, got, err)
		}
	}
	if _, err := configValueForKey(valueCfg, "bad"); err == nil {
		t.Fatalf("expected unknown config key error")
	}

	mutable := config.Config{}
	updates := []struct {
		key   string
		value string
		check func(config.Config) bool
	}{
		{key: "base-url", value: "base", check: func(cfg config.Config) bool { return cfg.BaseURL == "base" }},
		{key: "cloud-url", value: "cloud", check: func(cfg config.Config) bool { return cfg.CloudURL == "cloud" }},
		{key: "prom-url", value: "prom", check: func(cfg config.Config) bool { return cfg.PrometheusURL == "prom" }},
		{key: "logs-url", value: "logs", check: func(cfg config.Config) bool { return cfg.LogsURL == "logs" }},
		{key: "traces-url", value: "traces", check: func(cfg config.Config) bool { return cfg.TracesURL == "traces" }},
		{key: "oncall-url", value: "oncall", check: func(cfg config.Config) bool { return cfg.OnCallURL == "oncall" }},
		{key: "org-id", value: "11", check: func(cfg config.Config) bool { return cfg.OrgID == 11 }},
	}
	for _, update := range updates {
		if err := setConfigValue(&mutable, update.key, update.value); err != nil || !update.check(mutable) {
			t.Fatalf("unexpected config update for %s: cfg=%+v err=%v", update.key, mutable, err)
		}
	}
	if err := setConfigValue(&mutable, "org-id", "-1"); err == nil {
		t.Fatalf("expected invalid negative org id")
	}

	values, err := applyJQ([]any{map[string]any{"x": 1}, map[string]any{"x": 2}}, ".[].x")
	if err != nil {
		t.Fatalf("unexpected jq multi-result error: %v", err)
	}
	results, ok := values.([]any)
	if !ok || len(results) != 2 {
		t.Fatalf("unexpected jq multi-result payload: %#v", values)
	}

	value, err := applyJQ(map[string]any{"x": 1}, ".x")
	if err != nil || value != 1 {
		t.Fatalf("unexpected jq single result: value=%v err=%v", value, err)
	}
	value, err = applyJQ(map[string]any{"x": 1}, "empty")
	if err != nil || value != nil {
		t.Fatalf("unexpected jq empty result: value=%v err=%v", value, err)
	}
	if _, err := applyJQ(map[string]any{"x": 1}, ".["); err == nil {
		t.Fatalf("expected jq parse error")
	}

	builder := &strings.Builder{}
	if err := renderTemplate(builder, map[string]any{"name": "grafana"}, "{{.name}}"); err != nil {
		t.Fatalf("unexpected template render error: %v", err)
	}
	if builder.String() != "grafana\n" {
		t.Fatalf("unexpected template render output: %q", builder.String())
	}
	builder.Reset()
	if err := renderTemplate(builder, map[string]any{"name": "grafana"}, "{{json .}}"); err != nil {
		t.Fatalf("unexpected template json render error: %v", err)
	}
	if builder.String() != "{\"name\":\"grafana\"}\n" {
		t.Fatalf("unexpected template json output: %q", builder.String())
	}
	if err := renderTemplate(&strings.Builder{}, map[string]any{}, "{{"); err == nil {
		t.Fatalf("expected template parse error")
	}
	if err := renderTemplate(&strings.Builder{}, map[string]any{"bad": make(chan int)}, "{{json .bad}}"); err == nil {
		t.Fatalf("expected template execution error")
	}

	if !isScalar(nil) || !isScalar(true) || !isScalar(json.Number("7")) {
		t.Fatalf("expected scalar values to be detected")
	}
	if isScalar([]any{1}) {
		t.Fatalf("expected array to be non-scalar")
	}
	if scalarString(nil) != "null" || scalarString(7) != "7" {
		t.Fatalf("unexpected scalar string conversion")
	}
}

func TestContextConfigAndOutputEdgeBranches(t *testing.T) {
	store := &fakeContextStore{
		current: "default",
		cfgs: map[string]config.Config{
			"default": {Token: "token", BaseURL: "https://default.grafana.net", CloudURL: "https://grafana.com"},
		},
	}
	app, _, errOut := newTestApp(store, &fakeClient{})

	if code := app.Run(context.Background(), []string{"context", "list", "extra"}); code != 1 {
		t.Fatalf("expected context list usage failure")
	}
	if !strings.Contains(errOut.String(), "usage: context list") {
		t.Fatalf("expected context list usage error, got %s", errOut.String())
	}

	errOut.Reset()
	if code := app.Run(context.Background(), []string{"config"}); code != 0 {
		t.Fatalf("expected config summary to succeed")
	}

	errOut.Reset()
	if code := app.Run(context.Background(), []string{"config", "get", "--context"}); code != 1 {
		t.Fatalf("expected missing context value failure")
	}
	if !strings.Contains(errOut.String(), "--context requires a value") {
		t.Fatalf("expected missing context value error, got %s", errOut.String())
	}

	errOut.Reset()
	if code := app.Run(context.Background(), []string{"config", "get", "--context=default", "base-url"}); code != 0 {
		t.Fatalf("expected inline context arg to succeed: %s", errOut.String())
	}

	if _, _, err := parseGlobalOptions([]string{"--json=context", "--jq=.context", "context", "view"}); err != nil {
		t.Fatalf("expected inline json/jq options to succeed: %v", err)
	}

	loadFailApp := &App{Store: &fakeStore{loadErr: errors.New("load fail")}}
	if _, _, err := loadFailApp.loadConfigForContext(""); err == nil {
		t.Fatalf("expected loadConfigForContext store failure")
	}

	if _, err := applyJQ(map[string]any{"x": "value"}, ".x + 1"); err == nil {
		t.Fatalf("expected jq runtime error")
	}

	builder := &strings.Builder{}
	if err := renderTemplate(builder, map[string]any{"name": "grafana"}, "{{.name}}\n"); err != nil {
		t.Fatalf("unexpected template render with trailing newline error: %v", err)
	}
	if builder.String() != "grafana\n" {
		t.Fatalf("unexpected template output with trailing newline: %q", builder.String())
	}
	builder.Reset()
	if err := renderTemplate(builder, map[string]any{"name": "grafana"}, "{{json .}}"); err != nil {
		t.Fatalf("unexpected template json render error: %v", err)
	}
	if builder.String() != "{\"name\":\"grafana\"}\n" {
		t.Fatalf("unexpected template json output: %q", builder.String())
	}
}

func TestBuildDashboardSharePath(t *testing.T) {
	if got := buildDashboardSharePath("ops", "", 0, "", "", "", 0); got != "/d/ops/share" {
		t.Fatalf("unexpected dashboard share path: %s", got)
	}
	if got := buildDashboardSharePath("ops", "overview", 4, "now-1h", "now", "dark", 12); got != "/d-solo/ops/overview?from=now-1h&orgId=12&panelId=4&theme=dark&to=now" {
		t.Fatalf("unexpected panel share path: %s", got)
	}
}
