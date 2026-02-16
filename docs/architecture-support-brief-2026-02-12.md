# Proton Calendar Bridge — Architecture Support Brief (Overnight Build)

Date: 2026-02-12  
Scope sources: `proton-git-lfs`, `proton-drive-cli`

## TL;DR Priority Order (Do Tonight)

1. **Implement secure session manager first** (0700 dir, 0600 file, atomic writes, no password persistence).
2. **Implement credential providers abstraction** with `pass-cli` default + `git-credential` fallback.
3. **Build bridge IPC as subprocess JSON over stdin/stdout** (no localhost HTTP).
4. **Add token refresh policy** (on 401 + provider-specific auth error codes; retry once).
5. **Ship test harness skeleton now** (unit + bridge contract + mocked integration + race tests in CI).
6. **Add tray watcher using file-based status channel** (polling + atomic status writes).

---

## 1) Reusable Auth Flow Patterns

## A. Credential sourcing pattern (recommended)
- Use a provider switch:
  - `pass-cli` (default for bridge mode)
  - `git-credential` (OS keychain/GCM mode)
- Keep credentials out of CLI args and logs.
- Resolve credentials once at startup in pass-cli mode; keep in memory; zero buffers on terminate.

**Observed reusable patterns**
- `proton-git-lfs/cmd/adapter/passcli.go`
  - Reads pass refs (`pass://...`) via subprocess.
  - Supports JSON/plain secret output fallback parsing.
- `proton-drive-cli/src/utils/git-credential.ts`
  - Uses `git credential fill/approve/reject` via stdin/stdout protocol.

## B. Session/token storage pattern
- Session dir: `~/.proton-calendar-bridge/` with `0700`.
- Session file: `session.json` with `0600`.
- Persist **tokens only** (sessionId, accessToken, refreshToken, uid, scopes, userHash).
- Never persist mailbox/account password.
- Use atomic save: write temp file + rename.

**Observed reusable patterns**
- `proton-drive-cli/src/auth/session.ts`
  - strips mailboxPassword before save
  - atomic temp-write + rename
  - concurrent-safe approach and cleanup on failure

## C. Refresh + session reuse pattern
- Reuse session when valid and same user (`userHash`).
- Refresh token when:
  - HTTP 401
  - explicit API auth-expiry codes (e.g., 9101 / invalid token codes)
- Deduplicate concurrent refresh attempts (single-flight lock/promise).
- Retry original request once after refresh.
- If refresh fails, re-read session file (another process may have refreshed).

**Observed reusable patterns**
- `proton-drive-cli/src/sdk/httpClientAdapter.ts`
- `proton-drive-cli/src/sdk/client.ts`
- `proton-drive-cli/src/cli/bridge.ts` (`sessionReused` behavior)

## D. Secret handling/policy integration with pass-cli
- Keep pass refs configurable:
  - `PROTON_PASS_CLI_BIN`
  - `PROTON_PASS_USERNAME_REF`
  - `PROTON_PASS_PASSWORD_REF`
- Add helper script (export env refs) for local shell bootstrap.
- Optional: support `pass-cli user info` fallback for username when only password ref exists.

**Directly reusable from current ecosystem**
- `proton-git-lfs/scripts/export-pass-env.sh`
- `proton-git-lfs/tests/integration/credential_security_test.go`

---

## 2) Tray App ↔ Local Service Interaction Patterns

## A. IPC/API pattern
**Recommended:** subprocess JSON bridge over stdin/stdout, not localhost HTTP.
- Strong isolation
- no network-exposed surface
- deterministic request/response contract

**Observed reusable patterns**
- `proton-git-lfs/cmd/adapter/bridge.go`
- `proton-drive-cli/src/cli/bridge.ts`

## B. Tray to backend status channel
Use **file-based status report**:
- backend writes atomic status JSON (`state`, `lastOp`, `error`, `timestamp`)
- tray polls every 3–5s and updates icon/menu

**Observed reusable patterns**
- writer/reader: `proton-git-lfs/internal/config/status.go`
- tray poller UX: `proton-git-lfs/cmd/tray/status.go`

## C. Lifecycle + UX
- Tray menu should include:
  - health/status line
  - last successful transfer/activity
  - credential provider toggle
  - setup credentials action
  - register integration action
  - launch-at-login toggle
  - quit
- On quit/terminate: zero credentials and stop watchers cleanly.

**Observed reusable patterns**
- `proton-git-lfs/cmd/tray/menu.go`
- `proton-git-lfs/cmd/tray/setup.go`

---

## 3) CI/CD + Quality Gates to Reuse

## A. CI shape
Adopt split pipelines:
1. lint/typecheck
2. unit tests (+ race)
3. integration tests (mocked/no real creds)
4. optional real integration lane (manual/secret-gated)
5. release build + artifacts + checksums

## B. Concrete reusable practices seen
- Matrix testing by OS for tray and integration semantics.
- `go test -race -cover` on critical packages.
- mocked pass-cli and mocked bridge E2E tests in CI.
- separate release workflow with packaging + checksum + release notes.

