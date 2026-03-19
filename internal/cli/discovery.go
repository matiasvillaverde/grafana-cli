package cli

import (
	"errors"
	"strings"
)

const cliVersion = "0.1.0"

type discoveryFlag struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Default     any    `json:"default,omitempty"`
	Description string `json:"description,omitempty"`
}

type discoveryCommand struct {
	Name             string
	Description      string
	ReadOnly         bool
	Flags            []discoveryFlag
	Examples         []string
	OutputShape      string
	RelatedCommands  []string
	TokenCost        string
	DocumentationURL string
	Notes            []string
	Subcommands      []discoveryCommand
}

type discoveryWorkflow struct {
	Name      string   `json:"name"`
	Steps     []string `json:"steps"`
	TokenCost string   `json:"token_cost,omitempty"`
}

type discoveryTimeFormats struct {
	Relative []string `json:"relative,omitempty"`
	Absolute []string `json:"absolute,omitempty"`
	Examples []string `json:"examples,omitempty"`
}

func discoveryCatalog() []discoveryCommand {
	return []discoveryCommand{
		{
			Name:        "schema",
			Description: "Emit the machine-readable discovery schema for the CLI",
			ReadOnly:    true,
			Flags: []discoveryFlag{
				{Name: "--compact", Type: "bool", Default: false, Description: "Return a smaller schema intended for agent discovery loops"},
			},
			Examples: []string{
				"grafana schema --compact",
				"grafana schema runtime",
			},
			OutputShape:     `{"version":"0.1.0","commands":[...]}`,
			RelatedCommands: []string{"runtime", "incident", "agent"},
			TokenCost:       "small",
		},
		{
			Name:        "auth",
			Description: "Authenticate and inspect local Grafana configuration",
			ReadOnly:    false,
			Subcommands: []discoveryCommand{
				{
					Name:        "login",
					Description: "Store token and endpoint configuration for the current context",
					ReadOnly:    false,
					Flags: []discoveryFlag{
						{Name: "--token", Type: "string", Description: "Grafana token"},
						{Name: "--context", Type: "string", Description: "Context name"},
						{Name: "--stack", Type: "string", Description: "Grafana Cloud stack slug or https://<stack>.grafana.net URL"},
						{Name: "--cloud-token", Type: "string", Description: "Grafana Cloud API token used only for endpoint discovery"},
						{Name: "--base-url", Type: "string", Description: "Grafana base URL"},
						{Name: "--cloud-url", Type: "string", Description: "Grafana Cloud API URL"},
						{Name: "--prom-url", Type: "string", Description: "Prometheus query URL"},
						{Name: "--logs-url", Type: "string", Description: "Loki query URL"},
						{Name: "--traces-url", Type: "string", Description: "Tempo query URL"},
						{Name: "--oncall-url", Type: "string", Description: "Grafana OnCall API URL"},
						{Name: "--org-id", Type: "int", Default: 0, Description: "Grafana organization ID"},
					},
					Examples: []string{
						`grafana auth login --token "$GRAFANA_TOKEN" --stack prod-observability`,
						`grafana auth login --token "$GRAFANA_TOKEN" --stack https://prod-observability.grafana.net`,
						`grafana auth login --context prod --token "$GRAFANA_TOKEN" --base-url "https://prod.grafana.net"`,
					},
					OutputShape:     `{"status":"authenticated","base_url":"https://stack.grafana.net","missing":["oncall_url"]}`,
					RelatedCommands: []string{"auth status", "auth doctor", "context list"},
					TokenCost:       "small",
				},
				{
					Name:            "status",
					Description:     "Show the current authentication status and configured endpoints",
					ReadOnly:        true,
					Examples:        []string{"grafana auth status"},
					OutputShape:     `{"status":"authenticated","capabilities":[{"name":"runtime_logs","ok":true}]}`,
					RelatedCommands: []string{"auth doctor", "context view"},
					TokenCost:       "small",
				},
				{
					Name:            "doctor",
					Description:     "Diagnose missing auth and endpoint configuration by capability",
					ReadOnly:        true,
					Examples:        []string{"grafana auth doctor"},
					OutputShape:     `{"authenticated":true,"capabilities":[{"name":"runtime_logs","ok":false}]}`,
					RelatedCommands: []string{"auth status", "config set"},
					TokenCost:       "small",
				},
				{
					Name:            "logout",
					Description:     "Clear the current context token and persisted auth state",
					ReadOnly:        false,
					Examples:        []string{"grafana auth logout"},
					OutputShape:     `{"status":"logged_out"}`,
					RelatedCommands: []string{"auth login", "auth status"},
					TokenCost:       "small",
				},
			},
		},
		{
			Name:        "context",
			Description: "Manage local CLI contexts",
			ReadOnly:    false,
			Subcommands: []discoveryCommand{
				{Name: "list", Description: "List configured contexts", ReadOnly: true, Examples: []string{"grafana context list"}, OutputShape: `[{"name":"prod","current":true}]`, RelatedCommands: []string{"context use", "context view"}, TokenCost: "small"},
				{Name: "use", Description: "Switch the active context", ReadOnly: false, Examples: []string{"grafana context use prod"}, OutputShape: `{"context":"prod"}`, RelatedCommands: []string{"context list", "context view"}, TokenCost: "small"},
				{Name: "view", Description: "Show the current context configuration", ReadOnly: true, Examples: []string{"grafana context view"}, OutputShape: `{"context":"default","base_url":"https://..."}`, RelatedCommands: []string{"context list", "config get"}, TokenCost: "small"},
			},
		},
		{
			Name:        "config",
			Description: "Inspect and modify persisted CLI configuration",
			ReadOnly:    false,
			Subcommands: []discoveryCommand{
				{Name: "list", Description: "List the config for a context", ReadOnly: true, Flags: []discoveryFlag{{Name: "--context", Type: "string", Description: "Context name"}}, Examples: []string{"grafana config list", "grafana config list --context prod"}, OutputShape: `{"base_url":"https://..."}`, RelatedCommands: []string{"config get", "config set"}, TokenCost: "small"},
				{Name: "get", Description: "Read one config key", ReadOnly: true, Flags: []discoveryFlag{{Name: "--context", Type: "string", Description: "Context name"}}, Examples: []string{"grafana config get base-url"}, OutputShape: `{"key":"base_url","value":"https://..."}`, RelatedCommands: []string{"config list", "config set"}, TokenCost: "small"},
				{Name: "set", Description: "Persist one config key", ReadOnly: false, Flags: []discoveryFlag{{Name: "--context", Type: "string", Description: "Context name"}}, Examples: []string{"grafana config set org-id 12"}, OutputShape: `{"org_id":12}`, RelatedCommands: []string{"config get", "auth doctor"}, TokenCost: "small"},
			},
		},
		{
			Name:        "api",
			Description: "Call the raw Grafana HTTP API when no dedicated command exists",
			ReadOnly:    false,
			Flags: []discoveryFlag{
				{Name: "--body", Type: "json", Description: "JSON request body"},
			},
			Examples: []string{
				`grafana api GET /api/health`,
				`grafana api POST /api/dashboards/db --body '{"dashboard":{"title":"Ops"}}'`,
			},
			OutputShape:     `{"commit":"abc123","database":"ok"}`,
			RelatedCommands: []string{"schema", "dashboards list"},
			TokenCost:       "small",
		},
		{
			Name:        "cloud",
			Description: "Inspect Grafana Cloud control-plane resources",
			ReadOnly:    true,
			Subcommands: []discoveryCommand{
				{
					Name:        "stacks",
					Description: "Inspect Grafana Cloud stacks",
					ReadOnly:    true,
					Subcommands: []discoveryCommand{
						{Name: "list", Description: "List available Grafana Cloud stacks", ReadOnly: true, Examples: []string{"grafana cloud stacks list"}, OutputShape: `{"items":[...]}`, RelatedCommands: []string{"auth doctor", "context list"}, TokenCost: "small"},
						{Name: "inspect", Description: "Inspect one stack's inferred runtime endpoints and connectivity. Normal CLI mode fails if datasource or connection discovery is incomplete; agent mode returns warnings.", ReadOnly: true, Flags: []discoveryFlag{{Name: "--stack", Type: "string", Description: "Grafana Cloud stack slug or https://<stack>.grafana.net URL"}, {Name: "--include-raw", Type: "bool", Default: false, Description: "Include raw datasource and connection payloads"}}, Examples: []string{`grafana cloud stacks inspect --stack prod-observability`, `grafana cloud stacks inspect --stack https://prod-observability.grafana.net --include-raw`}, OutputShape: `{"stack":{"slug":"prod-observability"},"datasource_summary":{"count":3},"inferred_endpoints":{"prometheus_url":"https://...","logs_url":"https://...","traces_url":"https://...","oncall_url":"https://..."}}`, RelatedCommands: []string{"cloud stacks list", "auth login", "datasources list"}, TokenCost: "small"},
						{Name: "plugins", Description: "Inspect plugins installed on a stack", ReadOnly: true, Subcommands: []discoveryCommand{
							{Name: "list", Description: "List installed plugins for a stack. The CLI follows Cloud pagination automatically up to --limit.", ReadOnly: true, Flags: []discoveryFlag{{Name: "--stack", Type: "string", Description: "Grafana Cloud stack slug or https://<stack>.grafana.net URL"}, {Name: "--query", Type: "string", Description: "Plugin ID or name filter"}, {Name: "--limit", Type: "int", Default: 100, Description: "Maximum plugins"}}, Examples: []string{`grafana cloud stacks plugins list --stack prod-observability`, `grafana cloud stacks plugins list --stack prod-observability --query incident`}, OutputShape: `{"items":[{"id":"grafana-oncall-app","name":"Grafana OnCall"}]}`, RelatedCommands: []string{"cloud stacks plugins get", "cloud stacks inspect"}, TokenCost: "small"},
							{Name: "get", Description: "Get one installed stack plugin", ReadOnly: true, Flags: []discoveryFlag{{Name: "--stack", Type: "string", Description: "Grafana Cloud stack slug or https://<stack>.grafana.net URL"}, {Name: "--plugin", Type: "string", Description: "Plugin ID"}}, Examples: []string{`grafana cloud stacks plugins get --stack prod-observability --plugin grafana-oncall-app`}, OutputShape: `{"id":"grafana-oncall-app","version":"1.0.0"}`, RelatedCommands: []string{"cloud stacks plugins list", "cloud stacks inspect"}, TokenCost: "small"},
						}},
					},
				},
				{
					Name:        "access-policies",
					Description: "Inspect Cloud access policies",
					ReadOnly:    true,
					Subcommands: []discoveryCommand{
						{Name: "list", Description: "List access policies. The CLI follows Cloud pagination automatically up to --limit.", ReadOnly: true, Flags: []discoveryFlag{{Name: "--region", Type: "string", Description: "Policy region"}, {Name: "--name", Type: "string", Description: "Name filter"}, {Name: "--realm-type", Type: "string", Description: "org or stack"}, {Name: "--realm-identifier", Type: "string", Description: "Realm ID"}, {Name: "--status", Type: "string", Description: "active or inactive"}, {Name: "--limit", Type: "int", Default: 100, Description: "Maximum access policies"}, {Name: "--page-size", Type: "int", Default: 100, Description: "Page size per Cloud API request"}, {Name: "--page-cursor", Type: "string", Description: "Page cursor to start from"}}, Examples: []string{`grafana cloud access-policies list --region us`, `grafana cloud access-policies list --region eu --realm-type stack --realm-identifier 12345 --limit 200`}, OutputShape: `{"items":[{"id":"uuid","name":"stack-readers"}]}`, RelatedCommands: []string{"cloud access-policies get", "service-accounts list"}, TokenCost: "small"},
						{Name: "get", Description: "Get one access policy", ReadOnly: true, Flags: []discoveryFlag{{Name: "--id", Type: "string", Description: "Policy ID"}, {Name: "--region", Type: "string", Description: "Policy region"}}, Examples: []string{`grafana cloud access-policies get --id c45485b6-8321-4cf2-bcec-12006df755ff --region us`}, OutputShape: `{"id":"uuid","name":"stack-readers","scopes":["metrics:read"]}`, RelatedCommands: []string{"cloud access-policies list", "auth doctor"}, TokenCost: "small"},
					},
				},
				{
					Name:        "billed-usage",
					Description: "Inspect billed usage for a Grafana Cloud organization",
					ReadOnly:    true,
					Subcommands: []discoveryCommand{
						{Name: "get", Description: "Get billed usage for one month", ReadOnly: true, Flags: []discoveryFlag{{Name: "--org-slug", Type: "string", Description: "Grafana Cloud organization slug"}, {Name: "--year", Type: "int", Description: "Billing year"}, {Name: "--month", Type: "int", Description: "Billing month number"}}, Examples: []string{`grafana cloud billed-usage get --org-slug grafanacom --year 2024 --month 9`}, OutputShape: `{"org_slug":"grafanacom","year":2024,"month":9,"summary":{"items":3,"total_amount_due":966.91},"items":[...]}`, RelatedCommands: []string{"cloud stacks list", "cloud access-policies list"}, TokenCost: "small"},
					},
				},
			},
		},
		{
			Name:        "service-accounts",
			Description: "Inspect service accounts",
			ReadOnly:    true,
			Subcommands: []discoveryCommand{
				{Name: "list", Description: "List service accounts", ReadOnly: true, Flags: []discoveryFlag{{Name: "--query", Type: "string", Description: "Name filter"}, {Name: "--page", Type: "int", Default: 1, Description: "Page"}, {Name: "--limit", Type: "int", Default: 100, Description: "Page size"}}, Examples: []string{`grafana service-accounts list`, `grafana service-accounts list --query writer --page 2 --limit 20`}, OutputShape: `{"totalCount":2,"serviceAccounts":[{"id":1,"name":"grafana"}],"page":1,"perPage":10}`, RelatedCommands: []string{"service-accounts get", "cloud access-policies list"}, TokenCost: "small"},
				{Name: "get", Description: "Get one service account", ReadOnly: true, Flags: []discoveryFlag{{Name: "--id", Type: "int", Description: "Service account ID"}}, Examples: []string{`grafana service-accounts get --id 1`}, OutputShape: `{"id":1,"name":"grafana","tokens":0}`, RelatedCommands: []string{"service-accounts list", "auth doctor"}, TokenCost: "small"},
			},
		},
		{
			Name:        "dashboards",
			Description: "Inspect and manage dashboards",
			ReadOnly:    false,
			Subcommands: []discoveryCommand{
				{Name: "list", Description: "Search dashboards by query and tag", ReadOnly: true, Flags: []discoveryFlag{{Name: "--query", Type: "string", Description: "Search text"}, {Name: "--tag", Type: "string", Description: "Tag filter"}, {Name: "--limit", Type: "int", Default: 100, Description: "Maximum dashboards"}}, Examples: []string{"grafana dashboards list --query latency --tag prod"}, OutputShape: `[{"uid":"incident-overview"}]`, RelatedCommands: []string{"dashboards get", "dashboards versions"}, TokenCost: "small"},
				{Name: "get", Description: "Fetch one dashboard by UID", ReadOnly: true, Flags: []discoveryFlag{{Name: "--uid", Type: "string", Description: "Dashboard UID"}}, Examples: []string{"grafana dashboards get --uid incident-overview"}, OutputShape: `{"dashboard":{"uid":"incident-overview"}}`, RelatedCommands: []string{"dashboards list", "dashboards render"}, TokenCost: "medium"},
				{Name: "create", Description: "Create a dashboard from flags or JSON", ReadOnly: false, Flags: []discoveryFlag{{Name: "--title", Type: "string", Description: "Dashboard title"}, {Name: "--uid", Type: "string", Description: "Dashboard UID"}, {Name: "--schema-version", Type: "int", Default: 39, Description: "Schema version"}, {Name: "--folder-id", Type: "int", Default: 0, Description: "Folder ID"}, {Name: "--overwrite", Type: "bool", Default: true, Description: "Overwrite existing dashboard"}, {Name: "--tags", Type: "csv", Description: "Comma-separated tags"}, {Name: "--template-json", Type: "json", Description: "Full dashboard JSON"}}, Examples: []string{`grafana dashboards create --title "Incident Overview"`, `grafana dashboards create --template-json '{"title":"Ops","panels":[]}'`}, OutputShape: `{"status":"success","uid":"incident-overview"}`, RelatedCommands: []string{"dashboards get", "dashboards delete"}, TokenCost: "medium"},
				{Name: "delete", Description: "Delete a dashboard by UID", ReadOnly: false, Flags: []discoveryFlag{{Name: "--uid", Type: "string", Description: "Dashboard UID"}}, Examples: []string{"grafana dashboards delete --uid incident-overview"}, OutputShape: `{"status":"deleted"}`, RelatedCommands: []string{"dashboards get", "dashboards list"}, TokenCost: "small"},
				{Name: "versions", Description: "List dashboard versions", ReadOnly: true, Flags: []discoveryFlag{{Name: "--uid", Type: "string", Description: "Dashboard UID"}, {Name: "--limit", Type: "int", Default: 20, Description: "Maximum versions"}}, Examples: []string{"grafana dashboards versions --uid incident-overview --limit 5"}, OutputShape: `[{"version":1}]`, RelatedCommands: []string{"dashboards get", "dashboards render"}, TokenCost: "small"},
				{Name: "render", Description: "Render a dashboard or panel to a PNG file", ReadOnly: true, Flags: []discoveryFlag{{Name: "--uid", Type: "string", Description: "Dashboard UID"}, {Name: "--slug", Type: "string", Description: "Dashboard slug"}, {Name: "--panel-id", Type: "int", Default: 0, Description: "Panel ID"}, {Name: "--width", Type: "int", Default: 1600, Description: "Output width"}, {Name: "--height", Type: "int", Default: 900, Description: "Output height"}, {Name: "--theme", Type: "string", Default: "light", Description: "Render theme"}, {Name: "--from", Type: "string", Default: "now-6h", Description: "Time range start"}, {Name: "--to", Type: "string", Default: "now", Description: "Time range end"}, {Name: "--tz", Type: "string", Default: "UTC", Description: "Time zone"}, {Name: "--out", Type: "path", Description: "Destination PNG path"}}, Examples: []string{"grafana dashboards render --uid incident-overview --panel-id 4 --out /tmp/panel.png"}, OutputShape: `{"path":"/tmp/panel.png","bytes":12345}`, RelatedCommands: []string{"dashboards get", "annotations list"}, TokenCost: "medium"},
				{Name: "share", Description: "Create a Grafana short URL for a dashboard or panel share link", ReadOnly: true, Flags: []discoveryFlag{{Name: "--uid", Type: "string", Description: "Dashboard UID"}, {Name: "--slug", Type: "string", Description: "Dashboard slug"}, {Name: "--panel-id", Type: "int", Default: 0, Description: "Panel ID for solo-panel shares"}, {Name: "--from", Type: "string", Description: "Time range start"}, {Name: "--to", Type: "string", Description: "Time range end"}, {Name: "--theme", Type: "string", Description: "Theme override"}, {Name: "--org-id", Type: "int", Default: 0, Description: "Organization ID override"}}, Examples: []string{"grafana dashboards share --uid incident-overview", "grafana dashboards share --uid incident-overview --panel-id 4 --from now-6h --to now"}, OutputShape: `{"uid":"incident-overview","share_path":"/d/incident-overview/share","url":"/goto/abc123"}`, RelatedCommands: []string{"dashboards get", "dashboards render"}, TokenCost: "small"},
			},
		},
		{
			Name:        "datasources",
			Description: "Inspect and query datasources through Grafana",
			ReadOnly:    true,
			Subcommands: datasourceDiscoveryCommands(),
		},
		{
			Name:        "folders",
			Description: "Inspect dashboard folders",
			ReadOnly:    true,
			Subcommands: []discoveryCommand{
				{Name: "list", Description: "List folders", ReadOnly: true, Examples: []string{"grafana folders list"}, OutputShape: `[{"uid":"root"}]`, RelatedCommands: []string{"folders get", "dashboards list"}, TokenCost: "small"},
				{Name: "get", Description: "Get one folder by UID", ReadOnly: true, Flags: []discoveryFlag{{Name: "--uid", Type: "string", Description: "Folder UID"}}, Examples: []string{"grafana folders get --uid ops"}, OutputShape: `{"uid":"ops"}`, RelatedCommands: []string{"folders list", "dashboards list"}, TokenCost: "small"},
			},
		},
		{
			Name:        "annotations",
			Description: "Inspect dashboard and panel annotations",
			ReadOnly:    true,
			Subcommands: []discoveryCommand{
				{Name: "list", Description: "List annotations for a dashboard or panel", ReadOnly: true, Flags: []discoveryFlag{{Name: "--dashboard-uid", Type: "string", Description: "Dashboard UID"}, {Name: "--panel-id", Type: "int", Default: 0, Description: "Panel ID"}, {Name: "--limit", Type: "int", Default: 100, Description: "Maximum annotations"}, {Name: "--from", Type: "string", Description: "Time range start"}, {Name: "--to", Type: "string", Description: "Time range end"}, {Name: "--tags", Type: "csv", Description: "Tag filters"}, {Name: "--type", Type: "string", Description: "Annotation type"}}, Examples: []string{"grafana annotations list --dashboard-uid ops --tags prod,error"}, OutputShape: `[{"id":1}]`, RelatedCommands: []string{"dashboards render", "folders get"}, TokenCost: "small"},
			},
		},
		{
			Name:        "alerting",
			Description: "Inspect alerting configuration",
			ReadOnly:    true,
			Subcommands: []discoveryCommand{
				{Name: "rules", Description: "Inspect alert rules", ReadOnly: true, Subcommands: []discoveryCommand{{Name: "list", Description: "List alert rules", ReadOnly: true, Examples: []string{"grafana alerting rules list"}, OutputShape: `[{"uid":"rule-1"}]`, RelatedCommands: []string{"alerting contact-points list", "alerting policies get"}, TokenCost: "small"}}},
				{Name: "contact-points", Description: "Inspect alert contact points", ReadOnly: true, Subcommands: []discoveryCommand{{Name: "list", Description: "List contact points", ReadOnly: true, Examples: []string{"grafana alerting contact-points list"}, OutputShape: `[{"name":"pagerduty"}]`, RelatedCommands: []string{"alerting rules list", "alerting policies get"}, TokenCost: "small"}}},
				{Name: "policies", Description: "Inspect alert routing policies", ReadOnly: true, Subcommands: []discoveryCommand{{Name: "get", Description: "Get the alert policy tree", ReadOnly: true, Examples: []string{"grafana alerting policies get"}, OutputShape: `{"receiver":"default"}`, RelatedCommands: []string{"alerting rules list", "alerting contact-points list"}, TokenCost: "small"}}},
			},
		},
		{
			Name:        "query-history",
			Description: "Inspect saved Explore query history with bounded pagination metadata",
			ReadOnly:    true,
			Subcommands: []discoveryCommand{
				{Name: "list", Description: "List query history entries with server-side search and paging", ReadOnly: true, Flags: []discoveryFlag{{Name: "--datasource-uid", Type: "csv", Description: "Datasource UID filter"}, {Name: "--search", Type: "string", Description: "Search text across queries and comments"}, {Name: "--starred", Type: "bool", Default: false, Description: "Only starred queries"}, {Name: "--sort", Type: "string", Default: "time-desc", Description: "time-desc or time-asc"}, {Name: "--page", Type: "int", Default: 1, Description: "Page number"}, {Name: "--limit", Type: "int", Default: 100, Description: "Page size"}, {Name: "--from", Type: "string", Description: "Time range start"}, {Name: "--to", Type: "string", Description: "Time range end"}}, Examples: []string{`grafana query-history list --search checkout --limit 20`, `grafana query-history list --datasource-uid loki-uid --starred --from 24h`}, OutputShape: `{"result":{"queryHistory":[...],"totalCount":42}}`, RelatedCommands: []string{"datasources list", "runtime logs query"}, TokenCost: "small"},
			},
		},
		{
			Name:        "slo",
			Description: "Inspect SLO definitions from the Grafana SLO app",
			ReadOnly:    true,
			Subcommands: []discoveryCommand{
				{Name: "list", Description: "List SLO definitions with local filtering and truncation metadata", ReadOnly: true, Flags: []discoveryFlag{{Name: "--query", Type: "string", Description: "Match by name or description"}, {Name: "--limit", Type: "int", Default: 100, Description: "Maximum SLOs to return"}}, Examples: []string{`grafana slo list`, `grafana slo list --query checkout --limit 20`}, OutputShape: `[{"name":"checkout-availability"}]`, RelatedCommands: []string{"incident analyze", "alerting rules list"}, TokenCost: "small"},
			},
		},
		{
			Name:        "irm",
			Description: "Inspect Grafana IRM incidents with compact preview payloads",
			ReadOnly:    true,
			Subcommands: []discoveryCommand{
				{Name: "incidents", Description: "Inspect incidents", ReadOnly: true, Subcommands: []discoveryCommand{{Name: "list", Description: "List incident previews from Grafana IRM", ReadOnly: true, Flags: []discoveryFlag{{Name: "--query", Type: "string", Description: "Incident search text"}, {Name: "--limit", Type: "int", Default: 20, Description: "Maximum incidents"}, {Name: "--order-field", Type: "string", Default: "createdAt", Description: "Incident sort field"}, {Name: "--order-direction", Type: "string", Default: "desc", Description: "asc or desc"}}, Examples: []string{`grafana irm incidents list --query checkout --limit 10`, `grafana irm incidents list --order-field updatedAt --order-direction asc`}, OutputShape: `{"results":[{"incident":{"title":"Checkout latency"}}]}`, RelatedCommands: []string{"incident analyze", "oncall schedules list"}, TokenCost: "small"}}},
			},
		},
		{
			Name:        "oncall",
			Description: "Inspect Grafana OnCall schedules through the OnCall API",
			ReadOnly:    true,
			Subcommands: []discoveryCommand{
				{Name: "schedules", Description: "Inspect OnCall schedules", ReadOnly: true, Subcommands: []discoveryCommand{{Name: "list", Description: "List OnCall schedules with compact filtering metadata", ReadOnly: true, Flags: []discoveryFlag{{Name: "--query", Type: "string", Description: "Schedule name or team filter"}, {Name: "--limit", Type: "int", Default: 50, Description: "Maximum schedules"}}, Examples: []string{`grafana oncall schedules list --query primary`, `grafana oncall schedules list --limit 20`}, OutputShape: `{"results":[{"name":"Primary OnCall"}]}`, RelatedCommands: []string{"auth doctor", "irm incidents list"}, TokenCost: "small"}}},
			},
		},
		{
			Name:        "assistant",
			Description: "Interact with Grafana Assistant",
			ReadOnly:    false,
			Subcommands: []discoveryCommand{
				{Name: "chat", Description: "Send a prompt to Grafana Assistant", ReadOnly: false, Flags: []discoveryFlag{{Name: "--prompt", Type: "string", Description: "Assistant prompt"}, {Name: "--chat-id", Type: "string", Description: "Existing chat ID"}}, Examples: []string{`grafana assistant chat --prompt "Investigate elevated error rate"`}, OutputShape: `{"chatId":"chat_123"}`, RelatedCommands: []string{"assistant status", "assistant skills"}, TokenCost: "medium"},
				{Name: "investigate", Description: "Start an assistant investigation", ReadOnly: false, Flags: []discoveryFlag{{Name: "--goal", Type: "string", Description: "Investigation goal"}, {Name: "--chat-id", Type: "string", Description: "Existing chat ID"}}, Examples: []string{`grafana assistant investigate --goal "Investigate checkout latency spike"`}, OutputShape: `{"goal":"Investigate checkout latency spike","chat":{"chatId":"chat_123"}}`, RelatedCommands: []string{"assistant status", "incident analyze", "runtime logs query"}, TokenCost: "medium"},
				{Name: "status", Description: "Poll Assistant chat status", ReadOnly: true, Flags: []discoveryFlag{{Name: "--chat-id", Type: "string", Description: "Chat ID"}}, Examples: []string{"grafana assistant status --chat-id chat_123"}, OutputShape: `{"status":"completed"}`, RelatedCommands: []string{"assistant chat", "assistant skills"}, TokenCost: "small"},
				{Name: "skills", Description: "List Assistant skills", ReadOnly: true, Examples: []string{"grafana assistant skills"}, OutputShape: `{"items":[...]}`, RelatedCommands: []string{"assistant chat", "incident analyze"}, TokenCost: "small"},
			},
		},
		{
			Name:        "synthetics",
			Description: "Inspect Synthetic Monitoring checks",
			ReadOnly:    true,
			Subcommands: []discoveryCommand{
				{Name: "checks", Description: "Inspect Synthetic Monitoring checks", ReadOnly: true, Subcommands: []discoveryCommand{
					{Name: "list", Description: "List checks", ReadOnly: true, Flags: []discoveryFlag{{Name: "--backend-url", Type: "string", Description: "Backend address"}, {Name: "--token", Type: "string", Description: "Access token"}, {Name: "--include-alerts", Type: "bool", Default: false, Description: "Include alerts"}}, Examples: []string{`grafana synthetics checks list --backend-url synthetic-monitoring-api-us-east-0.grafana.net --token "$GRAFANA_SYNTHETICS_TOKEN"`}, OutputShape: `[{"id":123,"job":"checkout"}]`, RelatedCommands: []string{"synthetics checks get", "assistant investigate"}, TokenCost: "small"},
					{Name: "get", Description: "Get one check", ReadOnly: true, Flags: []discoveryFlag{{Name: "--backend-url", Type: "string", Description: "Backend address"}, {Name: "--token", Type: "string", Description: "Access token"}, {Name: "--id", Type: "int", Description: "Check ID"}}, Examples: []string{`grafana synthetics checks get --backend-url synthetic-monitoring-api-us-east-0.grafana.net --token "$GRAFANA_SYNTHETICS_TOKEN" --id 123`}, OutputShape: `{"check":{"id":123,"job":"checkout"}}`, RelatedCommands: []string{"synthetics checks list", "runtime metrics query"}, TokenCost: "small"},
				}},
			},
		},
		{
			Name:        "runtime",
			Description: "Query metrics, logs, and traces for incident investigation",
			ReadOnly:    true,
			Subcommands: []discoveryCommand{
				{Name: "metrics", Description: "Query metrics", ReadOnly: true, Subcommands: []discoveryCommand{{Name: "query", Description: "Run a PromQL range query", ReadOnly: true, Flags: []discoveryFlag{{Name: "--expr", Type: "string", Description: "PromQL expression"}, {Name: "--start", Type: "string", Description: "Time range start"}, {Name: "--end", Type: "string", Description: "Time range end"}, {Name: "--step", Type: "string", Default: "30s", Description: "Query step"}}, Examples: []string{`grafana runtime metrics query --expr 'sum(rate(http_requests_total[5m]))' --start 1h`}, OutputShape: `{"data":{"result":[...]}}`, RelatedCommands: []string{"runtime logs query", "aggregate snapshot"}, TokenCost: "medium"}}},
				{Name: "logs", Description: "Query logs", ReadOnly: true, Subcommands: []discoveryCommand{
					{Name: "query", Description: "Run a LogQL range query", ReadOnly: true, Flags: []discoveryFlag{{Name: "--query", Type: "string", Description: "LogQL query"}, {Name: "--start", Type: "string", Description: "Time range start"}, {Name: "--end", Type: "string", Description: "Time range end"}, {Name: "--limit", Type: "int", Default: 200, Description: "Maximum log streams"}}, Examples: []string{`grafana runtime logs query --query '{app="checkout"} |= "error"' --start 30m`}, OutputShape: `{"data":{"result":[...]}}`, RelatedCommands: []string{"runtime logs aggregate", "incident analyze"}, TokenCost: "medium"},
					{Name: "aggregate", Description: "Summarize a LogQL query into stream and label counts", ReadOnly: true, Flags: []discoveryFlag{{Name: "--query", Type: "string", Description: "LogQL query"}, {Name: "--start", Type: "string", Description: "Time range start"}, {Name: "--end", Type: "string", Description: "Time range end"}, {Name: "--limit", Type: "int", Default: 200, Description: "Maximum log streams to inspect"}}, Examples: []string{`grafana runtime logs aggregate --query '{app="checkout"} |= "error"' --start 30m`}, OutputShape: `{"streams":12,"entries":96,"label_keys":["app","level"]}`, RelatedCommands: []string{"runtime logs query", "incident analyze"}, TokenCost: "small"},
				}},
				{Name: "traces", Description: "Search traces", ReadOnly: true, Subcommands: []discoveryCommand{
					{Name: "search", Description: "Run a TraceQL search", ReadOnly: true, Flags: []discoveryFlag{{Name: "--query", Type: "string", Description: "TraceQL query"}, {Name: "--start", Type: "string", Description: "Time range start"}, {Name: "--end", Type: "string", Description: "Time range end"}, {Name: "--limit", Type: "int", Default: 200, Description: "Maximum trace matches"}}, Examples: []string{`grafana runtime traces search --query '{ span.http.status_code >= 500 }' --start 30m`}, OutputShape: `{"traces":[...]}`, RelatedCommands: []string{"runtime traces aggregate", "incident analyze"}, TokenCost: "medium"},
					{Name: "aggregate", Description: "Summarize a TraceQL search into services and root operations", ReadOnly: true, Flags: []discoveryFlag{{Name: "--query", Type: "string", Description: "TraceQL query"}, {Name: "--start", Type: "string", Description: "Time range start"}, {Name: "--end", Type: "string", Description: "Time range end"}, {Name: "--limit", Type: "int", Default: 200, Description: "Maximum trace matches to inspect"}}, Examples: []string{`grafana runtime traces aggregate --query '{ status = error }' --start 30m`}, OutputShape: `{"trace_matches":18,"services":["checkout"],"root_operations":["GET /checkout"]}`, RelatedCommands: []string{"runtime traces search", "incident analyze"}, TokenCost: "small"},
				}},
			},
		},
		{
			Name:        "aggregate",
			Description: "Fetch a compact multi-signal snapshot across metrics, logs, and traces",
			ReadOnly:    true,
			Subcommands: []discoveryCommand{
				{Name: "snapshot", Description: "Query metrics, logs, and traces in one bounded call", ReadOnly: true, Flags: []discoveryFlag{{Name: "--metric-expr", Type: "string", Description: "PromQL expression"}, {Name: "--log-query", Type: "string", Description: "LogQL query"}, {Name: "--trace-query", Type: "string", Description: "TraceQL query"}, {Name: "--start", Type: "string", Description: "Time range start"}, {Name: "--end", Type: "string", Description: "Time range end"}, {Name: "--step", Type: "string", Default: "30s", Description: "Metrics step"}, {Name: "--limit", Type: "int", Default: 200, Description: "Maximum logs and traces"}}, Examples: []string{`grafana aggregate snapshot --metric-expr 'sum(rate(http_requests_total[5m]))' --log-query '{app="checkout"}' --trace-query '{ resource.service.name = "checkout" }'`}, OutputShape: `{"metrics":{...},"logs":{...},"traces":{...}}`, RelatedCommands: []string{"incident analyze", "runtime metrics query"}, TokenCost: "medium"},
			},
		},
		{
			Name:        "incident",
			Description: "Run a summary-first incident investigation workflow",
			ReadOnly:    true,
			Subcommands: []discoveryCommand{
				{Name: "analyze", Description: "Generate a playbook-driven incident summary", ReadOnly: true, Flags: []discoveryFlag{{Name: "--goal", Type: "string", Description: "Incident goal"}, {Name: "--metric-expr", Type: "string", Description: "Override metric expression"}, {Name: "--log-query", Type: "string", Description: "Override log query"}, {Name: "--trace-query", Type: "string", Description: "Override trace query"}, {Name: "--start", Type: "string", Description: "Time range start"}, {Name: "--end", Type: "string", Description: "Time range end"}, {Name: "--step", Type: "string", Description: "Metrics step"}, {Name: "--limit", Type: "int", Description: "Maximum logs and traces"}, {Name: "--include-raw", Type: "bool", Default: false, Description: "Include the raw snapshot payload"}}, Examples: []string{`grafana incident analyze --goal "Investigate checkout latency spike"`}, OutputShape: `{"goal":"...","summary":{"metrics_series":1},"datasource_summary":{"count":3},"query_hints":[{"family":"prometheus","command":"grafana datasources prometheus query --uid ..."}]}`, RelatedCommands: []string{"aggregate snapshot", "assistant chat", "datasources list"}, TokenCost: "medium"},
			},
		},
		{
			Name:        "agent",
			Description: "Run compact planning and execution workflows for agents",
			ReadOnly:    true,
			Subcommands: []discoveryCommand{
				{Name: "plan", Description: "Return the investigation plan without executing it", ReadOnly: true, Flags: []discoveryFlag{{Name: "--goal", Type: "string", Description: "Automation goal"}, {Name: "--include-raw", Type: "bool", Default: false, Description: "Include raw payloads when running"}}, Examples: []string{`grafana agent plan --goal "Investigate elevated error rate"`}, OutputShape: `{"goal":"...","actions":[{"type":"datasource_inventory","command":"grafana datasources list"}]}`, RelatedCommands: []string{"agent run", "incident analyze", "datasources list"}, TokenCost: "small"},
				{Name: "run", Description: "Execute the investigation plan against Grafana", ReadOnly: true, Flags: []discoveryFlag{{Name: "--goal", Type: "string", Description: "Automation goal"}, {Name: "--include-raw", Type: "bool", Default: false, Description: "Include raw payloads"}}, Examples: []string{`grafana agent run --goal "Investigate elevated error rate"`}, OutputShape: `{"plan":{...},"summary":{...},"datasource_summary":{"count":3},"query_hints":[{"family":"loki","command":"grafana datasources loki query --uid ..."}]}`, RelatedCommands: []string{"agent plan", "incident analyze", "datasources list"}, TokenCost: "medium"},
			},
		},
	}
}

