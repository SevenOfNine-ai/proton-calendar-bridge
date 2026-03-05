# Testing Strategy

## Principles

- Deterministic tests only — no external network calls during unit tests
- Provider and crypto layers tested with fakes/mocks, not live Proton credentials
- Write path tested end-to-end through the CalDAV handler using `httptest`
- Explicit tests for `ErrNotSupported` behaviour on read-only providers

## Test layers

### Unit tests

| Package | What is tested |
|---|---|
| `internal/config` | Env parsing, defaults, validation errors |
| `internal/security` | Bearer token and Basic auth acceptance/rejection |
| `internal/crypto` | Encrypt/decrypt round-trips, VCALENDAR encode/decode |
| `internal/protonapi` | Auth flow, client session management, write HTTP calls (against `httptest` server) |
| `internal/provider` | ProtonProvider and ICSProvider with mock Proton client |
| `internal/caldav` | PROPFIND, REPORT, GET, PUT, DELETE against mock CalendarProvider |
| `internal/api` | Full JSON API routes including auth middleware |
| `internal/app` | Server + tray lifecycle |

### Crypto round-trip tests (`internal/crypto`)

The encrypt/decrypt tests generate ephemeral PGP key rings and verify that:

1. `EncryptSharedEvent` → `decryptEventParts` round-trips the VCALENDAR payload
2. `EncryptPersonalEvent` → `decryptEventParts` round-trips alarm data
3. Tampered ciphertext or signature returns an error (not silent data corruption)

### Write path integration tests (`internal/caldav`, `internal/provider`)

CalDAV PUT and DELETE requests are exercised against a mock `CalendarProvider`
using `net/http/httptest`. The tests assert:

- Correct HTTP status codes (201 Created, 204 No Content, 404 Not Found)
- CalDAV XML response structure for PROPFIND and REPORT
- `ErrNotSupported` from a read-only provider → HTTP 501

### API contract tests (`internal/api`)

- `GET /v1/capabilities` returns the correct feature flags per provider
- `POST /v1/events/create` with a valid body calls `CreateEvent` on the provider
- Write requests to a read-only provider return HTTP 501 with a JSON error body

## Running tests

```bash
# All tests with coverage
go test ./... -coverprofile=coverage.out

# View per-function coverage
go tool cover -func=coverage.out

# Race detector (recommended before PRs)
go test -race ./...

# Single package, verbose
go test -v ./internal/crypto/...
go test -v ./internal/caldav/...
```

## Coverage gate

CI fails if total statement coverage falls below **85%**. The threshold is set in
`scripts/coverage_gate.sh` (Linux/macOS) and `scripts/coverage_gate.ps1` (Windows).

## Cross-platform validation

CI runs format check (`gofmt`), vet, tests, and a binary build on:

- Ubuntu (latest)
- macOS (latest)
- Windows (latest)

See `.github/workflows/ci.yml` for the full matrix.

## Manual smoke test (against live Proton account)

These steps require a real Proton account and are **not** run in CI:

```bash
# 1. Start the bridge
PCB_PROVIDER=proton \
PCB_BEARER_TOKEN=test-token \
PCB_KEY_PASSWORD=your-key-password \
  go run ./cmd/proton-calendar-bridge &

# 2. Authenticate
curl -s -X POST http://127.0.0.1:9842/v1/auth/login \
  -H "Authorization: Bearer test-token" \
  -H "Content-Type: application/json" \
  -d '{"username":"you@proton.me","password":"your-password"}'

# 3. CalDAV discovery
curl -s -X PROPFIND -H "Authorization: Bearer test-token" \
  -H "Depth: 1" http://127.0.0.1:9842/caldav/

# 4. Create an event via CalDAV PUT
curl -s -X PUT \
  -H "Authorization: Bearer test-token" \
  -H "Content-Type: text/calendar" \
  http://127.0.0.1:9842/caldav/calendars/<calID>/test-event.ics \
  --data-binary @test-event.ics

# 5. Verify the event appears in the Proton web calendar
```
