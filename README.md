# grafana-cli

Agent-first CLI for Grafana and Grafana Cloud. Built for engineers working with AI coding agents like Codex and Claude Code.

## Why Your Agent Will Love It

- **Discoverable** - `grafana schema --compact` returns a machine-readable command catalog in one bounded call
- **Structured** - compact JSON by default with `--json`, `--jq`, and `--template` for output shaping
- **Secure** - OS keyring token storage, `--read-only` guardrail, `--yes` for explicit destructive confirms
- **Broad** - dashboards, alerting, logs, metrics, traces, assistant, incidents, OnCall, SLOs, service accounts, access policies, and synthetics
- **Token-aware** - every command is designed to minimize token usage in agent loops

## Try It

```bash
# Authenticate (--stack resolves Prometheus, Loki, and Tempo endpoints automatically)
grafana auth login --token "$GRAFANA_TOKEN" --stack my-stack

# Check what's configured
grafana auth doctor

# Inspect activation surfaces
grafana service-accounts list
grafana cloud access-policies list --region us
grafana cloud billed-usage get --org-slug my-org --year 2024 --month 9
grafana cloud stacks inspect --stack my-stack
grafana cloud stacks plugins list --stack my-stack

# Inspect Synthetic Monitoring checks
grafana synthetics checks list \
  --backend-url "$GRAFANA_SYNTHETICS_BACKEND_URL" \
  --token "$GRAFANA_SYNTHETICS_TOKEN"

# Search dashboards
grafana dashboards list --query latency --tag prod

# Inspect one datasource and its health
grafana datasources get --uid mysql-uid
grafana datasources health --uid mysql-uid

# List datasource capabilities without losing the raw Grafana payload
grafana datasources list --type prometheus

# Query a datasource through Grafana using plugin JSON
grafana datasources mysql query \
  --uid mysql-uid \
  --query-json '{"rawSql":"SELECT 1","format":"table"}'

# Use typed flags for documented query-editor workflows
grafana datasources prometheus query \
  --uid prometheus-uid \
  --expr 'sum(rate(http_requests_total[5m])) by (service)' \
  --min-step 1m

grafana datasources cloudwatch query \
  --uid cloudwatch-uid \
  --namespace AWS/EC2 \
  --metric-name CPUUtilization \
  --region us-east-1 \
  --statistic Average \
  --dimensions InstanceId=i-123 \
  --match-exact

# Query logs from the last 30 minutes
grafana runtime logs query --query '{app="checkout"} |= "error"' --start 30m

# Start an assistant-led investigation
grafana assistant investigate --goal "Investigate checkout latency spike"

# Investigate an incident
grafana incident analyze --goal "Investigate checkout latency spike"

# Shape output for your agent
grafana --json summary incident analyze --goal "Latency spike"
grafana --jq '.summary' incident analyze --goal "Latency spike"
# Wrap in a deterministic envelope for downstream parsing
grafana --agent incident analyze --goal "Latency spike"
```

## Installation

### Shell installer

Install the latest macOS or Linux release straight from GitHub Releases:

```bash
curl -fsSL https://raw.githubusercontent.com/matiasvillaverde/grafana-cli/main/scripts/install.sh | sh
```

Install into a custom directory or pin a version:

```bash
curl -fsSL https://raw.githubusercontent.com/matiasvillaverde/grafana-cli/main/scripts/install.sh | \
  BINDIR="$HOME/.local/bin" GRAFANA_INSTALL_VERSION=<release-tag> sh
```

The shell installer works with the current GitHub release archives on macOS and Linux.

### GitHub Releases

