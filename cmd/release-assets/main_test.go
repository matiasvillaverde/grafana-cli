package main

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	releasepkg "github.com/matiasvillaverde/grafana-cli/internal/release"
)

func TestRun(t *testing.T) {
	t.Run("requires subcommand", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		if code := run(nil, &stdout, &stderr, nil); code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		if !strings.Contains(stderr.String(), "usage: release-assets") {
			t.Fatalf("unexpected stderr: %s", stderr.String())
		}
	})

	t.Run("rejects unknown subcommand", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		if code := run([]string{"bad"}, &stdout, &stderr, nil); code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		if !strings.Contains(stderr.String(), "unknown subcommand") {
			t.Fatalf("unexpected stderr: %s", stderr.String())
		}
	})

	t.Run("renders homebrew formula", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		readFile := func(string) ([]byte, error) {
			return []byte(strings.Join([]string{
				"111 grafana_v1.2.3_darwin_amd64.tar.gz",
				"222 grafana_v1.2.3_darwin_arm64.tar.gz",
				"333 grafana_v1.2.3_linux_amd64.tar.gz",
				"444 grafana_v1.2.3_linux_arm64.tar.gz",
			}, "\n")), nil
		}
		code := run([]string{"homebrew", "--repo", "matiasvillaverde/grafana-cli", "--tag", "v1.2.3", "--checksums", "dist/checksums.txt"}, &stdout, &stderr, readFile)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "class GrafanaCli < Formula") {
			t.Fatalf("unexpected stdout: %s", stdout.String())
		}

		stdout.Reset()
		stderr.Reset()
		code = run([]string{"homebrew", "--repo", "matiasvillaverde/grafana-cli", "--tag", "v1.2.3", "--download-base-url", "http://127.0.0.1:8080/releases/v1.2.3", "--checksums", "dist/checksums.txt"}, &stdout, &stderr, readFile)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), `url "http://127.0.0.1:8080/releases/v1.2.3/grafana_v1.2.3_darwin_arm64.tar.gz"`) {
			t.Fatalf("unexpected custom download base stdout: %s", stdout.String())
		}
	})

	t.Run("reports read and parse errors", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		readErr := func(string) ([]byte, error) {
			return nil, errors.New("boom")
		}
		if code := run([]string{"homebrew", "--repo", "matiasvillaverde/grafana-cli", "--tag", "v1.2.3", "--checksums", "dist/checksums.txt"}, &stdout, &stderr, readErr); code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		if !strings.Contains(stderr.String(), "read checksums") {
			t.Fatalf("unexpected stderr for read error: %s", stderr.String())
		}

		stdout.Reset()
		stderr.Reset()
		parseErr := func(string) ([]byte, error) {
			return []byte("111 grafana_v1.2.3_darwin_amd64.tar.gz"), nil
		}
		if code := run([]string{"homebrew", "--repo", "matiasvillaverde/grafana-cli", "--tag", "v1.2.3", "--checksums", "dist/checksums.txt"}, &stdout, &stderr, parseErr); code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		if !strings.Contains(stderr.String(), "parse checksums") {
			t.Fatalf("unexpected stderr for parse error: %s", stderr.String())
		}
	})

	t.Run("reports flag parse errors", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		readFile := func(string) ([]byte, error) { return nil, nil }
		if code := run([]string{"homebrew", "--bad"}, &stdout, &stderr, readFile); code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		if !strings.Contains(stderr.String(), "flag provided but not defined") {
			t.Fatalf("unexpected stderr: %s", stderr.String())
		}
	})

	t.Run("reports usage and render errors", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		readFile := func(string) ([]byte, error) { return nil, nil }
		if code := runHomebrew(nil, &stdout, &stderr, readFile); code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		if !strings.Contains(stderr.String(), "usage: release-assets") {
			t.Fatalf("unexpected stderr: %s", stderr.String())
		}

		oldRender := renderHomebrewFormula
		defer func() { renderHomebrewFormula = oldRender }()
		renderHomebrewFormula = func(releasepkg.HomebrewFormulaSpec) (string, error) {
			return "", errors.New("render failed")
		}

		stdout.Reset()
		stderr.Reset()
		readFile = func(string) ([]byte, error) {
			return []byte(strings.Join([]string{
				"111 grafana_v1.2.3_darwin_amd64.tar.gz",
				"222 grafana_v1.2.3_darwin_arm64.tar.gz",
				"333 grafana_v1.2.3_linux_amd64.tar.gz",
				"444 grafana_v1.2.3_linux_arm64.tar.gz",
			}, "\n")), nil
		}
		if code := run([]string{"homebrew", "--repo", "matiasvillaverde/grafana-cli", "--tag", "v1.2.3", "--checksums", "dist/checksums.txt"}, &stdout, &stderr, readFile); code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		if !strings.Contains(stderr.String(), "render formula") {
			t.Fatalf("unexpected stderr: %s", stderr.String())
		}
	})
}

func TestMain(t *testing.T) {
	oldArgs := os.Args
	oldExit := exitFn
	defer func() {
		os.Args = oldArgs
		exitFn = oldExit
	}()

	os.Args = []string{"release-assets"}
	exitCode := -1
	exitFn = func(code int) {
		exitCode = code
	}
	main()
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
}
