package agent

import (
	"strings"
	"time"

	"github.com/matiasvillaverde/grafana-cli/internal/grafana"
)

// Playbook is a deterministic agent workflow recipe.
type Playbook string

const (
	PlaybookIncident Playbook = "incident"
	PlaybookLatency  Playbook = "latency"
	PlaybookCost     Playbook = "cost"
	PlaybookHealth   Playbook = "health"
)

// Action is an execution step for an automation agent.
type Action struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Command string `json:"command"`
	Purpose string `json:"purpose"`
}

// Plan is the agent-first, machine-executable workflow.
type Plan struct {
	Goal        string    `json:"goal"`
	Playbook    Playbook  `json:"playbook"`
	Actions     []Action  `json:"actions"`
	GeneratedAt time.Time `json:"generated_at"`
}

func BuildPlan(goal string, now time.Time) Plan {
	playbook := selectPlaybook(goal)
	return Plan{
		Goal:        goal,
		Playbook:    playbook,
		Actions:     actionsFor(playbook),
		GeneratedAt: now.UTC(),
	}
}

func (p Plan) AggregateRequest(now time.Time) grafana.AggregateRequest {
	start := now.Add(-15 * time.Minute).UTC().Format(time.RFC3339)
	end := now.UTC().Format(time.RFC3339)
	return grafana.AggregateRequest{
		MetricExpr: defaultMetricExpr(p.Playbook),
		LogQuery:   defaultLogQuery(p.Playbook),
		TraceQuery: defaultTraceQuery(p.Playbook),
		Start:      start,
		End:        end,
		Step:       "30s",
		Limit:      200,
	}
}

func selectPlaybook(goal string) Playbook {
	g := strings.ToLower(goal)
	switch {
	case strings.Contains(g, "latency") || strings.Contains(g, "slow"):
		return PlaybookLatency
	case strings.Contains(g, "cost") || strings.Contains(g, "cardinality"):
		return PlaybookCost
	case strings.Contains(g, "health") || strings.Contains(g, "availability"):
		return PlaybookHealth
	default:
		return PlaybookIncident
	}
}

func actionsFor(playbook Playbook) []Action {
	return []Action{
		{
			ID:      "datasource-inventory",
			Type:    "datasource_inventory",
			Command: "grafana datasources list",
			Purpose: "discover datasource instances, typed query adapters, and query help before investigating",
		},
		{
			ID:      "cloud-stacks",
			Type:    "cloud_inventory",
			Command: "grafana cloud stacks list",
			Purpose: "discover cloud stack and service endpoints",
		},
		{
			ID:      "runtime-snapshot",
			Type:    "runtime_aggregate",
			Command: "grafana aggregate snapshot",
			Purpose: "collect cross-signal runtime data using " + string(playbook) + " playbook",
		},
	}
}

func defaultMetricExpr(playbook Playbook) string {
	switch playbook {
	case PlaybookLatency:
		return `histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))`
	case PlaybookCost:
		return `topk(20, sum by (__name__)({__name__=~".+"}))`
	case PlaybookHealth:
		return `sum(up)`
	default:
		return `sum(rate(http_requests_total{status=~"5.."}[5m]))`
	}
}

func defaultLogQuery(playbook Playbook) string {
	switch playbook {
	case PlaybookLatency:
		return `{job=~".+"} |= "timeout"`
	case PlaybookCost:
		return `{job=~".+"} |= "cardinality"`
	case PlaybookHealth:
		return `{job=~".+"} |= "unhealthy"`
	default:
		return `{job=~".+"} |= "error"`
	}
}

func defaultTraceQuery(playbook Playbook) string {
	switch playbook {
	case PlaybookLatency:
		return `{ duration > 500ms }`
	case PlaybookCost:
		return `{ span.name =~ ".*query.*" }`
	case PlaybookHealth:
		return `{ status = error } | count() > 0`
	default:
		return `{ status = error }`
	}
}
