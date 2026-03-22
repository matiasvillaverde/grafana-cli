package cli

import (
	"errors"
	"flag"
	"fmt"
	"strings"
)

type datasourceQueryFamily struct {
	Name             string
	Description      string
	Syntax           string
	ExampleQueryJSON string
	SupportedTypes   []string
	DocumentationURL string
	Notes            []string
}

type datasourceQueryOptions struct {
	Selector      datasourceSelector
	From          string
	To            string
	RefID         string
	IntervalMS    int
	MaxDataPoints int
	QueryJSON     string
	QueriesJSON   string

	SQL           string
	Format        string
	Expr          string
	Query         string
	QueryLanguage string
	LegendFormat  string
	MinStep       string
	QueryType     string
	Limit         int
	Instant       bool

	CloudWatchNamespace  string
	CloudWatchMetricName string
	CloudWatchRegion     string
	CloudWatchStatistic  string
	CloudWatchDimensions string
	CloudWatchMatchExact bool
}

type datasourceQueryStrategy interface {
	Family() datasourceQueryFamily
	DiscoveryFlags() []discoveryFlag
	Examples() []string
	BindFlags(fs *flag.FlagSet, opts *datasourceQueryOptions)
	BuildQueries(opts datasourceQueryOptions, resolved resolvedDatasource) ([]map[string]any, error)
	SupportsType(value string) bool
}

type passthroughDatasourceStrategy struct {
	family datasourceQueryFamily
}

func (s passthroughDatasourceStrategy) Family() datasourceQueryFamily { return s.family }

func (s passthroughDatasourceStrategy) DiscoveryFlags() []discoveryFlag { return nil }

func (s passthroughDatasourceStrategy) Examples() []string {
	return []string{"grafana datasources " + s.family.Name + " query --uid " + s.family.Name + `-uid --query-json '` + s.family.ExampleQueryJSON + `'`}
}

func (s passthroughDatasourceStrategy) BindFlags(fs *flag.FlagSet, opts *datasourceQueryOptions) {
	_ = fs
	_ = opts
}

func (s passthroughDatasourceStrategy) BuildQueries(opts datasourceQueryOptions, resolved resolvedDatasource) ([]map[string]any, error) {
	return buildDatasourceQueries(resolved.UID, resolved.Type, opts.RefID, opts.IntervalMS, opts.MaxDataPoints, strings.TrimSpace(opts.QueryJSON), strings.TrimSpace(opts.QueriesJSON))
}

func (s passthroughDatasourceStrategy) SupportsType(value string) bool {
	if len(s.family.SupportedTypes) == 0 {
		return true
	}
	return datasourceTypeMatches(value, s.family.SupportedTypes)
}

type sqlDatasourceStrategy struct {
	passthroughDatasourceStrategy
}

const (
	clickhouseFormatTimeSeries = 0
	clickhouseFormatTable      = 1
	clickhouseFormatLogs       = 2
	clickhouseFormatTraces     = 3
)

func (s sqlDatasourceStrategy) DiscoveryFlags() []discoveryFlag {
	return []discoveryFlag{
		{Name: "--sql", Type: "string", Description: "SQL query text using the macros documented by Grafana for this datasource"},
		{Name: "--format", Type: "string", Default: "table", Description: "Grafana query format such as table or time_series"},
	}
}

func (s sqlDatasourceStrategy) Examples() []string {
	if s.family.Name == "clickhouse" {
		return []string{`grafana datasources clickhouse query --uid clickhouse-uid --sql 'SELECT now() AS time, count() AS value' --format table`}
	}
	return []string{"grafana datasources " + s.family.Name + " query --uid " + s.family.Name + `-uid --sql 'SELECT $__time(created_at), count(*) AS value FROM orders WHERE $__timeFilter(created_at) GROUP BY 1 ORDER BY 1' --format time_series`}
}

func (s sqlDatasourceStrategy) BindFlags(fs *flag.FlagSet, opts *datasourceQueryOptions) {
	fs.StringVar(&opts.SQL, "sql", "", "SQL query text")
	fs.StringVar(&opts.Format, "format", "table", "Grafana query format")
}

