#!/bin/sh

set -eu

if ! command -v brew >/dev/null 2>&1; then
  echo "Homebrew is not installed; skipping brew smoke test" >&2
  exit 0
fi

TMP_ROOT="$(mktemp -d)"
RELEASE_DIR="${TMP_ROOT}/release"
VERSION="v0.0.0-test"
FORMULA_PATH="${TMP_ROOT}/grafana-cli.rb"
TAP_NAMES=""

cleanup() {
  HOMEBREW_NO_AUTO_UPDATE=1 brew uninstall --formula grafana-cli >/dev/null 2>&1 || true
  for tap_name in ${TAP_NAMES}; do
    HOMEBREW_NO_AUTO_UPDATE=1 brew untap "${tap_name}" >/dev/null 2>&1 || true
  done
  if [ -n "${SERVER_PID:-}" ]; then
    kill "${SERVER_PID}" >/dev/null 2>&1 || true
  fi
  rm -rf "${TMP_ROOT}"
}
trap cleanup EXIT INT TERM

mkdir -p "${RELEASE_DIR}"

case "$(uname -s)" in
  Darwin) OS="darwin" ;;
  Linux) OS="linux" ;;
  *) echo "unsupported operating system: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

GOOS="${OS}" GOARCH="${ARCH}" CGO_ENABLED=0 go build -trimpath -o "${RELEASE_DIR}/grafana" ./cmd/grafana
VERSIONED_BINARY="grafana_${VERSION}_${OS}_${ARCH}"
cp "${RELEASE_DIR}/grafana" "${RELEASE_DIR}/${VERSIONED_BINARY}"

pack_archives() {
  layout="$1"

  rm -f "${RELEASE_DIR}"/*.tar.gz "${RELEASE_DIR}/checksums.txt"
  for target in darwin_amd64 darwin_arm64 linux_amd64 linux_arm64; do
    ARCHIVE="grafana_${VERSION}_${target}.tar.gz"
    case "$layout" in
      flat)
        tar -C "${RELEASE_DIR}" -czf "${RELEASE_DIR}/${ARCHIVE}" grafana
        ;;
      versioned)
        tar -C "${RELEASE_DIR}" -czf "${RELEASE_DIR}/${ARCHIVE}" "${VERSIONED_BINARY}"
        ;;
      *)
        echo "unsupported archive layout: ${layout}" >&2
        exit 1
        ;;
    esac
  done

  (
    cd "${RELEASE_DIR}"
    if command -v sha256sum >/dev/null 2>&1; then
      sha256sum *.tar.gz > checksums.txt
    else
      shasum -a 256 *.tar.gz > checksums.txt
    fi
  )
}

verify_install() {
  layout="$1"
  TAP_NAME="local/grafana-cli-smoke-$PPID-$layout"
  TAP_NAMES="${TAP_NAMES} ${TAP_NAME}"

  pack_archives "${layout}"

  go run ./cmd/release-assets homebrew \
    --repo matiasvillaverde/grafana-cli \
    --tag "${VERSION}" \
    --download-base-url "http://127.0.0.1:${PORT}" \
    --checksums "${RELEASE_DIR}/checksums.txt" > "${FORMULA_PATH}"

  HOMEBREW_NO_AUTO_UPDATE=1 brew tap-new "${TAP_NAME}" --no-git >/dev/null
  TAP_DIR="$(brew --repo "${TAP_NAME}")"
  mkdir -p "${TAP_DIR}/Formula"
  cp "${FORMULA_PATH}" "${TAP_DIR}/Formula/grafana-cli.rb"

  HOMEBREW_NO_AUTO_UPDATE=1 brew install "${TAP_NAME}/grafana-cli"
  HOMEBREW_NO_AUTO_UPDATE=1 brew test grafana-cli

  BREW_BIN="$(brew --prefix)/bin/grafana"
  HELP_OUTPUT="$("${BREW_BIN}" help)"
  printf '%s\n' "${HELP_OUTPUT}" | grep '"auth"'

  HOMEBREW_NO_AUTO_UPDATE=1 brew uninstall --formula grafana-cli >/dev/null
  HOMEBREW_NO_AUTO_UPDATE=1 brew untap "${TAP_NAME}" >/dev/null
}

PORT="$(python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
)"

python3 -m http.server "${PORT}" --bind 127.0.0.1 --directory "${RELEASE_DIR}" >/dev/null 2>&1 &
SERVER_PID=$!
sleep 1

verify_install flat
verify_install versioned
