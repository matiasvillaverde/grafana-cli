package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	releasepkg "github.com/matiasvillaverde/grafana-cli/internal/release"
)

var exitFn = os.Exit
var renderHomebrewFormula = releasepkg.RenderHomebrewFormula

func main() {
	exitFn(run(os.Args[1:], os.Stdout, os.Stderr, os.ReadFile))
}

func run(args []string, stdout, stderr io.Writer, readFile func(string) ([]byte, error)) int {
	if len(args) < 1 {
		return fail(stderr, "usage: release-assets homebrew --repo owner/repo --tag v0.1.0 --checksums dist/checksums.txt")
	}

	switch args[0] {
	case "homebrew":
		return runHomebrew(args[1:], stdout, stderr, readFile)
	default:
		return fail(stderr, "unknown subcommand: %s", args[0])
	}
}

func runHomebrew(args []string, stdout, stderr io.Writer, readFile func(string) ([]byte, error)) int {
	fs := flag.NewFlagSet("homebrew", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repo := fs.String("repo", "", "GitHub repository in owner/repo form")
	tag := fs.String("tag", "", "release tag")
	downloadBaseURL := fs.String("download-base-url", "", "override base URL for release archives")
	checksumsPath := fs.String("checksums", "", "path to checksums.txt")
	if err := fs.Parse(args); err != nil {
		return fail(stderr, err.Error())
	}

	if *repo == "" || *tag == "" || *checksumsPath == "" {
		return fail(stderr, "usage: release-assets homebrew --repo owner/repo --tag v0.1.0 --checksums dist/checksums.txt")
	}

	contents, err := readFile(*checksumsPath)
	if err != nil {
		return fail(stderr, "read checksums: %v", err)
	}
	checksums, err := releasepkg.ParseHomebrewChecksums(string(contents), *tag)
	if err != nil {
		return fail(stderr, "parse checksums: %v", err)
	}
	formula, err := renderHomebrewFormula(releasepkg.HomebrewFormulaSpec{
		Repo:            *repo,
		Tag:             *tag,
		DownloadBaseURL: *downloadBaseURL,
		Checksums:       checksums,
	})
	if err != nil {
		return fail(stderr, "render formula: %v", err)
	}
	_, _ = io.WriteString(stdout, formula)
	return 0
}

func fail(stderr io.Writer, format string, args ...any) int {
	fmt.Fprintf(stderr, format+"\n", args...)
	return 1
}
