# Proton Mail Bridge + OpenClaw Integration Brief

Date: 2026-02-12  
Scope: communication model research for `proton-calendar-bridge` local integration.

## Executive summary

- Proton Mail Bridge is a **local mail bridge** (IMAP/SMTP on loopback) and is **not a calendar API surface**.
- Its source and docs confirm a hardened local model: loopback listeners, TLS on local channels, keychain-backed credential handling, and local gRPC control channel with token + TLS.
- For `proton-calendar-bridge`, the recommended model is to **keep an independent local HTTP/Unix-socket bridge contract** (already implemented here), rather than coupling directly to Proton Mail Bridge internals.
- For OpenClaw compatibility, use the existing plugin/tool pattern: explicit tool schemas, local transport, optional write tools, and gateway status methods.

---

## 1) Proton Mail Bridge communication model (official docs/source findings)

### Process and local protocol surfaces

From Proton’s official sources:

- Bridge starts local **IMAP/SMTP** servers for email clients.
- Local server bind target is loopback (`127.0.0.1`) in source (`constants.Host = "127.0.0.1"`, IMAP/SMTP listeners bind to `constants.Host:port`).
- Proton’s support/security docs describe the local channel as loopback-only and additionally protected with STARTTLS/SSL using a self-signed cert.

Operationally, this means Bridge exposes:

1. **Mail client interface:** IMAP/SMTP on localhost (configurable ports, default historically 1143/1025 in tooling/examples).
2. **Internal frontend control interface:** local gRPC service:
   - **macOS/Linux:** Unix domain socket (temp path like `bridge####`)
   - **Windows:** localhost TCP with OS-assigned port
   - gRPC is protected with server TLS + per-instance token metadata (`server-token`) persisted in local config JSON.

### Auth/session boundaries

Documented model:

- User authenticates to Proton via SRP; password does not leave machine.
- Access token is short-lived in memory; refresh token stored in OS credential manager.
- Mailbox password hash/salt and Bridge client password are stored in OS keychain/credential store.
- PGP private keys are unlocked in memory; not persisted in plaintext.

Code/docs confirm keychain backends:

- macOS Keychain
- Windows Credential Manager
- Linux secret-service / gnome-keyring / pass

### IPC exposure and risk notes

- IMAP/SMTP listeners are local loopback only by default.
- gRPC control channel is local-only (Unix socket on Unix-like systems, localhost TCP on Windows) with TLS + token guard.
- Proton explicitly assumes users do not expose these local ports externally.

**Important architectural implication:** Proton Mail Bridge internals are optimized for mail flows and GUI control; they are not a stable, documented calendar integration contract.

---

## 2) OpenClaw calendar/plugin interface expectations (for local bridge integration)

Based on OpenClaw plugin patterns in workspace (`tmp/openclaw` + `openclaw-plugin-reddit`):

### Plugin contract shape

OpenClaw plugin API expects registration of:

- `registerTool(tool, opts?)`
- `registerService({start, stop})`
- `registerGatewayMethod(method, handler)`
- optional CLI and hooks

Tool design pattern seen in production plugin:

- Explicit JSON-schema-like `parameters`
- Deterministic text `content` result payload
- Optional `details` + `isError`
- Strong input validation before bridge call
- Read/write separation with policy gates and optional write tool registration

### Current calendar expectation in this workspace

- Current human-run policy (`CALENDAR_AUTOMATION.md`) says OpenClaw currently uses browser automation for Proton Calendar.
- `proton-calendar-bridge` already defines a local bridge API intended to replace brittle UI automation for read/create/modify/delete flows where supported.

### Bridge API mapping (already present in project)

`proton-calendar-bridge` exposes:

- `GET /healthz`
- `GET /v1/capabilities`
- `GET /v1/calendars`
- `GET /v1/events?calendar_id=&from=&to=`
- `POST /v1/events/create`
- `POST /v1/events/update`
- `POST /v1/events/delete`

This shape aligns well with OpenClaw tool decomposition and capability-driven behavior.

---

## 3) Recommended communication model for Proton Calendar Bridge

### Recommendation

Adopt a **two-boundary local model**:

1. **OpenClaw plugin/tool layer**
   - Strict schema validation + policy/rate controls
   - Calls only local bridge endpoint/socket
