# Proton Calendar Bridge Specification

## Goal
Provide a secure local bridge so local automation tools (e.g., OpenClaw) can query Proton calendar data via a stable local API, while isolating provider-specific and unofficial Proton internals.

## Scope
- Cross-platform binaries and release artifacts for macOS, Linux, Windows
- Local-only API (HTTP on loopback and/or Unix socket)
- Provider abstraction for multiple backends
- Initial provider: ICS read-only
- CRUD contract exposed at bridge layer; unsupported provider features return explicit `501 Not Implemented`
- Capability discovery endpoint for client feature negotiation
- Token-based local auth

## API
- `GET /healthz`
- `GET /v1/capabilities`
- `GET /v1/calendars`
- `GET /v1/events?calendar_id=&from=&to=` (RFC3339)
- `POST /v1/events/create`
- `POST /v1/events/update`
- `POST /v1/events/delete`

## Security Requirements
- Bind to `127.0.0.1` by default
- Optional Unix socket mode, chmod `0600`
- Bearer token required by default
- No secrets persisted in repo
- Clear separation of official vs unofficial capabilities

## Provider Contract
`CalendarProvider`:
- `ListCalendars`
- `ListEvents`
- `CreateEvent`
- `UpdateEvent`
- `DeleteEvent`

Unsupported operations must return wrapped `ErrNotSupported`.

## Non-Goals (v0)
- Native Proton write API implementation (official API unavailable)
- Attendee/invite parity with Proton UI
- Multi-user remote exposure
