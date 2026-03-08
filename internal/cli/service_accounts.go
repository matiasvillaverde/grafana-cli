package cli

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"

	"github.com/matiasvillaverde/grafana-cli/internal/grafana"
)

func (a *App) runServiceAccounts(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"service-accounts"}, true)
	}

	cfg, err := a.requireAuthConfig()
	if err != nil {
		return err
	}
	client := a.NewClient(cfg)

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("service-accounts list", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		query := fs.String("query", "", "service account name filter")
		page := fs.Int("page", 1, "page number")
		limit := fs.Int("limit", 100, "page size")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *page < 1 {
			return errors.New("--page must be at least 1")
		}
		if *limit < 1 {
			return errors.New("--limit must be at least 1")
		}
		result, err := client.ServiceAccounts(ctx, grafana.ServiceAccountListRequest{
			Query: strings.TrimSpace(*query),
			Page:  *page,
			Limit: *limit,
		})
		if err != nil {
			return err
		}
		return a.emitWithMetadata(opts, result, serviceAccountMetadata(result))
	case "get":
		fs := flag.NewFlagSet("service-accounts get", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		id := fs.Int64("id", 0, "service account ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id < 1 {
			return errors.New("--id must be at least 1")
		}
		result, err := client.ServiceAccount(ctx, *id)
		if err != nil {
			return err
		}
		return a.emit(opts, result)
	default:
		return errors.New("usage: service-accounts <list|get> [flags]")
	}
}

func serviceAccountMetadata(payload any) *responseMetadata {
	meta := &responseMetadata{Command: "service-accounts list"}
	if count := countPath(payload, "serviceAccounts"); count > 0 {
		meta.Count = &count
	}
	if totalCount, ok := intPath(payload, "totalCount"); ok && meta.Count != nil && totalCount > *meta.Count {
		meta.Truncated = true
		meta.NextAction = "Use --page to inspect more service accounts or narrow --query"
	}
	return meta
}
