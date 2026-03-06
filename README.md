# grafana-cli

Agent-first CLI to control Grafana and Grafana Cloud.

This project is a **WIP hackathon build by @matiasvillaverde and @marctc**.

## Why This Exists

Grafana is one of the most important systems engineers use to understand production. But most Grafana workflows are still human-first: dashboards, clicks, long API payloads, and broad UI context.

Agents need a different interface. This CLI gives Grafana a small, deterministic execution surface that an agent can use repeatedly during incident response and debugging.

This matters when an engineer is working with Codex or Claude Code and wants the agent to:

- understand and triage incidents
- query logs/metrics/traces fast
- inspect cloud stacks and data sources
- create dashboards programmatically
- run deterministic workflows with low token usage

## Why A CLI Matters For Agents

This project is about **context engineering** for observability workflows.

For Grafana tasks, a focused CLI is often better than a generic tool layer:

- CLI commands return narrow, task-specific JSON instead of broad tool schemas and verbose context.
- Agents can request only the fields they need (`--fields`) to reduce token usage.
- Command contracts are stable and deterministic, which improves reliability in long agent loops.
- Execution is composable in scripts/CI, so agents can chain investigation steps with minimal prompt overhead.

MCP is still useful as an integration layer, but it is often too wide for repeated debugging loops. The goal here is to make Grafana itself **agent-first**.

## Built For Codex And Claude Code

Engineers can instruct Codex or Claude Code to call this CLI directly to:

- debug incidents across metrics, logs, and traces
- inspect and reason about dashboards and datasources
- query Grafana Assistant for guided investigation
- automate dashboard creation and remediation playbooks

Example instruction to an agent:

```text
Use grafana-cli to investigate a latency spike in checkout, summarize only key metrics/log streams/trace matches, and propose a dashboard update.
```

## Agent-First Contract

- **Compact JSON by default** (`--output json` implied)
- **Optional readable output**: `--output pretty`
- **Optional tabular output**: `--output table` for common list and object responses
- **Structured selection**: `--json`, `--jq`, and `--template` for agent-safe output shaping
- **Token minimization**: `--fields` remains available as a compatibility alias for `--json`
- **Schema-driven discovery**: `grafana --help`, `grafana <domain> --help`, and `grafana schema` emit machine-readable command metadata
- **Agent envelopes**: `--agent` wraps responses in `{status,data,metadata}` for deterministic downstream handling
- **Read-only guardrail**: `--read-only` blocks mutating commands while keeping investigation workflows available
- **Explicit destructive confirmations**: `--yes` acknowledges destructive commands such as `auth logout`, `dashboards delete`, and raw API writes
- **Deterministic command behavior**: stable flags, stable output shapes
- **Composability**: each command is script/agent safe

## Discovery-First Interface

The CLI now treats discovery as part of the product rather than a side effect of help text.

- `grafana --help` returns a compact root schema for low-token discovery loops.
- `grafana <domain> --help` returns a compact subtree for that domain.
- `grafana <leaf command> --help` expands automatically so the scoped command includes flags, examples, output shape, and related commands.
- `grafana schema` returns the compact contract explicitly.
- `grafana schema --full [path...]` returns the richer command catalog, workflows, query syntax, and time-format guidance.

This is designed so an agent can answer four questions without opening external docs:

- What commands exist?
- Which flags are required?
- What shape will the output have?
- Which workflows are recommended for investigation?

## Installation

### GitHub Releases (recommended)

Prebuilt binaries are published automatically when commits are merged to `main`:

- https://github.com/matiasvillaverde/grafana-cli/releases

Download the archive for your platform, extract it, and move `grafana` (or `grafana.exe`) into your `PATH`.

### Go install

```bash
go install github.com/matiasvillaverde/grafana-cli/cmd/grafana@latest
```

Then ensure your Go bin directory is in `PATH`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

## Current Capabilities

- Discovery + interface contract
  - `schema [--compact|--full] [path...]`
  - structured help at root and subtree level
- Auth/session
  - `auth login|status|doctor|logout`
  - platform-aware token storage: native keyring first, local token file fallback
- Context and config management
  - `context list|use|view`
  - `config list|get|set`
- Raw API access
  - `api <METHOD> <PATH> [--body JSON]`
- Cloud inventory
  - `cloud stacks list`
- Investigation context
  - `query-history list [--search ... --starred --page ... --limit ...]`
  - `slo list [--query ... --limit ...]`
  - `irm incidents list [--query ... --limit ...]`
  - `oncall schedules list [--query ... --limit ...]`
- Incident + runtime investigation
  - `incident analyze --goal ...`
  - `runtime metrics query --expr ...`
  - `runtime logs query --query ...`
  - `runtime logs aggregate --query ...`
  - `runtime traces search --query ...`
  - `runtime traces aggregate --query ...`
  - `aggregate snapshot --metric-expr ... --log-query ... --trace-query ...`
  - relative or absolute time inputs: `30m`, `now-2h`, `2026-03-06T12:00:00Z`
- Dashboard and datasource operations
  - `dashboards list --query ... --tag ... --limit ...`
  - `dashboards get --uid ...`
  - `dashboards create --title ...` or `--template-json ...`
  - `dashboards delete --uid ...`
  - `dashboards versions --uid ... [--limit ...]`
  - `dashboards render --uid ... [--panel-id ...] --out ...`
  - `datasources list --type ... --name ...`
