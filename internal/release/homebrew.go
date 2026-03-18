package release

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
	"text/template"
)

type HomebrewChecksums struct {
	DarwinAMD64 string
	DarwinARM64 string
	LinuxAMD64  string
	LinuxARM64  string
}

type HomebrewFormulaSpec struct {
	Repo            string
	Tag             string
	DownloadBaseURL string
	Checksums       HomebrewChecksums
}

func ParseHomebrewChecksums(contents, tag string) (HomebrewChecksums, error) {
	if strings.TrimSpace(tag) == "" {
		return HomebrewChecksums{}, errors.New("tag is required")
	}

	checksums := HomebrewChecksums{}
	required := map[string]*string{
		fmt.Sprintf("grafana_%s_darwin_amd64.tar.gz", tag): &checksums.DarwinAMD64,
		fmt.Sprintf("grafana_%s_darwin_arm64.tar.gz", tag): &checksums.DarwinARM64,
		fmt.Sprintf("grafana_%s_linux_amd64.tar.gz", tag):  &checksums.LinuxAMD64,
		fmt.Sprintf("grafana_%s_linux_arm64.tar.gz", tag):  &checksums.LinuxARM64,
	}

	for _, line := range strings.Split(contents, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		sum := strings.TrimSpace(fields[0])
		name := strings.TrimSpace(fields[len(fields)-1])
		if target, ok := required[name]; ok {
			*target = sum
		}
	}

	missing := make([]string, 0, len(required))
	for name, target := range required {
		if strings.TrimSpace(*target) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return HomebrewChecksums{}, fmt.Errorf("missing checksums for %s", strings.Join(missing, ", "))
	}

	return checksums, nil
}

func RenderHomebrewFormula(spec HomebrewFormulaSpec) (string, error) {
	if strings.TrimSpace(spec.Repo) == "" {
		return "", errors.New("repo is required")
	}
	if strings.TrimSpace(spec.Tag) == "" {
		return "", errors.New("tag is required")
	}
	if err := validateHomebrewChecksums(spec.Checksums); err != nil {
		return "", err
	}

	data := struct {
		Repo            string
		Tag             string
		Version         string
		DownloadBaseURL string
		Checksums       HomebrewChecksums
	}{
		Repo:            strings.TrimSpace(spec.Repo),
		Tag:             strings.TrimSpace(spec.Tag),
		Version:         strings.TrimPrefix(strings.TrimSpace(spec.Tag), "v"),
		DownloadBaseURL: strings.TrimSpace(spec.DownloadBaseURL),
		Checksums:       spec.Checksums,
	}
	if data.DownloadBaseURL == "" {
		data.DownloadBaseURL = "https://github.com/" + data.Repo + "/releases/download/" + data.Tag
	}

	var out bytes.Buffer
	if err := homebrewFormulaTemplate.Execute(&out, data); err != nil {
		return "", err
	}
	return out.String(), nil
}

func validateHomebrewChecksums(checksums HomebrewChecksums) error {
	missing := make([]string, 0, 4)
	if strings.TrimSpace(checksums.DarwinAMD64) == "" {
		missing = append(missing, "darwin/amd64")
	}
	if strings.TrimSpace(checksums.DarwinARM64) == "" {
		missing = append(missing, "darwin/arm64")
	}
	if strings.TrimSpace(checksums.LinuxAMD64) == "" {
		missing = append(missing, "linux/amd64")
	}
	if strings.TrimSpace(checksums.LinuxARM64) == "" {
		missing = append(missing, "linux/arm64")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing homebrew checksums for %s", strings.Join(missing, ", "))
	}
	return nil
}

var homebrewFormulaTemplate = template.Must(template.New("homebrew").Parse(`class GrafanaCli < Formula
  desc "Agent-first CLI to control Grafana and Grafana Cloud"
  homepage "https://github.com/{{ .Repo }}"
  version "{{ .Version }}"

  on_macos do
    if Hardware::CPU.arm?
      url "{{ .DownloadBaseURL }}/grafana_{{ .Tag }}_darwin_arm64.tar.gz"
      sha256 "{{ .Checksums.DarwinARM64 }}"
    else
      url "{{ .DownloadBaseURL }}/grafana_{{ .Tag }}_darwin_amd64.tar.gz"
      sha256 "{{ .Checksums.DarwinAMD64 }}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "{{ .DownloadBaseURL }}/grafana_{{ .Tag }}_linux_arm64.tar.gz"
      sha256 "{{ .Checksums.LinuxARM64 }}"
    else
      url "{{ .DownloadBaseURL }}/grafana_{{ .Tag }}_linux_amd64.tar.gz"
      sha256 "{{ .Checksums.LinuxAMD64 }}"
    end
  end

  def install
    bin.install "grafana"
  end

  test do
    assert_match "auth", shell_output("#{bin}/grafana help")
  end
end
`))
