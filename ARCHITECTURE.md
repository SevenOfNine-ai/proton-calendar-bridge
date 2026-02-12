# Architecture

## High-level
1. `cmd/proton-calendar-bridge/main.go` loads config and starts app.
2. `internal/app` orchestrates server and tray lifecycle.
3. `internal/api` exposes local HTTP/Unix API.
4. `internal/provider` abstracts calendar backends.
5. `internal/security` provides local bearer auth.

## Components
- `config`: env-driven config + validation
- `provider/ics`: read-only ICS adapter
- `api/server`: request routing and provider calls
- `tray`: no-op by default, systray behind build tag `systray`

## Trust boundaries
- Local clients → bridge API (authenticated)
- Bridge API → provider adapter
- ICS provider → remote ICS URL

## Risk boundaries
- Official safe path: read-only ICS links
- Unofficial Proton internals: must be implemented in dedicated adapter package in future and isolated behind interface
- Write endpoints remain contract-only for unsupported providers
