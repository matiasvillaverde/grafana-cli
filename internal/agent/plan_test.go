package agent

import (
	"testing"
	"time"
)

func TestBuildPlanSelectsPlaybook(t *testing.T) {
	now := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		goal     string
		playbook Playbook
	}{
		{goal: "Investigate latency spike", playbook: PlaybookLatency},
		{goal: "Reduce cardinality cost", playbook: PlaybookCost},
		{goal: "Check service health", playbook: PlaybookHealth},
		{goal: "Investigate elevated errors", playbook: PlaybookIncident},
	}

	for _, tc := range cases {
		plan := BuildPlan(tc.goal, now)
		if plan.Playbook != tc.playbook {
			t.Fatalf("expected %s, got %s", tc.playbook, plan.Playbook)
		}
		if plan.Goal != tc.goal {
			t.Fatalf("goal mismatch")
		}
		if plan.GeneratedAt != now {
			t.Fatalf("generated time mismatch")
		}
		if len(plan.Actions) != 2 {
			t.Fatalf("expected 2 actions, got %d", len(plan.Actions))
		}
	}
}

func TestAggregateRequestDefaultsPerPlaybook(t *testing.T) {
	now := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)

	plans := []Plan{
		{Playbook: PlaybookIncident},
		{Playbook: PlaybookLatency},
		{Playbook: PlaybookCost},
		{Playbook: PlaybookHealth},
	}

	for _, plan := range plans {
		req := plan.AggregateRequest(now)
		if req.Start == "" || req.End == "" || req.Step == "" {
			t.Fatalf("expected populated time window")
		}
		if req.Limit != 200 {
			t.Fatalf("expected default limit 200, got %d", req.Limit)
		}
		if req.MetricExpr == "" || req.LogQuery == "" || req.TraceQuery == "" {
			t.Fatalf("expected non-empty query defaults")
		}
	}
}