func datasourceDiscoveryCommands() []discoveryCommand {
	commands := []discoveryCommand{
		{
			Name:             "list",
			Description:      "List datasources with optional filtering",
			ReadOnly:         true,
			DocumentationURL: "https://grafana.com/docs/grafana/latest/developers/http_api/data_source/",
			Flags: []discoveryFlag{
				{Name: "--type", Type: "string", Description: "Datasource type filter"},
				{Name: "--name", Type: "string", Description: "Datasource name substring"},
			},
			Examples:        []string{"grafana datasources list --type loki", "grafana datasources list --name checkout"},
			OutputShape:     `[{"uid":"loki-uid","name":"loki","type":"loki","typed_family":"loki","documentation_url":"https://grafana.com/docs/grafana/latest/datasources/loki/query-editor/","capabilities":{"typed_query":true},"raw":{"uid":"loki-uid","name":"loki","type":"loki"}}]`,
			RelatedCommands: []string{"datasources get", "datasources health", "datasources query"},
			TokenCost:       "small",
			Notes: []string{
				"List output is normalized for agents and preserves the original Grafana datasource object under raw.",
			},
		},
		{
			Name:             "get",
			Description:      "Get one datasource by UID or unique name",
			ReadOnly:         true,
			DocumentationURL: "https://grafana.com/docs/grafana/latest/developers/http_api/data_source/",
			Flags:            datasourceSelectionDiscoveryFlags(),
			Examples:         []string{"grafana datasources get --uid loki-uid", "grafana datasources get --name loki --datasource-type loki"},
			OutputShape:      `{"uid":"loki-uid","name":"loki","type":"loki","typed_family":"loki","typed_flags":["--expr","--query-type"],"help":{"typed_query_help":"grafana datasources loki query --help"},"raw":{"uid":"loki-uid","name":"loki","type":"loki"}}`,
			RelatedCommands: []string{
				"datasources list",
				"datasources health",
				"datasources query",
			},
			TokenCost: "small",
			Notes: []string{
				"Get output preserves the raw Grafana payload and adds capability sugar such as typed_family, typed_flags, and help commands.",
			},
		},
		{
			Name:             "health",
			Description:      "Run the datasource health check by UID or unique name",
			ReadOnly:         true,
			DocumentationURL: "https://grafana.com/docs/grafana/latest/developers/http_api/data_source/",
			Flags:            datasourceSelectionDiscoveryFlags(),
			Examples:         []string{"grafana datasources health --uid prometheus-uid", "grafana datasources health --name prometheus"},
			OutputShape:      `{"status":"OK","message":"Data source is working"}`,
			RelatedCommands: []string{
				"datasources get",
				"datasources query",
				"auth doctor",
			},
			TokenCost: "small",
		},
		{
			Name:             "resources",
			Description:      "Call datasource resource endpoints exposed by the plugin backend",
			ReadOnly:         true,
			DocumentationURL: "https://grafana.com/docs/grafana/latest/developers/http_api/data_source/",
			Subcommands: []discoveryCommand{
				{
					Name:             "get",
					Description:      "GET one datasource resource path",
					ReadOnly:         true,
					DocumentationURL: "https://grafana.com/docs/grafana/latest/developers/http_api/data_source/",
					Flags: append(datasourceSelectionDiscoveryFlags(),
						discoveryFlag{Name: "--path", Type: "string", Description: "Resource path below /resources/"},
					),
					Examples:        []string{`grafana datasources resources get --uid cloudwatch-uid --path namespaces`, `grafana datasources resources get --name cloudwatch --datasource-type cloudwatch --path namespaces`},
					OutputShape:     `{"items":[...]}`,
					RelatedCommands: []string{"datasources get", "datasources resources post", "datasources query"},
					TokenCost:       "small",
				},
				{
					Name:             "post",
					Description:      "POST JSON to one datasource resource path",
					ReadOnly:         true,
					DocumentationURL: "https://grafana.com/docs/grafana/latest/developers/http_api/data_source/",
					Flags: append(datasourceSelectionDiscoveryFlags(),
						discoveryFlag{Name: "--path", Type: "string", Description: "Resource path below /resources/"},
						discoveryFlag{Name: "--body", Type: "json", Description: "JSON request body"},
					),
					Examples:        []string{`grafana datasources resources post --uid clickhouse-uid --path validate --body '{"query":"SELECT 1"}'`},
					OutputShape:     `{"status":"ok"}`,
					RelatedCommands: []string{"datasources resources get", "datasources query", "api"},
					TokenCost:       "small",
				},
			},
		},
		{
			Name:             "query",
			Description:      "Run a generic datasource query via POST /api/ds/query",
			ReadOnly:         true,
			DocumentationURL: "https://grafana.com/docs/grafana/latest/developers/http_api/data_source/",
			Notes: []string{
				"Use the generic query command when a datasource family has no typed flags for the payload you need.",
				"Grafana accepts datasource-specific query objects, so unsupported plugin fields should be passed with --query-json or --queries-json.",
			},
			Flags:           datasourceQueryDiscoveryFlags(nil),
			Examples:        []string{`grafana datasources query --uid mysql-uid --query-json '{"rawSql":"SELECT 1","format":"table"}'`, `grafana datasources query --name cloudwatch --datasource-type cloudwatch --queries-json '[{"refId":"A","namespace":"AWS/EC2","metricName":"CPUUtilization","statistics":["Average"],"region":"us-east-1"}]'`},
			OutputShape:     `{"results":{"A":{"frames":[...]}}}`,
			RelatedCommands: []string{"datasources get", "datasources health", "api"},
			TokenCost:       "medium",
		},
	}
	for _, strategy := range datasourceQueryStrategies() {
		family := strategy.Family()
		commands = append(commands, discoveryCommand{
			Name:             family.Name,
			Description:      family.Description,
			ReadOnly:         true,
			DocumentationURL: family.DocumentationURL,
			Subcommands: []discoveryCommand{
				{
					Name:             "query",
					Description:      "Run a " + family.Name + " datasource query through Grafana",
					ReadOnly:         true,
					DocumentationURL: family.DocumentationURL,
					Notes:            family.Notes,
					Flags:            datasourceQueryDiscoveryFlags(strategy),
					Examples:         strategy.Examples(),
					OutputShape:      `{"results":{"A":{"frames":[...]}}}`,
					RelatedCommands:  []string{"datasources query", "datasources get", "datasources health"},
					TokenCost:        "medium",
				},
			},
		})
	}
	return commands
}

