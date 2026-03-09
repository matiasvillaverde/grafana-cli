#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT_FILE="${1:-${TMPDIR:-/tmp}/grafana-cli-integration.env}"

GRAFANA_URL="${GRAFANA_URL:-http://127.0.0.1:3000}"
PROM_URL="${PROM_URL:-http://127.0.0.1:9090}"
PUSHGATEWAY_URL="${PUSHGATEWAY_URL:-http://127.0.0.1:9091}"
LOKI_URL="${LOKI_URL:-http://127.0.0.1:3100}"
TEMPO_URL="${TEMPO_URL:-http://127.0.0.1:3200}"
ZIPKIN_URL="${ZIPKIN_URL:-http://127.0.0.1:9411}"
CLICKHOUSE_URL="${CLICKHOUSE_URL:-http://127.0.0.1:8123}"
CLICKHOUSE_USER="${CLICKHOUSE_USER:-grafana}"
CLICKHOUSE_PASSWORD="${CLICKHOUSE_PASSWORD:-grafana-integration}"
RENDERER_URL="${RENDERER_URL:-http://127.0.0.1:8081}"
GRAFANA_ADMIN_USER="${GRAFANA_ADMIN_USER:-admin}"
GRAFANA_ADMIN_PASSWORD="${GRAFANA_ADMIN_PASSWORD:-admin}"
SERVICE_ACCOUNT_NAME="${SERVICE_ACCOUNT_NAME:-grafana-cli-integration}"
TOKEN_NAME="${TOKEN_NAME:-grafana-cli-integration-token}"
SYNTHETICS_TOKEN="${SYNTHETICS_TOKEN:-synthetics-integration-token}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

wait_for_http() {
  local url="$1"
  local attempts="${2:-60}"
  local delay="${3:-2}"

  for ((i = 1; i <= attempts; i++)); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep "$delay"
  done

  echo "timed out waiting for $url" >&2
  exit 1
}

wait_for_query_result() {
  local url="$1"
  local jq_filter="$2"
  local attempts="${3:-30}"
  local delay="${4:-2}"

  for ((i = 1; i <= attempts; i++)); do
    if curl -fsS "$url" | jq -e "$jq_filter" >/dev/null 2>&1; then
      return 0
    fi
    sleep "$delay"
  done

  echo "timed out waiting for query result from $url" >&2
  exit 1
}

grafana_api() {
  local method="$1"
  local path="$2"
  local body="${3:-}"

  if [[ -n "$body" ]]; then
    curl -fsS \
      -u "${GRAFANA_ADMIN_USER}:${GRAFANA_ADMIN_PASSWORD}" \
      -H "Content-Type: application/json" \
      -X "$method" \
      "${GRAFANA_URL}${path}" \
      -d "$body"
    return
  fi

  curl -fsS \
    -u "${GRAFANA_ADMIN_USER}:${GRAFANA_ADMIN_PASSWORD}" \
    -X "$method" \
    "${GRAFANA_URL}${path}"
}

clickhouse_query() {
  local sql="$1"
  local response
  if ! response="$(curl -fsS --user "${CLICKHOUSE_USER}:${CLICKHOUSE_PASSWORD}" "${CLICKHOUSE_URL}/" --data-binary "$sql")"; then
    echo "clickhouse query failed" >&2
    echo "$sql" >&2
    return 1
  fi
  if [[ -n "$response" ]]; then
    printf '%s\n' "$response"
  fi
}

clickhouse_time() {
  local offset_seconds="${1:-0}"
  perl -MPOSIX=strftime -e 'my $offset = shift @ARGV; print strftime("%Y-%m-%d %H:%M:%S", gmtime(time + $offset))' -- "$offset_seconds"
}

require_cmd curl
require_cmd jq
require_cmd perl

unix_time_ns() {
  perl -MTime::HiRes=time -e 'printf "%.0f\n", time() * 1000000000'
}

unix_time_us() {
  perl -MTime::HiRes=time -e 'printf "%.0f\n", time() * 1000000'
}

wait_for_http "${GRAFANA_URL}/api/health"
wait_for_http "${PROM_URL}/-/ready"
wait_for_http "${LOKI_URL}/ready"
wait_for_http "${TEMPO_URL}/ready"
wait_for_http "${CLICKHOUSE_URL}/ping"
wait_for_http "${RENDERER_URL}/render/version"

service_account_id="$(grafana_api GET "/api/serviceaccounts/search?query=${SERVICE_ACCOUNT_NAME}" \
  | jq -r --arg name "$SERVICE_ACCOUNT_NAME" '.serviceAccounts[]? | select(.name == $name) | .id' \
  | head -n1)"

if [[ -z "$service_account_id" ]]; then
  service_account_id="$(grafana_api POST "/api/serviceaccounts" "$(jq -nc --arg name "$SERVICE_ACCOUNT_NAME" '{name: $name, role: "Admin"}')" \
    | jq -r '.id')"
