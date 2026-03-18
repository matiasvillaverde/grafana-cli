package cli

import (
	"errors"
	"flag"
	"io"
	"strings"
)

const cloudStackFlagUsage = "Grafana Cloud stack slug or https://<stack>.grafana.net URL"

type cloudStackTarget struct {
	Slug    string
	BaseURL string
}

func (t *cloudStackTarget) Set(value string) error {
	slug, baseURL, err := normalizeStackIdentifier(value)
	if err != nil {
		return err
	}
	t.Slug = slug
	t.BaseURL = baseURL
	return nil
}

func (t *cloudStackTarget) String() string {
	if strings.TrimSpace(t.BaseURL) != "" {
		return t.BaseURL
	}
	return t.Slug
}

func (t *cloudStackTarget) required() (cloudStackTarget, error) {
	if strings.TrimSpace(t.Slug) == "" {
		return cloudStackTarget{}, errors.New("--stack is required")
	}
	return *t, nil
}

func newCloudStackFlagSet(name string) (*flag.FlagSet, *cloudStackTarget) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	target := &cloudStackTarget{}
	fs.Var(target, "stack", cloudStackFlagUsage)
	return fs, target
}
