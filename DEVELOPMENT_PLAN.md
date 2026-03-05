# Development Plan: Full Read-Write CalDAV Bridge for Proton Calendar

## 1. Codebase Audit Summary

### Current Architecture

The bridge is a Go application exposing a custom JSON HTTP API. It supports two providers:
- **ICS provider**: Fetches a remote `.ics` URL and serves it read-only.
- **Proton provider**: Authenticates with the Proton API, decrypts encrypted calendar events, and serves them read-only.

### Read Logic (Proton Provider)

The read path is fully implemented across these layers:

| Layer | File | Responsibility |
|-------|------|----------------|
| HTTP API | `internal/api/server.go` | Routes `GET /v1/events` → provider |
| Provider | `internal/provider/proton.go` | Orchestrates key unwrapping + decryption |
| Proton API client | `internal/protonapi/calendar.go` | Wraps `go-proton-api` read calls |
| Decryption | `internal/crypto/decrypt.go` | Decrypts PGP-encrypted event parts |
| VCalendar parsing | `internal/crypto/vcalendar.go` | Parses iCal text into `ParsedEvent` |
| Auth & keyrings | `internal/auth/keyring.go` | Unlocks address & calendar key rings |

Read flow per event:
1. Fetch paginated events from `GET /calendar/v1/{calID}/events`
2. Obtain calendar passphrase → decrypt with address key ring
3. Unlock calendar key ring with passphrase
4. `SharedEventContent`: decrypt with calendar key ring (split key packet)
5. `PersonalEventContent`: decrypt with address key ring (armored PGP)
6. Parse VCALENDAR text → `domain.Event`

### Write Blockers Identified

1. **No write methods in `go-proton-api` v0.4.0**: The upstream library has no `CreateCalendarEvent`, `UpdateCalendarEvent`, or `DeleteCalendarEvent` methods. We must implement direct HTTP calls.
2. **`CalendarAPI` interface is read-only**: The `protonapi.Client` wraps the upstream library via an interface that only covers read operations.
3. **No VCALENDAR encoder**: Only a decoder (`vcalendar.go`) exists; we need a writer.
4. **No event encryption**: Only decryption logic exists in `crypto/decrypt.go`; we need to encrypt VCALENDAR strings to submit to Proton.
5. **Provider stubs return `NotSupportedError`**: `CreateEvent`, `UpdateEvent`, `DeleteEvent` in `provider/proton.go` are not implemented.
6. **No CalDAV layer**: The bridge speaks JSON, not CalDAV. GNOME Calendar uses CalDAV (RFC 4791). A translation layer is required.

---

## 2. External Research Findings (2025/2026)

### go-proton-api (v0.4.0 — current)
- Calendar write endpoints (`POST`/`PUT`/`DELETE /calendar/v1/{id}/events`) exist in the Proton backend but are **not exposed** in the Go library.
- Solution: Implement direct HTTP calls using session tokens already stored in `protonapi.Client.auth`.

### hydroxide project
- hydroxide implements Proton Mail ↔ CardDAV/IMAP bridges, but **does not include calendar write support**.
- Its approach to direct API calls (using resty + Bearer auth) confirms our strategy.

### rclone Proton backend
- rclone's Proton Drive backend handles file uploads but **has no calendar component**.

### Proton Calendar API (unofficial)
- Proton's web client (TypeScript) reveals the create/update event request structure.
- Key insight: `SharedEventContent` uses a **split-key encryption** model:
  - A random AES session key is generated.
  - The session key is encrypted for the calendar key ring → `SharedKeyPacket` (base64).
  - The VCALENDAR plaintext is encrypted with the session key → data packet (base64), stored in `SharedEventContent[0].Data`.
  - A detached PGP signature of the plaintext is created with the address key → stored in `SharedEventContent[0].Signature`.
- `PersonalEventContent` (alarms) is encrypted + signed with the address key ring (armored PGP).

---

## 3. Implementation Plan

### Phase 1: VCALENDAR Encoder (`internal/crypto/vcalendar_encode.go`)
Build the inverse of `ParseVCalendar`:
- `EncodeSharedVCalendar(m EventMutation) string` — formats DTSTART, DTEND, SUMMARY, DESCRIPTION, LOCATION, RRULE, ATTENDEEs
- `EncodePersonalVCalendar(reminders []string) string` — formats VALARM blocks

### Phase 2: Event Encryption (`internal/crypto/encrypt.go`)
Mirror of `decrypt.go`:
- `EncryptSharedEvent(vcal string, calKR, addrKR *gopenpgp.KeyRing) (keyPacket, dataPacket []byte, signature string, err error)`
  - Uses `calKR.EncryptSessionKey(sk)` → key packet
  - Uses `sk.Encrypt(plaintext)` → data packet
  - Uses `addrKR.SignDetached(plaintext)` → armored signature
- `EncryptPersonalEvent(vcal string, addrKR *gopenpgp.KeyRing) (pgpArmored, signature string, err error)`

