package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/matiasvillaverde/grafana-cli/internal/grafana"
)

const (
	defaultDatasourceQueryFrom          = "now-1h"
	defaultDatasourceQueryTo            = "now"
	defaultDatasourceQueryIntervalMS    = 1000
	defaultDatasourceQueryMaxDataPoints = 43200
)

type datasourceQueryExecutor struct {
	app      *App
	resolver datasourceResolver
}

func (a *App) runDatasources(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"datasources"}, true)
	}

	switch args[0] {
	case "list":
		return a.runDatasourcesList(ctx, opts, args[1:])
	case "get":
		return a.runDatasourceGet(ctx, opts, "datasources get", args[1:])
	case "health":
		return a.runDatasourceHealth(ctx, opts, "datasources health", args[1:])
	case "resources":
		return a.runDatasourceResources(ctx, opts, args[1:])
	case "query":
		return a.datasourceExecutor().runQuery(ctx, opts, "datasources query", genericDatasourceStrategy(), args[1:])
	default:
		strategy, ok := findDatasourceStrategy(args[0])
		if !ok {
			return errors.New("usage: datasources <list|get|health|resources|query|cloudwatch|clickhouse|mysql|postgres|mssql|influxdb|elasticsearch|opensearch|graphite|prometheus|loki|tempo|azure-monitor>")
		}
		if len(args) < 2 || args[1] != "query" {
			return fmt.Errorf("usage: datasources %s query (--uid UID | --name NAME) [--datasource-type TYPE] [family flags] [generic query flags]", strategy.Family().Name)
		}
		return a.datasourceExecutor().runQuery(ctx, opts, "datasources "+strategy.Family().Name+" query", strategy, args[2:])
	}
}

func (a *App) datasourceExecutor() datasourceQueryExecutor {
	return datasourceQueryExecutor{
		app:      a,
		resolver: listDatasourceResolver{},
	}
}

func (a *App) runDatasourcesList(ctx context.Context, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("datasources list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	typeFilter := fs.String("type", "", "datasource type filter")
	nameFilter := fs.String("name", "", "name substring filter")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := a.requireAuthConfig()
	if err != nil {
		return err
	}
	result, err := a.NewClient(cfg).ListDatasources(ctx)
	if err != nil {
		return err
	}
	result = filterDatasources(result, *typeFilter, *nameFilter)
	normalized := normalizeDatasourceCollection(result)
	return a.emitWithMetadata(opts, normalized, collectionMetadata("datasources list", normalized, 0, ""))
}

func (a *App) runDatasourceGet(ctx context.Context, opts globalOptions, command string, args []string) error {
	selector, err := parseDatasourceSelection(command, args)
	if err != nil {
		return err
	}

	cfg, err := a.requireAuthConfig()
	if err != nil {
		return err
	}
	client := a.NewClient(cfg)
	resolved, err := a.datasourceExecutor().resolver.Resolve(ctx, client, selector, nil)
	if err != nil {
		return err
	}
	result, err := client.GetDatasource(ctx, resolved.UID)
	if err != nil {
		return err
	}
	record, _ := result.(map[string]any)
	return a.emitWithMetadata(opts, normalizeDatasourceRecord(record), withCommandMetadata(nil, command))
}

func (a *App) runDatasourceHealth(ctx context.Context, opts globalOptions, command string, args []string) error {
	selector, err := parseDatasourceSelection(command, args)
	if err != nil {
		return err
	}

	cfg, err := a.requireAuthConfig()
	if err != nil {
		return err
	}
	client := a.NewClient(cfg)
	resolved, err := a.datasourceExecutor().resolver.Resolve(ctx, client, selector, nil)
	if err != nil {
		return err
	}
	result, err := client.DatasourceHealth(ctx, resolved.UID)
	if err != nil {
		return err
	}
	return a.emitWithMetadata(opts, result, withCommandMetadata(nil, command))
}

func (a *App) runDatasourceResources(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"datasources", "resources"}, true)
	}
	if args[0] != "get" && args[0] != "post" {
		return errors.New("usage: datasources resources <get|post> (--uid UID | --name NAME) --path RESOURCE_PATH [--datasource-type TYPE] [--body JSON]")
	}

	command := "datasources resources " + args[0]
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var selector datasourceSelector
	fs.StringVar(&selector.UID, "uid", "", "datasource UID")
	fs.StringVar(&selector.Name, "name", "", "datasource name")
	fs.StringVar(&selector.DatasourceType, "datasource-type", "", "optional datasource plugin type")
	resourcePath := fs.String("path", "", "resource path below /resources/")
	body := fs.String("body", "", "JSON request body")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if err := validateDatasourceSelector(selector); err != nil {
		return err
	}
	if strings.TrimSpace(*resourcePath) == "" {
		return errors.New("--path is required")
	}

	var parsedBody any
	if strings.TrimSpace(*body) != "" {
		if err := json.Unmarshal([]byte(*body), &parsedBody); err != nil {
			return fmt.Errorf("invalid --body JSON: %w", err)
		}
	}

	cfg, err := a.requireAuthConfig()
	if err != nil {
		return err
	}
	client := a.NewClient(cfg)
	resolved, err := a.datasourceExecutor().resolver.Resolve(ctx, client, selector, nil)
	if err != nil {
		return err
	}
	method := map[string]string{"get": "GET", "post": "POST"}[args[0]]
	result, err := client.DatasourceResource(ctx, method, resolved.UID, strings.TrimSpace(*resourcePath), parsedBody)
	if err != nil {
		return err
	}
	return a.emitWithMetadata(opts, result, withCommandMetadata(nil, command))
}