func datasourceSelectionDiscoveryFlags() []discoveryFlag {
	return []discoveryFlag{
		{Name: "--uid", Type: "string", Description: "Datasource UID"},
		{Name: "--name", Type: "string", Description: "Datasource name when it resolves uniquely"},
		{Name: "--datasource-type", Type: "string", Description: "Optional datasource plugin type"},
	}
}

func datasourceQueryDiscoveryFlags(strategy datasourceQueryStrategy) []discoveryFlag {
	flags := append([]discoveryFlag{}, datasourceSelectionDiscoveryFlags()...)
	flags = append(flags,
		discoveryFlag{Name: "--from", Type: "string", Default: "now-1h", Description: "Grafana query range start"},
		discoveryFlag{Name: "--to", Type: "string", Default: "now", Description: "Grafana query range end"},
		discoveryFlag{Name: "--ref-id", Type: "string", Description: "Override refId for a single query"},
		discoveryFlag{Name: "--interval-ms", Type: "int", Default: 1000, Description: "Default intervalMs applied to each query"},
		discoveryFlag{Name: "--max-data-points", Type: "int", Default: 43200, Description: "Default maxDataPoints applied to each query"},
	)
	if strategy != nil {
		flags = append(flags, strategy.DiscoveryFlags()...)
	}
	flags = append(flags,
		discoveryFlag{Name: "--query-json", Type: "json", Description: "One datasource query object"},
		discoveryFlag{Name: "--queries-json", Type: "json", Description: "Full datasource query array"},
	)
	return flags
}

