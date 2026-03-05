# Architecture

## High-level

```
CalDAV client (GNOME Calendar, Thunderbird, Apple Calendar, …)
        │  HTTP  OPTIONS / PROPFIND / REPORT / GET / PUT / DELETE
        ▼
┌──────────────────────────────────────────────────────────────┐
│  internal/caldav   RFC 4791 CalDAV handler                   │
│    OPTIONS  /*           → DAV capability headers            │
│    PROPFIND /            → principal + calendar-home-set     │
│    PROPFIND /calendars/  → calendar collection listing       │
│    PROPFIND /calendars/{calID}/  → calendar properties       │
│    REPORT   /calendars/{calID}/  → calendar-query/multiget   │
│    GET      /calendars/{calID}/{uid}.ics  → event iCal       │
│    PUT      /calendars/{calID}/{uid}.ics  → create/update    │
│    DELETE   /calendars/{calID}/{uid}.ics  → delete           │
├──────────────────────────────────────────────────────────────┤
│  internal/api      JSON API + auth middleware                 │
│    GET  /healthz                                             │
│    GET  /v1/capabilities                                     │
│    GET  /v1/calendars                                        │
│    GET  /v1/events                                           │
│    POST /v1/events/create|update|delete                      │
│    POST /v1/auth/login|2fa                                   │
└────────────────────────┬─────────────────────────────────────┘
                         │  domain.Event / domain.EventMutation
                         ▼
┌──────────────────────────────────────────────────────────────┐
│  internal/provider   CalendarProvider interface              │
│                                                              │
│  ┌─────────────────────────┐  ┌────────────────────────┐    │
│  │  provider/proton        │  │  provider/ics          │    │
│  │  ProtonProvider         │  │  ICSProvider           │    │
│  │  (read + write)         │  │  (read-only)           │    │
│  └────────────┬────────────┘  └────────────────────────┘    │
└───────────────┼──────────────────────────────────────────────┘
                │
        ┌───────┴────────┐
        ▼                ▼
┌───────────────┐  ┌─────────────────────────────────────────┐
│  internal/    │  │  internal/crypto                        │
│  protonapi    │  │    decrypt.go   — PGP event decryption  │
│               │  │    encrypt.go   — PGP event encryption  │
│  client.go    │  │    vcalendar.go — iCal parser            │
│  calendar.go  │  │    vcalendar_encode.go — iCal encoder   │
│  write.go     │  └─────────────────────────────────────────┘
│  auth.go      │
└───────┬───────┘
        │  HTTP  go-proton-api (read) + write.go (direct HTTP)
        ▼
  Proton Calendar API
  api.proton.me/calendar/v1/…
```

## Components

### `cmd/proton-calendar-bridge`
Entry point. Loads config, wires the provider, starts the HTTP server (TCP + optional
Unix socket), and optionally shows a system-tray icon.

### `internal/config`
Environment-driven configuration with validation. All tunables are prefixed `PCB_`.

### `internal/api`
HTTP server that routes both the JSON API and the CalDAV handler. Applies bearer-token
authentication middleware to all routes. The CalDAV subtree is mounted at `/caldav/`
and stripped before being passed to `internal/caldav`.

### `internal/caldav`
Minimal RFC 4791 CalDAV handler. Translates CalDAV HTTP verbs into calls on the
`CalendarProvider` interface. XML responses are hand-written to avoid a heavyweight
WebDAV dependency. Supports `PROPFIND`, `REPORT` (calendar-query and calendar-multiget),
`GET`, `PUT`, and `DELETE`.

### `internal/provider`
Defines the `CalendarProvider` interface and ships two implementations:

- **ProtonProvider** (`provider/proton.go`): authenticates with Proton, decrypts events
  on read, encrypts events on write, and maps between `domain.Event` / `domain.EventMutation`
  and the Proton API wire format.
- **ICSProvider** (`provider/ics.go`): fetches a remote `.ics` URL and serves it
  read-only. Write operations return `ErrNotSupported`.

