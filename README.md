# grafana-cli

Agent-first CLI to control Grafana and Grafana Cloud.

This project is a **WIP hackathon build by @matiasvillaverde and @marctc**.

## Why This Exists

Most Grafana tooling is human-first. This CLI is designed for **agents** that need to:

- understand and triage incidents
- query logs/metrics/traces fast
- inspect cloud stacks and data sources
- create dashboards programmatically
- run deterministic workflows with low token usage

## Motivation: CLI First For Agents

This project is about **context engineering** for observability workflows.

For this use case, a focused CLI is often better than generic MCP interactions:

- CLI commands return narrow, task-specific JSON instead of broad tool schemas and verbose context.
- Agents can request only the fields they need (`--fields`) to reduce token usage.
- Command contracts are stable and deterministic, which improves reliability in long agent loops.
- Execution is composable in scripts/CI, so agents can chain investigation steps with minimal prompt overhead.

MCP is still useful as an integration layer, but this CLI is the optimized execution surface for high-frequency Grafana agent tasks.

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
- **Token minimization**: `--fields` projection to return only required keys
- **Deterministic command behavior**: stable flags, stable output shapes
- **Composability**: each command is script/agent safe

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

- Auth/session
  - `auth login|status|logout`
- Raw API access
  - `api <METHOD> <PATH> [--body JSON]`
- Cloud inventory
  - `cloud stacks list`
- Incident + runtime investigation
  - `incident analyze --goal ...`
  - `runtime metrics query --expr ...`
  - `runtime logs query --query ...`
  - `runtime traces search --query ...`
  - `aggregate snapshot --metric-expr ... --log-query ... --trace-query ...`
- Dashboard and datasource operations
  - `dashboards list --query ... --tag ... --limit ...`
  - `dashboards create --title ...` or `--template-json ...`
  - `datasources list --type ... --name ...`
- Grafana Assistant operations
  - `assistant chat --prompt ... [--chat-id ...]`
  - `assistant status --chat-id ...`
  - `assistant skills`
- Agent workflows
  - `agent plan --goal ...`
  - `agent run --goal ...`

## Quick Start

```bash
grafana auth login \
  --token "$GRAFANA_TOKEN" \
  --base-url "https://your-stack.grafana.net" \
  --cloud-url "https://grafana.com" \
  --prom-url "https://prometheus-prod-01-eu-west-0.grafana.net" \
  --logs-url "https://logs-prod-01-eu-west-0.grafana.net" \
  --traces-url "https://tempo-prod-01-eu-west-0.grafana.net"

# Incident analysis (compact JSON)
grafana incident analyze --goal "Investigate elevated error rate"

# Return only what the agent needs
grafana --fields summary.metrics_series,summary.log_streams incident analyze --goal "Latency spike"

# Talk with Grafana Assistant
grafana assistant chat --prompt "Investigate elevated error rate in checkout service"

# Continue a specific assistant conversation
grafana assistant chat --chat-id "chat_123" --prompt "Correlate with logs and traces for the last 30m"

# Poll assistant chat status
grafana assistant status --chat-id "chat_123"

# Create a dashboard from JSON template
grafana dashboards create --template-json '{"title":"Incident Overview","schemaVersion":39,"version":0,"panels":[]}'
```

## Product Coverage Plan (WIP)

Based on current Grafana product/docs research, this CLI targets:

- Grafana core API (dashboards, datasources, folders, alerting, RBAC)
- Grafana Cloud stacks/control-plane operations
- Runtime observability data (metrics/logs/traces)
- Grafana Assistant chat + skills for incident workflows
- Next planned domains:
  - IRM/incident response
  - SLO
  - Synthetic Monitoring
  - k6 performance testing
  - Asserts
  - OnCall

## Design Inspiration

This README and CLI structure were informed by strong open-source CLI patterns from:

- `cli/cli` (GitHub CLI)
- `cloudflare/workers-sdk` (`wrangler`)
- `supabase/cli`
- `vercel/vercel` (CLI package)
- `Aider-AI/aider`

We borrowed patterns around command discoverability, non-interactive execution, stable JSON outputs, and strong automation ergonomics.

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