func discoveryGlobalFlags() []discoveryFlag {
	return []discoveryFlag{
		{Name: "--output", Type: "string", Default: "json", Description: "Output format: json, pretty, table"},
		{Name: "--fields", Type: "csv", Description: "Project only selected fields from the JSON payload"},
		{Name: "--json", Type: "csv", Description: "Alias for --fields with JSON output"},
		{Name: "--jq", Type: "string", Description: "Apply a jq expression to the payload"},
		{Name: "--template", Type: "string", Description: "Render a Go template against the payload"},
		{Name: "--agent", Type: "bool", Default: false, Description: "Wrap results in an agent envelope with metadata"},
		{Name: "--read-only", Type: "bool", Default: false, Description: "Block commands that mutate local or remote state"},
		{Name: "--yes", Type: "bool", Default: false, Description: "Confirm destructive commands without an interactive prompt"},
	}
}

func discoveryAuthDocs() map[string]any {
	return map[string]any{
		"login":         `grafana auth login --token "$GRAFANA_TOKEN" --stack "<stack-slug>"`,
		"diagnostics":   "grafana auth doctor",
		"token_storage": "Token is stored via the OS keyring when available, with file fallback",
		"expanded_help": "grafana schema --full runtime",
	}
}

func discoveryBestPractices(path []string) []string {
	practices := []string{
		"Use subtree help before reading external docs: grafana <domain> --help",
		"Prefer summary-first commands for investigations before fetching raw payloads",
		"Use --json, --jq, or --template to keep responses narrow in agent loops",
		"Keep default time windows small unless the investigation requires a wider range",
	}
	if hasPathPrefix(path, "runtime") || hasPathPrefix(path, "incident") || hasPathPrefix(path, "aggregate") || hasPathPrefix(path, "query-history") || hasPathPrefix(path, "datasources") {
		practices = append(practices, "Start with a 30m or 1h window, then widen only when the signal is too sparse")
	}
	if hasPathPrefix(path, "datasources") {
		practices = append(practices, "Use datasources get or datasources health before the first query when the datasource capabilities are unclear")
		practices = append(practices, "Prefer --query-json for one query and --queries-json only when you need multiple refIds in one round trip")
	}
	return practices
}

