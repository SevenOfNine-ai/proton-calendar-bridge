# Proton Calendar Programmatic Access Research Report

## Executive summary

As of February 11, 2026, Proton does **not** appear to offer a **public, officially supported developer API** for Proton Calendar comparable to Proton Mail Bridge’s IMAP/SMTP interface (i.e., a stable, documented interface intended for third-party automation and integrations). Proton’s **official** interoperability story for Calendar is primarily **iCalendar/ICS-based import/export and subscription/sharing links**, which are **read-only** from the perspective of external systems. citeturn23view1turn25view0turn34view0

For collaboration, Proton supports **shared calendars among Proton users** with **View** or **Edit** permissions, including the ability (with Edit permission) to **create, edit, and delete** events—but this remains inside Proton’s own clients (web/mobile) rather than via a published external API. citeturn24view0

If you require **programmatic read/write/modify** access (including shared calendars) using standard tooling, the most capable path found is **unofficial**: community bridges such as **ferroxide**, which explicitly advertises **CalDAV** support by translating CalDAV/IMAP/SMTP/CardDAV into Proton API requests. citeturn37view0 A related open-source project (**hydroxide**) has a CalDAV implementation under development, with stated support for **read, delete, create/update events** (but **without attendee invitation functionality** at the time captured). citeturn36view0 These approaches carry higher stability and policy risk than Proton’s official sharing features.

## What Proton officially supports for calendar interoperability

### No official developer API program surfaced for Proton Calendar

Across Proton’s own Calendar support materials, Proton documents user-facing features (create/edit events, invites, recurring events, sharing), but the official “integration” mechanisms exposed to third parties are **links and files**, not a documented developer API surface (no OAuth app registration, no published endpoints, no published scopes). Proton’s own community feedback forum contains a long-standing request explicitly asking for a “Calendar API” because users “would be nice to have an API for adding events… programmatically,” which strongly suggests the absence of an officially supported automation API for typical users. citeturn39view0

### iCalendar/ICS export and import

Proton officially supports exporting calendars as **iCalendar (ICS)** files, which can then be imported into many third-party calendar apps/services. citeturn34view0 This is useful for portability and backup, but it is not an API and does not provide continuous bidirectional sync.

### Read-only sharing via “share calendar with anyone via a link”

Proton supports sharing a calendar via a **URL link**. Anyone with the link can subscribe from other calendar products; Proton notes it can take **up to eight hours** for others to see changes. citeturn25view0

The shared-link mechanism offers two visibility modes:

- **Limited view**: shows busy times without event details. citeturn25view0  
- **Full view**: includes event details such as title/description, participants, and location. citeturn25view0  

A key security detail: Proton states that when using **Full view**, the generated URL contains the **key required to decrypt the calendar**, and when the URL is used, “Proton Mail has access to this key to decrypt the calendar” (and “at no other time” can they access it). citeturn25view0 This implies the link is effectively a bearer-secret that should be treated like a private credential.

Also, Proton allows up to **five links per calendar**, which can help segment audiences and revoke access by deleting a specific link. citeturn25view0

### Proton can subscribe to external calendars, but it is view-only

Proton supports subscribing to external calendars via a public calendar link; when you subscribe, you can **view** events but **cannot edit or delete** them. citeturn23view1 This is the inverse of “share Proton outward,” and likewise does not provide write access.

### Sharing with Proton users: true collaboration, but inside Proton clients

Proton supports sharing calendars with other Proton Mail users (paid plan required) and explicitly distinguishes this as the “more secure” method because Proton shares the **calendar encryption key** with individually invited Proton users. citeturn24view0

Permissions include:

- **View**: invitees can view events but cannot modify them. citeturn24view0  
- **Edit**: invitees can “view, create, edit, and delete events.” citeturn24view0  

This meets collaborative needs, but it is not exposed as a public automation API.

## Authentication and security posture implications

### Proton Calendar’s encryption model complicates third-party API design