2. **`proton-calendar-bridge` service layer**
   - Local-only HTTP on `127.0.0.1` and/or Unix socket (`0600`)
   - Bearer token required by default
   - Provider abstraction (`CalendarProvider`), with explicit `ErrNotSupported` → HTTP 501

### Why not direct Proton Mail Bridge coupling?

- Proton Mail Bridge is mail-centric (IMAP/SMTP + internal gRPC for its frontend lifecycle), not a calendar contract.
- Tight coupling to internal/private IPC increases break risk across Bridge releases.
- License boundary: Proton Bridge repo is GPL-3; keep this project MIT-compatible by avoiding code reuse/linking and relying on independent implementation + public behavior/docs.

### Suggested OpenClaw tool set for calendar plugin

- `calendar_list_calendars`
- `calendar_list_events`
- `calendar_create_event` (optional if provider writable)
- `calendar_update_event` (optional)
- `calendar_delete_event` (optional)
- `calendar_bridge_status` (gateway method/tool for health + capability snapshot)

Use `/v1/capabilities` to dynamically disable unsupported writes.

---

## 4) Security implications

### Positive controls to keep

- Loopback/Unix-socket local-only transport
- Bearer auth on all non-health endpoints
- Principle of least privilege for plugin subprocess/env
- No plaintext secrets in repo/config committed to VCS
- Structured audit logging for write paths

### Threats to explicitly handle

- Local malware/process snooping on same host
- Token leakage in logs/shell history
- Accidental external exposure via misconfigured bind address
- Unsafe fallback to unauthenticated mode in production

### Concrete hardening checklist

- Default bind stays `127.0.0.1`.
- Prefer Unix socket on macOS/Linux with `0600`.
- Keep `PCB_REQUIRE_TOKEN=true` by default.
- Reject non-loopback TCP binds unless explicit override + warning.
- Support token rotation without restart (future).
- Redact `Authorization` headers in logs.

---

## 5) Cross-platform constraints (macOS/Linux/Windows)

- **macOS:** Unix socket available; OS keychain available; tray support via build tags.
- **Linux:** Unix socket available; keychain backend availability varies (secret-service/gnome-keyring/pass).
- **Windows:** No standard Unix socket parity in this stack; use loopback TCP + token auth; ensure firewall profile keeps local-only semantics.

For OpenClaw plugin distribution, keep transport abstraction simple:

- Prefer Unix socket when configured and supported.
- Fallback to `http://127.0.0.1:<port>`.
- Keep identical JSON API contract across OSes.

---

## Compatibility mapping (OpenClaw ↔ bridge)

- OpenClaw read tool → `GET /v1/events`
- OpenClaw create tool → `POST /v1/events/create`
- OpenClaw modify tool → `POST /v1/events/update`
- OpenClaw delete tool → `POST /v1/events/delete`
- OpenClaw startup parity/status check → `GET /healthz` + `GET /v1/capabilities`
- Unsupported provider features → map to tool error with clear `not implemented` guidance

---

## Sources (official/high-confidence)

1. Proton Mail Bridge README (official source mirror): local IMAP/SMTP startup, keychain requirements, local gRPC config artifacts.  
   https://github.com/ProtonMail/proton-bridge
2. Proton support: Bridge settings (ports, STARTTLS/SSL mode, export TLS cert/key, local encrypted cache).  
   https://proton.me/support/comprehensive-guide-to-bridge-settings
3. Proton support: loopback-only explanation and self-signed cert behavior for local client connection.  
   https://proton.me/support/bridge-ssl-connection-issue
4. Proton blog: Bridge security model (SRP flow, token/key handling, localhost assumptions, TLS pinning).  
   https://proton.me/blog/bridge-security-model
5. Proton Bridge source references inspected locally (communication internals):
   - `internal/constants/constants.go`
   - `internal/services/imapsmtpserver/listener.go`
   - `internal/frontend/grpc/service.go`
6. OpenClaw plugin references inspected locally:
   - `tmp/openclaw/src/plugins/types.ts`
   - `openclaw-plugin-reddit/src/openclaw-api.ts`
   - `openclaw-plugin-reddit/src/index.ts`
   - `proton-calendar-bridge/internal/api/server.go`
   - `docs/openclaw-integration.md`

---

## MIT-compatibility note

This brief contains only architecture analysis and behavioral observations from public docs/source; no copied proprietary secrets or embedded GPL code. Recommended implementation strategy keeps this repository’s code independent.