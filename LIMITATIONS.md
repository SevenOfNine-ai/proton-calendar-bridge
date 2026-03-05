# Known Limitations

## 1. Write Operations Require Correct Encryption Key Access

**Status**: Implemented — encryption layer complete.

**Detail**: Proton Calendar uses end-to-end encryption. To write events, the bridge must:
1. Unlock the address key ring (requires the key password passed via `PCB_KEY_PASSWORD`).
2. Unlock the calendar key ring (derived from the calendar passphrase + address keys).
3. Generate a random AES-256 session key, encrypt it for the calendar key ring.
4. Encrypt the VCALENDAR payload with the session key.
5. Sign the payload with the address private key.

If the `PCB_KEY_PASSWORD` environment variable is not set, or if the address keys are
locked with a different password than what was supplied, write operations will fail with:
```
get address key ring: unlock address keys: ...
```

**Workaround**: Set `PCB_KEY_PASSWORD` to the Proton account's key password (this is
usually the same as the login password unless you have a separate mailbox password).

---

## 2. go-proton-api Write Endpoints Not Upstream

**Status**: Worked around — direct HTTP calls implemented in `internal/protonapi/write.go`.

**Detail**: The upstream Go library (`github.com/ProtonMail/go-proton-api v0.4.0`) does
**not** expose `CreateCalendarEvent`, `UpdateCalendarEvent`, or `DeleteCalendarEvent`
methods. The Proton backend API does support these (they are used by the official web
client), but the library only wraps read operations.

**Solution implemented**: The bridge makes direct authenticated HTTP requests using the
session tokens already stored in `protonapi.Client.auth`:
- `POST /calendar/v1/{calendarID}/events`
- `PUT  /calendar/v1/{calendarID}/events/{eventID}`
- `DELETE /calendar/v1/{calendarID}/events/{eventID}`

**Risk**: Proton may change the event create/update request schema in a future API
version. If writes break, check for changes to the `CreateCalendarEventReq` struct in
`internal/protonapi/types.go`.

---

## 3. Shared Calendar Write Support is Uncertain

**Status**: Partially implemented — the bridge checks `ReadOnly` on each calendar.

**Detail**: For calendars shared with you by another Proton user, the calendar passphrase
is encrypted to your address key, but you may not have the private calendar key needed
to generate new `SharedKeyPackets`. The `CalendarPermissions` bitmask is checked, and
write operations are attempted regardless; however, Proton may return a permission error
for calendars where you are not the owner.

**Workaround**: Check the `read_only` field in `GET /v1/calendars` before attempting
writes to shared calendars.

---

## 4. Recurring Event Update Scope

**Status**: Not implemented — `UpdateEvent` always replaces the full event.

**Detail**: CalDAV allows updating a single occurrence of a recurring event (THISANDFUTURE
or THIS semantics). The current implementation always issues a full `PUT` to the Proton
API, which replaces the entire series. Proton's API may support partial-series updates
via separate `EventException` endpoints, but this is not yet implemented.

---

## 5. Attendee Write Support

**Status**: Partial — attendees are included in the VCALENDAR payload, but no
`AttendeesEventContent` encryption is performed.

**Detail**: Proton Calendar uses per-attendee encrypted content (`AttendeesEventContent`)
for invitations. The bridge currently writes attendees as plain ATTENDEE lines in the
`SharedEventContent`, which is visible to all calendar key holders. Proton may not send
invitation emails to external attendees when using this bridge's write path.

---

## 6. CalDAV Basic Authentication (GNOME Online Accounts)

**Status**: Implemented — bridge advertises `WWW-Authenticate: Basic` for CalDAV paths.

**Detail**: GNOME Online Accounts sends HTTP Basic credentials when adding a CalDAV
account. The bridge's `PCB_BEARER_TOKEN` is used as **both** the Basic auth password
**and** the `Authorization: Bearer` token.

To configure in GNOME Online Accounts:
1. Choose "Other CalDAV account" (or use GOA "GNOME Calendar" option).
2. Set the **Server URL** to: `http://127.0.0.1:9842/caldav/`
3. Set **Username** to any value (e.g. `bridge`).
4. Set **Password** to the value of `PCB_BEARER_TOKEN`.

**Note**: The bridge listens on `127.0.0.1` by default. If you change `PCB_BIND_ADDRESS`
to a non-loopback address, ensure the bearer token is strong and the port is firewalled.

---

## 7. No Real-Time Push (No CalDAV `PUSH` / WebSockets)

**Status**: Not implemented.

**Detail**: CalDAV supports server-sent push notifications (RFC 8144). The bridge does
not implement push; GNOME Calendar polls for changes. The default polling interval is
controlled by evolution-data-server (typically every 10–60 minutes).

---

## 8. Timezone Handling

**Status**: Limited — all timestamps normalised to UTC.

**Detail**: The bridge converts all event times to UTC when reading and writing. Events
with timezone-aware DTSTART/DTEND (e.g. `DTSTART;TZID=America/New_York:...`) are
converted to UTC on ingest. Events written back always use UTC. If your Proton Calendar
events used named timezones, they will appear in UTC after a round-trip through the bridge.

---

## 9. Running as a System Daemon

**Status**: Documented — no code changes required.

The bridge can be run as a `systemd` user service. Example unit file:

```ini
# ~/.config/systemd/user/proton-calendar-bridge.service
[Unit]
Description=Proton Calendar Bridge (CalDAV)
After=network.target

[Service]
ExecStart=/usr/local/bin/proton-calendar-bridge
Restart=on-failure
RestartSec=5s
Environment=PCB_PROVIDER=proton
Environment=PCB_BIND_ADDRESS=127.0.0.1:9842
Environment=PCB_BEARER_TOKEN=your-secret-token
Environment=PCB_KEY_PASSWORD=your-key-password
# UID/AccessToken/RefreshToken set after first login via the auth endpoint

[Install]
WantedBy=default.target
```

Enable with: `systemctl --user enable --now proton-calendar-bridge`