**Reference workflows**
- `proton-git-lfs/.github/workflows/test.yml`
- `proton-git-lfs/.github/workflows/lint.yml`
- `proton-git-lfs/.github/workflows/release-bundle.yml`
- `proton-drive-cli/.github/workflows/ci.yml`
- `proton-drive-cli/.github/workflows/release.yml`

## C. Overnight minimum gates
- required:
  - `go vet`
  - `gofmt` clean
  - `golangci-lint`
  - `go test -race ./...`
  - `go test -cover ./...`
- branch protection: require lint + test jobs before merge.

---

## 4) Reusable Test Harness Patterns (High Coverage)

## A. Test layers to copy
1. **Unit tests** for parser/validators/auth/session storage.
2. **Bridge protocol contract tests** (stdin JSON request/response, error mapping).
3. **Integration mocked E2E** (full flow, no real Proton creds).
4. **Failure-mode tests** (timeouts, malformed JSON, unauthorized, corrupted session).
5. **Concurrency/race/soak tests**.

## B. High-value cases to implement immediately
- credential leak prevention in stderr/log outputs
- session file permission checks (`0700` dir, `0600` file)
- token refresh race dedupe (parallel 401s)
- session reuse only when userHash matches
- corrupted session file fallback behavior
- subprocess timeout and kill behavior

## C. Pattern examples to mirror
- `proton-git-lfs/tests/integration/credential_security_test.go`
- `proton-git-lfs/tests/integration/git_lfs_custom_transfer_timeout_semantics_test.go`
- `proton-git-lfs/tests/integration/git_lfs_custom_transfer_concurrency_stress_test.go`
- `proton-drive-cli/src/auth/session.test.ts`
- `proton-drive-cli/src/sdk/httpClientAdapter.test.ts`
- `proton-drive-cli/src/cli/e2e.test.ts`

---

## 5) Recommended Baseline Repo Layout (Go Tray Bridge)

```text
proton-calendar-bridge/
  cmd/
    bridge/                 # main service/adapter entrypoint
    tray/                   # systray app
  internal/
    auth/
      provider.go           # provider interface
      passcli.go            # pass-cli resolver
      gitcred.go            # git credential resolver
      session.go            # token/session persistence + refresh state
    bridge/
      client.go             # subprocess JSON bridge client
      protocol.go           # request/response structs + validation
      errors.go             # typed errors + mapping
    status/
      report.go             # status file model + atomic write/read
    config/
      config.go             # env/flags/defaults
      prefs.go              # tray prefs (credential provider, UI options)
    lifecycle/
      shutdown.go           # signal handling, cleanup, zeroing
    security/
      redact.go             # log redaction helpers
      zeroize.go            # credential zeroization utilities
  tests/
    integration/
      bridge_mock_test.go
      credential_security_test.go
      refresh_race_test.go
      timeout_semantics_test.go
      tray_status_integration_test.go
    testdata/
      mock-pass-cli.sh
      mock-bridge-service.js
  scripts/
    export-pass-env.sh
    package-bundle.sh
  docs/
    architecture/
    security/
    operations/
    testing/
  .github/workflows/
    lint.yml
    test.yml
    release.yml
  Makefile
```

---

## Overnight Implementation Checklist (Actionable)

## P0 (must finish tonight)
- [ ] Create `internal/auth/session.go` with atomic save/load/clear, `0700/0600`, tokens-only schema.
- [ ] Create auth provider interface + `pass-cli` resolver implementation.
- [ ] Create bridge subprocess client (stdin/stdout JSON) with strict timeout and error sanitization.
- [ ] Add refresh-on-401 + single retry path.
- [ ] Add credential zeroization on terminate.
- [ ] Add unit tests for session manager + provider + bridge error mapping.

## P1 (strongly recommended tonight)
- [ ] Implement tray status file watcher + icon/menu state machine.
- [ ] Add integration tests with mocked pass-cli + mocked bridge binary.
- [ ] Add failure-mode tests (timeout, malformed response, unauthorized).
- [ ] Add CI workflows (lint/test/integration-mocked) with race + coverage artifacts.

## P2 (next day hardening)
- [ ] Add git-credential provider mode end-to-end.
- [ ] Add refresh race multi-process handling via session file re-read.
- [ ] Add release packaging/checksum workflow.
- [ ] Add optional real integration lane gated by manual dispatch/secrets.

---

## Known Risks to Account For
- Proton CAPTCHA flow remains awkward for pure CLI auth (known upstream issue).
- Token refresh reliability requires robust retry + re-read logic under concurrency.
- Avoid leaking tokens in debug/error outputs (must redact aggressively).

---

## Practical Recommendation for Seven
Ship the **session/auth/bridge core first** and keep UI minimal. A stable subprocess contract + secure token lifecycle will unblock everything else (tray polish, packaging, advanced flows).
