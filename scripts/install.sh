#!/bin/sh

set -eu

REPO="${GRAFANA_INSTALL_REPO:-matiasvillaverde/grafana-cli}"
VERSION="${GRAFANA_INSTALL_VERSION:-}"
BASE_URL="${GRAFANA_INSTALL_BASE_URL:-}"

detect_os() {
  case "$(uname -s)" in
    Darwin) printf '%s' "darwin" ;;
    Linux) printf '%s' "linux" ;;
    *) echo "unsupported operating system: $(uname -s)" >&2; exit 1 ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf '%s' "amd64" ;;
    arm64|aarch64) printf '%s' "arm64" ;;
    *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
  esac
}

resolve_version() {
  if [ -n "$VERSION" ]; then
    printf '%s' "$VERSION"
    return
  fi
  final_url="$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest")"
  version="${final_url##*/}"
  if [ -z "$version" ]; then
    echo "failed to resolve latest release version" >&2
    exit 1
  fi
  printf '%s' "$version"
}

checksum_tool() {
  if command -v sha256sum >/dev/null 2>&1; then
    printf '%s' "sha256sum"
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    printf '%s' "shasum -a 256"
    return
  fi
  echo "missing checksum tool: install sha256sum or shasum" >&2
  exit 1
}

verify_checksum() {
  archive_path="$1"
  checksums_path="$2"
  archive_name="$(basename "$archive_path")"
  expected="$(awk -v name="$archive_name" '$2 == name { print $1 }' "$checksums_path")"
  if [ -z "$expected" ]; then
    echo "missing checksum for $archive_name" >&2
    exit 1
  fi
  actual="$(eval "$(checksum_tool) \"$archive_path\"" | awk '{ print $1 }')"
  if [ "$expected" != "$actual" ]; then
    echo "checksum mismatch for $archive_name" >&2
    exit 1
  fi
}

pick_extracted_binary() {
  archive_name="$1"

  for candidate in \
    "${TMPDIR}/grafana" \
    "${TMPDIR}/grafana_${VERSION}_${OS}_${ARCH}"
  do
    if [ -f "$candidate" ]; then
      printf '%s' "$candidate"
      return
    fi
  done

  echo "failed to locate grafana binary in ${archive_name}" >&2
  exit 1
}

pick_bindir() {
  if [ -n "${BINDIR:-}" ]; then
    printf '%s' "$BINDIR"
    return
  fi
  if [ -w "/usr/local/bin" ]; then
    printf '%s' "/usr/local/bin"
    return
  fi
  printf '%s' "${HOME}/.local/bin"
}

OS="$(detect_os)"
ARCH="$(detect_arch)"
VERSION="$(resolve_version)"
ARCHIVE="grafana_${VERSION}_${OS}_${ARCH}.tar.gz"
if [ -z "$BASE_URL" ]; then
  BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
fi
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT INT TERM

curl -fsSL "${BASE_URL}/${ARCHIVE}" -o "${TMPDIR}/${ARCHIVE}"
curl -fsSL "${BASE_URL}/checksums.txt" -o "${TMPDIR}/checksums.txt"
verify_checksum "${TMPDIR}/${ARCHIVE}" "${TMPDIR}/checksums.txt"
tar -xzf "${TMPDIR}/${ARCHIVE}" -C "${TMPDIR}"
EXTRACTED_BINARY="$(pick_extracted_binary "${ARCHIVE}")"

BINDIR="$(pick_bindir)"
mkdir -p "$BINDIR"
install -m 0755 "${EXTRACTED_BINARY}" "${BINDIR}/grafana"

echo "Installed grafana ${VERSION} to ${BINDIR}/grafana"
case ":$PATH:" in
  *":${BINDIR}:"*) ;;
  *) echo "Add ${BINDIR} to your PATH if it is not already there." ;;
esac