fi

existing_token_ids="$(grafana_api GET "/api/serviceaccounts/${service_account_id}/tokens" \
  | jq -r --arg name "$TOKEN_NAME" '.[]? | select(.name == $name) | .id')"

while IFS= read -r token_id; do
  if [[ -n "$token_id" ]]; then
    grafana_api DELETE "/api/serviceaccounts/${service_account_id}/tokens/${token_id}" >/dev/null
  fi
done <<< "$existing_token_ids"

token_value="$(grafana_api POST "/api/serviceaccounts/${service_account_id}/tokens" "$(jq -nc --arg name "$TOKEN_NAME" '{name: $name}')" \
  | jq -r '.key')"

cat <<'EOF' | curl -fsS --data-binary @- "${PUSHGATEWAY_URL}/metrics/job/grafana-cli-integration/instance/local"
# TYPE grafana_cli_test_requests_total counter
grafana_cli_test_requests_total{service="checkout",status="500"} 7
# TYPE grafana_cli_test_latency_seconds gauge
grafana_cli_test_latency_seconds{service="checkout"} 0.42
EOF

wait_for_query_result \
  "${PROM_URL}/api/v1/query?query=grafana_cli_test_requests_total" \
  '.data.result | length > 0'

log_time_one="$(unix_time_ns)"
log_time_two="$((log_time_one + 1000000))"
jq -nc \
  --arg t1 "$log_time_one" \
  --arg t2 "$log_time_two" \
  '{
    streams: [
      {
        stream: {
          app: "checkout",
          level: "error",
          source: "grafana-cli-integration"
        },
        values: [
          [$t1, "checkout request failed"],
          [$t2, "payment dependency timeout"]
        ]
      }
    ]
  }' \
  | curl -fsS -H "Content-Type: application/json" -X POST "${LOKI_URL}/loki/api/v1/push" --data-binary @-

wait_for_query_result \
  "${LOKI_URL}/loki/api/v1/query_range?query=%7Bapp%3D%22checkout%22%7D&start=$((log_time_one - 1000000))&end=$((log_time_two + 1000000))&limit=10" \
  '.data.result | length > 0'

clickhouse_query "
  CREATE TABLE IF NOT EXISTS default.grafana_cli_integration_clickhouse (
    ts DateTime,
    service String,
    level String,
    requests UInt64
  )
  ENGINE = MergeTree
  ORDER BY ts
"

clickhouse_query "TRUNCATE TABLE default.grafana_cli_integration_clickhouse"

clickhouse_time_one="$(clickhouse_time -300)"
clickhouse_time_two="$(clickhouse_time)"
clickhouse_query "
  INSERT INTO default.grafana_cli_integration_clickhouse (ts, service, level, requests) VALUES
    ('${clickhouse_time_one}', 'checkout-clickhouse', 'error', 7),
    ('${clickhouse_time_two}', 'payments-clickhouse', 'warn', 3)
"

wait_for_query_result \
  "${CLICKHOUSE_URL}/?user=${CLICKHOUSE_USER}&password=${CLICKHOUSE_PASSWORD}&query=SELECT%20count()%20AS%20row_count%20FROM%20default.grafana_cli_integration_clickhouse%20FORMAT%20JSON" \
  '.data[0].row_count == "2"'

trace_timestamp_us="$(unix_time_us)"
trace_id="463ac35c9f6413ad48485a3953bb6124"
span_id="a2fb4a1d1a96d312"
jq -nc \
  --arg trace_id "$trace_id" \
  --arg span_id "$span_id" \
  --argjson timestamp "$trace_timestamp_us" \
  '[
    {
      traceId: $trace_id,
      id: $span_id,
      kind: "SERVER",
      name: "GET /checkout",
      timestamp: $timestamp,
      duration: 250000,
      localEndpoint: {
        serviceName: "checkout"
      },
      tags: {
        "http.method": "GET",
        "http.status_code": "500"
      }
    }
  ]' \
  | curl -fsS -H "Content-Type: application/json" -X POST "${ZIPKIN_URL}/api/v2/spans" --data-binary @-

cat >"$OUT_FILE" <<EOF
GRAFANA_CLI_INTEGRATION_GRAFANA_URL=${GRAFANA_URL}
GRAFANA_CLI_INTEGRATION_PROM_URL=${PROM_URL}
GRAFANA_CLI_INTEGRATION_LOGS_URL=${LOKI_URL}
GRAFANA_CLI_INTEGRATION_TRACES_URL=${TEMPO_URL}
GRAFANA_CLI_INTEGRATION_TOKEN=${token_value}
GRAFANA_CLI_INTEGRATION_SERVICE_ACCOUNT_ID=${service_account_id}
GRAFANA_CLI_INTEGRATION_SYNTHETICS_TOKEN=${SYNTHETICS_TOKEN}
EOF

echo "wrote integration environment to ${OUT_FILE}"
