package cli

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/matiasvillaverde/grafana-cli/internal/agent"
	"github.com/matiasvillaverde/grafana-cli/internal/grafana"
)

func datasourceFamilyForType(dataType string) (datasourceQueryFamily, bool) {
	for _, strategy := range datasourceQueryStrategies() {
		if strategy.SupportsType(dataType) {
			return strategy.Family(), true
		}
	}
	return datasourceQueryFamily{}, false
}

func normalizeDatasourceCollection(payload any) []any {
	items, ok := payload.([]any)
	if !ok {
		return nil
	}
	out := make([]any, 0, len(items))
	for _, item := range items {
		record, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, normalizeDatasourceRecord(record))
	}
	return out
}

func normalizeDatasourceRecord(record map[string]any) map[string]any {
	raw := cloneAnyMap(record)
	resolved, ok := datasourceFromPayload(record)
	if !ok {
		cloned := cloneAnyMap(raw)
		cloned["raw"] = raw
		cloned["typed_family"] = ""
		cloned["documentation_url"] = ""
		cloned["typed_flags"] = []string{}
		cloned["capabilities"] = map[string]any{
			"inspect":     false,
			"health":      false,
			"resources":   false,
			"raw_query":   false,
			"typed_query": false,
		}
		cloned["notes"] = []string{"Datasource payload is missing uid or type metadata; inspect the raw object to resolve the datasource manually."}
		return cloned
	}

	family, hasFamily := datasourceFamilyForType(resolved.Type)
	typedFamily := ""
	documentationURL := ""
	notes := []string{"Use datasources query --query-json or --queries-json when typed flags do not cover the query mode you need."}
	typedFlags := []string{}
	help := map[string]any{
		"inspect":            "grafana datasources get --uid " + resolved.UID,
		"health":             "grafana datasources health --uid " + resolved.UID,
		"resources":          "grafana datasources resources get --uid " + resolved.UID + " --path ...",
		"generic_query_help": "grafana datasources query --help",
	}
	if hasFamily {
		typedFamily = family.Name
		documentationURL = family.DocumentationURL
		notes = append(family.Notes, notes...)
		if strategy, ok := findDatasourceStrategy(family.Name); ok {
			typedFlags = typedFlagNames(strategy)
		}
		help["typed_query_help"] = "grafana datasources " + family.Name + " query --help"
	} else {
		notes = append([]string{"No typed family adapter is registered for this datasource type; use the generic query command and plugin JSON."}, notes...)
	}

	return map[string]any{
		"uid":               resolved.UID,
		"name":              resolved.Name,
		"type":              resolved.Type,
		"url":               firstNonEmptyString(record, "url"),
		"access":            firstNonEmptyString(record, "access"),
		"is_default":        record["isDefault"] == true,
		"typed_family":      typedFamily,
		"documentation_url": documentationURL,
		"typed_flags":       typedFlags,
		"capabilities": map[string]any{
			"inspect":     true,
			"health":      true,
			"resources":   true,
			"raw_query":   true,
			"typed_query": hasFamily,
		},
		"notes": notes,
		"help":  help,
		"raw":   raw,
	}
}

func typedFlagNames(strategy datasourceQueryStrategy) []string {
	flags := strategy.DiscoveryFlags()
	names := make([]string, 0, len(flags))
	for _, item := range flags {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		names = append(names, item.Name)
	}
	return names
}

func datasourceInventorySummary(payload any) map[string]any {
	items := normalizeDatasourceCollection(payload)
	types := make([]string, 0, len(items))
	families := make([]string, 0, len(items))
	countsByType := map[string]int{}
	countsByFamily := map[string]int{}
	for _, item := range items {
		record := item.(map[string]any)
		dataType, _ := record["type"].(string)
		if strings.TrimSpace(dataType) != "" {
			countsByType[dataType]++
			if !slices.Contains(types, dataType) {
				types = append(types, dataType)
			}
		}
		family, _ := record["typed_family"].(string)
		if strings.TrimSpace(family) != "" {
			countsByFamily[family]++
			if !slices.Contains(families, family) {
				families = append(families, family)
			}
		}
	}
	sort.Strings(types)
	sort.Strings(families)
	return map[string]any{
		"count":            len(items),
		"types":            types,
		"typed_families":   families,
		"counts_by_type":   countsByType,
		"counts_by_family": countsByFamily,
	}
}

func datasourceQueryHints(req grafana.AggregateRequest, playbook agent.Playbook, payload any) []map[string]any {
	items := normalizeDatasourceCollection(payload)
	hints := make([]map[string]any, 0, 3)
	for _, familyName := range []string{"prometheus", "loki", "tempo"} {
		for _, item := range items {
			record := item.(map[string]any)
			if record["typed_family"] != familyName {
				continue
			}
			uid, _ := record["uid"].(string)
			switch familyName {
			case "prometheus":
				hints = append(hints, map[string]any{
					"family":  familyName,
					"purpose": fmt.Sprintf("Inspect %s metrics with the playbook metric expression", playbook),
					"command": fmt.Sprintf("grafana datasources prometheus query --uid %s --expr %q", uid, req.MetricExpr),
				})
			case "loki":
				hints = append(hints, map[string]any{
					"family":  familyName,
					"purpose": fmt.Sprintf("Inspect %s logs with the playbook log query", playbook),
					"command": fmt.Sprintf("grafana datasources loki query --uid %s --expr %q --query-type range", uid, req.LogQuery),
				})
			case "tempo":
				hints = append(hints, map[string]any{
					"family":  familyName,
					"purpose": fmt.Sprintf("Inspect %s traces with the playbook trace query", playbook),
					"command": fmt.Sprintf("grafana datasources tempo query --uid %s --query %q --limit %d", uid, req.TraceQuery, req.Limit),
				})
			}
			break
		}
	}
	return hints
}
