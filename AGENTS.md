# Repository Guidelines

## Project Structure & Module Organization
- `cmd/grafana/`: CLI entrypoint (`main.go`) and top-level startup tests.
- `internal/cli/`: command routing, flag parsing, output shaping, and command tests.
- `internal/grafana/`: Grafana/Grafana Cloud HTTP client and API-facing logic.
- `internal/agent/`: agent-playbook planning logic for incident workflows.
- `internal/config/`: local auth/config persistence.
- `docs/`: architecture and product research notes.
- `.github/workflows/`: CI and release automation; `.github/pull_request_template.md` defines PR checklist requirements.

Tests live next to source files as `*_test.go`.

## Build, Test, and Development Commands
- Build binary:
```bash
go build ./cmd/grafana
```
- Run locally:
```bash
go run ./cmd/grafana help
```
- Run linter (required):
```bash
$(go env GOPATH)/bin/golangci-lint run --timeout=5m
```
- Run tests + coverage gate:
```bash
go test ./... -covermode=atomic -coverprofile=coverage.out
go tool cover -func=coverage.out | tail -n 1
```

## Coding Style & Naming Conventions
- Use standard Go formatting (`gofmt`, `goimports`) and idiomatic naming.
- Keep exported symbols `CamelCase`, internal helpers `camelCase`, tests `TestXxx`.
- CLI behavior must stay deterministic: stable flags, stable JSON shapes, minimal noisy output.
- Prefer small, focused changes and avoid introducing breaking command contract changes without explicit documentation.

## Testing Guidelines
- Use Go’s `testing` package; no external test framework is required.
- Add tests for both success paths and error/edge branches.
- Repository policy is strict: **100.0% statement coverage** is required in CI and release workflows.

## Commit & Pull Request Guidelines
- Follow Conventional Commits (examples from history): `feat(...)`, `docs(...)`, `ci(...)`, `fix(...)`.
- Keep commits small and clearly scoped.
- PRs should include: summary, agent impact (token usage/output contract/behavior), and completed quality checklist.
- If command UX or output contracts change, update `README.md` and relevant docs in the same PR.

## Security & Configuration Tips
- Never commit Grafana tokens or secrets.
- Use environment variables for local auth (example: `--token "$GRAFANA_TOKEN"`).
- Verify endpoint URLs explicitly when testing against Grafana Cloud stacks.
