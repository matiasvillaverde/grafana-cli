package cli

import (
	"context"
	"encoding/json"
	"errors"
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

type fakeClient struct {
	rawResult           any
	rawErr              error
	cloudResult         any
	cloudErr            error
	searchDashResult    any
	searchDashErr       error
	createDashResult    any
	createDashErr       error
	listDSResult        any
	listDSErr           error
	assistantChatResult any
	assistantChatErr    error
	assistantStatusResp any
	assistantStatusErr  error
	assistantSkillsResp any
	assistantSkillsErr  error
	assistantPrompt     string
	assistantChatID     string
	assistantStatusID   string
	metricsResult       any
	metricsErr          error
	logsResult          any
	logsErr             error
	tracesResult        any
	tracesErr           error
	aggregateResult     grafana.AggregateSnapshot
	aggregateErr        error
	aggregateReq        grafana.AggregateRequest
	createDashboardArg  map[string]any
	createFolderID      int64
	createOverwrite     bool
}

func (f *fakeClient) Raw(_ context.Context, _, _ string, _ any) (any, error) {
	return f.rawResult, f.rawErr
}

func (f *fakeClient) CloudStacks(_ context.Context) (any, error) {
	return f.cloudResult, f.cloudErr
}

func (f *fakeClient) SearchDashboards(_ context.Context, _, _ string, _ int) (any, error) {
	return f.searchDashResult, f.searchDashErr
}

func (f *fakeClient) CreateDashboard(_ context.Context, dashboard map[string]any, folderID int64, overwrite bool) (any, error) {
	f.createDashboardArg = dashboard
	f.createFolderID = folderID
	f.createOverwrite = overwrite
	return f.createDashResult, f.createDashErr
}

func (f *fakeClient) ListDatasources(_ context.Context) (any, error) {
	return f.listDSResult, f.listDSErr
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

func (f *fakeClient) MetricsRange(_ context.Context, _, _, _, _ string) (any, error) {
	return f.metricsResult, f.metricsErr
}

func (f *fakeClient) LogsRange(_ context.Context, _, _, _ string, _ int) (any, error) {
	return f.logsResult, f.logsErr
}

func (f *fakeClient) TracesSearch(_ context.Context, _, _, _ string, _ int) (any, error) {
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

func TestParseGlobalOptions(t *testing.T) {
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

func TestRunHelpAndUnknown(t *testing.T) {
	store := &fakeStore{}
	client := &fakeClient{}
	app, out, errOut := newTestApp(store, client)

	if code := app.Run(context.Background(), []string{}); code != 0 {
		t.Fatalf("expected success for help")
	}
	resp := decodeJSON(t, out.String())
	commands, ok := resp["commands"].([]any)
	if !ok {
		t.Fatalf("expected commands output")
	}
	foundAssistant := false
	for _, command := range commands {
		if value, _ := command.(string); value == "assistant" {
			foundAssistant = true
		}
	}
	if !foundAssistant {
		t.Fatalf("expected assistant command in help output")
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
	if resp["status"] != "authenticated" {
		t.Fatalf("expected authenticated status")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"auth", "logout"}); code != 0 {
		t.Fatalf("logout should succeed")
	}
	resp = decodeJSON(t, out.String())
	if resp["status"] != "logged_out" {
		t.Fatalf("unexpected logout response")
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
	if code := app.Run(context.Background(), []string{"auth", "logout"}); code != 1 {
		t.Fatalf("expected clear failure")
	}
	if !strings.Contains(errOut.String(), "clear fail") {
		t.Fatalf("expected clear fail error")
	}
}

func TestAPICloudDashboardDatasourceCommands(t *testing.T) {
	store := &fakeStore{cfg: config.Config{Token: "token"}}
	client := &fakeClient{
		rawResult:        map[string]any{"ok": true},
		cloudResult:      map[string]any{"items": []any{map[string]any{"id": 1}}},
		searchDashResult: []any{map[string]any{"uid": "x"}},
		createDashResult: map[string]any{"status": "success"},
		listDSResult: []any{
			map[string]any{"name": "prom", "type": "prometheus"},
			map[string]any{"name": "loki", "type": "loki"},
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
	if code := app.Run(context.Background(), []string{"cloud"}); code != 1 {
		t.Fatalf("cloud usage should fail")
	}
	if code := app.Run(context.Background(), []string{"cloud", "bad"}); code != 1 {
		t.Fatalf("unknown cloud should fail")
	}
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "bad"}); code != 1 {
		t.Fatalf("cloud stacks bad verb should fail")
	}

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
	if code := app.Run(context.Background(), []string{"dashboards"}); code != 1 {
		t.Fatalf("missing dashboards subcommand should fail")
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
	if len(filtered) != 1 || filtered[0]["type"] != "loki" {
		t.Fatalf("unexpected datasource filter output: %+v", filtered)
	}
	if code := app.Run(context.Background(), []string{"datasources", "bad"}); code != 1 {
		t.Fatalf("invalid datasources usage should fail")
	}
	if code := app.Run(context.Background(), []string{"datasources", "list", "--bad"}); code != 1 {
		t.Fatalf("datasources list parse should fail")
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

	if code := app.Run(context.Background(), []string{"assistant"}); code != 1 {
		t.Fatalf("assistant usage should fail")
	}
	if code := app.Run(context.Background(), []string{"assistant", "chat"}); code != 1 {
		t.Fatalf("assistant chat missing prompt should fail")
	}
	if code := app.Run(context.Background(), []string{"assistant", "chat", "--bad"}); code != 1 {
		t.Fatalf("assistant chat parse should fail")
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
		logsResult:    map[string]any{"l": 1},
		tracesResult:  map[string]any{"t": 1},
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
	if code := app.Run(context.Background(), []string{"runtime"}); code != 1 {
		t.Fatalf("runtime usage should fail")
	}
	if code := app.Run(context.Background(), []string{"runtime", "metrics", "bad"}); code != 1 {
		t.Fatalf("runtime metrics bad verb should fail")
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
	if code := app.Run(context.Background(), []string{"runtime", "traces", "search", "--query", "{}"}); code != 0 {
		t.Fatalf("runtime traces failed")
	}
	if code := app.Run(context.Background(), []string{"runtime", "metrics", "query"}); code != 1 {
		t.Fatalf("missing expr should fail")
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
	if code := app.Run(context.Background(), []string{"aggregate"}); code != 1 {
		t.Fatalf("aggregate usage should fail")
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
	if code := app.Run(context.Background(), []string{"incident", "analyze"}); code != 1 {
		t.Fatalf("missing goal should fail")
	}
	if code := app.Run(context.Background(), []string{"incident", "bad"}); code != 1 {
		t.Fatalf("incident usage should fail")
	}
	if code := app.Run(context.Background(), []string{"incident", "analyze", "--goal", "slow", "--metric-expr", "m", "--log-query", "l", "--trace-query", "t", "--start", "s", "--end", "e", "--step", "1m", "--limit", "10", "--include-raw"}); code != 0 {
		t.Fatalf("incident include-raw should succeed")
	}

	out.Reset()
	if code := app.Run(context.Background(), []string{"agent", "plan", "--goal", "latency"}); code != 0 {
		t.Fatalf("agent plan should succeed")
	}
	plan := decodeJSON(t, out.String())
	if plan["playbook"] != "latency" {
		t.Fatalf("expected latency playbook")
	}
	if code := app.Run(context.Background(), []string{"agent", "run", "--goal", "errors"}); code != 0 {
		t.Fatalf("agent run should succeed")
	}
	if code := app.Run(context.Background(), []string{"agent", "run", "--goal", "errors", "--include-raw"}); code != 0 {
		t.Fatalf("agent include-raw should succeed")
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

	store = &fakeStore{cfg: config.Config{Token: "x"}}
	client = &fakeClient{cloudErr: errors.New("cloud fail")}
	app, _, errOut = newTestApp(store, client)
	if code := app.Run(context.Background(), []string{"cloud", "stacks", "list"}); code != 1 {
		t.Fatalf("expected cloud client failure")
	}
	if !strings.Contains(errOut.String(), "cloud fail") {
		t.Fatalf("unexpected cloud error: %s", errOut.String())
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
	if code := app.Run(context.Background(), []string{"agent"}); code != 1 {
		t.Fatalf("agent usage should fail")
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
	if inferCollectionCount("x") != 0 {
		t.Fatalf("unexpected fallback count")
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
}