func (e datasourceQueryExecutor) runQuery(ctx context.Context, opts globalOptions, command string, strategy datasourceQueryStrategy, args []string) error {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	queryOpts := datasourceQueryOptions{
		From:          defaultDatasourceQueryFrom,
		To:            defaultDatasourceQueryTo,
		IntervalMS:    defaultDatasourceQueryIntervalMS,
		MaxDataPoints: defaultDatasourceQueryMaxDataPoints,
	}
	bindDatasourceQueryFlags(fs, &queryOpts)
	strategy.BindFlags(fs, &queryOpts)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := validateDatasourceSelector(queryOpts.Selector); err != nil {
		return err
	}

	cfg, err := e.app.requireAuthConfig()
	if err != nil {
		return err
	}
	client := e.app.NewClient(cfg)
	resolved, err := e.resolver.Resolve(ctx, client, queryOpts.Selector, strategy)
	if err != nil {
		return err
	}
	queries, err := strategy.BuildQueries(queryOpts, resolved)
	if err != nil {
		return err
	}
	result, err := client.DatasourceQuery(ctx, grafana.DatasourceQueryRequest{
		From:    strings.TrimSpace(queryOpts.From),
		To:      strings.TrimSpace(queryOpts.To),
		Queries: queries,
	})
	if err != nil {
		return err
	}
	return e.app.emitWithMetadata(opts, result, withCommandMetadata(nil, command))
}

func bindDatasourceQueryFlags(fs *flag.FlagSet, opts *datasourceQueryOptions) {
	fs.StringVar(&opts.Selector.UID, "uid", "", "datasource UID")
	fs.StringVar(&opts.Selector.Name, "name", "", "datasource name")
	fs.StringVar(&opts.Selector.DatasourceType, "datasource-type", "", "optional datasource plugin type")
	fs.StringVar(&opts.From, "from", defaultDatasourceQueryFrom, "Grafana query range start")
	fs.StringVar(&opts.To, "to", defaultDatasourceQueryTo, "Grafana query range end")
	fs.StringVar(&opts.RefID, "ref-id", "", "override refId for a single query")
	fs.IntVar(&opts.IntervalMS, "interval-ms", defaultDatasourceQueryIntervalMS, "intervalMs default applied to each query")
	fs.IntVar(&opts.MaxDataPoints, "max-data-points", defaultDatasourceQueryMaxDataPoints, "maxDataPoints default applied to each query")
	fs.StringVar(&opts.QueryJSON, "query-json", "", "single datasource query object")
	fs.StringVar(&opts.QueriesJSON, "queries-json", "", "full datasource query array")
}