func (s sqlDatasourceStrategy) BuildQueries(opts datasourceQueryOptions, resolved resolvedDatasource) ([]map[string]any, error) {
	trimmedSQL := strings.TrimSpace(opts.SQL)
	if trimmedSQL != "" {
		queryPayload := map[string]any{
			"rawSql": trimmedSQL,
		}
		if s.family.Name == "clickhouse" {
			queryPayload["editorType"] = "sql"
			queryPayload["format"] = clickhouseQueryFormat(normalizeDefault(opts.Format, "table"))
		} else {
			queryPayload["format"] = normalizeDefault(opts.Format, "table")
		}
		return []map[string]any{
			applyDatasourceQueryDefaults(queryPayload, resolved.UID, resolved.Type, chooseDefaultRefID(opts.RefID, 0), opts.IntervalMS, opts.MaxDataPoints),
		}, nil
	}
	return s.passthroughDatasourceStrategy.BuildQueries(opts, resolved)
}

func clickhouseQueryFormat(value string) int {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "time_series", "timeseries", "graph":
		return clickhouseFormatTimeSeries
	case "logs":
		return clickhouseFormatLogs
	case "traces":
		return clickhouseFormatTraces
	default:
		return clickhouseFormatTable
	}
}

type prometheusDatasourceStrategy struct {
	passthroughDatasourceStrategy
}

func (s prometheusDatasourceStrategy) DiscoveryFlags() []discoveryFlag {
	return []discoveryFlag{
		{Name: "--expr", Type: "string", Description: "PromQL expression"},
		{Name: "--instant", Type: "bool", Default: false, Description: "Run an instant query instead of the default range query"},
		{Name: "--legend-format", Type: "string", Description: "Grafana legend template"},
		{Name: "--min-step", Type: "string", Description: "Prometheus min step or resolution such as 30s or 1m"},
		{Name: "--format", Type: "string", Default: "time_series", Description: "Grafana query format such as time_series or table"},
	}
}

func (s prometheusDatasourceStrategy) Examples() []string {
	return []string{
		`grafana datasources prometheus query --uid prometheus-uid --expr 'sum(rate(http_requests_total[5m])) by (service)' --min-step 1m`,
		`grafana datasources prometheus query --uid prometheus-uid --expr 'up{job="api"}' --instant --legend-format '{{instance}}'`,
	}
}

func (s prometheusDatasourceStrategy) BindFlags(fs *flag.FlagSet, opts *datasourceQueryOptions) {
	fs.StringVar(&opts.Expr, "expr", "", "PromQL expression")
	fs.BoolVar(&opts.Instant, "instant", false, "Run an instant query")
	fs.StringVar(&opts.LegendFormat, "legend-format", "", "Grafana legend template")
	fs.StringVar(&opts.MinStep, "min-step", "", "Prometheus min step or resolution")
	fs.StringVar(&opts.Format, "format", "time_series", "Grafana query format")
}

func (s prometheusDatasourceStrategy) BuildQueries(opts datasourceQueryOptions, resolved resolvedDatasource) ([]map[string]any, error) {
	if strings.TrimSpace(opts.Expr) != "" {
		query := map[string]any{
			"expr":    strings.TrimSpace(opts.Expr),
			"instant": opts.Instant,
			"format":  normalizeDefault(opts.Format, "time_series"),
		}
		if legend := strings.TrimSpace(opts.LegendFormat); legend != "" {
			query["legendFormat"] = legend
		}
		if minStep := strings.TrimSpace(opts.MinStep); minStep != "" {
			query["interval"] = minStep
		}
		return []map[string]any{
			applyDatasourceQueryDefaults(query, resolved.UID, resolved.Type, chooseDefaultRefID(opts.RefID, 0), opts.IntervalMS, opts.MaxDataPoints),
		}, nil
	}
	return s.passthroughDatasourceStrategy.BuildQueries(opts, resolved)
}

