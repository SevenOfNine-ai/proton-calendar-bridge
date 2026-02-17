# Proton Calendar Bridge — Implementation Plan v2

## Current State
- Go scaffold exists with: ICS read-only provider, local HTTP API, bearer auth, capability negotiation
- Deep research report confirms: NO official Proton Calendar API exists
- Two viable unofficial paths: ferroxide/hydroxide CalDAV bridge pattern OR direct Proton API via `go-proton-api`
- `go-proton-api` (henrybear327) is **archived** as of Feb 5, 2026 — use as reference only, vendor or fork needed
- Proton Mail Bridge source is available locally at `tmp/proton-bridge-src/`

## Architecture Decision: Use Official `ProtonMail/go-proton-api`

### Why `github.com/ProtonMail/go-proton-api` (OFFICIAL)
- **Maintained by Proton themselves** — stays compatible with their API
- 199 stars, 203 commits, active `master` branch, NOT archived
- Clean Go library: Calendar types (`Calendar`, `CalendarEvent`, `CalendarKey`, `CalendarMember`, `CalendarPassphrase`)
- Full auth: SRP via `proton.Manager` → `proton.Client` with auto token refresh
- Calendar endpoints: `/calendar/v1`, events, keys, members, passphrase
- Encryption: `CalendarKeys.Unlock(passphrase)` → `gopenpgp` decryption
- Import as normal Go module: `go get github.com/ProtonMail/go-proton-api@latest`

⚠️ NOTE: `henrybear327/go-proton-api` is an ARCHIVED fork — do NOT use it.

### Why NOT ferroxide
- Full standalone bridge (IMAP/SMTP/CardDAV/CalDAV) — too heavy
- We only need Calendar
- Different auth model than our bridge

## Authentication Flow (from go-proton-api + Proton Mail Bridge patterns)

```
1. SRP Auth → POST /auth/v4 (username + SRP proof)
   → Returns: UID, AccessToken, RefreshToken, ServerProof, 2FA requirement

2. If 2FA required → POST /auth/v4/2fa (TOTP code)

3. Token refresh → POST /auth/v4/refresh (UID + RefreshToken)
   → Returns: new AccessToken, new RefreshToken

4. All API calls use: Authorization: Bearer {AccessToken}, x-pm-uid: {UID}

5. On 401 → automatic refresh; on 429 → respect Retry-After header

6. Session persistence: store UID + RefreshToken encrypted locally
   (follow Proton Mail Bridge pattern: encrypt with user-provided bridge password)
```

## Calendar Data Flow (Encrypted)

```
1. GetCalendars → list all calendars (personal + shared)
2. GetCalendarMembers → get member ID for the authenticated user
3. GetCalendarPassphrase → get encrypted passphrase for the member
4. Decrypt passphrase using user's address key ring (from gopenpgp)
5. GetCalendarKeys → get calendar encryption keys
6. Unlock calendar keys with decrypted passphrase
7. GetCalendarEvents → get encrypted event data
8. Decrypt event SharedEventContent/PersonalEventContent using calendar key ring
9. Parse decrypted ICS/VCALENDAR data → domain.Event
```

## Implementation Phases

### Phase 1: Vendor go-proton-api + Auth Provider (NEW)
**Branch:** `feat/proton-api-provider`

1. Vendor `go-proton-api` as `internal/protonapi/` (copy needed files, not full module — it's archived)
   - Needed: `manager.go`, `auth*.go`, `calendar*.go`, `client.go`, `types.go`, SRP helpers
   - Dependencies: `github.com/ProtonMail/gopenpgp/v2`, `github.com/ProtonMail/go-srp`, `github.com/go-resty/resty/v2`
2. Create `internal/auth/` package:
   - `session.go`: SRP login, 2FA, token refresh, session persistence
   - `keyring.go`: address key unlock, calendar passphrase decryption
   - `store.go`: encrypted session storage (bridge password → AES-GCM encrypted JSON file)
3. Create `internal/provider/proton.go` implementing `CalendarProvider`:
   - `ListCalendars()` → GetCalendars + GetCalendarMembers
   - `ListEvents()` → GetCalendarEvents + decrypt each event
   - `CreateEvent()` → encrypt + POST (Phase 2)
   - `UpdateEvent()` → encrypt + PUT (Phase 2)  
   - `DeleteEvent()` → DELETE (Phase 2)
4. Wire new provider into `internal/app/app.go` config selection

### Phase 2: Read Path Complete
1. Full event decryption pipeline:
   - Decrypt SharedEventContent (calendar key ring)
   - Decrypt PersonalEventContent (address key ring)
   - Parse decrypted VCALENDAR → domain.Event fields
2. Shared calendar support:
   - GetCalendarMembers → identify shared calendars
   - Permissions mapping → domain.Calendar.Permissions
3. Recurring event expansion (RRULE parsing)
4. Attendee extraction from decrypted VCALENDAR

### Phase 3: Write Path
1. Event creation: build VCALENDAR → encrypt with calendar key → POST
2. Event update: re-encrypt → PUT
3. Event deletion: DELETE
4. Capability negotiation: `write_supported: true` when Proton provider active

### Phase 4: CLI + Integration
1. `proton-calendar-bridge auth <username>` — interactive SRP login + 2FA + bridge password
2. `proton-calendar-bridge serve` — start HTTP API with Proton provider
3. OpenClaw integration contract: update `/v1/capabilities` response
4. Automated token refresh daemon (background goroutine)

## Security Constraints (CRITICAL)
- **NO plaintext credential storage** — all at rest encrypted with bridge password
- **NO login in automated tests against real Proton** — use mocked responses
- **Session file permissions:** 0600
- **Rate limiting:** respect 429 + Retry-After; default 1 req/sec ceiling
- **2FA:** must support TOTP flow
- **Bridge password:** user-chosen, used to derive AES-256 key for session encryption

## Testing Strategy
- Unit tests: mock HTTP responses for all Proton API endpoints
- Integration tests: use saved/recorded API response fixtures
- **NO real Proton login in CI** — only manual local testing with explicit flag
- Encryption round-trip tests using test keypairs from gopenpgp

## Key Dependencies
```
github.com/ProtonMail/gopenpgp/v2  — PGP encryption/decryption
github.com/ProtonMail/go-srp       — SRP authentication  
github.com/go-resty/resty/v2       — HTTP client
github.com/getlantern/systray       — existing tray dependency
```

## Files to Read for Implementation
- `go-proton-api`: calendar.go, calendar_event.go, calendar_types.go, manager.go, auth.go
- ferroxide: CalDAV handler for reference on VCALENDAR ↔ Proton event mapping
- Proton Mail Bridge source (`tmp/proton-bridge-src/`): auth patterns, credential storage
- hydroxide PR #282: CalDAV event create/update/delete patterns

## Risk Register
| Risk | Mitigation |
|------|-----------|
| Proton API breaks (undocumented) | Pin to known-working endpoint versions; integration test suite |
| Account suspension from automation | Rate limit; use dedicated Proton account; minimal footprint |
| go-proton-api archived | Vendor code; we own maintenance |
| Encryption complexity | gopenpgp handles PGP; test with known fixtures |
| 2FA complicates headless auth | Initial interactive auth; persist session; refresh handles ongoing access |