func discoveryAntiPatterns(path []string) []string {
	antiPatterns := []string{
		"Do not fetch broad raw payloads first when a summary or filtered query would do",
		"Do not widen time ranges and limits at the same time unless the first query was clearly too narrow",
		"Do not use raw api calls when a dedicated command already exposes a stable contract",
	}
	if hasPathPrefix(path, "runtime") || hasPathPrefix(path, "incident") || hasPathPrefix(path, "aggregate") {
		antiPatterns = append(antiPatterns, "Do not omit the query goal and then expect the CLI to infer a useful incident scope")
	}
	if hasPathPrefix(path, "datasources") {
		antiPatterns = append(antiPatterns, "Do not assume plugin query JSON is portable across datasource families")
		antiPatterns = append(antiPatterns, "Do not bypass datasource health and resource endpoints when the plugin exposes discovery helpers you can call first")
	}
	return antiPatterns
}

func discoveryTimeFormatsDoc(path []string) discoveryTimeFormats {
	if !(hasPathPrefix(path, "runtime") || hasPathPrefix(path, "aggregate") || hasPathPrefix(path, "incident") || hasPathPrefix(path, "query-history") || hasPathPrefix(path, "datasources") || len(path) == 0) {
		return discoveryTimeFormats{}
	}
	return discoveryTimeFormats{
		Relative: []string{"5m", "30m", "1h", "24h", "7d", "1w", "now-30m", "now"},
		Absolute: []string{"RFC3339 timestamps such as 2026-03-06T14:04:00Z", "Unix timestamps are passed through unchanged"},
		Examples: []string{"--start 30m --end now", "--start now-2h --end now", "--start 2026-03-06T13:00:00Z --end 2026-03-06T14:00:00Z"},
	}
}

