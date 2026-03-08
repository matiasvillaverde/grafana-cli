package cli

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"

	"github.com/matiasvillaverde/grafana-cli/internal/grafana"
)

func (a *App) runCloudAccessPolicies(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"cloud", "access-policies"}, true)
	}

	cfg, err := a.requireAuthConfig()
	if err != nil {
		return err
	}
	client := a.NewClient(cfg)

	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("cloud access-policies list", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		name := fs.String("name", "", "access policy name filter")
		realmType := fs.String("realm-type", "", "realm type: org or stack")
		realmIdentifier := fs.String("realm-identifier", "", "realm identifier")
		pageSize := fs.Int("page-size", 100, "page size")
		pageCursor := fs.String("page-cursor", "", "cursor for the next page")
		region := fs.String("region", "", "access policy region")
		status := fs.String("status", "", "policy status: active or inactive")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*region) == "" {
			return errors.New("--region is required")
		}
		if *pageSize < 1 {
			return errors.New("--page-size must be at least 1")
		}
		if strings.TrimSpace(*realmIdentifier) != "" && strings.TrimSpace(*realmType) == "" {
			return errors.New("--realm-type is required when --realm-identifier is set")
		}
		if strings.TrimSpace(*realmType) != "" && *realmType != "org" && *realmType != "stack" {
			return errors.New("--realm-type must be org or stack")
		}
		if strings.TrimSpace(*status) != "" && *status != "active" && *status != "inactive" {
			return errors.New("--status must be active or inactive")
		}
		result, err := client.CloudAccessPolicies(ctx, grafana.CloudAccessPolicyListRequest{
			Name:            strings.TrimSpace(*name),
			RealmType:       strings.TrimSpace(*realmType),
			RealmIdentifier: strings.TrimSpace(*realmIdentifier),
			PageSize:        *pageSize,
			PageCursor:      strings.TrimSpace(*pageCursor),
			Region:          strings.TrimSpace(*region),
			Status:          strings.TrimSpace(*status),
		})
		if err != nil {
			return err
		}
		return a.emitWithMetadata(opts, result, accessPolicyMetadata(result))
	case "get":
		fs := flag.NewFlagSet("cloud access-policies get", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		id := fs.String("id", "", "access policy ID")
		region := fs.String("region", "", "access policy region")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*id) == "" {
			return errors.New("--id is required")
		}
		if strings.TrimSpace(*region) == "" {
			return errors.New("--region is required")
		}
		result, err := client.CloudAccessPolicy(ctx, strings.TrimSpace(*id), strings.TrimSpace(*region))
		if err != nil {
			return err
		}
		return a.emit(opts, result)
	default:
		return errors.New("usage: cloud access-policies <list|get> [flags]")
	}
}

func accessPolicyMetadata(payload any) *responseMetadata {
	meta := &responseMetadata{Command: "cloud access-policies list"}
	if count := countPath(payload, "items"); count > 0 {
		meta.Count = &count
	}
	if payloadHasNextPage(payload) {
		meta.Truncated = true
		meta.NextAction = "Use --page-cursor to continue listing access policies"
	}
	return meta
}