type lokiDatasourceStrategy struct {
	passthroughDatasourceStrategy
}

func (s lokiDatasourceStrategy) DiscoveryFlags() []discoveryFlag {
	return []discoveryFlag{
		{Name: "--expr", Type: "string", Description: "LogQL expression"},
		{Name: "--query-type", Type: "string", Default: "range", Description: "Loki query type: range or instant"},
		{Name: "--legend-format", Type: "string", Description: "Grafana legend template for metric queries"},
	}
}

func (s lokiDatasourceStrategy) Examples() []string {
	return []string{
		`grafana datasources loki query --uid loki-uid --expr '{app="checkout"} |= "error"' --query-type range`,
		`grafana datasources loki query --uid loki-uid --expr 'sum(rate({app="checkout"} |= "error" [5m]))' --query-type instant --legend-format '{{service}}'`,
	}
}

func (s lokiDatasourceStrategy) BindFlags(fs *flag.FlagSet, opts *datasourceQueryOptions) {
	fs.StringVar(&opts.Expr, "expr", "", "LogQL expression")
	fs.StringVar(&opts.QueryType, "query-type", "range", "Loki query type")
	fs.StringVar(&opts.LegendFormat, "legend-format", "", "Grafana legend template")
}

func (s lokiDatasourceStrategy) BuildQueries(opts datasourceQueryOptions, resolved resolvedDatasource) ([]map[string]any, error) {
	if strings.TrimSpace(opts.Expr) != "" {
		query := map[string]any{
			"expr":      strings.TrimSpace(opts.Expr),
			"queryType": normalizeDefault(opts.QueryType, "range"),
		}
		if legend := strings.TrimSpace(opts.LegendFormat); legend != "" {
			query["legendFormat"] = legend
		}
		return []map[string]any{
			applyDatasourceQueryDefaults(query, resolved.UID, resolved.Type, chooseDefaultRefID(opts.RefID, 0), opts.IntervalMS, opts.MaxDataPoints),
		}, nil
	}
	return s.passthroughDatasourceStrategy.BuildQueries(opts, resolved)
}

type queryDatasourceStrategy struct {
	passthroughDatasourceStrategy
	queryLabel string
	defaults   map[string]any
}

func (s queryDatasourceStrategy) DiscoveryFlags() []discoveryFlag {
	flags := []discoveryFlag{{Name: "--query", Type: "string", Description: s.queryLabel}}
	if s.family.Name == "influxdb" {
		flags = append(flags, discoveryFlag{Name: "--query-language", Type: "string", Description: "InfluxDB query language such as flux, influxql, or sql"})
	}
	return flags
}

func (s queryDatasourceStrategy) Examples() []string {
	example := "grafana datasources " + s.family.Name + " query --uid " + s.family.Name + `-uid --query 'up'`
	if s.family.Name == "influxdb" {
		example = `grafana datasources influxdb query --uid influxdb-uid --query 'from(bucket:"prod") |> range(start:-1h)' --query-language flux`
	}
	return []string{example}
}

func (s queryDatasourceStrategy) BindFlags(fs *flag.FlagSet, opts *datasourceQueryOptions) {
	fs.StringVar(&opts.Query, "query", "", s.queryLabel)
	if s.family.Name == "influxdb" {
		fs.StringVar(&opts.QueryLanguage, "query-language", "", "InfluxDB query language")
	}
}

func (s queryDatasourceStrategy) BuildQueries(opts datasourceQueryOptions, resolved resolvedDatasource) ([]map[string]any, error) {
	if strings.TrimSpace(opts.Query) != "" {
		query := cloneAnyMap(s.defaults)
		query["query"] = strings.TrimSpace(opts.Query)
		if strings.TrimSpace(opts.QueryLanguage) != "" {
			query["queryLanguage"] = strings.TrimSpace(opts.QueryLanguage)
		}
		return []map[string]any{
			applyDatasourceQueryDefaults(query, resolved.UID, resolved.Type, chooseDefaultRefID(opts.RefID, 0), opts.IntervalMS, opts.MaxDataPoints),
		}, nil
	}
	return s.passthroughDatasourceStrategy.BuildQueries(opts, resolved)
}