- Folder, annotation, and alerting reads
  - `folders list`
  - `folders get --uid ...`
  - `annotations list [--dashboard-uid ... --panel-id ... --tags ...]`
  - `alerting rules list`
  - `alerting contact-points list`
  - `alerting policies get`
- Grafana Assistant operations
  - `assistant chat --prompt ... [--chat-id ...]`
  - `assistant status --chat-id ...`
  - `assistant skills`
- Agent workflows
  - `agent plan --goal ...`
  - `agent run --goal ...`

## Quick Start

```bash
grafana --help
grafana runtime --help
grafana schema
grafana schema --full runtime metrics

grafana auth login \
  --token "$GRAFANA_TOKEN" \
  --stack "your-stack"

# The token is stored outside config.json using the OS keyring when available.

# Diagnose which capabilities are configured and which endpoints are missing.
grafana auth doctor

# Create and switch named contexts, like separate stacks or environments.
grafana auth login --context prod --token "$GRAFANA_TOKEN" --base-url "https://prod.grafana.net"
grafana context list
grafana context use prod
grafana config get base-url
grafana config set org-id 12

# Incident analysis (compact JSON)
grafana incident analyze --goal "Investigate elevated error rate"

# Pull saved exploration context from recent query history
grafana query-history list --search checkout --from 24h --limit 20

# Inspect matching SLOs before widening an incident scope
grafana slo list --query checkout --limit 20

# Inspect recent IRM incidents and OnCall schedules around the same service
grafana irm incidents list --query checkout --limit 10
grafana oncall schedules list --query checkout --limit 20

# Return only what the agent needs
grafana --json summary.metrics_series,summary.log_streams incident analyze --goal "Latency spike"
grafana --jq '.summary' incident analyze --goal "Latency spike"
grafana --template '{{.context}} {{.base_url}}' context view

# Use the agent envelope for deterministic downstream parsing
grafana --agent incident analyze --goal "Latency spike"

# Human-readable inspection for list/object responses
grafana --output table datasources list
grafana --output table auth doctor

# Use relative time ranges directly
grafana runtime logs query --query '{app="checkout"} |= "error"' --start 30m --end now
grafana runtime logs aggregate --query '{app="checkout"} |= "error"' --start 30m
grafana runtime traces aggregate --query '{ status = error }' --start 30m

# Prevent accidental writes during investigation
grafana --read-only dashboards list --query incident

# Talk with Grafana Assistant
grafana assistant chat --prompt "Investigate elevated error rate in checkout service"

# Continue a specific assistant conversation
grafana assistant chat --chat-id "chat_123" --prompt "Correlate with logs and traces for the last 30m"

# Poll assistant chat status
grafana assistant status --chat-id "chat_123"

# Create a dashboard from JSON template
grafana dashboards create --template-json '{"title":"Incident Overview","schemaVersion":39,"version":0,"panels":[]}'

# Delete a dashboard explicitly
grafana --yes dashboards delete --uid "incident-overview"

# Fetch dashboard metadata
grafana dashboards get --uid "incident-overview"

# Render a panel screenshot for agent inspection
grafana dashboards render --uid "incident-overview" --panel-id 4 --out /tmp/incident-overview-panel.png
```

## Product Coverage Plan (WIP)

Based on current Grafana product/docs research, this CLI targets:

- Grafana core API (dashboards, datasources, folders, alerting, RBAC)
- Grafana Cloud stacks/control-plane operations
- Runtime observability data (metrics/logs/traces) plus bounded aggregate summaries
- Investigation context (query history, SLO definitions, IRM incident previews, and OnCall schedules)
- Grafana Assistant chat + skills for incident workflows
- Next planned domains:
  - Synthetic Monitoring
  - k6 performance testing
  - Asserts

## CLI Design Principles

The design goal is straightforward:

- explicit commands for high-frequency Grafana workflows
- non-interactive execution for agents and CI
- compact structured output by default
- a raw API escape hatch for full product coverage

The point is not to copy other CLIs. The point is to give agents the shortest, most reliable path to understand and operate Grafana.

## Quality Gate

CI enforces **100% unit test coverage**.

```bash
go test ./... -covermode=atomic -coverprofile=coverage.out
```

## Release Process

Releases are automatic on every merge to `main` via GitHub Actions.

Versioning is Semantic Versioning with Conventional Commit signals:

- `feat:` -> minor bump
- `fix:`, `docs:`, `chore:`, etc -> patch bump
- `BREAKING CHANGE` in commit body or `!` in commit type -> major bump

Example:

- `feat(runtime): add incident root-cause summary` -> `v0.2.0`
- `fix(cli): handle empty datasource response` -> `v0.2.1`
- `feat(api)!: drop legacy auth flag` -> `v1.0.0`

## Roadmap

- broader Grafana Cloud product coverage (alerting, access control, reporting, synthetic monitoring, OnCall, k6)
- richer agent execution plans and remediation actions
- **Graph RAG for past incidents** to reuse historical context during incident triage and diagnosis

## Architecture

See [docs/architecture.md](docs/architecture.md).

Research references: [docs/product-research.md](docs/product-research.md).

Contribution guide: [CONTRIBUTING.md](CONTRIBUTING.md).
