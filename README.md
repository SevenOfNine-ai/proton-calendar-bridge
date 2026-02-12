# proton-calendar-bridge

Local-first bridge service for exposing Proton calendar data to local automation tools (e.g., OpenClaw) with explicit security boundaries.

## Current status
- ✅ Local API scaffold (HTTP loopback + Unix socket)
- ✅ Provider abstraction (`CalendarProvider`)
- ✅ Read-safe ICS provider implementation
- ✅ CRUD bridge contracts with `NotSupported` semantics for unsupported providers
- ✅ Structured config/validation and bearer auth
- ✅ Tray lifecycle scaffold (no-op by default, real systray via build tag)

## Why this shape
As of current research, Proton does not provide an official public write API for Calendar. This project therefore:
1. Uses official/safe read capability (ICS) now.
2. Keeps write contracts and provider interfaces ready for future adapters.
3. Isolates risk if unofficial adapters are added later.

## Run
```bash
go run ./cmd/proton-calendar-bridge
```

Required environment:
- `PCB_ICS_URL` (for `provider=ics`)
- `PCB_BEARER_TOKEN` (unless `PCB_REQUIRE_TOKEN=false`)

Optional:
- `PCB_BIND_ADDRESS` (default `127.0.0.1:9842`)
- `PCB_UNIX_SOCKET`
- `PCB_LOG_LEVEL` (`debug|info|warn|error`)
- `PCB_ENABLE_TRAY` (`true|false`, default false)

## API quick check
```bash
curl -H "Authorization: Bearer $PCB_BEARER_TOKEN" http://127.0.0.1:9842/v1/calendars
```

## Build with tray icon support
```bash
go build -tags systray ./cmd/proton-calendar-bridge
```

## Limitations
- ICS provider is read-only.
- Write endpoints return 501 for read-only providers.
- Recurrence expansion/invite workflows are not implemented in v0.

## Roadmap
- Add pluggable unofficial Proton adapter package with strict risk controls.
- Add event recurrence normalization.
- Add stronger integration harness and replay fixtures.
- Add binary release pipeline and signing.

## Docs
- `SPEC.md`
- `ARCHITECTURE.md`
- `TESTING.md`
- `deep-research-report.md`
