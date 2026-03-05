# Contributing To grafana-cli

Thanks for contributing. This project is agent-first and quality-gated.

## Core Quality Requirements

Every PR must pass:

- strict linting (`golangci-lint`)
- full test suite
- **100% statement coverage** across the repository

If a change cannot realistically hit 100% coverage, split/refactor the change until it can.

## Local Setup

Prerequisites:

- Go `1.24+`
- `golangci-lint` `v1.64.8`

Install linter:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
```

## Required Local Checks Before Push

Run exactly:

```bash
$(go env GOPATH)/bin/golangci-lint run --timeout=5m
go test ./... -covermode=atomic -coverprofile=coverage.out
go tool cover -func=coverage.out | tail -n 1
```

Expected coverage output:

```text
total: ... 100.0%
```

## Development Rules

- Keep outputs deterministic and machine-parseable.
- Prefer stable JSON shapes for commands.
- Keep command flags explicit and backward-compatible.
- Add/adjust tests in the same PR as behavior changes.
- Do not reduce lint strictness or coverage gates.

## Commit And PR Guidelines

- Use Conventional Commit style (`feat:`, `fix:`, `docs:`, `ci:`, etc.).
- Keep commits small and focused.
- Include tests for each behavior change.
- Describe agent impact in the PR description:
  - token usage impact
  - output contract changes
  - new command semantics

## CI Policy

GitHub Actions enforces lint and coverage gates on:

- pull requests
- pushes
- release workflow

A failing gate blocks merges and releases.
