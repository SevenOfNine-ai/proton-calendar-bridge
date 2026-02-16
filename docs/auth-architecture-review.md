# Auth Architecture Review: proton-calendar-bridge vs. Proton Ecosystem Best Practices

Date: 2026-02-16
Status: Draft
Scope: Compare current bearer-token auth in proton-calendar-bridge with production Proton ecosystem patterns (Proton Mail Bridge, go-proton-api, proton-drive-cli) and recommend a path toward a shared `proton-auth` Go package.

---

## 1. Current State: Bearer Token Auth in proton-calendar-bridge

### How auth works today

The bridge uses a simple bearer token mechanism implemented in `internal/security/auth.go`:

```go
type BearerAuth struct {
    Enabled bool
    Token   string
}

func (a BearerAuth) Authorize(r *http.Request) bool {
    if !a.Enabled {
        return true
    }
    // Extract "Bearer <token>" from Authorization header
    // Constant-time comparison via crypto/subtle
    return subtle.ConstantTimeCompare([]byte(candidate), []byte(a.Token)) == 1
}
```

Key characteristics:

- **Static token**: A single bearer token is loaded from the `PCB_BEARER_TOKEN` environment variable at startup (via `internal/config/config.go`). It never rotates during the process lifetime.
- **Constant-time comparison**: Uses `crypto/subtle.ConstantTimeCompare` to prevent timing attacks. This is correct and good practice.
- **Bypass mode**: When `PCB_REQUIRE_TOKEN=false`, auth is completely disabled. The `/healthz` endpoint is always unauthenticated.
- **Middleware pattern**: Auth is applied as an HTTP handler wrapper in `internal/api/server.go` via `wrapAuth()`, which rejects non-healthz requests that fail authorization with HTTP 401.
- **Scope**: This auth layer protects the **bridge's local API** (the interface between OpenClaw and the bridge). It does not authenticate against Proton's upstream services.

### What the bridge does NOT do today

| Concern | Status |
|---------|--------|
| Authenticate to Proton's backend (SRP-6a) | Not implemented. ICS provider uses a pre-shared URL containing embedded decryption key material. |
| Store or manage Proton session tokens | Not implemented. No session file, no token persistence. |
| Refresh expired tokens | Not applicable. The ICS URL is a static bearer secret; the bridge-local token has no expiry. |
| Credential sourcing abstraction | Not implemented. Token comes from a single env var. |
| Session file locking / multi-process safety | Not applicable. No session file exists. |
| Upstream error mapping (401/403/429) | Not implemented at the provider level. ICS fetch errors are mapped generically to HTTP 502 (Bad Gateway). |

### Trust boundaries

```
OpenClaw Gateway
    |
    | Bearer token (PCB_BEARER_TOKEN)
    v
proton-calendar-bridge (local HTTP / Unix socket)
    |
    | ICS URL (contains embedded decryption key)
    v
Proton ICS endpoint (read-only, up to 8h sync delay)
```

The bridge currently operates at a single trust boundary: local clients authenticate to the bridge with a static token. The bridge itself makes unauthenticated (or URL-secret-authenticated) HTTP GET requests to fetch ICS data. There is no Proton API session in play.

---

## 2. Proton Ecosystem Patterns

The following patterns are observed in production Proton ecosystem tools: Proton Mail Bridge (official), go-proton-api (community/semi-official), proton-drive-cli (community), and ferroxide (community CalDAV bridge). These represent the established way to authenticate against Proton services.

### 2.1 SRP-6a Authentication

Proton uses the Secure Remote Password protocol (SRP-6a) for all user authentication. This is a zero-knowledge proof protocol where the user's password never leaves the client.

**Flow (as implemented in go-proton-api and Proton Mail Bridge):**

1. Client sends username to `/auth/info` endpoint.
2. Server returns salt and SRP server ephemeral value (B).
3. Client derives SRP verifier from password + salt, computes client ephemeral (A) and proof (M1).
4. Client sends A + M1 to `/auth` endpoint.
5. Server verifies M1, returns server proof (M2) + session tokens (UID, AccessToken, RefreshToken).
6. Client verifies M2 to confirm server identity.
7. If 2FA/TOTP is required, a follow-up `/auth/2fa` call is made.

