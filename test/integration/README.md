# Integration Tests

This suite runs the CLI against a local Grafana stack started with Docker Compose plus deterministic stub servers for APIs that are not available in the local stack.

Most command coverage already runs against a real Grafana instance: auth, dashboards, datasources, folders, annotations, alerting, and runtime observability hit the local stack directly. Stubs are reserved for Grafana Cloud, OnCall, Synthetic Monitoring, Assistant, SLO, and IRM surfaces that are not shipped by the Compose stack.

## Stack

- Grafana `11.6.13`
- Prometheus `v3.10.0`
- Pushgateway `v1.11.2`
- Loki pinned by digest
- Tempo `v2.10.1` pinned by digest and started in single-binary mode
- ClickHouse `24.8`
- Grafana image renderer `v5.6.3`
- Grafana ClickHouse datasource plugin `v4.14.0`

The bootstrap script creates a service-account token and seeds telemetry into Prometheus, Loki, Tempo, and ClickHouse.

## Run locally

Start the stack:

```bash
docker compose -f test/integration/docker-compose.yml up -d
```

Seed Grafana and write the integration environment file:

```bash
env_file="${TMPDIR:-/tmp}/grafana-cli-integration.env"
./test/integration/bootstrap.sh "$env_file"
```

Run all integration shards:

```bash
set -a
source "$env_file"
set +a
export GRAFANA_CLI_DISABLE_KEYRING=1
go test -tags=integration ./cmd/grafana -count=1
```

Run one shard:

```bash
set -a
source "$env_file"
set +a
export GRAFANA_CLI_DISABLE_KEYRING=1
go test -tags=integration ./cmd/grafana -run '^TestRuntimeObservability$' -count=1
```

Stop and remove the stack:

```bash
docker compose -f test/integration/docker-compose.yml down -v
```

## Shards

The workflow follows the command-coverage groups used by the CLI tests:

- `TestSchemaGlobalFlags`
- `TestAuthConfig`
- `TestDashboardsDatasources`
- `TestFoldersAnnotationsAlerting`
- `TestRuntimeObservability`
- `TestInvestigationIncidents`
- `TestAssistantAccessCloud`
- `TestAgentWorkflows`

## Notes

Trace ingestion is real, but the Go integration harness serves `/api/search` through a local proxy. The minimal Tempo fixture accepts the spans, but its recent-search API is not stable enough in this setup to rely on directly for deterministic CI.

Cloud, OnCall, and Synthetic Monitoring assertions run against dedicated local stub servers so the commands still exercise their real base-URL separation without depending on external Grafana Cloud services.

## Real Cloud contract tests

There is also an opt-in Grafana Cloud contract suite for read-only Cloud commands. It is intentionally separate from the local Docker-based suite and only runs when you provide live Cloud credentials.

Required environment:

- `GRAFANA_CLI_INTEGRATION_CLOUD_URL`
- `GRAFANA_CLI_INTEGRATION_CLOUD_TOKEN`
- `GRAFANA_CLI_INTEGRATION_CLOUD_STACK`

Optional environment for broader coverage:

- `GRAFANA_CLI_INTEGRATION_CLOUD_ACCESS_REGION`
- `GRAFANA_CLI_INTEGRATION_CLOUD_ORG_SLUG`
- `GRAFANA_CLI_INTEGRATION_CLOUD_BILLED_USAGE_YEAR`
- `GRAFANA_CLI_INTEGRATION_CLOUD_BILLED_USAGE_MONTH`

Run the live Cloud contract suite:

```bash
export GRAFANA_CLI_DISABLE_KEYRING=1
go test -tags=integrationcloud ./cmd/grafana -run '^TestRealCloudCommands$' -count=1
```

Assistant, SLO, and IRM assertions use deterministic plugin-response stubs on top of the local Grafana proxy because the Docker stack does not ship those Grafana apps.

The integration harness forces `GRAFANA_CLI_DISABLE_KEYRING=1` so `auth login` uses file-backed token storage instead of depending on a desktop keyring service in CI or headless local environments.

The shard-to-command mapping lives in `test/integration/command-coverage.json`. A unit test fails if the discovery schema adds a new leaf command that is not assigned to an integration shard.

If you do want a repository-local env file for debugging, pass an explicit output path such as `./test/integration/bootstrap.sh test/integration/integration.env`. That file is gitignored. The bootstrap script reuses the service account and replaces any existing token with the same `TOKEN_NAME` instead of accumulating stale tokens.