type tempoDatasourceStrategy struct {
	passthroughDatasourceStrategy
}

func (s tempoDatasourceStrategy) DiscoveryFlags() []discoveryFlag {
	return []discoveryFlag{
		{Name: "--query", Type: "string", Description: "TraceQL query"},
		{Name: "--limit", Type: "int", Description: "Result limit for TraceQL search"},
	}
}

func (s tempoDatasourceStrategy) Examples() []string {
	return []string{
		`grafana datasources tempo query --uid tempo-uid --query '{ resource.service.name = "checkout" && status = error }' --limit 20`,
		`grafana datasources tempo query --uid tempo-uid --query '{ span.http.method = "POST" }'`,
	}
}

func (s tempoDatasourceStrategy) BindFlags(fs *flag.FlagSet, opts *datasourceQueryOptions) {
	fs.StringVar(&opts.Query, "query", "", "TraceQL query")
	fs.IntVar(&opts.Limit, "limit", 0, "Result limit for TraceQL search")
}

func (s tempoDatasourceStrategy) BuildQueries(opts datasourceQueryOptions, resolved resolvedDatasource) ([]map[string]any, error) {
	if strings.TrimSpace(opts.Query) != "" {
		query := map[string]any{
			"query":     strings.TrimSpace(opts.Query),
			"queryType": "traceqlSearch",
		}
		if opts.Limit > 0 {
			query["limit"] = opts.Limit
		}
		return []map[string]any{
			applyDatasourceQueryDefaults(query, resolved.UID, resolved.Type, chooseDefaultRefID(opts.RefID, 0), opts.IntervalMS, opts.MaxDataPoints),
		}, nil
	}
	return s.passthroughDatasourceStrategy.BuildQueries(opts, resolved)
}

type graphiteDatasourceStrategy struct {
	passthroughDatasourceStrategy
}

func (s graphiteDatasourceStrategy) DiscoveryFlags() []discoveryFlag {
	return []discoveryFlag{
		{Name: "--expr", Type: "string", Description: "Graphite target expression"},
		{Name: "--format", Type: "string", Default: "time_series", Description: "Grafana query format such as time_series or table"},
	}
}

func (s graphiteDatasourceStrategy) Examples() []string {
	return []string{`grafana datasources graphite query --uid graphite-uid --expr 'sumSeries(stats.counters.checkout.errors)' --format time_series`}
}

func (s graphiteDatasourceStrategy) BindFlags(fs *flag.FlagSet, opts *datasourceQueryOptions) {
	fs.StringVar(&opts.Expr, "expr", "", "Graphite target expression")
	fs.StringVar(&opts.Format, "format", "time_series", "Grafana query format")
}

func (s graphiteDatasourceStrategy) BuildQueries(opts datasourceQueryOptions, resolved resolvedDatasource) ([]map[string]any, error) {
	if strings.TrimSpace(opts.Expr) != "" {
		return []map[string]any{
			applyDatasourceQueryDefaults(map[string]any{
				"target": strings.TrimSpace(opts.Expr),
				"format": normalizeDefault(opts.Format, "time_series"),
			}, resolved.UID, resolved.Type, chooseDefaultRefID(opts.RefID, 0), opts.IntervalMS, opts.MaxDataPoints),
		}, nil
	}
	return s.passthroughDatasourceStrategy.BuildQueries(opts, resolved)
}

type cloudWatchDatasourceStrategy struct {
	passthroughDatasourceStrategy
}

func (s cloudWatchDatasourceStrategy) DiscoveryFlags() []discoveryFlag {
	return []discoveryFlag{
		{Name: "--namespace", Type: "string", Description: "CloudWatch metric namespace"},
		{Name: "--metric-name", Type: "string", Description: "CloudWatch metric name"},
		{Name: "--region", Type: "string", Description: "AWS region"},
		{Name: "--statistic", Type: "string", Description: "Statistic such as Average or Sum"},
		{Name: "--dimensions", Type: "csv", Description: "Dimension filters as key=value,key=value"},
		{Name: "--match-exact", Type: "bool", Default: false, Description: "Require exact dimension matching in the CloudWatch metric query"},
	}
}