func discoveryQuerySyntax(path []string) map[string]string {
	all := map[string]string{
		"metrics":          `PromQL expressions such as sum(rate(http_requests_total[5m])) by (service)`,
		"logs":             `LogQL expressions such as {app="checkout"} |= "error"`,
		"traces":           `TraceQL expressions such as { resource.service.name = "checkout" && span.http.status_code >= 500 }`,
		"datasource_query": datasourceQuerySyntaxDocs()["datasource_query"],
	}
	for key, value := range datasourceQuerySyntaxDocs() {
		all[key] = value
	}
	switch {
	case len(path) == 0:
		return all
	case hasPathPrefix(path, "datasources"):
		if len(path) >= 2 {
			if family, ok := findDatasourceQueryFamily(path[1]); ok {
				return map[string]string{
					"datasource_query": all["datasource_query"],
					family.Name:        family.Syntax,
				}
			}
		}
		return datasourceQuerySyntaxDocs()
	case hasPathPrefix(path, "runtime", "metrics"):
		return map[string]string{"metrics": all["metrics"]}
	case hasPathPrefix(path, "runtime", "logs"):
		return map[string]string{"logs": all["logs"]}
	case hasPathPrefix(path, "runtime", "traces"):
		return map[string]string{"traces": all["traces"]}
	case hasPathPrefix(path, "runtime"), hasPathPrefix(path, "aggregate"), hasPathPrefix(path, "incident"):
		return all
	default:
		return nil
	}
}

