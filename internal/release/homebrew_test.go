package release

import (
	"strings"
	"testing"
	"text/template"
)

func TestParseHomebrewChecksums(t *testing.T) {
	contents := strings.Join([]string{
		"",
		"junk",
		"111 grafana_v1.2.3_darwin_amd64.tar.gz",
		"222 grafana_v1.2.3_darwin_arm64.tar.gz",
		"333 grafana_v1.2.3_linux_amd64.tar.gz",
		"444 grafana_v1.2.3_linux_arm64.tar.gz",
		"555 grafana_v1.2.3_windows_amd64.zip",
	}, "\n")

	checksums, err := ParseHomebrewChecksums(contents, "v1.2.3")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if checksums.DarwinAMD64 != "111" || checksums.DarwinARM64 != "222" || checksums.LinuxAMD64 != "333" || checksums.LinuxARM64 != "444" {
		t.Fatalf("unexpected checksums: %+v", checksums)
	}
}

func TestParseHomebrewChecksumsRequiresTagAndFiles(t *testing.T) {
	if _, err := ParseHomebrewChecksums("", ""); err == nil {
		t.Fatalf("expected missing tag error")
	}
	if _, err := ParseHomebrewChecksums("111 grafana_v1.2.3_darwin_amd64.tar.gz", "v1.2.3"); err == nil {
		t.Fatalf("expected missing checksum files error")
	}
}

func TestRenderHomebrewFormula(t *testing.T) {
	formula, err := RenderHomebrewFormula(HomebrewFormulaSpec{
		Repo: "matiasvillaverde/grafana-cli",
		Tag:  "v1.2.3",
		Checksums: HomebrewChecksums{
			DarwinAMD64: "111",
			DarwinARM64: "222",
			LinuxAMD64:  "333",
			LinuxARM64:  "444",
		},
	})
	if err != nil {
		t.Fatalf("unexpected render error: %v", err)
	}
	for _, part := range []string{
		`class GrafanaCli < Formula`,
		`version "1.2.3"`,
		`https://github.com/matiasvillaverde/grafana-cli/releases/download/v1.2.3/grafana_v1.2.3_darwin_arm64.tar.gz`,
		`sha256 "333"`,
		`binary = Dir["grafana", "grafana_*"].find { |path| File.file?(path) }`,
		`bin.install binary => "grafana"`,
		`assert_match "auth"`,
	} {
		if !strings.Contains(formula, part) {
			t.Fatalf("expected formula to contain %q\n%s", part, formula)
		}
	}
}

func TestRenderHomebrewFormulaWithCustomDownloadBaseURL(t *testing.T) {
	formula, err := RenderHomebrewFormula(HomebrewFormulaSpec{
		Repo:            "matiasvillaverde/grafana-cli",
		Tag:             "v1.2.3",
		DownloadBaseURL: "http://127.0.0.1:8080/releases/v1.2.3",
		Checksums: HomebrewChecksums{
			DarwinAMD64: "111",
			DarwinARM64: "222",
			LinuxAMD64:  "333",
			LinuxARM64:  "444",
		},
	})
	if err != nil {
		t.Fatalf("unexpected render error: %v", err)
	}
	if !strings.Contains(formula, `url "http://127.0.0.1:8080/releases/v1.2.3/grafana_v1.2.3_darwin_arm64.tar.gz"`) {
		t.Fatalf("unexpected custom download base formula:\n%s", formula)
	}
}

func TestRenderHomebrewFormulaValidation(t *testing.T) {
	if _, err := RenderHomebrewFormula(HomebrewFormulaSpec{}); err == nil {
		t.Fatalf("expected missing repo/tag validation error")
	}
	if _, err := RenderHomebrewFormula(HomebrewFormulaSpec{Repo: "matiasvillaverde/grafana-cli"}); err == nil {
		t.Fatalf("expected missing tag validation error")
	}
	if _, err := RenderHomebrewFormula(HomebrewFormulaSpec{
		Repo: "matiasvillaverde/grafana-cli",
		Tag:  "v1.2.3",
		Checksums: HomebrewChecksums{
			DarwinAMD64: "111",
		},
	}); err == nil {
		t.Fatalf("expected missing checksum validation error")
	}

	if err := validateHomebrewChecksums(HomebrewChecksums{
		DarwinARM64: "222",
		LinuxAMD64:  "333",
		LinuxARM64:  "444",
	}); err == nil {
		t.Fatalf("expected explicit checksum validation error")
	}
}

func TestRenderHomebrewFormulaTemplateError(t *testing.T) {
	oldTemplate := homebrewFormulaTemplate
	defer func() { homebrewFormulaTemplate = oldTemplate }()
	homebrewFormulaTemplate = template.Must(template.New("broken").Parse(`{{ call .Repo }}`))

	if _, err := RenderHomebrewFormula(HomebrewFormulaSpec{
		Repo: "matiasvillaverde/grafana-cli",
		Tag:  "v1.2.3",
		Checksums: HomebrewChecksums{
			DarwinAMD64: "111",
			DarwinARM64: "222",
			LinuxAMD64:  "333",
			LinuxARM64:  "444",
		},
	}); err == nil {
		t.Fatalf("expected template execute error")
	}
}
