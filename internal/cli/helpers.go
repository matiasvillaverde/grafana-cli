package cli

import (
	"fmt"
	"strings"
)

func mapValue(payload any, path ...string) map[string]any {
	current, ok := payload.(map[string]any)
	if !ok {
		return nil
	}
	for _, key := range path {
		next, ok := current[key].(map[string]any)
		if !ok {
			return nil
		}
		current = next
	}
	return current
}

func investigationPrompt(goal string) string {
	return strings.TrimSpace(fmt.Sprintf(`
Investigate the following operational issue in Grafana.

Goal: %s

	Respond with:
	1. the most likely impacted services, dashboards, or signals
	2. the next concrete Grafana queries or views to inspect, including datasource inspection when relevant
	3. the top hypotheses and evidence gaps

	Prefer the documented datasource query editors and suggest generic datasource JSON only when a typed query path does not fit.
	Keep the answer concise, evidence-first, and operational.
	`, strings.TrimSpace(goal)))
}