func discoveryWorkflows(path []string) []discoveryWorkflow {
	workflows := []discoveryWorkflow{
		{
			Name:      "Inspect Runtime Signals",
			TokenCost: "medium",
			Steps: []string{
				`grafana runtime metrics query --expr 'sum(rate(http_requests_total[5m])) by (service)' --start 30m`,
				`grafana runtime logs query --query '{app="checkout"} |= "error"' --start 30m --limit 20`,
				`grafana runtime traces search --query '{ resource.service.name = "checkout" }' --start 30m --limit 20`,
			},
		},
		{
			Name:      "Summarize An Incident",
			TokenCost: "small",
			Steps: []string{
				`grafana query-history list --search checkout --from 24h`,
				`grafana slo list --query checkout`,
				`grafana irm incidents list --query checkout --limit 10`,
				`grafana oncall schedules list --query checkout`,
				`grafana incident analyze --goal "Investigate checkout latency spike"`,
				`grafana --json summary incident analyze --goal "Investigate checkout latency spike"`,
			},
		},
		{
			Name:      "Inspect A Datasource Safely",
			TokenCost: "small",
			Steps: []string{
				`grafana datasources list --name checkout`,
				`grafana datasources get --uid mysql-uid`,
				`grafana datasources health --uid mysql-uid`,
				`grafana datasources mysql query --uid mysql-uid --query-json '{"rawSql":"SELECT 1","format":"table"}'`,
			},
		},
	}
	if len(path) == 0 || hasPathPrefix(path, "runtime") || hasPathPrefix(path, "incident") || hasPathPrefix(path, "aggregate") || hasPathPrefix(path, "query-history") || hasPathPrefix(path, "slo") || hasPathPrefix(path, "datasources") {
		return workflows
	}
	return nil
}

