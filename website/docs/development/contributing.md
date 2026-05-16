# Contributing

## Getting Started

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `go test ./...`
5. Run vet: `go vet ./...`
6. Submit a pull request

## Code Style

- Follow Go conventions (`gofmt`)
- Use existing patterns and libraries
- Add tests for new functionality
- Update documentation as needed
- Use descriptive commit messages

## Pull Request Process

1. Ensure all tests pass
2. Update documentation if needed
3. Add tests for new features
4. Request review from maintainers
5. Squash commits before merge

## Commit Messages

```
<type>(<scope>): <description>

Examples:
feat(consensus): add view change timeout
fix(p2p): handle peer disconnection gracefully
docs(api): update JSON-RPC method list
test(node): add StatePruner mock tests
```

## Code of Conduct

Be respectful and constructive in all interactions. Focus on the code, not the person.