func (s cloudWatchDatasourceStrategy) Examples() []string {
	return []string{`grafana datasources cloudwatch query --uid cloudwatch-uid --namespace AWS/EC2 --metric-name CPUUtilization --region us-east-1 --statistic Average --dimensions InstanceId=i-123`}
}

func (s cloudWatchDatasourceStrategy) BindFlags(fs *flag.FlagSet, opts *datasourceQueryOptions) {
	fs.StringVar(&opts.CloudWatchNamespace, "namespace", "", "CloudWatch namespace")
	fs.StringVar(&opts.CloudWatchMetricName, "metric-name", "", "CloudWatch metric name")
	fs.StringVar(&opts.CloudWatchRegion, "region", "", "AWS region")
	fs.StringVar(&opts.CloudWatchStatistic, "statistic", "", "CloudWatch statistic")
	fs.StringVar(&opts.CloudWatchDimensions, "dimensions", "", "CloudWatch dimensions")
	fs.BoolVar(&opts.CloudWatchMatchExact, "match-exact", false, "Require exact dimension matching")
}

func (s cloudWatchDatasourceStrategy) BuildQueries(opts datasourceQueryOptions, resolved resolvedDatasource) ([]map[string]any, error) {
	if strings.TrimSpace(opts.CloudWatchNamespace) != "" || strings.TrimSpace(opts.CloudWatchMetricName) != "" || strings.TrimSpace(opts.CloudWatchRegion) != "" || strings.TrimSpace(opts.CloudWatchStatistic) != "" || strings.TrimSpace(opts.CloudWatchDimensions) != "" {
		if strings.TrimSpace(opts.CloudWatchNamespace) == "" || strings.TrimSpace(opts.CloudWatchMetricName) == "" || strings.TrimSpace(opts.CloudWatchRegion) == "" {
			return nil, errors.New("--namespace, --metric-name, and --region are required for typed cloudwatch queries")
		}
		query := map[string]any{
			"namespace":  strings.TrimSpace(opts.CloudWatchNamespace),
			"metricName": strings.TrimSpace(opts.CloudWatchMetricName),
			"region":     strings.TrimSpace(opts.CloudWatchRegion),
		}
		if statistic := strings.TrimSpace(opts.CloudWatchStatistic); statistic != "" {
			query["statistics"] = []any{statistic}
		}
		if dimensions, err := parseCloudWatchDimensions(opts.CloudWatchDimensions); err != nil {
			return nil, err
		} else if len(dimensions) > 0 {
			query["dimensions"] = dimensions
		}
		if opts.CloudWatchMatchExact {
			query["matchExact"] = true
		}
		return []map[string]any{
			applyDatasourceQueryDefaults(query, resolved.UID, resolved.Type, chooseDefaultRefID(opts.RefID, 0), opts.IntervalMS, opts.MaxDataPoints),
		}, nil
	}
	return s.passthroughDatasourceStrategy.BuildQueries(opts, resolved)
}