### Phase 3: Proton API Write Methods (`internal/protonapi/write.go`)
Direct HTTP calls bypassing the upstream library:
- `CreateCalendarEvent(ctx, calID, req) (CalendarEvent, error)` → `POST /calendar/v1/{calID}/events`
- `UpdateCalendarEvent(ctx, calID, eventID, req) (CalendarEvent, error)` → `PUT /calendar/v1/{calID}/events/{eventID}`
- `DeleteCalendarEvent(ctx, calID, eventID) error` → `DELETE /calendar/v1/{calID}/events/{eventID}`
- Auth headers: `Authorization: Bearer {token}`, `x-pm-uid: {uid}`, `x-pm-appversion: {appver}`

### Phase 4: Extend Provider Interface (`internal/provider/proton.go`)
- Update `protonCalendarClient` interface to include write methods
- Implement `CreateEvent`: build VCALENDAR → encrypt → POST
- Implement `UpdateEvent`: build VCALENDAR → encrypt → PUT
- Implement `DeleteEvent`: DELETE
- Update `Capabilities()` to return `WriteSupported: true, ReadOnly: false`

### Phase 5: CalDAV Layer (`internal/caldav/handler.go`)
Minimal CalDAV (RFC 4791) server for GNOME Calendar compatibility:
- `OPTIONS /caldav/` → `DAV: 1, 3, calendar-access`
- `PROPFIND /caldav/` → principal/home-set XML
- `PROPFIND /caldav/calendars/` → calendar collection listing
- `PROPFIND /caldav/calendars/{calID}/` → calendar properties (display-name, ctag, supported-calendar-component-set)
- `REPORT /caldav/calendars/{calID}/` → calendar-query / calendar-multiget
- `GET /caldav/calendars/{calID}/{uid}.ics` → single event iCal
- `PUT /caldav/calendars/{calID}/{uid}.ics` → create/update event (parse incoming iCal → `EventMutation` → `CreateEvent`/`UpdateEvent`)
- `DELETE /caldav/calendars/{calID}/{uid}.ics` → delete event

CalDAV URL for GNOME Online Accounts:
```
http://127.0.0.1:9842/caldav/
```

### Phase 6: Daemon Mode (`cmd/proton-calendar-bridge/main.go`)
- Already works as a foreground process.
- Document systemd unit file example in README.
- Add `--pid-file` flag for daemon monitoring (optional, lightweight).

---

## 4. Encryption Architecture for Write Operations

```
domain.EventMutation
        │
        ▼
EncodeSharedVCalendar()          EncodePersonalVCalendar()
        │                                   │
        ▼                                   ▼
  sharedPlain (string)             personalPlain (string)
        │                                   │
        ├─ GenerateSessionKey()             │
        │         │                         │
        │   calKR.EncryptSessionKey(sk)     │
        │   → SharedKeyPacket (base64)      │
        │         │                         │
        │   sk.Encrypt(sharedPlain)         │
        │   → DataPacket (base64)           │
        │         │                         │
        │   addrKR.SignDetached(shared)     addrKR.Encrypt(personal, addrKR)
        │   → Signature (armored)           → PersonalData (armored PGP)
        │                                   addrKR.SignDetached(personal)
        │                                   → PersonalSig (armored)
        ▼                                   ▼
  CreateCalendarEventReq { SharedKeyPacket, SharedEventContent, PersonalEventContent, ... }
        │
        ▼
  POST /calendar/v1/{calID}/events
```

---

## 5. Risk Assessment

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Proton API changes event format | Medium | Use `LIMITATIONS.md` to track; fail gracefully |
| Encryption key mismatch on write | Low | Mirror existing decrypt test patterns |
| CalDAV PROPFIND XML compatibility | Medium | Tested against evolution-data-server (GNOME) |
| Rate limiting on write operations | Low | Respect `Retry-After` header |
| go-proton-api auth token expiry mid-write | Low | Existing `StatusDisconnected` handler covers this |

---

## 6. Files Created/Modified

| File | Action | Purpose |
|------|--------|---------|
| `DEVELOPMENT_PLAN.md` | **CREATE** | This document |
| `LIMITATIONS.md` | **CREATE** | Known limitations |
| `internal/crypto/vcalendar_encode.go` | **CREATE** | VCALENDAR encoder |
| `internal/crypto/encrypt.go` | **CREATE** | PGP event encryption |
| `internal/protonapi/types.go` | **MODIFY** | Add write request types |
| `internal/protonapi/write.go` | **CREATE** | Direct HTTP write calls |
| `internal/protonapi/client.go` | **MODIFY** | Add write methods to interface |
| `internal/provider/proton.go` | **MODIFY** | Implement write methods |
| `internal/caldav/handler.go` | **CREATE** | CalDAV HTTP handler |
| `internal/api/server.go` | **MODIFY** | Mount CalDAV handler |

---

## 7. Success Criteria

- [x] `go build ./...` passes with no errors
- [ ] `POST /v1/events/create` returns a real `domain.Event` (not 501)
- [ ] `POST /v1/events/update` updates an event
- [ ] `POST /v1/events/delete` deletes an event
- [ ] `GET /v1/capabilities` returns `"write_supported": true` for Proton provider
- [ ] `PROPFIND http://127.0.0.1:9842/caldav/` returns valid CalDAV XML
- [ ] GNOME Online Accounts can add the bridge as a CalDAV account
- [ ] GNOME Calendar shows events from Proton Calendar
- [ ] Creating an event in GNOME Calendar syncs to Proton Calendar