func parseDatasourceSelection(command string, args []string) (datasourceSelector, error) {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var selector datasourceSelector
	fs.StringVar(&selector.UID, "uid", "", "datasource UID")
	fs.StringVar(&selector.Name, "name", "", "datasource name")
	fs.StringVar(&selector.DatasourceType, "datasource-type", "", "optional datasource plugin type")
	if err := fs.Parse(args); err != nil {
		return datasourceSelector{}, err
	}
	if err := validateDatasourceSelector(selector); err != nil {
		return datasourceSelector{}, err
	}
	return selector, nil
}

func genericDatasourceStrategy() datasourceQueryStrategy {
	return passthroughDatasourceStrategy{
		family: datasourceQueryFamily{
			Name:             "generic",
			Description:      "Run a generic datasource query via Grafana",
			Syntax:           `Use --query-json for one plugin query object or --queries-json for a full Grafana queries array`,
			ExampleQueryJSON: `{"rawSql":"SELECT 1","format":"table"}`,
			SupportedTypes:   nil,
		},
	}
}

func buildDatasourceQueries(uid, datasourceType, refID string, intervalMS, maxDataPoints int, queryJSON, queriesJSON string) ([]map[string]any, error) {
	switch {
	case queryJSON == "" && queriesJSON == "":
		return nil, errors.New("--query-json or --queries-json is required")
	case queryJSON != "" && queriesJSON != "":
		return nil, errors.New("--query-json and --queries-json cannot be used together")
	case queryJSON != "":
		var query map[string]any
		if err := json.Unmarshal([]byte(queryJSON), &query); err != nil {
			return nil, fmt.Errorf("invalid --query-json: %w", err)
		}
		return []map[string]any{applyDatasourceQueryDefaults(query, uid, datasourceType, chooseDefaultRefID(refID, 0), intervalMS, maxDataPoints)}, nil
	default:
		var raw []any
		if err := json.Unmarshal([]byte(queriesJSON), &raw); err != nil {
			return nil, fmt.Errorf("invalid --queries-json: %w", err)
		}
		if len(raw) == 0 {
			return nil, errors.New("--queries-json must contain at least one query object")
		}
		out := make([]map[string]any, 0, len(raw))
		for index, item := range raw {
			query, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("--queries-json item %d must be an object", index)
			}
			out = append(out, applyDatasourceQueryDefaults(query, uid, datasourceType, chooseDefaultRefID(refID, index), intervalMS, maxDataPoints))
		}
		return out, nil
	}
}

func applyDatasourceQueryDefaults(query map[string]any, uid, datasourceType, refID string, intervalMS, maxDataPoints int) map[string]any {
	cloned := cloneAnyMap(query)
	datasource, _ := cloned["datasource"].(map[string]any)
	datasource = cloneAnyMap(datasource)
	if strings.TrimSpace(uid) != "" {
		if _, exists := datasource["uid"]; !exists {
			datasource["uid"] = uid
		}
	}
	if strings.TrimSpace(datasourceType) != "" {
		if _, exists := datasource["type"]; !exists {
			datasource["type"] = datasourceType
		}
	}
	if len(datasource) > 0 {
		cloned["datasource"] = datasource
	}
	if strings.TrimSpace(refID) != "" {
		if _, exists := cloned["refId"]; !exists {
			cloned["refId"] = refID
		}
	}
	if intervalMS > 0 {
		if _, exists := cloned["intervalMs"]; !exists {
			cloned["intervalMs"] = intervalMS
		}
	}
	if maxDataPoints > 0 {
		if _, exists := cloned["maxDataPoints"]; !exists {
			cloned["maxDataPoints"] = maxDataPoints
		}
	}
	return cloned
}

func chooseDefaultRefID(refID string, index int) string {
	if strings.TrimSpace(refID) != "" {
		return refID
	}
	if index < 26 {
		return string(rune('A' + index))
	}
	return "Q" + strconv.Itoa(index+1)
}

func cloneAnyMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(value))
	for key, item := range value {
		cloned[key] = item
	}
	return cloned
}