func datasourceQueryStrategies() []datasourceQueryStrategy {
	families := []datasourceQueryFamily{
		{
			Name:             "cloudwatch",
			Description:      "Query Amazon CloudWatch metrics and logs through Grafana",
			Syntax:           `CloudWatch datasource JSON, or typed metric flags such as --namespace, --metric-name, --region, --dimensions, and --match-exact. Typed flags cover metric queries; use --query-json for Logs Insights, OpenSearch SQL, or PPL.`,
			ExampleQueryJSON: `{"namespace":"AWS/ApplicationELB","metricName":"TargetResponseTime","dimensions":{"LoadBalancer":["app/prod"]},"statistics":["Average"],"region":"us-east-1"}`,
			SupportedTypes:   []string{"cloudwatch"},
			DocumentationURL: "https://grafana.com/docs/grafana/latest/datasources/aws-cloudwatch/query-editor/",
			Notes: []string{
				"Typed flags cover the metric query editor documented by Grafana.",
				"Use --query-json or --queries-json for CloudWatch Logs Insights, OpenSearch SQL, or PPL modes.",
			},
		},
		{
			Name:             "clickhouse",
			Description:      "Query ClickHouse through Grafana",
			Syntax:           `ClickHouse datasource JSON, or --sql with optional --format`,
			ExampleQueryJSON: `{"rawSql":"SELECT now() AS time, count() AS value","format":"table"}`,
			SupportedTypes:   []string{"clickhouse", "grafana-clickhouse-datasource", "vertamedia-clickhouse-datasource"},
			DocumentationURL: "https://grafana.com/grafana/plugins/grafana-clickhouse-datasource/",
			Notes: []string{
				"Typed flags follow the ClickHouse SQL editor pattern exposed by the official Grafana ClickHouse plugin.",
			},
		},
		{
			Name:             "mysql",
			Description:      "Query MySQL through Grafana",
			Syntax:           `MySQL datasource JSON, or --sql with optional --format and Grafana SQL macros such as $__timeFilter`,
			ExampleQueryJSON: `{"rawSql":"SELECT $__time(created_at), count(*) AS value FROM orders WHERE $__timeFilter(created_at) GROUP BY 1 ORDER BY 1","format":"time_series"}`,
			SupportedTypes:   []string{"mysql"},
			DocumentationURL: "https://grafana.com/docs/grafana/latest/datasources/mysql/query-editor/",
			Notes: []string{
				"Typed SQL follows Grafana's MySQL query editor and supports documented macros such as $__time and $__timeFilter.",
			},
		},
		{
			Name:             "postgres",
			Description:      "Query PostgreSQL through Grafana",
			Syntax:           `PostgreSQL datasource JSON, or --sql with optional --format and PostgreSQL macros`,
			ExampleQueryJSON: `{"rawSql":"SELECT $__time(ts), avg(latency_ms) AS value FROM request_latency WHERE $__timeFilter(ts) GROUP BY 1 ORDER BY 1","format":"time_series"}`,
			SupportedTypes:   []string{"postgres", "postgresql"},
			DocumentationURL: "https://grafana.com/docs/grafana/latest/datasources/postgres/query-editor/",
			Notes: []string{
				"Typed SQL follows Grafana's PostgreSQL query editor and documented macros.",
			},
		},
		{
			Name:             "mssql",
			Description:      "Query Microsoft SQL Server through Grafana",
			Syntax:           `MSSQL datasource JSON, or --sql with optional --format`,
			ExampleQueryJSON: `{"rawSql":"SELECT $__time(time_column), avg(duration_ms) AS value FROM request_log WHERE $__timeFilter(time_column) GROUP BY 1 ORDER BY 1","format":"time_series"}`,
			SupportedTypes:   []string{"mssql", "sqlserver"},
			DocumentationURL: "https://grafana.com/docs/grafana/latest/datasources/mssql/query-editor/",
			Notes: []string{
				"Typed SQL follows Grafana's Microsoft SQL Server query editor and documented macros.",
			},
		},
		{
			Name:             "influxdb",
			Description:      "Query InfluxDB through Grafana",
			Syntax:           `InfluxDB datasource JSON, or --query with optional --query-language such as flux or influxql`,
			ExampleQueryJSON: `{"query":"from(bucket: \"prod\") |> range(start: -1h) |> filter(fn: (r) => r._measurement == \"cpu\")","queryLanguage":"flux"}`,
			SupportedTypes:   []string{"influxdb"},
			DocumentationURL: "https://grafana.com/docs/grafana/latest/datasources/influxdb/query-editor/",
			Notes: []string{
				"Set --query-language to match the Grafana editor mode: flux, influxql, or sql.",
			},
		},
		{
			Name:             "elasticsearch",
			Description:      "Query Elasticsearch through Grafana",
			Syntax:           `Elasticsearch datasource JSON using Lucene query text and Grafana aggregation fields`,
			ExampleQueryJSON: `{"query":"service:checkout AND level:error","bucketAggs":[{"type":"date_histogram","field":"@timestamp","id":"2"}],"metrics":[{"type":"count","id":"1"}]}`,
			SupportedTypes:   []string{"elasticsearch"},
			DocumentationURL: "https://grafana.com/docs/grafana/latest/datasources/elasticsearch/query-editor/",
			Notes: []string{
				"Use --query-json for bucket aggregations, metrics, and Lucene clauses exactly as the Grafana editor models them.",
			},
		},
		{
			Name:             "opensearch",
			Description:      "Query OpenSearch through Grafana",
			Syntax:           `OpenSearch datasource JSON using Lucene, SQL, or PPL fields supported by the plugin`,
			ExampleQueryJSON: `{"query":"service:checkout AND status:error","bucketAggs":[{"type":"date_histogram","field":"@timestamp","id":"2"}],"metrics":[{"type":"count","id":"1"}]}`,
			SupportedTypes:   []string{"opensearch", "open-search", "grafana-opensearch-datasource"},
			DocumentationURL: "https://grafana.com/grafana/plugins/grafana-opensearch-datasource/",
			Notes: []string{
				"Use --query-json for Lucene, SQL, or PPL payloads as modeled by the Grafana OpenSearch plugin.",
			},
		},
		{
			Name:             "graphite",
			Description:      "Query Graphite through Grafana",
			Syntax:           `Graphite datasource JSON, or --expr for a Graphite target expression`,
			ExampleQueryJSON: `{"target":"sumSeries(stats.counters.checkout.errors)","format":"time_series"}`,
			SupportedTypes:   []string{"graphite"},
			DocumentationURL: "https://grafana.com/docs/grafana/latest/datasources/graphite/query-editor/",
			Notes: []string{
				"Typed flags follow the Graphite query editor target expression and format fields.",
			},
		},
		{
			Name:             "prometheus",
			Description:      "Query Prometheus-compatible datasources through Grafana",
			Syntax:           `Prometheus datasource JSON, or typed PromQL flags such as --expr, --instant, --legend-format, and --min-step`,
			ExampleQueryJSON: `{"expr":"sum(rate(http_requests_total[5m])) by (service)","instant":false}`,
			SupportedTypes:   []string{"prometheus", "mimir"},
			DocumentationURL: "https://grafana.com/docs/grafana/latest/datasources/prometheus/query-editor/",
			Notes: []string{
				"Typed flags follow the Prometheus query editor. The default typed query is a range query; add --instant for instant vector queries.",
			},
		},
		{
			Name:             "loki",
			Description:      "Query Loki through Grafana",
			Syntax:           `Loki datasource JSON, or typed LogQL flags such as --expr, --query-type, and --legend-format`,
			ExampleQueryJSON: `{"expr":"{app=\"checkout\"} |= \"error\"","queryType":"range"}`,
			SupportedTypes:   []string{"loki"},
			DocumentationURL: "https://grafana.com/docs/grafana/latest/datasources/loki/query-editor/",
			Notes: []string{
				"Typed flags follow the Loki query editor. Default typed mode is range; set --query-type instant for instant queries.",
			},
		},
		{
			Name:             "tempo",
			Description:      "Query Tempo through Grafana",
			Syntax:           `Tempo datasource JSON, or typed TraceQL search flags such as --query and --limit`,
			ExampleQueryJSON: `{"query":"{ resource.service.name = \"checkout\" && status = error }","queryType":"traceqlSearch"}`,
			SupportedTypes:   []string{"tempo"},
			DocumentationURL: "https://grafana.com/docs/grafana/latest/datasources/tempo/query-editor/",
			Notes: []string{
				"Typed flags cover TraceQL search. Use --query-json for service graph or other Tempo plugin query modes.",
			},
		},
		{
			Name:             "azure-monitor",
			Description:      "Query Azure Monitor through Grafana",
			Syntax:           `Azure Monitor datasource JSON for Metrics, Logs, Resource Graph, or Traces modes supported by the plugin`,
			ExampleQueryJSON: `{"queryType":"Azure Monitor","azureMonitor":{"metricDefinition":"Percentage CPU","metricNamespace":"Microsoft.Compute/virtualMachines","aggregation":"Average","resourceGroup":"prod-rg","resourceName":"vm-1","subscription":"00000000-0000-0000-0000-000000000000"}}`,
			SupportedTypes:   []string{"azure-monitor", "grafana-azure-monitor-datasource"},
			DocumentationURL: "https://grafana.com/docs/grafana/latest/datasources/azure-monitor/query-editor/",
			Notes: []string{
				"Use --query-json for Metrics, Logs, Resource Graph, and Traces payloads as modeled by the Grafana Azure Monitor plugin.",
			},
		},
	}

	strategies := make([]datasourceQueryStrategy, 0, len(families))
	for _, family := range families {
		base := passthroughDatasourceStrategy{family: family}
		switch family.Name {
		case "mysql", "postgres", "mssql", "clickhouse":
			strategies = append(strategies, sqlDatasourceStrategy{passthroughDatasourceStrategy: base})
		case "prometheus":
			strategies = append(strategies, prometheusDatasourceStrategy{passthroughDatasourceStrategy: base})
		case "loki":
			strategies = append(strategies, lokiDatasourceStrategy{passthroughDatasourceStrategy: base})
		case "graphite":
			strategies = append(strategies, graphiteDatasourceStrategy{passthroughDatasourceStrategy: base})
		case "tempo":
			strategies = append(strategies, tempoDatasourceStrategy{passthroughDatasourceStrategy: base})
		case "influxdb":
			strategies = append(strategies, queryDatasourceStrategy{passthroughDatasourceStrategy: base, queryLabel: "InfluxDB query", defaults: map[string]any{}})
		case "cloudwatch":
			strategies = append(strategies, cloudWatchDatasourceStrategy{passthroughDatasourceStrategy: base})
		default:
			strategies = append(strategies, base)
		}
	}
	return strategies
}