Proton markets Proton Calendar as end-to-end encrypted for events, and (for business messaging) states that “all your Proton Calendar events are automatically secured with end-to-end encryption,” with events from non-Proton calendars secured with “zero-knowledge encryption.” citeturn33view0 Proton also claims use of “ECC Curve25519” and highlights that the Calendar web app is open source and audited. citeturn33view0

This security model matters operationally: a third-party API that supports full read/write of event content would need a strategy for handling encryption keys and encrypted payloads without breaking Proton’s privacy properties. Proton’s own sharing-with-Proton-users flow explicitly frames collaboration as key-sharing among Proton users. citeturn24view0

### OAuth, API keys, CalDAV, CardDAV: not in Proton’s official Calendar docs

In the reviewed Proton Calendar support documentation, there is no official mention of OAuth client registration for Calendar, API keys, or native CalDAV endpoints. Instead, Proton documents iCalendar/ICS-based sharing and subscriptions. citeturn23view1turn25view0turn34view0

Accordingly, any CalDAV-style integration observed in the ecosystem is best understood as **unofficial bridging**, not a Proton-supported protocol endpoint.

### Rate limiting and “scopes” signals exist at the Proton API layer, but are not Calendar-public

Proton publishes (via PyPI) a “Proton API Python Client” that can authenticate, store sessions, refresh tokens, and make API calls to “various endpoints.” This client documents:

- AccessToken / RefreshToken usage and refresh flow citeturn32view0  
- Error handling including **401** (refresh needed), **403** (missing scopes), and **429** (too many requests) including a **Retry-After** header pattern citeturn32view0  
- TLS pinning considerations (and warning about disabling it) citeturn32view0  

However, this does not constitute a Proton Calendar developer program: it does **not** publish a Calendar-specific API contract, endpoint list, or stable event model intended for third parties. citeturn32view0

### Policy constraints relevant to automation

Proton’s Terms of Service (last modified December 2, 2025) state: “Accounts registered by ‘bots’ or automated methods are not authorized and will be terminated.” citeturn41view0 While this is specifically about account registration, it signals Proton’s sensitivity toward automation patterns; integrations that resemble abusive automation could create account risk, especially if rate limits are exceeded or behavior appears suspicious. citeturn41view0turn32view0

## Options landscape and comparison

### Comparative options table

| Option | Authentication | Supported operations | Shared calendar support | Stability | Legal/ethical risk | Implementation effort |
|---|---|---|---|---|---|---|
| Official Proton Calendar public API | Not available (no official developer API found) citeturn39view0turn23view1 | N/A | N/A | N/A | Low (no method) | N/A |
| Official “share calendar via link” (ICS subscription outward) | Bearer-style secret URL (no user login); Full view link contains decryption key material per Proton citeturn25view0 | **Read-only** from external systems: subscribe/view; can download ICS via the URL; updates may take up to 8 hours citeturn25view0 | Yes, in the sense you can share a calendar to anyone; access levels limited/full; up to 5 links per calendar citeturn25view0 | Medium–High (official feature, but sync delay) citeturn25view0 | Medium (link secrecy is critical; Full-view link implies key exposure through link usage) citeturn25view0 | Low |
| Official “share with Proton users” | Proton account invitation/acceptance; permissions View/Edit citeturn24view0 | Collaborative CRUD **within Proton clients**; Edit permission allows create/edit/delete events citeturn24view0 | Yes (explicit), and framed as more secure due to key sharing citeturn24view0 | High (official) | Low–Medium (normal product use) | Low (human workflow), High (if you need automation—no API) |
| Unofficial direct use of Proton internal API (reverse-engineered) | Proton login + tokens (AccessToken/RefreshToken), scopes, 2FA considerations; rate-limit signals (429 Retry-After) citeturn32view0turn10view0 | Potentially broad but **not specified** publicly; go-proton-api exposes read methods for calendars/events/members/keys citeturn10view0 | Likely possible in principle (internal models include members/permissions), but **not confirmed by official docs** citeturn10view0turn24view0 | Low–Medium (endpoints may change; no published contract) | Medium–High (unofficial; risk of breakage and account action if abused; automation sensitivity) citeturn41view0turn32view0 | High |
| Community CalDAV bridge (ferroxide / hydroxide-derived) | Local CalDAV server; ferroxide uses a generated “bridge password” and stores Proton credentials encrypted with it citeturn37view0 | CalDAV-style event operations; hydroxide CalDAV PR: read, delete, create/update events (no attendee invite support yet), notifications; multi-calendar support; shared event support mentioned citeturn36view0turn37view0 | Partial-to-good: supports multiple calendars and “shared event support” (per PR commit message list) but attendee inviting is missing in stated functionality citeturn36view0turn37view0 | Medium (community project; depends on Proton internals) | Medium–High (unofficial protocol translation; maintenance risk) | Medium–High |
| Web scraping / browser automation against calendar.proton.me | Proton web login session; often brittle; may trigger anti-abuse controls | Whatever the UI does, but fragile; hard to do safely for invites/recurrence | Potentially, but fragile | Low | High (breaks easily; can violate site expectations; may resemble abuse) citeturn41view0 | High |
| Proton Mail Bridge | Local IMAP/SMTP for **mail** clients; documented as integrating “your inbox” via IMAP/SMTP citeturn28view0 | Email only (not calendar) per official description citeturn28view0 | N/A | High (official) | Low | Low |

