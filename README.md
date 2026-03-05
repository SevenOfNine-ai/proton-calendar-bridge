# proton-calendar-bridge

A local CalDAV bridge that exposes Proton Calendar to any RFC 4791-compliant client
(GNOME Calendar, Thunderbird, Apple Calendar, etc.). Events are read from and written
back to Proton using end-to-end encryption — the bridge never stores your keys or
calendar data on disk.

## Status

| Feature | Status |
|---|---|
| Read events (all calendars) | ✅ Implemented |
| Write events (create / update / delete) | ✅ Implemented |
| End-to-end encryption on write | ✅ Implemented |
| CalDAV interface (RFC 4791) | ✅ Implemented |
| GNOME Online Accounts / GNOME Calendar | ✅ Supported |
| Daemon mode (systemd user service) | ✅ Documented |
| ICS read-only provider | ✅ Implemented |
| Cross-platform builds (macOS / Linux / Windows) | ✅ CI + release |
| Recurring event partial-series update | ⚠️ Not implemented (full replace only) |
| Per-attendee encrypted invites | ⚠️ Partial (see LIMITATIONS.md) |
| CalDAV push notifications | ❌ Not implemented (polling only) |

## Quick start

### 1. Build

```bash
go build -o proton-calendar-bridge ./cmd/proton-calendar-bridge
```

Or with a system-tray icon (Linux/macOS):

```bash
go build -tags systray -o proton-calendar-bridge ./cmd/proton-calendar-bridge
```

### 2. Configure

All configuration is via environment variables:

| Variable | Required | Default | Description |
|---|---|---|---|
| `PCB_PROVIDER` | No | `ics` | `proton` or `ics` |
| `PCB_BEARER_TOKEN` | Yes* | — | Token clients send to authenticate to the bridge. Also used as the CalDAV Basic auth password. *Not required when `PCB_REQUIRE_TOKEN=false`. |
| `PCB_REQUIRE_TOKEN` | No | `true` | Set `false` to disable local auth (loopback-only, not recommended) |
| `PCB_BIND_ADDRESS` | No | `127.0.0.1:9842` | TCP address to listen on |
| `PCB_UNIX_SOCKET` | No | — | Path to a Unix domain socket (chmod 0600) |
| `PCB_LOG_LEVEL` | No | `info` | `debug`, `info`, `warn`, or `error` |
| `PCB_ENABLE_TRAY` | No | `false` | Show system-tray icon (requires `systray` build tag) |
| `PCB_KEY_PASSWORD` | Proton write | — | Your Proton key password (usually your login password). Required to encrypt events on write. |
| `PCB_ICS_URL` | ICS provider | — | Remote `.ics` URL (when `PCB_PROVIDER=ics`) |

### 3. Authenticate with Proton

Start the bridge, then POST your Proton credentials:

```bash
export PCB_PROVIDER=proton
export PCB_BEARER_TOKEN=my-local-secret
export PCB_KEY_PASSWORD=my-proton-password
./proton-calendar-bridge &

# Login
curl -s -X POST http://127.0.0.1:9842/v1/auth/login \
  -H "Authorization: Bearer $PCB_BEARER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"username":"you@proton.me","password":"your-proton-password"}'

# If 2FA is enabled on your account:
curl -s -X POST http://127.0.0.1:9842/v1/auth/2fa \
  -H "Authorization: Bearer $PCB_BEARER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"totp":"123456"}'
```

### 4. Connect GNOME Calendar

In **GNOME Online Accounts** → **Other CalDAV account**:

| Field | Value |
|---|---|
| Server URL | `http://127.0.0.1:9842/caldav/` |
| Username | `bridge` (any value) |
| Password | value of `PCB_BEARER_TOKEN` |

GNOME Calendar will discover your calendars automatically via PROPFIND.

### 5. Verify

```bash
# Health check
curl http://127.0.0.1:9842/healthz

# Capabilities (write_supported should be true for the proton provider)
curl -H "Authorization: Bearer $PCB_BEARER_TOKEN" \
  http://127.0.0.1:9842/v1/capabilities

# List calendars
curl -H "Authorization: Bearer $PCB_BEARER_TOKEN" \
  http://127.0.0.1:9842/v1/calendars

# CalDAV principal discovery
curl -s -X PROPFIND \
  -H "Authorization: Bearer $PCB_BEARER_TOKEN" \
  -H "Depth: 0" \
  http://127.0.0.1:9842/caldav/
```

## Run as a background service (systemd)

Create `~/.config/systemd/user/proton-calendar-bridge.service`:

```ini
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

[Install]
WantedBy=default.target
```

```bash
systemctl --user enable --now proton-calendar-bridge
```

The bridge starts automatically at login and restarts on failure. Authenticate once
via `/v1/auth/login` after the service starts; the session is kept in memory for the
lifetime of the process.

## JSON API

The bridge also exposes a JSON API for local automation tools (e.g. OpenClaw):

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Liveness check |
| `GET` | `/v1/capabilities` | Feature flags for the active provider |
| `GET` | `/v1/calendars` | List calendars |
| `GET` | `/v1/events?calendar_id=&from=&to=` | List events (RFC 3339 timestamps) |
| `POST` | `/v1/events/create` | Create an event |
| `POST` | `/v1/events/update` | Update an event |
| `POST` | `/v1/events/delete` | Delete an event |
| `POST` | `/v1/auth/login` | Authenticate with Proton |
| `POST` | `/v1/auth/2fa` | Submit TOTP code |

## Architecture overview

```
CalDAV client (GNOME Calendar, Thunderbird, …)
        │  HTTP  PUT / DELETE / PROPFIND / REPORT
        ▼
┌─────────────────────────────────────────────────────┐
│  internal/caldav  (RFC 4791 handler)                │
│  internal/api     (JSON API + auth middleware)      │
└────────────────────┬────────────────────────────────┘
                     │  domain.EventMutation / domain.Event
                     ▼
┌─────────────────────────────────────────────────────┐
│  internal/provider/proton  (ProtonProvider)         │
│    ├─ internal/crypto/encrypt.go  (PGP encrypt)     │
│    ├─ internal/crypto/decrypt.go  (PGP decrypt)     │
│    └─ internal/crypto/vcalendar*  (iCal encode/decode)│
└────────────────────┬────────────────────────────────┘
                     │  HTTP (go-proton-api + write.go)
                     ▼
              Proton Calendar API
              api.proton.me/calendar/v1/…
```

## Limitations

See [LIMITATIONS.md](LIMITATIONS.md) for known constraints, including partial support
for recurring event updates, attendee encryption, and timezone handling.

## Docs

- [ARCHITECTURE.md](ARCHITECTURE.md) — component map and trust model
- [SPEC.md](SPEC.md) — API and CalDAV specification
- [TESTING.md](TESTING.md) — test strategy and commands
- [DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md) — audit, research, and implementation plan
- [LIMITATIONS.md](LIMITATIONS.md) — known limitations and workarounds
- [docs/implementation-notes.md](docs/implementation-notes.md) — low-level notes

## Reusability

The authentication, session management, HTTP write client, and PGP crypto layers are
generic to the Proton platform — not specific to Calendar. The same patterns apply to
a Proton Drive (WebDAV) or Proton Contacts (CardDAV) bridge. See
[docs/implementation-notes.md](docs/implementation-notes.md) for details.

## License

MIT
