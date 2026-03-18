package cli

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"

	"github.com/matiasvillaverde/grafana-cli/internal/grafana"
)

func (a *App) runCloudBilledUsage(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"cloud", "billed-usage"}, true)
	}

	if args[0] != "get" {
		return errors.New("usage: cloud billed-usage get --org-slug ORG --year YYYY --month MM")
	}

	fs := flag.NewFlagSet("cloud billed-usage get", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	orgSlug := fs.String("org-slug", "", "Grafana Cloud organization slug")
	year := fs.Int("year", 0, "billing year")
	month := fs.Int("month", 0, "billing month number")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if strings.TrimSpace(*orgSlug) == "" {
		return errors.New("--org-slug is required")
	}
	if *year < 1 {
		return errors.New("--year must be at least 1")
	}
	if *month < 1 || *month > 12 {
		return errors.New("--month must be between 1 and 12")
	}

	cfg, err := a.requireAuthConfig()
	if err != nil {
		return err
	}
	result, err := a.NewClient(cfg).CloudBilledUsage(ctx, grafana.CloudBilledUsageRequest{
		OrgSlug: strings.TrimSpace(*orgSlug),
		Year:    *year,
		Month:   *month,
	})
	if err != nil {
		return err
	}
	payload := buildCloudBilledUsagePayload(strings.TrimSpace(*orgSlug), *year, *month, result)
	return a.emitWithMetadata(opts, payload, cloudBilledUsageMetadata(payload))
}

func buildCloudBilledUsagePayload(orgSlug string, year, month int, payload any) map[string]any {
	items := collectionAtPath(payload, "items")
	return map[string]any{
		"org_slug": orgSlug,
		"year":     year,
		"month":    month,
		"summary":  cloudBilledUsageSummary(items),
		"items":    items,
	}
}

func cloudBilledUsageSummary(items []any) map[string]any {
	dimensions := map[string]struct{}{}
	stacks := map[string]struct{}{}
	totalAmountDue := 0.0
	periodStart := ""
	periodEnd := ""

	for _, item := range items {
		record, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if dimension := strings.TrimSpace(firstNonEmptyString(record, "dimensionName")); dimension != "" {
			dimensions[dimension] = struct{}{}
		}
		if amountDue, ok := record["amountDue"].(float64); ok {
			totalAmountDue += amountDue
		}
		if periodStart == "" {
			periodStart = strings.TrimSpace(firstNonEmptyString(record, "periodStart"))
		}
		if periodEnd == "" {
			periodEnd = strings.TrimSpace(firstNonEmptyString(record, "periodEnd"))
		}
		for _, usage := range collectionAtPath(record, "usages") {
			usageRecord, ok := usage.(map[string]any)
			if !ok {
				continue
			}
			if stackName := strings.TrimSpace(firstNonEmptyString(usageRecord, "stackName")); stackName != "" {
				stacks[stackName] = struct{}{}
			}
		}
	}

	return map[string]any{
		"items":            len(items),
		"dimensions":       sortedSet(dimensions),
		"stack_count":      len(stacks),
		"stacks":           sortedSet(stacks),
		"total_amount_due": totalAmountDue,
		"period_start":     periodStart,
		"period_end":       periodEnd,
	}
}

func cloudBilledUsageMetadata(payload any) *responseMetadata {
	meta := &responseMetadata{Command: "cloud billed-usage get"}
	if count := countPath(payload, "items"); count > 0 {
		meta.Count = &count
	}
	return meta
}
