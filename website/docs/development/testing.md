# Testing

## Running Tests

```bash
# All tests
go test ./...

# Specific package
go test ./internal/layer1/consensus/...

# Verbose output
go test -v ./internal/layer1/node/...

# With race detection
go test -race ./internal/layer1/...

# Code coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Test Categories

| Category | Location | Description |
|----------|----------|-------------|
| Unit Tests | `internal/*/...` | Individual component tests |
| Integration | `tests/integration/` | Multi-component integration |
| E2E | `internal/e2e/` | End-to-end network tests |
| Benchmarks | `tests/benchmarks/` | Performance benchmarks |
| Fuzz | `tests/fuzz/` | Fuzzing tests |
| Contracts | `tests/contracts/` | Smart contract tests |

## Writing Tests

Follow existing patterns in the codebase:

- Use table-driven tests with `testing`
- Mock interfaces for unit tests (e.g., `StateDeleter`)
- Use `httptest` for HTTP handler tests
- Use `bytes.Buffer` for capturing stdout/stderr

## CI Pipeline

The project uses GitHub Actions for CI:

1. `go vet ./...` — static analysis
2. `go test -race ./...` — tests with race detection
3. `go build ./...` — build verification
4. `npm run build` — frontend build
