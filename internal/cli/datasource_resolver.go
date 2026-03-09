package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

type datasourceSelector struct {
	UID            string
	Name           string
	DatasourceType string
}

type resolvedDatasource struct {
	UID  string
	Name string
	Type string
	Raw  map[string]any
}

type datasourceResolver interface {
	Resolve(context.Context, APIClient, datasourceSelector, datasourceQueryStrategy) (resolvedDatasource, error)
}

type listDatasourceResolver struct{}

func (r listDatasourceResolver) Resolve(ctx context.Context, client APIClient, selector datasourceSelector, strategy datasourceQueryStrategy) (resolvedDatasource, error) {
	if err := validateDatasourceSelector(selector); err != nil {
		return resolvedDatasource{}, err
	}
	if strings.TrimSpace(selector.UID) != "" {
		payload, err := client.GetDatasource(ctx, strings.TrimSpace(selector.UID))
		if err != nil {
			return resolvedDatasource{}, err
		}
		resolved, ok := datasourceFromPayload(payload)
		if !ok {
			return resolvedDatasource{}, errors.New("datasource payload missing uid or type metadata")
		}
		if selector.DatasourceType != "" && !datasourceTypeMatches(resolved.Type, []string{selector.DatasourceType}) {
			return resolvedDatasource{}, fmt.Errorf("datasource %s resolved to type %q, expected %q", resolved.UID, resolved.Type, selector.DatasourceType)
		}
		if strategy != nil && !strategy.SupportsType(resolved.Type) {
			return resolvedDatasource{}, fmt.Errorf("datasource %s has type %q, which is not supported by the %s strategy", resolved.UID, resolved.Type, strategy.Family().Name)
		}
		return resolved, nil
	}

	payload, err := client.ListDatasources(ctx)
	if err != nil {
		return resolvedDatasource{}, err
	}
	candidates := datasourceCandidates(payload)
	matches := filterResolvedDatasources(candidates, selector, strategy)
	switch len(matches) {
	case 0:
		name := strings.TrimSpace(selector.Name)
		if selector.DatasourceType != "" {
			return resolvedDatasource{}, fmt.Errorf("no datasource matched --name %q and --datasource-type %q", name, selector.DatasourceType)
		}
		if strategy != nil {
			return resolvedDatasource{}, fmt.Errorf("no datasource matched --name %q for family %s", name, strategy.Family().Name)
		}
		return resolvedDatasource{}, fmt.Errorf("no datasource matched --name %q", name)
	case 1:
		return matches[0], nil
	default:
		return resolvedDatasource{}, fmt.Errorf("ambiguous datasource selection for --name %q: %s", selector.Name, formatDatasourceCandidates(matches))
	}
}

func validateDatasourceSelector(selector datasourceSelector) error {
	hasUID := strings.TrimSpace(selector.UID) != ""
	hasName := strings.TrimSpace(selector.Name) != ""
	switch {
	case hasUID && hasName:
		return errors.New("use either --uid or --name, not both")
	case !hasUID && !hasName:
		return errors.New("--uid or --name is required")
	default:
		return nil
	}
}

func datasourceFromPayload(payload any) (resolvedDatasource, bool) {
	record, ok := payload.(map[string]any)
	if !ok {
		return resolvedDatasource{}, false
	}
	uid := firstNonEmptyString(record, "uid", "datasourceUid")
	dataType := firstNonEmptyString(record, "type", "typeName", "pluginType")
	if strings.TrimSpace(uid) == "" || strings.TrimSpace(dataType) == "" {
		return resolvedDatasource{}, false
	}
	return resolvedDatasource{
		UID:  uid,
		Name: firstNonEmptyString(record, "name"),
		Type: dataType,
		Raw:  record,
	}, true
}

func datasourceCandidates(payload any) []resolvedDatasource {
	items, _, ok := collectionPayload(payload)
	if !ok {
		return nil
	}
	out := make([]resolvedDatasource, 0, len(items))
	for _, item := range items {
		resolved, ok := datasourceFromPayload(item)
		if !ok {
			continue
		}
		out = append(out, resolved)
	}
	return out
}

func filterResolvedDatasources(candidates []resolvedDatasource, selector datasourceSelector, strategy datasourceQueryStrategy) []resolvedDatasource {
	name := strings.ToLower(strings.TrimSpace(selector.Name))
	dataType := strings.TrimSpace(selector.DatasourceType)
	filtered := make([]resolvedDatasource, 0, len(candidates))
	for _, candidate := range candidates {
		if name != "" && strings.ToLower(strings.TrimSpace(candidate.Name)) != name {
			continue
		}
		if dataType != "" && !datasourceTypeMatches(candidate.Type, []string{dataType}) {
			continue
		}
		if strategy != nil && !strategy.SupportsType(candidate.Type) {
			continue
		}
		filtered = append(filtered, candidate)
	}
	return filtered
}

func formatDatasourceCandidates(candidates []resolvedDatasource) string {
	parts := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		parts = append(parts, fmt.Sprintf("%s(%s,%s)", candidate.UID, candidate.Name, candidate.Type))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}
