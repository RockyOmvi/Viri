# Contributing

Thank you for your interest in contributing to Viri. This project is in active development and welcomes contributions of all kinds.

## Code of Conduct

Be respectful, inclusive, and constructive. Harassment, trolling, and personal attacks will not be tolerated.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/viri.git`
3. Create a branch: `git checkout -b feature/your-feature`
4. Make your changes
5. Run tests: `go test ./... -count=1 -timeout 5m`
6. Run vet: `go vet ./...`
7. Commit and push: `git commit -m "feat: description" && git push`
8. Open a pull request

## What to Work On

### Good First Issues

- Add missing test coverage
- Improve error messages
- Add documentation comments
- Benchmark and performance analysis

### High Priority

- Browser wallet (Chrome extension, Manifest V3)
- Public testnet deployment scripts
- Explorer integration
- Faucet server
- Smart contract examples and tutorials

### Architecture

- secp256k1 migration for EVM tooling compatibility
- gnark real ZK-SNARK integration
- Libp2p integration for production P2P
- Hardware security module (HSM) support

## Pull Request Guidelines

- One feature/fix per PR
- Include tests for new functionality
- Ensure `go test ./...` passes
- Ensure `go vet ./...` passes
- Follow existing code style (no comments unless necessary)
- Update CHANGELOG.md if adding or changing features
- Reference any related issues

## Commit Message Format

```
<type>: <short description>

<optional body>
```

Types: `feat`, `fix`, `test`, `docs`, `refactor`, `chore`, `perf`, `sec`

## Test Requirements

All contributions must pass:

```bash
go build ./...
go vet ./...
go test ./internal/... -count=1 -timeout 5m
go test ./cmd/... -count=1 -timeout 5m
go test ./tests/... -count=1 -timeout 5m
```

## Project Structure

```
internal/
├── layer1/     # Core: consensus, crypto, ledger, p2p, state, sync
├── layer2/     # Execution: EVM, WASM, ZK, gas, MEV, AA, rollups
├── layer3/     # Application: governance, bridge, interop, intent, API
└── pkg/        # Shared utilities: metrics, audit, security
```

## Questions?

Open a [GitHub Discussion](https://github.com/viri-chain/viri/discussions) or reach out on the issue tracker.
