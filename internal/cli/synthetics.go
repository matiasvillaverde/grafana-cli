package cli

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"strings"

	"github.com/matiasvillaverde/grafana-cli/internal/config"
	"github.com/matiasvillaverde/grafana-cli/internal/grafana"
)

func (a *App) runSynthetics(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return a.emitHelp(opts, []string{"synthetics"}, true)
	}
	if len(args) < 2 || args[0] != "checks" {
		return errors.New("usage: synthetics checks <list|get> [flags]")
	}

	client := a.NewClient(config.Config{})

	switch args[1] {
	case "list":
		fs := flag.NewFlagSet("synthetics checks list", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		backendURL := fs.String("backend-url", "", "Synthetic Monitoring backend address")
		token := fs.String("token", "", "Synthetic Monitoring access token")
		includeAlerts := fs.Bool("include-alerts", false, "include alert definitions")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		req, err := syntheticCheckListRequest(*backendURL, *token, *includeAlerts)
		if err != nil {
			return err
		}
		result, err := client.SyntheticChecks(ctx, req)
		if err != nil {
			return err
		}
		return a.emitWithMetadata(opts, result, collectionMetadata("synthetics checks list", result, 0, ""))
	case "get":
		fs := flag.NewFlagSet("synthetics checks get", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		backendURL := fs.String("backend-url", "", "Synthetic Monitoring backend address")
		token := fs.String("token", "", "Synthetic Monitoring access token")
		id := fs.Int64("id", 0, "synthetic check ID")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if *id < 1 {
			return errors.New("--id must be at least 1")
		}
		req, err := syntheticCheckGetRequest(*backendURL, *token, *id)
		if err != nil {
			return err
		}
		result, err := client.SyntheticCheck(ctx, req)
		if err != nil {
			return err
		}
		return a.emit(opts, result)
	default:
		return errors.New("usage: synthetics checks <list|get> [flags]")
	}
}

func syntheticCheckListRequest(backendURL, token string, includeAlerts bool) (grafana.SyntheticCheckListRequest, error) {
	resolvedBackendURL, resolvedToken, err := resolveSyntheticsAuth(backendURL, token)
	if err != nil {
		return grafana.SyntheticCheckListRequest{}, err
	}
	return grafana.SyntheticCheckListRequest{
		BackendURL:    resolvedBackendURL,
		Token:         resolvedToken,
		IncludeAlerts: includeAlerts,
	}, nil
}

func syntheticCheckGetRequest(backendURL, token string, id int64) (grafana.SyntheticCheckGetRequest, error) {
	resolvedBackendURL, resolvedToken, err := resolveSyntheticsAuth(backendURL, token)
	if err != nil {
		return grafana.SyntheticCheckGetRequest{}, err
	}
	return grafana.SyntheticCheckGetRequest{
		BackendURL: resolvedBackendURL,
		Token:      resolvedToken,
		ID:         id,
	}, nil
}

func resolveSyntheticsAuth(backendURL, token string) (string, string, error) {
	resolvedBackendURL := strings.TrimSpace(backendURL)
	if resolvedBackendURL == "" {
		resolvedBackendURL = strings.TrimSpace(os.Getenv("GRAFANA_SYNTHETICS_BACKEND_URL"))
	}
	if resolvedBackendURL == "" {
		return "", "", errors.New("--backend-url is required or set GRAFANA_SYNTHETICS_BACKEND_URL")
	}
	resolvedToken := strings.TrimSpace(token)
	if resolvedToken == "" {
		resolvedToken = strings.TrimSpace(os.Getenv("GRAFANA_SYNTHETICS_TOKEN"))
	}
	if resolvedToken == "" {
		return "", "", errors.New("--token is required or set GRAFANA_SYNTHETICS_TOKEN")
	}
	return resolvedBackendURL, resolvedToken, nil
}