func datasourceQueryFamilies() []datasourceQueryFamily {
	families := make([]datasourceQueryFamily, 0, len(datasourceQueryStrategies()))
	for _, strategy := range datasourceQueryStrategies() {
		families = append(families, strategy.Family())
	}
	return families
}

func findDatasourceQueryFamily(name string) (datasourceQueryFamily, bool) {
	strategy, ok := findDatasourceStrategy(name)
	if !ok {
		return datasourceQueryFamily{}, false
	}
	return strategy.Family(), true
}

func findDatasourceStrategy(name string) (datasourceQueryStrategy, bool) {
	for _, strategy := range datasourceQueryStrategies() {
		if strategy.Family().Name == name {
			return strategy, true
		}
	}
	return nil, false
}

func datasourceQuerySyntaxDocs() map[string]string {
	out := map[string]string{
		"datasource_query": `Use --query-json for one plugin query object or --queries-json for a full Grafana queries array`,
	}
	for _, strategy := range datasourceQueryStrategies() {
		out[strategy.Family().Name] = strategy.Family().Syntax
	}
	return out
}

func datasourceTypeMatches(value string, accepted []string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return len(accepted) == 0
	}
	for _, item := range accepted {
		candidate := strings.ToLower(strings.TrimSpace(item))
		if candidate == normalized || strings.Contains(normalized, candidate) || strings.Contains(candidate, normalized) {
			return true
		}
	}
	return false
}

func normalizeDefault(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func parseCloudWatchDimensions(value string) (map[string]any, error) {
	out := map[string]any{}
	for _, item := range splitCSV(value) {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return nil, fmt.Errorf("invalid --dimensions entry %q: expected key=value", item)
		}
		key := strings.TrimSpace(parts[0])
		out[key] = []any{strings.TrimSpace(parts[1])}
	}
	return out, nil
}