### Key capability reality check against your requirement set

Your target capability set includes CRUD events (including recurring), attendees/invitations, reminders/notifications, and shared calendar ACLs with stable auth and permissions.

From primary sources:

- **Attendees/invitations in Proton proper** exist as product features (and shared calendars exist), but that does **not** imply an external API. Proton’s share-with-Proton-users explicitly supports **Edit** permission for invited members (create/edit/delete). citeturn24view0  
- **Shared-link ICS** can expose participants/location in Full view and supports broad sharing, but remains **read-only** externally. citeturn25view0  
- **Hydroxide CalDAV PR** explicitly states “Create / Update events (currently without attendee invitation functionality)” and lists attendee invitations as a TODO, meaning meeting scheduling workflows likely won’t be fully correct yet for standards-based scheduling. citeturn36view0  
- **Ferroxide** asserts CalDAV support and protocol translation into Proton API requests, suggesting the bridge handles encryption/protocol mapping internally; however, its completeness for invitations/ACL semantics beyond basic CalDAV is not guaranteed by Proton. citeturn37view0  

## Practical integration patterns and step-by-step examples

### Read-only integration using Proton’s official share link

This is the lowest-risk approach if read-only is acceptable (e.g., analytics, availability display, downstream mirroring).

```mermaid
flowchart LR
  A[Proton Calendar] -->|Share link (Limited/Full view)| B[ICS subscription URL]
  B -->|Periodic fetch| C[Integration service]
  C --> D[(Database / Cache)]
  C --> E[Downstream apps: dashboards, reporting, other calendars]
```

Operational notes grounded in Proton docs:
- The audience may experience up to **eight hours** latency before seeing updates. citeturn25view0  
- Full view sharing generates a URL containing the key required to decrypt the calendar; treat the URL as a secret credential. citeturn25view0  

Example: fetch an ICS file from a Proton Calendar shared link (URL shape varies; Proton provides it in the UI).

```bash
# 1) Create a "Share with anyone" link in Proton Calendar (web app):
#    Settings → All settings → Calendars → [calendar] → "Share with anyone" → Create link
#    Choose Limited or Full view, then copy the URL.  (Proton docs)
#
# 2) Download the ICS payload (read-only):
curl -L 'PASTE_YOUR_PROTON_SHARED_CALENDAR_URL_HERE' -o proton-calendar.ics

# 3) Validate it looks like an iCalendar file:
head -n 20 proton-calendar.ics
```

What you can and cannot expect:
- You can parse standard ICS components and build read-only views. Proton explicitly positions this as a sharing/subscription mechanism, not an editing channel. citeturn25view0  
- If you need only free/busy style visibility, prefer “Limited view.” citeturn25view0  

### Two-way integration using a community CalDAV bridge

