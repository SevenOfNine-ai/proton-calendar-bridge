# Testing Strategy

## Principles
- Deterministic tests only
- No external network during unit tests
- Explicit unsupported behavior tests

## Test layers
- Unit tests: config, auth, ICS parsing/fetch behavior, API handlers, app lifecycle
- Contract tests: provider unsupported CRUD behavior
- Integration-style local server tests via `httptest`

## Commands
- `go test ./... -coverprofile=coverage.out`
- `go tool cover -func=coverage.out`

## Coverage gate
CI fails if total coverage < 85% (adjustable as codebase grows).