Download a prebuilt archive from [Releases](https://github.com/matiasvillaverde/grafana-cli/releases), extract it, and move `grafana` (or `grafana.exe`) into your `PATH`.

Binaries are published automatically on every merge to `main` for macOS, Linux, and Windows (`amd64` + `arm64`).

### Go install

Requires Go 1.24+.

```bash
go install github.com/matiasvillaverde/grafana-cli/cmd/grafana@latest
```

Ensure your Go bin directory is in your shell profile:

```bash
# Add to ~/.zshrc or ~/.bashrc
export PATH="$PATH:$(go env GOPATH)/bin"
```

## Command Coverage

<details>
<summary><strong>Auth & Config</strong></summary>

| Command | Description |
|---------|-------------|
| `auth login` | Store token and endpoint configuration |
| `auth status` | Show current auth status and endpoints |
| `auth doctor` | Diagnose missing configuration by capability |
| `auth logout` | Clear the current context token |
| `context list` | List configured contexts |
| `context use <name>` | Switch the active context |
| `context view` | Show current context configuration |
| `config list` | List config for a context |
| `config get <key>` | Read one config key |
| `config set <key> <value>` | Persist one config key |

</details>

<details>
<summary><strong>Dashboards & Datasources</strong></summary>

| Command | Description |
|---------|-------------|
| `dashboards list` | Search dashboards by query and tag |
| `dashboards get --uid ...` | Fetch one dashboard by UID |
| `dashboards create` | Create a dashboard from flags or `--template-json` |
| `dashboards delete --uid ...` | Delete a dashboard by UID |
| `dashboards versions --uid ...` | List dashboard version history |
| `dashboards render --uid ...` | Render a dashboard or panel to PNG |
| `datasources list` | List datasources with optional type/name filtering |
| `datasources get --uid ...` | Get one datasource by UID |
| `datasources health --uid ...` | Run a datasource health check |
| `datasources resources <get\|post>` | Call plugin resource endpoints |
| `datasources query --uid ...` | Run a generic datasource query via Grafana |
| `datasources <family> query --uid ...` | Query a supported datasource family with teachable help |

</details>

<details>
<summary><strong>Folders, Annotations & Alerting</strong></summary>

| Command | Description |
|---------|-------------|
| `folders list` | List dashboard folders |
| `folders get --uid ...` | Get one folder by UID |
| `annotations list` | List annotations for a dashboard or panel |
| `alerting rules list` | List alert rules |
| `alerting contact-points list` | List alert contact points |
| `alerting policies get` | Get the alert routing policy tree |

</details>

<details>
<summary><strong>Runtime Observability</strong></summary>

| Command | Description |
|---------|-------------|
| `runtime metrics query --expr ...` | Run a PromQL range query |
| `runtime logs query --query ...` | Run a LogQL range query |
| `runtime logs aggregate --query ...` | Summarize logs into stream and label counts |
| `runtime traces search --query ...` | Run a TraceQL search |
| `runtime traces aggregate --query ...` | Summarize traces into services and root operations |
| `aggregate snapshot` | Query metrics, logs, and traces in one bounded call |

All runtime commands support relative time inputs: `30m`, `1h`, `now-2h`, `2026-03-06T12:00:00Z`.

Datasource query commands also support Grafana-style relative bounds with `--from` and `--to`, defaulting to `now-1h` and `now`.

Family query help is doc-backed. `grafana datasources <family> query --help` and `grafana schema datasources <family> query` include the typed flags, examples, notes, and the matching Grafana documentation URL for that datasource family.

`datasources list` and `datasources get` return normalized agent-friendly fields such as `typed_family`, `typed_flags`, `documentation_url`, and `capabilities`, while preserving the original Grafana datasource object under `raw`.

`incident analyze` and `agent run` now include `datasource_summary` and `query_hints` so an agent can pivot from the high-level investigation result into concrete datasource commands immediately.

Supported datasource families:
`cloudwatch`, `clickhouse`, `mysql`, `postgres`, `mssql`, `influxdb`, `elasticsearch`, `opensearch`, `graphite`, `prometheus`, `loki`, `tempo`, `azure-monitor`.

</details>

<details>
<summary><strong>Investigation & Incidents</strong></summary>

| Command | Description |
|---------|-------------|
| `incident analyze --goal ...` | Generate a playbook-driven incident summary |
| `irm incidents list` | List IRM incident previews |
| `oncall schedules list` | List OnCall schedules |
| `query-history list` | Search saved Explore query history |
| `slo list` | List SLO definitions |

</details>

<details>
<summary><strong>Grafana Assistant</strong></summary>

| Command | Description |
|---------|-------------|
| `assistant chat --prompt ...` | Send a prompt to Grafana Assistant |
| `assistant investigate --goal ...` | Start an assistant-led investigation for an operational goal |
| `assistant status --chat-id ...` | Poll assistant chat status |
| `assistant skills` | List available assistant skills |

</details>

<details>
<summary><strong>Access & Synthetics</strong></summary>

| Command | Description |
|---------|-------------|
| `service-accounts list` | Search Grafana service accounts with paging metadata |
| `service-accounts get --id ...` | Fetch one service account by ID |
| `cloud access-policies list --region ...` | List Grafana Cloud access policies for a region with auto-pagination up to `--limit` |
| `cloud access-policies get --id ... --region ...` | Fetch one Grafana Cloud access policy |
| `cloud billed-usage get --org-slug ... --year ... --month ...` | Fetch billed usage for a Grafana Cloud organization and month |
| `cloud stacks plugins list --stack ...` | List plugins installed on a Grafana Cloud stack with auto-pagination up to `--limit` |
| `cloud stacks plugins get --stack ... --plugin ...` | Fetch one installed stack plugin |
| `synthetics checks list --backend-url ... --token ...` | List Synthetic Monitoring checks |
| `synthetics checks get --backend-url ... --token ... --id ...` | Fetch one Synthetic Monitoring check |

</details>

<details>
<summary><strong>Agent Workflows</strong></summary>

| Command | Description |
|---------|-------------|
| `agent plan --goal ...` | Return the investigation plan without executing |
| `agent run --goal ...` | Execute the investigation plan against Grafana |

</details>

<details>
<summary><strong>Cloud & Raw API</strong></summary>

| Command | Description |
|---------|-------------|
| `cloud stacks list` | List Grafana Cloud stacks |
| `cloud stacks inspect --stack ...` | Infer stack runtime endpoints and connectivity from the Cloud API |
| `api <METHOD> <PATH>` | Call the raw Grafana HTTP API directly |

</details>

## Global Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--output` | `string` | `json` | Output format: `json`, `pretty`, `table` |
| `--json` | `csv` | | Project only selected fields from the JSON payload |
| `--fields` | `csv` | | Alias for `--json` |
| `--jq` | `string` | | Apply a jq expression to the payload |
| `--template` | `string` | | Render a Go template against the payload |
| `--agent` | `bool` | `false` | Wrap results in `{status, data, metadata}` envelope |
| `--read-only` | `bool` | `false` | Block commands that mutate state |
| `--yes` | `bool` | `false` | Confirm destructive commands without prompt |

## Agent Mode

When `--agent` is passed, the CLI wraps every response in a deterministic envelope:

```json
{
  "status": "ok",
  "data": { "..." : "..." },
  "metadata": {
    "count": 12,
    "truncated": false,
    "command": "dashboards list"
  }
}
```

Metadata includes `count`, `truncated`, `command`, `next_action`, and optional `warnings`.

Cloud list commands follow server pagination up to `--limit`. `cloud stacks inspect` exits with an error in normal CLI mode if datasource or connection discovery is incomplete; in agent mode the same partial result is returned with warnings in `metadata.warnings`.

## Discovery

The CLI treats discovery as a first-class interface - agents can understand the full command surface without external docs.

```bash
grafana --help                        # Compact root schema
grafana runtime --help                # Compact runtime subtree
grafana runtime metrics query --help  # Expanded scoped help for one leaf command
grafana schema                       # Bounded machine-readable contract (compact by default)
grafana schema --full runtime metrics # Richer contract with examples, workflows, and query syntax
```

The schema includes command metadata, flag definitions, output shapes, query syntax examples, recommended workflows, best practices, and anti-patterns.

## Quality Gate

CI enforces **100% unit test coverage** and strict linting on every PR and release.

```bash
go test ./... -covermode=atomic -coverprofile=coverage.out
go tool cover -func=coverage.out | tail -n 1
$(go env GOPATH)/bin/golangci-lint run --timeout=5m
```

## Release Process

Releases are automatic on every merge to `main`. Versioning follows [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` -> minor bump
- `fix:`, `docs:`, `chore:` -> patch bump
- `BREAKING CHANGE` or `!` -> major bump

## Roadmap

- Broader Grafana Cloud product coverage (k6, profiles, frontend observability, deeper synthetic monitoring write flows)
- Richer agent execution plans and remediation actions
- Graph RAG for past incidents to reuse historical context during triage

See [docs/discovery-first-plan.md](docs/discovery-first-plan.md) for the detailed plan.

## Links

- [Architecture](docs/architecture.md)
- [Product Research](docs/product-research.md)
- [Contributing](CONTRIBUTING.md)
- [Agent Guidelines](AGENTS.md)