If you require bidirectional programmatic access using standard calendar tooling, the strongest evidence found points to community bridges (not Proton-supported) that translate CalDAV ↔ Proton internal APIs.

```mermaid
flowchart LR
  A[CalDAV client / automation<br/>e.g., Thunderbird, scripts] <-->|CalDAV| B[Community bridge<br/>ferroxide / hydroxide-derived]
  B <-->|Proton internal API calls| C[Proton backend]
  C --> D[Proton Calendar data (encrypted at rest)]
```

#### Ferroxide high-level setup flow (as documented by the project)

Ferroxide describes itself as translating SMTP/IMAP/CardDAV/**CalDAV** to Proton API requests. citeturn37view0 It documents:
- Install via `go install ...` citeturn37view0  
- Log in with `ferroxide auth <username>` which prints a “bridge password” and stores Proton credentials encrypted with that password citeturn37view0  
- Run CalDAV on port **8081** (and other services on other ports) citeturn37view0  

Illustrative command sequence based on ferroxide’s README:

```bash
# Install (per ferroxide README)
go install github.com/acheong08/ferroxide/cmd/ferroxide@latest

# Authenticate (per ferroxide README)
ferroxide auth <your-proton-username>

# Start CalDAV (per ferroxide README)
ferroxide caldav
# or run all services together if desired:
# ferroxide serve  (uses ports 1025/1143/8080/8081 per README)
```

From there, you’d connect a CalDAV-capable tool to the local endpoint and authenticate using the bridge password (the exact URL path and authentication scheme are determined by the bridge’s implementation). Ferroxide reports CalDAV testing with GNOME Evolution, Thunderbird, and KOrganizer. citeturn37view0

#### Hydroxide CalDAV status signals

A Hydroxide pull request (“CalDAV support”) reports these functions:
- Operate on multiple calendars
- Read events
- Delete events
- Create / update events (**without attendee invitation functionality**)
- Notifications  
and lists attendee invitations as TODO. citeturn36view0

This is important because meeting scheduling interoperability typically relies on standardized invitation flows; lacking attendee invitation support can be a blocker for business-grade syncing. citeturn36view0

### Direct Proton API calls (unofficial) using Proton’s published client wrapper patterns

Proton’s “Proton API Python Client” on PyPI shows a generic pattern for session auth, token refresh, and calling arbitrary endpoints, including how to respond to 401/403/429 conditions. citeturn32view0 Separately, Proton’s `go-proton-api` package exposes calendar-related client methods such as `GetCalendars`, `GetCalendarEvents`, and `GetCalendarMembers`, indicating an internal API surface exists. citeturn10view0

However, **endpoint names, request/response formats, and encryption payload structures for creating/updating calendar events are not described in any official Calendar developer documentation found**. Therefore, only a schematic example is responsible here:

```python
# Conceptual example using the patterns shown in Proton's proton-client documentation.
# This does NOT include real Calendar endpoints because Proton does not publish them for Calendar.

from proton.api import Session, ProtonError  # per proton-client docs

s = Session(
    api_url="https://...your_proton_api_base...",  # Proton-client requires api_url
    appversion="MyIntegration_0.1",
    user_agent="my-service",
)

try:
    s.authenticate("your_username", "your_password")  # per proton-client docs
    # Hypothetical call: endpoint name not specified by Proton Calendar docs
    # s.api_request(endpoint="calendar/....", method="get")
except ProtonError as e:
    if e.code == 401:
        s.refresh()  # per proton-client docs
    elif e.code == 429:
        # Retry after e.headers["Retry-After"] per docs
        raise
    else:
        raise
```

Use this pattern only if you accept the risk of relying on undocumented internals and encryption details; Proton’s documentation here is about the client mechanics, not a supported Calendar API contract. citeturn32view0turn10view0

## Recommendations for integrating read/write/modify personal and shared calendars

### If you need a stable, supportable integration

If “supportable” means “Proton will document and keep it stable and you can build on it with normal enterprise expectations,” the findings point to a hard constraint: **Proton Calendar is currently not positioned as an API-first calendar platform.** The official interoperability features are **ICS export/import and read-only subscription links**, plus in-Proton sharing with Proton users. citeturn25view0turn23view1turn24view0turn34view0

Therefore, for stable integrations requiring true programmatic write access, the recommended approach is:

- Use Proton Calendar as the human-facing calendar, but keep automation in a **standards-first system of record** (e.g., a CalDAV-capable calendar service) and accept that Proton will consume it read-only (via subscription) or via periodic ICS imports—recognizing Proton’s subscribed calendars are view-only inside Proton. citeturn23view1  
- Alternatively, if Proton must be the system of record, accept read-only downstream visibility via share links and do writes via Proton’s apps/UI (or via Proton-user collaboration). citeturn25view0turn24view0  

### If you must have programmatic read/write/modify against Proton Calendar itself

The options become explicitly **unofficial**:

- Prefer a **CalDAV-translation bridge** (e.g., ferroxide) over browser automation, because it gives you a standards-shaped interface and centralizes the reverse-engineering surface to one component. Ferroxide explicitly states it translates CalDAV into Proton API calls and documents an operational workflow. citeturn37view0  
- Plan for limitations: attendee invitations may be absent or incomplete in some implementations (as explicitly stated in a hydroxide CalDAV PR). citeturn36view0  
- Engineer for failure: pin versions, include test calendars, monitor for sync breakage after Proton updates, and implement cautious rate limiting (429 Retry-After behavior is documented in Proton’s client wrapper). citeturn32view0  

### Security guidance based on Proton’s designs

- Treat Proton share links as sensitive secrets, especially “Full view” links, because Proton states the URL contains the key required to decrypt the calendar. Store it like a password or API token. citeturn25view0  
- Expect that encryption/key management is central to any write-capable integration; Proton frames shared calendars with Proton users as secure specifically because the encryption key is shared with specific users. citeturn24view0  
- Respect rate limits and avoid behavior that looks abusive; Proton’s client documentation includes a 429 pathway and Proton’s Terms indicate a strict posture toward automated/bot behaviors (at minimum for account registration). citeturn32view0turn41view0  

### Bottom-line recommendation

For a production integration that must support **programmatic read/write/modify**, including **shared calendars**, Proton Calendar currently offers no official equivalent to Proton Mail Bridge. The pragmatic choices are:

- **Official + low risk:** Use Proton’s **share-via-link (ICS)** for read-only downstream consumption; use Proton’s native sharing among Proton users for collaboration; do not expect write APIs. citeturn25view0turn24view0  
- **Write-capable but unofficial:** Use a community **CalDAV bridge (ferroxide / hydroxide-derived)**, accepting limitations (notably invitations) and higher maintenance/legal risk. citeturn37view0turn36view0  
- **Avoid unless you accept high risk:** Web UI automation/scraping for CRUD operations, because it is brittle and can resemble abusive automation practices. citeturn41view0  

### Primary-source links included in this research

Because Proton does not provide a Calendar API reference page, the most relevant primary sources are their official support docs and their legal terms, plus open-source community bridges:

```text
Proton official docs (Calendar sharing/subscription)
- https://proton.me/support/subscribe-to-external-calendar
- https://proton.me/support/share-calendar-via-link
- https://proton.me/support/share-calendar-with-proton-users
- https://proton.me/support/protoncalendar-calendars

Proton Mail Bridge (official positioning: IMAP/SMTP for inbox)
- https://proton.me/support/mail/bridge/introduction-bridge

Proton API client mechanics (token refresh / 429 Retry-After)
- https://pypi.org/project/proton-client/

Community bridges (CalDAV translation)
- https://github.com/vcalv/ferroxide-systemd
- https://github.com/emersion/hydroxide/pull/282

Proton legal terms
- https://proton.me/legal/terms
```

(Standards RFC links are deliberately not expanded with claims here because Proton’s own documentation frames interoperability in terms of iCalendar/ICS and CalDAV naming, rather than citing specific RFC numbers. Proton nevertheless explicitly references the iCalendar standard in its support guidance.) citeturn23view1turn34view0turn37view0