func hasPathPrefix(path []string, prefix ...string) bool {
	if len(prefix) == 0 || len(path) < len(prefix) {
		return false
	}
	for i := range prefix {
		if path[i] != prefix[i] {
			return false
		}
	}
	return true
}

func compactCommandPayload(command discoveryCommand, prefix []string) map[string]any {
	path := append(append([]string{}, prefix...), command.Name)
	payload := map[string]any{
		"name":        command.Name,
		"full_path":   strings.Join(path, " "),
		"description": command.Description,
		"read_only":   command.ReadOnly,
	}
	if command.TokenCost != "" {
		payload["token_cost"] = command.TokenCost
	}
	if len(command.Flags) > 0 {
		payload["flags"] = command.Flags
	}
	if len(command.Subcommands) > 0 {
		children := make([]map[string]any, 0, len(command.Subcommands))
		for _, child := range command.Subcommands {
			children = append(children, compactCommandPayload(child, path))
		}
		payload["subcommands"] = children
	}
	return payload
}

func fullCommandPayload(command discoveryCommand, prefix []string) map[string]any {
	payload := compactCommandPayload(command, prefix)
	if command.DocumentationURL != "" {
		payload["documentation_url"] = command.DocumentationURL
	}
	if command.OutputShape != "" {
		payload["output_shape"] = command.OutputShape
	}
	if len(command.Examples) > 0 {
		payload["examples"] = command.Examples
	}
	if len(command.RelatedCommands) > 0 {
		payload["related_commands"] = command.RelatedCommands
	}
	if len(command.Notes) > 0 {
		payload["notes"] = command.Notes
	}
	return payload
}

func buildDiscoveryPayload(path []string, compact bool) (map[string]any, error) {
	commands := discoveryCatalog()
	if len(path) > 0 {
		command, ok := findDiscoveryCommand(commands, path)
		if !ok {
			return nil, errors.New("unknown schema path")
		}
		commands = []discoveryCommand{command}
	}

	commandPayloads := make([]map[string]any, 0, len(commands))
	for _, command := range commands {
		if compact {
			commandPayloads = append(commandPayloads, compactCommandPayload(command, nil))
			continue
		}
		commandPayloads = append(commandPayloads, fullCommandPayload(command, nil))
	}
	if compact && len(path) == 0 {
		for _, payload := range commandPayloads {
			trimCompactRootPayload(payload)
		}
	}

	payload := map[string]any{
		"version":      cliVersion,
		"description":  "Agent-first CLI for Grafana and Grafana Cloud with deterministic, token-aware command contracts.",
		"auth":         discoveryAuthDocs(),
		"global_flags": discoveryGlobalFlags(),
		"commands":     commandPayloads,
	}
	if len(path) > 0 {
		payload["scope"] = strings.Join(path, " ")
	}
	if syntax := discoveryQuerySyntax(path); len(syntax) > 0 && (!compact || len(path) > 0) {
		payload["query_syntax"] = syntax
	}
	if timeFormats := discoveryTimeFormatsDoc(path); (len(timeFormats.Relative) > 0 || len(timeFormats.Absolute) > 0 || len(timeFormats.Examples) > 0) && (!compact || len(path) > 0) {
		payload["time_formats"] = timeFormats
	}
	if !compact {
		payload["best_practices"] = discoveryBestPractices(path)
		payload["anti_patterns"] = discoveryAntiPatterns(path)
		if workflows := discoveryWorkflows(path); len(workflows) > 0 {
			payload["workflows"] = workflows
		}
	}
	return payload, nil
}

func trimCompactRootPayload(payload map[string]any) {
	delete(payload, "flags")
	children, ok := payload["subcommands"].([]map[string]any)
	if !ok {
		return
	}
	for _, child := range children {
		trimCompactRootPayload(child)
	}
}

func findDiscoveryCommand(commands []discoveryCommand, path []string) (discoveryCommand, bool) {
	if len(path) == 0 {
		return discoveryCommand{}, false
	}
	for _, command := range commands {
		if command.Name != path[0] {
			continue
		}
		if len(path) == 1 {
			return command, true
		}
		return findDiscoveryCommand(command.Subcommands, path[1:])
	}
	return discoveryCommand{}, false
}

func discoveryPathFromArgs(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	path := make([]string, 0, len(args))
	commands := discoveryCatalog()
	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		if trimmed == "" || isHelpArg(trimmed) || strings.HasPrefix(trimmed, "-") {
			break
		}
		command, ok := findDiscoveryCommand(commands, []string{trimmed})
		if !ok {
			break
		}
		path = append(path, trimmed)
		commands = command.Subcommands
	}
	return path
}

func requestedHelpPath(args []string) ([]string, bool) {
	for _, arg := range args {
		if isHelpArg(arg) {
			return discoveryPathFromArgs(args), true
		}
	}
	return nil, false
}

func helpCompactForPath(path []string) bool {
	if len(path) == 0 {
		return true
	}
	command, ok := findDiscoveryCommand(discoveryCatalog(), path)
	if !ok {
		return true
	}
	return len(command.Subcommands) > 0
}