**Key properties:**
- Password never transmitted, even encrypted.
- Mutual authentication (client and server prove knowledge).
- Session tokens are the only persistent artifacts.
- The SRP implementation in Go lives in `go-proton-api` and depends on `go-srp` (Proton's open-source SRP library).

**Implications for proton-calendar-bridge:**
Any future provider that calls Proton's API directly (as opposed to using ICS links) must implement or import SRP-6a. This is non-trivial and should not be reimplemented per-project.

### 2.2 CredentialProvider Interface

Production Proton tools abstract credential sourcing behind a provider interface:

```
CredentialProvider
  +-- pass-cli provider   (reads from Proton Pass via subprocess)
  +-- git-credential       (reads from OS keychain via git credential protocol)
  +-- env provider         (reads from environment variables)
  +-- keychain provider    (direct OS keychain access; used by Proton Mail Bridge)
```

**Patterns observed:**

- **proton-git-lfs** (`cmd/adapter/passcli.go`): Resolves `pass://Vault/Item/field` references by invoking `pass-cli` as a subprocess. Supports JSON and plain-text output parsing. Credentials are resolved once at startup and held in memory.
- **proton-drive-cli** (`src/utils/git-credential.ts`): Uses the `git credential fill/approve/reject` stdin/stdout protocol to leverage OS credential managers (macOS Keychain, Windows Credential Manager, Linux secret-service).
- **Proton Mail Bridge**: Uses OS keychain directly (macOS Keychain, Windows Credential Manager, Linux secret-service/gnome-keyring/pass) to store the refresh token, mailbox password hash, and per-instance bridge password.

**Design principles:**
- Credentials never appear in CLI args or log output.
- Provider selection is configurable (env var or config flag).
- Credential buffers are zeroed on process termination.
- The provider interface is small: `Resolve(ref string) (string, error)` or `Get(username string) (Credentials, error)`.

**Implications for proton-calendar-bridge:**
The current single-env-var approach (`PCB_BEARER_TOKEN`) is adequate for the bridge-local token but insufficient for Proton API credentials. A CredentialProvider abstraction would allow the bridge to source Proton credentials from pass-cli (the existing OpenClaw pattern) without hardcoding the mechanism.

### 2.3 Session File Locking and Persistence

Production Proton tools persist session state to disk for token reuse across process restarts.

**Pattern (as seen in proton-drive-cli and proton-git-lfs):**

```
~/.config/<tool>/session.json   (mode 0600, parent dir mode 0700)
{
    "uid": "...",
    "accessToken": "...",
    "refreshToken": "...",
    "scopes": ["..."],
    "userHash": "..."
}
```

**Key behaviors:**

1. **Atomic writes**: Write to a temp file in the same directory, then `os.Rename()` to the target path. This prevents corruption from crashes during write.
2. **Permission enforcement**: Session directory is `0700`, session file is `0600`. Permissions are checked on load; if too permissive, the file is rejected.
3. **No password persistence**: The mailbox password and account password are explicitly stripped before serialization. Only tokens and session metadata are saved.
4. **User hash validation**: On load, the stored `userHash` is compared against the current user's hash. If mismatched (different Proton account), the session file is discarded and a fresh login is required.
5. **File locking for multi-process safety**: When multiple processes (e.g., concurrent git-lfs transfers) share a session file, advisory file locks (`flock` / `fcntl`) prevent simultaneous writes. A process that fails to acquire the lock backs off and re-reads the file (another process may have refreshed the token).

**Implications for proton-calendar-bridge:**
The bridge currently has no session persistence. When a Proton API provider is added, it will need atomic session file management with proper permissions. This is boilerplate that should be shared, not reimplemented.

### 2.4 Proactive Token Refresh

Proton access tokens are short-lived (typically 30 minutes, though this is server-controlled and not guaranteed). Production tools implement a refresh strategy that goes beyond "retry on 401."

**Refresh triggers:**

| Trigger | Source |
|---------|--------|
| HTTP 401 Unauthorized | Standard OAuth-style signal |
| Proton API error code 9101 (invalid access token) | Proton-specific body-level error |
| Proton API error code 10013 (refresh token expired) | Triggers full re-login |
| HTTP 429 Too Many Requests | Back off per `Retry-After` header, then retry |
| Proactive timer | Some implementations refresh before known expiry (e.g., at 80% of TTL) |

**Concurrency handling (single-flight refresh):**

When multiple goroutines/requests hit a 401 simultaneously, only one should perform the refresh. Others block and reuse the result. This is typically implemented with `sync.Once`-style logic or a mutex guarding the refresh operation:

```
1. Request A gets 401
2. Request A acquires refresh lock
3. Request B gets 401, tries to acquire lock, blocks
4. Request A refreshes tokens, updates session file, releases lock
5. Request B wakes up, sees fresh token, retries original request
6. If Request A's refresh fails: Request B falls back to re-reading session file
   (another process may have refreshed) before attempting its own refresh
```

**Retry policy:**

- After a successful refresh, retry the original request exactly once.
- If the retry also fails with 401, trigger a full re-login or surface the error.
- On 429, respect `Retry-After` and do not count it as an auth failure.

**Implications for proton-calendar-bridge:**
The bridge has no token refresh logic because it has no Proton session. When a Proton API provider is added, token refresh with single-flight deduplication and proper error classification is essential for reliability. This is the most complex piece to get right and the strongest argument for a shared package.

---

## 3. Gap Analysis

### 3.1 Gap Summary Table

| Capability | Proton Ecosystem Standard | proton-calendar-bridge Status | Gap Severity |
|------------|--------------------------|-------------------------------|-------------|
| Proton SRP-6a authentication | Required for any Proton API access | Not implemented | **Critical** (blocks Proton API provider) |
| CredentialProvider abstraction | Interface with pass-cli / keychain / env backends | Single env var only | **High** (blocks secure credential management) |
| Session persistence | Atomic file writes, 0600 perms, no password storage | No session file | **High** (blocks token reuse across restarts) |
| Token refresh (401 / error codes) | Automatic refresh with single-flight dedup | Not implemented | **High** (blocks reliable long-running operation) |
| Session file locking | Advisory locks for multi-process safety | Not applicable (single process) | **Medium** (relevant when bridge coexists with other Proton tools) |
| Proactive token refresh (TTL-based) | Implemented in some tools (refresh before expiry) | Not implemented | **Medium** (reduces latency spikes from expired tokens) |
| Rate limit handling (429 + Retry-After) | Standard in Proton client libs | Not implemented at provider level | **Medium** (needed for Proton API provider robustness) |
| Credential zeroing on shutdown | Buffer zeroing in production tools | Not implemented | **Low** (defense-in-depth; single-user local process) |
| 2FA/TOTP support | Required for many Proton accounts | Not implemented | **Critical** (many Proton accounts require 2FA) |
| Log redaction | Authorization headers, tokens, passwords redacted | Not implemented (no sensitive data logged currently) | **Low** (becomes important when Proton tokens are in play) |

### 3.2 What Is Adequate for the Current ICS Provider

The current bearer-token auth is **fit for purpose** for the ICS provider use case:

- The ICS URL is a static secret (not a session token), so there is nothing to refresh.
- The bridge-local bearer token protects the API surface from unauthorized local processes.
- Constant-time comparison prevents timing side-channels.
- Loopback binding + Unix socket with 0600 permissions provides transport-level isolation.

The gaps above become relevant only when a **Proton API provider** is introduced (one that calls Proton's internal calendar endpoints rather than consuming an ICS link).

### 3.3 Architectural Debt If Gaps Are Not Addressed

If a Proton API provider were added without addressing these gaps:

1. **SRP-6a would be reimplemented** in the bridge, duplicating logic already in go-proton-api and ferroxide.
2. **Session management would be ad-hoc**, likely missing atomic writes or permission hardening.
3. **Token refresh would be fragile**, especially under concurrent requests from OpenClaw.
4. **Credential sourcing would remain env-var-only**, which conflicts with the OpenClaw pass-cli pattern.
5. **Other Proton tools in the ecosystem** (future proton-git-lfs-go, future proton-contacts-bridge) would face the same gaps independently.

---

## 4. Recommendation: Shared `proton-auth` Go Package

### 4.1 Rationale

The auth patterns described in Section 2 are not specific to calendar access. They are common to any Go tool that authenticates against Proton's API. Factoring them into a shared package:

- **Eliminates duplication** across proton-calendar-bridge, future proton-contacts-bridge, proton-git-lfs (if ported to Go), and any other Proton integration tool.
- **Concentrates security-critical code** in one auditable location rather than spreading SRP, session management, and token refresh across multiple repos.
- **Aligns with ecosystem precedent**: go-proton-api already provides SRP primitives; this package would build the higher-level session lifecycle on top.
- **Enables the bridge to stay focused** on its core job: provider abstraction, local API, and OpenClaw integration.

### 4.2 Proposed Package Scope

```
github.com/sevenofnine/proton-auth
    |
    +-- srp/            SRP-6a login flow (wraps go-srp + go-proton-api primitives)
    |   +-- login.go        Full login flow: /auth/info -> SRP -> /auth -> optional 2FA
    |   +-- login_test.go   Unit tests with mocked Proton API responses
    |
    +-- session/        Session persistence and lifecycle
    |   +-- store.go        Atomic file read/write, permission enforcement
    |   +-- lock.go         Advisory file locking (flock/fcntl)
    |   +-- types.go        SessionData struct (UID, AccessToken, RefreshToken, scopes, userHash)
    |   +-- store_test.go   Tests for atomic writes, permission checks, corruption recovery
    |
    +-- refresh/        Token refresh with single-flight deduplication
    |   +-- refresher.go    Refresh-on-401, single-flight lock, retry-once policy
    |   +-- errors.go       Proton-specific error code classification (9101, 10013, 429)
    |   +-- refresher_test.go
    |
    +-- credential/     CredentialProvider interface and implementations
    |   +-- provider.go     Interface: Resolve(ref) -> (secret, error)
    |   +-- passcli.go      pass-cli subprocess resolver
    |   +-- env.go          Environment variable resolver
    |   +-- gitcred.go      git credential protocol resolver (optional)
    |   +-- provider_test.go
    |
    +-- transport/      Authenticated HTTP client wrapper
        +-- client.go       http.RoundTripper that injects auth headers, handles refresh
        +-- client_test.go
```

### 4.3 Integration with proton-calendar-bridge

When the shared package exists, the bridge's provider layer gains a clean integration point:

```
internal/provider/
    +-- ics.go              (existing, unchanged)
    +-- protonapi.go        (new: Proton API provider, uses proton-auth for login + session)
    +-- protonapi_test.go
```

The `protonapi` provider would:

1. Accept a `proton-auth/credential.Provider` to source username and password.
2. Use `proton-auth/srp` to perform initial login.
3. Use `proton-auth/session` to persist and reload the session.
4. Use `proton-auth/transport` as the HTTP client, which handles 401 refresh transparently.
5. Call Proton's calendar-specific endpoints (GET calendars, GET events, POST events, etc.) using the authenticated transport.

The bridge's existing `security.BearerAuth` remains in place for the local API surface. The two auth layers serve different trust boundaries and do not interact:

```
OpenClaw Gateway
    |
    | Bearer token (bridge-local auth, existing)
    v
proton-calendar-bridge
    |
    | Proton session tokens (proton-auth, new)
    v
Proton API (calendar endpoints)
```

### 4.4 Implementation Phases

**Phase 1: Extract and publish `proton-auth` with credential + session packages**
- Implement `credential.Provider` interface with pass-cli and env backends.
- Implement `session.Store` with atomic file I/O, permission enforcement, and user hash validation.
- Unit tests with no Proton API dependency.
- This phase is useful immediately: even the ICS provider could use the credential provider to source the ICS URL from pass-cli instead of a raw env var.

**Phase 2: Add SRP login and token refresh**
- Implement `srp.Login()` wrapping go-proton-api's SRP primitives.
- Implement `refresh.Refresher` with single-flight dedup and Proton error code classification.
- Implement `transport.Client` as an `http.RoundTripper` that composes login, refresh, and session persistence.
- Integration tests against a mock Proton API server.

**Phase 3: Build Proton API calendar provider in the bridge**
- Implement `internal/provider/protonapi.go` using `proton-auth/transport.Client`.
- Map Proton's calendar API responses to the bridge's `domain.Event` and `domain.Calendar` types.
- Handle Proton's end-to-end encryption for calendar events (key management).
- Add integration tests with recorded/mocked Proton API responses.

**Phase 4: Add session file locking and multi-process coordination**
- Implement advisory file locking in `session.Store`.
- Add refresh-contention tests simulating multiple processes sharing a session file.
- This phase is relevant when the bridge coexists with other tools using the same Proton session.

### 4.5 Dependency Considerations

| Dependency | Purpose | License | Risk |
|-----------|---------|---------|------|
| `github.com/ProtonMail/go-srp` | SRP-6a primitives | MIT | Low (Proton-maintained, stable) |
| `github.com/ProtonMail/go-proton-api` | Proton API client types and helpers | MIT | Medium (community-maintained, API may shift) |
| `golang.org/x/sys` | File locking syscalls (flock) | BSD-3 | Low (Go standard ecosystem) |

The `proton-auth` package should depend on go-srp for SRP math but should **not** depend on go-proton-api's full client. Instead, it should define its own HTTP transport and use go-proton-api only for type definitions and SRP helpers, keeping the dependency surface minimal.

### 4.6 Risks and Mitigations

| Risk | Mitigation |
|------|-----------|
| Proton changes internal API endpoints or SRP parameters | Pin go-proton-api version; integration test suite with recorded responses; monitor ferroxide/hydroxide changelogs for breakage signals |
| SRP implementation bugs compromise authentication security | Use Proton's own go-srp library rather than reimplementing; fuzz test SRP flows |
| Session file corruption from crash during write | Atomic temp-file + rename pattern; validation on load with graceful fallback to re-login |
| Token leakage in logs | `transport.Client` redacts Authorization headers; session.Store never logs token values; add log-redaction tests |
| Proton ToS risk from automated access | Rate limiting in transport layer; respect 429 Retry-After; document that unofficial API usage is at operator's risk |
| Scope creep: proton-auth becomes a full Proton client | Keep package strictly limited to auth lifecycle (login, session, refresh, credential sourcing). Calendar-specific API calls belong in the bridge's provider, not in proton-auth. |

---

## Appendix A: Reference Implementations

These are the key files in the Proton ecosystem that informed this analysis:

**go-proton-api** (github.com/henrybear327/go-proton-api):
- `auth.go` -- SRP login flow, 2FA handling
- `session.go` -- Session struct with UID, AccessToken, RefreshToken
- `client.go` -- HTTP client with auth header injection
- `calendar.go` -- Calendar API method stubs (GetCalendars, GetCalendarEvents)

**proton-drive-cli** (github.com/nicjohnson/proton-drive-cli):
- `src/auth/session.ts` -- Atomic session file persistence, password stripping
- `src/sdk/httpClientAdapter.ts` -- Token refresh on 401, single-flight dedup
- `src/sdk/client.ts` -- Authenticated Proton API client
- `src/utils/git-credential.ts` -- OS keychain integration via git credential protocol

**proton-git-lfs** (not publicly available; patterns from architecture-support-brief):
- `cmd/adapter/passcli.go` -- pass-cli credential resolver
- `internal/config/status.go` -- Atomic status file writes
- `tests/integration/credential_security_test.go` -- Credential leak prevention tests

**Proton Mail Bridge** (github.com/ProtonMail/proton-bridge):
- `internal/constants/constants.go` -- Loopback binding
- `internal/frontend/grpc/service.go` -- Local gRPC with TLS + token metadata
- Keychain backends for credential storage

## Appendix B: Current proton-calendar-bridge Auth File Inventory

| File | Role |
|------|------|
| `internal/security/auth.go` | BearerAuth struct with constant-time token comparison |
| `internal/security/auth_test.go` | Unit tests for auth enabled/disabled paths |
| `internal/config/config.go` | Loads `PCB_BEARER_TOKEN` and `PCB_REQUIRE_TOKEN` from env |
| `internal/config/config_test.go` | Validates config constraints including token requirements |
| `internal/api/server.go` | `wrapAuth()` middleware applying BearerAuth to all non-healthz routes |
| `internal/api/server_test.go` | HTTP handler tests including auth rejection scenarios |

## Appendix C: Glossary

| Term | Definition |
|------|-----------|
| SRP-6a | Secure Remote Password protocol, version 6a. Zero-knowledge password proof used by Proton for all user authentication. |
| go-srp | Proton's open-source Go implementation of SRP-6a (`github.com/ProtonMail/go-srp`). |
| go-proton-api | Community Go client for Proton's internal API (`github.com/henrybear327/go-proton-api`). Provides SRP login helpers and typed API methods. |
| CredentialProvider | An interface abstraction for sourcing secrets (passwords, tokens) from different backends (pass-cli, env, keychain). |
| Single-flight refresh | A concurrency pattern where multiple goroutines needing a token refresh coalesce into a single refresh operation, with others blocking until it completes. |
| Session file | A local JSON file (mode 0600) storing Proton session tokens (UID, AccessToken, RefreshToken) for reuse across process restarts. |
| Bridge-local auth | The bearer token mechanism protecting proton-calendar-bridge's HTTP API from unauthorized local clients. Distinct from Proton upstream auth. |
| ferroxide | Community CalDAV/IMAP/SMTP bridge that translates standard protocols into Proton API calls. Successor to hydroxide. |