### `internal/protonapi`
Wraps the Proton API. Read operations use `go-proton-api` (the official Proton Go
library). Write operations (`CreateCalendarEvent`, `UpdateCalendarEvent`,
`DeleteCalendarEvent`) are implemented as direct authenticated HTTP calls in `write.go`
because the upstream library does not expose write endpoints.

### `internal/crypto`
PGP encryption and decryption using `gopenpgp/v2` (Proton's own PGP library).

- **decrypt.go** — decrypts `SharedEventContent` (split-key AES) and `PersonalEventContent`
  (armored PGP), verifies detached signatures.
- **encrypt.go** — generates a fresh AES-256 session key, encrypts it for the calendar
  key ring (`SharedKeyPacket`), encrypts the VCALENDAR payload with the session key,
  and signs with the address private key.
- **vcalendar.go** — parses VCALENDAR/VEVENT text into a `ParsedEvent`.
- **vcalendar_encode.go** — encodes a `domain.EventMutation` into VCALENDAR text for
  submission to the Proton API.

### `internal/auth`
Key-ring management. `KeyringManager` fetches address keys from Proton and unlocks
them with the key password (`PCB_KEY_PASSWORD`). Calendar key rings are derived from
the calendar passphrase, which is itself encrypted to the address key ring.

### `internal/security`
Provides `BearerAuth` middleware. Accepts both `Authorization: Bearer <token>` and
HTTP Basic auth (password field = token) to support GNOME Online Accounts, which
sends Basic credentials to CalDAV servers.

### `internal/tray`
System-tray integration, gated behind the `systray` build tag. A no-op factory is
compiled by default so the binary has no GUI dependency.

### `internal/domain`
Shared data types (`Event`, `EventMutation`, `Calendar`) used across the provider,
API, and CalDAV layers.

## Data flow — write path

```
CalDAV PUT /calendars/{calID}/{uid}.ics
        │
        │  parse iCal body → domain.EventMutation
        ▼
ProtonProvider.CreateEvent / UpdateEvent
        │
        ├─ EncodeSharedVCalendar(mutation)   → sharedVCal string
        ├─ EncodePersonalVCalendar(reminders) → personalVCal string
        │
        ├─ EncryptSharedEvent(sharedVCal, calKR, addrKR)
        │     GenerateSessionKey (AES-256)
        │     calKR.EncryptSessionKey(sk)  → SharedKeyPacket (base64)
        │     sk.Encrypt(sharedVCal)       → DataPacket (base64)
        │     addrKR.SignDetached(shared)  → Signature (armored PGP)
        │
        └─ EncryptPersonalEvent(personalVCal, addrKR)
              addrKR.Encrypt(personal, addrKR) → armored PGP
              addrKR.SignDetached(personal)     → Signature (armored PGP)
        │
        ▼
  POST /calendar/v1/{calID}/events
  (or PUT … /events/{eventID} for updates)
```

## Local communication model

- The bridge binds to `127.0.0.1` (and optionally a `0600` Unix socket) by default.
- A separate bridge-local bearer token authenticates local clients. This is distinct
  from the Proton session credentials.
- GNOME Online Accounts sends HTTP Basic auth; the bridge accepts the Basic password
  as equivalent to the bearer token.

## Trust boundaries

```
Local CalDAV/JSON client
  → bridge (bearer token)
    → ProtonProvider
      → Proton API (session token from login)
```

The bridge never exposes Proton session tokens to local clients, and never persists
the key password or calendar keys beyond process memory.

## Reusability for other Proton services

The following packages are not Calendar-specific and can be adapted for other Proton
bridges with minimal changes:

| Package | Reusable for |
|---|---|
| `internal/protonapi` (auth + client) | Any Proton service (Mail, Drive, Contacts) |
| `internal/protonapi/write.go` (`writeClient`) | Any Proton API write endpoint |
| `internal/crypto/encrypt.go` + `decrypt.go` | Proton Drive (file blocks), Contacts (vCard) |
| `internal/security` | Any HTTP bridge requiring bearer auth |
