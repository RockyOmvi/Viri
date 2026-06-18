# Contributing to Viri

Thank you for your interest in contributing.

## Development setup

- Go 1.22 or newer
- Clone the repository and work from the repo root

## Running tests

```bash
go test ./internal/... ./tests/... -count=1
```

Longer fault-injection tests:

```bash
go test -run TestJepsenFaultInjection -v -timeout 120s ./tests/jepsen/
```

## Pull requests

- Branch from `main`
- Keep changes focused; one logical change per PR
- Describe what changed and how you tested it
- Ensure `go test ./internal/... ./tests/...` passes before requesting review

## Reporting issues

Open a GitHub issue with steps to reproduce, expected behavior, and what you observed instead. Security-sensitive reports should not be filed in public issues.
