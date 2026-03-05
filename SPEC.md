# Proton Calendar Bridge — Specification

## Goal

Provide a secure local bridge so CalDAV-compliant clients (GNOME Calendar, Thunderbird,
Apple Calendar) and local automation tools (e.g. OpenClaw) can read and write Proton
Calendar events, while keeping all encryption within the bridge process and all
communication strictly local.

## Scope

- Cross-platform binaries for macOS, Linux, and Windows (CI + release pipeline)
- Local-only transport (TCP loopback `127.0.0.1` and/or Unix domain socket)
- Provider abstraction supporting multiple calendar backends
- **Proton provider**: full read-write using Proton's end-to-end encryption
- **ICS provider**: read-only via a remote `.ics` URL
- RFC 4791 CalDAV interface for standard calendar client compatibility
- JSON API for local automation tools
- Capability discovery endpoint so clients can negotiate features at runtime
- Bearer-token local authentication (compatible with both `Authorization: Bearer`
  and HTTP Basic, for GNOME Online Accounts support)

## CalDAV API (RFC 4791)

Mount prefix: `/caldav/`

| Method | Path | Description |
|---|---|---|
| `OPTIONS` | `/*` | Advertise `DAV: 1, 3, calendar-access` |
| `PROPFIND` | `/` | Principal + `calendar-home-set` discovery |
| `PROPFIND` | `/calendars/` | List all calendar collections |
| `PROPFIND` | `/calendars/{calID}/` | Calendar properties (`displayname`, `ctag`, `supported-calendar-component-set`) |
| `REPORT` | `/calendars/{calID}/` | `calendar-query` and `calendar-multiget` |
| `GET` | `/calendars/{calID}/{uid}.ics` | Fetch a single event as iCalendar |
| `PUT` | `/calendars/{calID}/{uid}.ics` | Create or update an event |
| `DELETE` | `/calendars/{calID}/{uid}.ics` | Delete an event |

GNOME Online Accounts CalDAV URL: `http://127.0.0.1:9842/caldav/`

## JSON API

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Liveness check — returns `{"ok":true}` |
| `GET` | `/v1/capabilities` | Feature flags for the active provider |
| `GET` | `/v1/calendars` | List calendars with `read_only` and `permissions` fields |
| `GET` | `/v1/events?calendar_id=&from=&to=` | List events (RFC 3339 time range) |
| `POST` | `/v1/events/create` | Create an event (`domain.EventMutation` body) |
| `POST` | `/v1/events/update` | Update an event |
| `POST` | `/v1/events/delete` | Delete an event (`{"event_id":"…"}` body) |
| `POST` | `/v1/auth/login` | Authenticate with Proton (`username`, `password`) |
| `POST` | `/v1/auth/2fa` | Submit TOTP code (`totp`) |

### Capabilities response

```json
{
  "provider":         "proton",
  "read_only":        false,
  "write_supported":  true,
  "shared_calendars": true,
  "attendees":        true,
  "reminders":        true,
  "recurrence":       true
}
```

Clients must call `/v1/capabilities` before attempting write operations. If `read_only`
is `true` or `write_supported` is `false`, write operations will return `501 Not Implemented`.

## Provider Contract

`CalendarProvider` interface:

```go
ListCalendars(ctx) ([]domain.Calendar, error)
ListEvents(ctx, calendarID, from, to) ([]domain.Event, error)
CreateEvent(ctx, domain.EventMutation) (domain.Event, error)
UpdateEvent(ctx, eventID, domain.EventMutation) (domain.Event, error)
DeleteEvent(ctx, eventID) error
Capabilities(ctx) (CapabilitySet, error)
```

Providers that do not support a given operation must return a wrapped `ErrNotSupported`.
The API layer translates `ErrNotSupported` to `501 Not Implemented`.

## Security Requirements

- Bind to `127.0.0.1` by default; Unix socket mode uses `chmod 0600`
- Bearer token required by default (`PCB_REQUIRE_TOKEN=true`)
- HTTP Basic auth password accepted as equivalent to bearer token (GNOME compatibility)
- No Proton credentials or PGP keys written to disk
- Calendar key rings cached in process memory only; cleared on process exit
- Proton session tokens stored in-memory; not exposed to local API clients

## Encryption Model (Proton provider, write path)

1. Derive address key ring: fetch address keys from Proton, unlock with `PCB_KEY_PASSWORD`
2. Derive calendar key ring: fetch calendar passphrase (encrypted to address key ring),
   decrypt it, use it to unlock the calendar private keys
3. For each event to write:
   - Encode VCALENDAR text (`EncodeSharedVCalendar`)
   - Generate a random AES-256 session key
   - Encrypt session key for calendar key ring → `SharedKeyPacket`
   - Encrypt VCALENDAR with session key → `DataPacket`
   - Sign VCALENDAR plaintext with address private key → `Signature`
   - POST/PUT to `/calendar/v1/{calID}/events[/{eventID}]`

## Non-Goals

- Native Proton write API via an official public SDK (none exists; direct HTTP used instead)
- Full CalDAV push notifications (RFC 8144) — polling only
- Per-attendee encrypted invitation emails (`AttendeesEventContent`)
- Partial-series recurring event updates (THISANDFUTURE / THIS scope)
- Named-timezone round-trips (all times normalised to UTC)
- Multi-user or remote exposure (loopback-only by design)
