# RegattaCentral API integration

How regattaClock exchanges data with the
[RegattaCentral](https://www.regattacentral.com/) v4 API — the regatta
**registration** service that also acts as the public results board. Two
directions:

- **read** the athlete / boat roster on the way in, so the Regatta Director
  stops copy-pasting it from a web page;
- **write** the race schedule, lane draws, race status, and approved results
  back out, so coaches, athletes, and spectators see them on RegattaCentral
  moments after the finish-line timers commit them.

RegattaCentral treats an application in regattaClock's position as a **timing /
regatta-management system** — a two-way peer, not a read-only consumer. See
[reference/RegattaCentral_APIV4_Cookbook.pdf](reference/RegattaCentral_APIV4_Cookbook.pdf)
(the vendor's integration guide, cited below by section as "Cookbook §N") and
its companion online guide at
<https://api.regattacentral.com/v4/apiV4.jsp>.

Companion to [sidecar-personas.md](sidecar-personas.md) (the write path is its
Increment 1), [persona-plan.md](persona-plan.md) §3b (the schedule-origin seam
the read path plugs into), and [README.md](README.md) §"Timer priority" (the
non-blocking rule any network work inherits).

**Status:** design. No code.

## Recommendation up front

Build **one client package**, `internal/regattacentral` — a leaf that owns auth
(OAuth2 **resource-owner password grant** with refresh), a `net/http` transport
with timeouts, retry/backoff and `context`, a typed **JSON** model mirroring the
RegattaCentral schema, and both read and write methods. Thin adapters consume
it; a persona's "read" vs "write" is just which methods it calls (RegattaCentral
does not issue scoped tokens — access is gated by the operator's **"staff"**
privilege on the regatta, so the split is convention, not enforcement).

Secrets — `client_id`, `client_secret`, and the operator's RegattaCentral
`username` / `password` — live in a new `internal/secretstore` over the OS
keyring, never in the synced `regattaData/` tree, never in Fyne `Preferences`.
Non-secret config (`BaseURL`, `RegattaID`) goes in the deployment
[persona-config-file.md](persona-config-file.md) JSON.

Ship it in phases: the client + secret store + a `cmd/rcprobe` developer CLI
first (no UI); then a read-only in-app roster view; then in-app heat-sheet
authoring that also seeds RegattaCentral (which is what finally retires the
Excel ingest); then the results write path.

## The scenario

RegattaCentral is where crews register for a regatta. It holds two layers of
data:

| Layer | Entities | Who owns it |
|-------|----------|-------------|
| **Registration** | `Regatta`, `Event`, `Entry` (a registered boat + its athletes, seats, eligibility), `Organization` (club / team), `Participant` | RegattaCentral / the registering coaches — regattaClock reads it |
| **Racing** | `Race`, `Lane` (draw / bow numbers), timing milestones, `Result`, race `status` | **starts empty** — the timing system authors and uploads it |

Today the Regatta Director reads the roster off the regattacentral.com web UI
and **copy-pastes it by hand into a "heat sheet" worksheet inside an `.xlsx`**,
building the race lineups there. That workbook's results tab is what
regattaClock ingests through [`internal/reader`](../../../internal/reader/) as
`.xlsx` / `.xlsm`.

The API replaces **both** halves of that: pull the roster with one `/bulk`
call, author the heat sheet in-app, and push the schedule, lane draws, status
transitions, and results back so RegattaCentral can display them live. Two
consumers of the API:

| Consumer | Direction | What it needs |
|----------|-----------|---------------|
| RD, and a future **Heat Sheet Author** (Executive team) | read + write | the roster surfaced in-app; an in-app builder that authors the heat sheet from that roster and uploads events / races / lane draws to RegattaCentral |
| A future **Register Results** persona | write | push each **approved** result (lane draw + finish times + status) to RegattaCentral |

Both hit the same API with the same session. The engineering task is a package
both import.

## The RegattaCentral workflow (V4 Cookbook)

The Cookbook lays out the call sequence for a timing system before, during, and
after a regatta. regattaClock's phases map onto it directly.

### Authentication (Cookbook §3)

The operator holds a pre-allocated **client id** and **client secret**; these
are combined with the operator's own RegattaCentral **username** and
**password**. That account must have **"staff" access** to the regatta.

```text
POST https://api.regattacentral.com/oauth2/api/token
     client_id={id}&client_secret={secret}
     &username={user}&password={pass}&grant_type=password
→ { access_token, refresh_token, expires_in }
```

- The `access_token` is sent **verbatim** in the `Authorization` header on every
  subsequent call (no `Bearer` prefix shown in the vendor examples — confirm).
- Refresh with `grant_type=refresh_token` before expiry and once on a `401`;
  the operator is not re-prompted.
- Connections require **TLS 1.2+**. A registered client id may also need an
  `Origin` header matching a configured referer.
- An API-Key is accepted as an alternative and also travels in `Authorization`;
  there is **no** separate `x-api-key` header in the V4 guide.

### Ingest the roster (Cookbook §5, §8)

```text
GET https://api.regattacentral.com/v4.0/regattas/{RegattaId}/bulk
```

- Returns the **whole regatta** in one response — events, entries, athletes,
  organizations.
- There is **no incremental sync** ("no way to provide only information updated
  after a certain time"). Reload in full; regattaClock owns the SYNC and
  conflict story (it already has one — see [Phase C](#phase-c--in-app-heat-sheet-authoring)).
- **Preserve every RegattaCentral id** (`eventId`, `entryId`, athlete id). They
  are echoed back on upload. Anything regattaClock invents locally uploads with
  `id: 0` plus a **self-assigned UUID**, and that UUID must be reused on every
  later reference (RegattaCentral maps it to an internal id).
- Birthdates are **obfuscated to January 1** of the birth year.

### Lookups (Cookbook §6, §7)

```text
GET /v4.0/organizations?name={term}
GET /v4.0/participants?lastname={term}&birthdate={yyyy-mm-dd}
```

Used when authoring adds or fixes a crew and needs the canonical
RegattaCentral organization / athlete id.

### Author and upload the schedule (Cookbook §9–§13)

All uploads are one `PUT` with a **combined JSON body** — events, entries,
races, and lanes can all travel together, and a re-upload **overwrites** the
prior record.

```text
PUT https://api.regattacentral.com/v4.0/regattas/{RegattaID}/upload
```

- `"flush": true` at regatta scope clears **only** race schedule, draws, and
  results — never registration or entry data. Scoped to one race, it clears
  that race's draw + results (used when promoting a race to `Official`:
  flush, then re-upload the whole race in one shot).
- Race `status` is the most display-significant field; drive it through the
  regatta: `PreDraw` → `Draw` → `Racing` → `Unofficial` → `Official`
  (plus `Stopped`, `StoppedCollision`, `StoppedFalseStart`,
  `StoppedStartZoneDamage`, `Protest`, `RemovedCancelled`,
  `RemovedConsolidated`, `RemovedNonEvent`, `Unknown`).

### Lane draw (Cookbook §14–§15)

- Report **all lanes for a race together** via the `lane:` tag (bow numbers,
  no times). This establishes timing milestone 0 (the start line).
- Optional per-entry status: `SCR` scratched, `DNS`, `DNF`, `DSQ`, `RMV`,
  `EXC`, `NJ` not judged, `INV` by invitation.
- **Lanes MUST be reported before any result for that race.**

### Results (Cookbook §14)

- A `result:` record is keyed by **race number + lane number** — crew and event
  are *not* in the record. Report any lane change via `lane:` first.
- `timingMilestoneId` `0` is always the start, `4` is always the finish; `1`–`3`
  are optional mid-course splits.
- `time` = cumulative; `splitTime` = since the previous split; `adjustedTime` =
  cumulative with handicaps / penalties. `time: 0` clears that race/lane/split.
- Results are **"cooked"** — the timing system computes places, margins,
  penalties and supplies milestone labels; RegattaCentral displays exactly what
  it is given. regattaClock already back-calculates every boat's time from the
  winning time and splits, which is precisely this contract.

### Read-back for other consumers

```text
GET /v4.0/regattas/{RegattaId}/races?active           # races In Progress
GET /v4.0/regattas/{RegattaId}/events/{EventId}/results
GET /v4.0/regattas/{RegattaId}/events/{EventId}/lanes
GET /v4.0/regattas/{RegattaId}/events/{EventId}/races
GET /v4.0/regattas/{RegattaId}/events/{EventId}/entries
GET /v4.0/regattas/{RegattaId}/organizations
```

### Transformers (Cookbook §4)

The Cookbook's recommended shape is a set of **transformer functions** that
convert each element between the RegattaCentral model and the application
model, in both directions. That is the same seam regattaClock already has as
`scheduleFromRegattaData` / `regattaDataFromSchedule` in
[`internal/regatta/schedule.go`](../../../internal/regatta/schedule.go); the
RegattaCentral adapter is one more pair.

## What the schema tells us — and does not

The schema (`rc-api.xsd`, linked from the online guide; also served at
`https://api.regattacentral.com/v4.0/schema`) documents the **data model**.
Wire format is **JSON** (XML is accepted only for legacy uploads). Responses
carry **HATEOAS links** so an adapter can drill from a regatta into its events,
entries, and results without hand-building URLs.

Entities that matter here:

- **Roster (read):** `Regatta`, `Event`, `EquipmentType` (boat class), `Entry`,
  the athlete-on-entry records (seat, eligibility), `Organization`, `Coach`,
  `Country`, `Blade`.
- **Racing (write, and read-back):** `Race`, `Lane`, timing milestones,
  `Result`, `ResultStatusType`, race `status`, `Points`.

Everything under [Open items](#open-items) still has to be confirmed against a
live account before Phase A code.

## `internal/regattacentral` — the client package

A **leaf**: standard library only (`net/http`, `context`, `encoding/json`,
`time`, `sync`) plus `internal/secretstore`. **No Fyne. No `internal/regatta`.**
This mirrors how [`internal/timesync`](../../../internal/timesync/) is a
self-contained process-level service with a background re-query goroutine.

### Auth

A `tokenSource`:

- performs the password grant on first use, holds `access_token` /
  `refresh_token` / expiry behind a `sync.Mutex`;
- refreshes with `grant_type=refresh_token` when `time.Now()` is within a skew
  of expiry, and once on a `401`;
- is `context`-aware and lives on the `*Client`;
- attaches the token verbatim as `Authorization` on every request.

`golang.org/x/oauth2` is an option but not required — this grant is ~80 lines of
stdlib. Decide once a live account confirms the exact token response.

### Transport

- One `*http.Client` with an explicit `Timeout`.
- A `do(ctx, method, path, body)` helper attaches the token, sets `Accept:
  application/json` (and `Origin` if the client id requires it), caps the
  response body, and on `429` / `5xx` retries with exponential backoff,
  honouring `Retry-After`, bounded by `ctx`.
- Every exported method takes `ctx context.Context` first.

### Wire format

`encoding/json` with typed structs named for the schema entities. Keep
encode / decode behind one function each. The typed structs are the package's
public surface; callers never see raw bytes.

### Config

- **Non-secret** — `BaseURL` (`https://api.regattacentral.com/v4.0/`, per
  machine), `RegattaID` (per regatta) — from the deployment
  [persona-config-file.md](persona-config-file.md) JSON. Add a `RegattaCentral`
  field group to `personacfg.Config`
  ([`internal/personacfg/personacfg.go`](../../../internal/personacfg/personacfg.go)).
- **Secret** — `client_id`, `client_secret`, `username`, `password` — from
  `internal/secretstore` only.

### Methods

Read (return typed structs):

- `Bulk(ctx, regattaID)` — the whole regatta: events, entries, athletes,
  organizations.
- `Events` / `EventEntries(eventID)` / `EventLanes(eventID)` /
  `EventRaces(eventID)` / `EventResults(eventID)` — targeted reads and
  read-backs.
- `ActiveRaces(ctx, regattaID)` — races currently "In Progress" (a polling
  hook).
- `Organizations(ctx, regattaID)` / `SearchOrganizations(ctx, name)` /
  `SearchParticipants(ctx, lastname, birthdate)`.

Write:

- `Upload(ctx, regattaID, payload)` — one combined body. Plus builders for
  `events` / `races` / `lane` / `result` records and a `flush` flag (regatta or
  per-race).
- Idempotent by construction — a re-upload overwrites — so re-publishing after a
  correction is safe.

### Capability split

The package exposes one `*Client`. A read persona calls only the getters — a
documented convention, not an enforced boundary; RegattaCentral gates by the
operator's **staff** privilege, not by token scope.

## `internal/secretstore` — OS keyring wrapper

A new **leaf** package: a thin wrapper over a cross-platform keyring library
(e.g. `github.com/zalando/go-keyring` → macOS Keychain, Windows Credential
Manager, freedesktop Secret Service):

```go
func Get(service, key string) (string, error)
func Set(service, key, value string) error
func Delete(service, key string) error
```

- `service` is `"regattaClock"`; keys are `regattacentral/client_id`,
  `regattacentral/client_secret`, `regattacentral/username`,
  `regattacentral/password`.
- This is the codebase's **first dependency of its kind** and its **first stored
  secret** — call that out in the PR that adds it.
- Headless-Linux fallback: when no Secret Service is present, return a typed
  `ErrUnavailable`; the caller may then read from `RC_*` env vars or an
  operator-provided file (a `personacfg` toggle), matching the options weighed
  in [sidecar-personas.md](sidecar-personas.md#config-and-secrets).
- CI: keyring round-trip tests `t.Skip` when no backend is available.

This is the concrete answer to the "Config and secrets" open item in
[sidecar-personas.md](sidecar-personas.md#config-and-secrets); Social Post
(sidecar Increment 2) reuses it for its OAuth token.

## `cmd/rcprobe` — developer CLI (Phase A)

A thin, **unshipped** `main` over `internal/regattacentral` +
`internal/secretstore`. Because the client is Fyne-free, a small CLI is the
fastest way to exercise it against the live API before any UI exists, and it is
what de-risks Phase A — confirming the token response, the JSON shapes, whether
`/bulk` carries per-seat eligibility, rate limits, and the `Origin` / referer
requirement.

- **Subcommands:** `token` (acquire, print expiry + refresh), `bulk
  <regattaID>`, `entries <regattaID> <eventID>`, `orgs <name>`, `participants
  <lastname> <yyyy-mm-dd>`; later `upload --dry-run <file>` (build and print the
  combined payload without sending).
- **Credentials** from `internal/secretstore` (same keys as the app) or `RC_*`
  env vars — never flags, never hard-coded. `BaseURL` / `RegattaID` from a
  `personacfg` file or flags.
- **`--out <dir>`** writes each raw JSON response to disk. Those captures become
  the **Phase A test goldens** (`/bulk`, event entries, organizations), which
  otherwise cannot be written without a real response.
- **Not released.** `release.yml` packages named binaries, not `./...`, so it is
  excluded automatically; CI's `go build ./...` still compiles it. Keep it
  in-tree as a debugging aid after Phase B, or delete it — maintainer's call.
- Precedent: the Cookbook (§3) notes RegattaCentral hands Java teams a
  request-builder utility class; a probe harness is expected practice for this
  API.

## Read integration

### Phase B — roster ingest and an in-app roster view

- A thin adapter maps the `/bulk` `Entry` + athlete records onto an in-app view
  model: athletes grouped by boat / entry, each with boat class, an eligibility
  flag, and a team; regatta name / date from the same response.
- A read-only panel — a sub-view under `internal/regatta`, or a small package it
  imports — renders that model. This is the **Heat Sheet Author seam**: a
  reference panel that replaces the trip to regattacentral.com. The RD still
  authors the heat sheet in the Excel worksheet.
- **The Excel ingest is unchanged in Phase B** — `reader.ReadExcelFile`,
  `store.Origin{Type: "excel"}`, and the origin-refresh poll in
  [`internal/regatta/origin.go`](../../../internal/regatta/origin.go) are not
  touched.
- The fetch is on demand (an explicit "Load roster from RegattaCentral"
  action), `context`-scoped, on its own goroutine, rendered via `fyne.Do`,
  best-effort — a failure is a `WARN` line plus a non-blocking notice, never a
  modal.

### Phase C — in-app heat-sheet authoring

Retires Excel; gets its own persona doc. The reframe: authoring produces a
local schedule **and** seeds RegattaCentral.

- The RD / Heat Sheet Author assembles races and lanes from the `/bulk` roster
  in-app. The app builds a `store.Schedule` directly, and derives the in-memory
  `*reader.RegattaData` the race tree and clock render from **through the single
  existing entry point** — `regattaDataFromSchedule` in
  [`internal/regatta/schedule.go`](../../../internal/regatta/schedule.go), which
  already calls `reader.NewRegattaData()`
  ([`internal/reader/regattaData.go`](../../../internal/reader/regattaData.go)).
  **Do not** hand-roll a `&reader.RegattaData{…}` literal or add a second
  constructor: `reader.NewRegattaData` is the one way to establish a
  `RegattaData` (used by the Excel reader, `regattaDataFromSchedule`,
  `persona_config.go`, and `regatta.go` today), and the heat-sheet origin joins
  that list rather than forking it.
- New origin `store.Origin{Type: "heatsheet"}` on the generalised
  `ScheduleOrigin` interface sketched in
  [persona-plan.md](persona-plan.md#keep-excel-out-of-the-long-term-core-origin-adapter)
  (`Fingerprint()` / `Load()`), fingerprinted by
  `store.Schedule.ContentHash()`, dispatched alongside `"excel"` in
  `setRegattaData`
  ([`internal/regatta/loader.go`](../../../internal/regatta/loader.go)) and
  `pollOrigin`
  ([`internal/regatta/origin.go`](../../../internal/regatta/origin.go)). The
  Excel path refactors onto the same interface.
- **RegattaCentral ids / UUIDs are carried on the schedule model** (new fields
  on `store.ScheduleRace` / `store.ScheduleEntry`, or a sidecar map keyed by
  race + lane) so a subsequent `Upload` can reference existing events / entries
  by id and new ones by the UUID regattaClock assigned. Exact placement is an
  [open item](#open-items).
- On author / apply, a best-effort `Client.Upload` pushes `events` / `races` /
  `lane` records and advances race `status` `PreDraw` → `Draw`. Off the click
  path, outbox-style (see [Write integration](#write-integration--register-results-phase-d)),
  `WARN` + non-blocking notice on failure.
- Re-fetching `/bulk` (new registrations, scratches, eligibility changes) raises
  an attention banner in the builder, reusing the RD's existing `pendingOrigin`
  / `applyPendingOrigin` / `dismissedContentHash` Apply / Dismiss affordance and
  the "don't rewrite on an unchanged content hash" rule from
  [persona-plan.md](persona-plan.md) §3b.
- The RegattaCentral roster is an **input** to authoring, not itself the origin.
- Excel is retired for an event once its heat sheet is authored in-app;
  authoring then replaces both the web-UI copy-paste and the Excel worksheet.
- This phase **defines the Heat Sheet Author persona** (Executive team,
  `persona.Role`, `Definition`, challenge) — out of scope here beyond naming the
  seam.

## Write integration — Register Results (Phase D)

The concrete build-out of [sidecar-personas.md](sidecar-personas.md)
**Increment 1**, now expressed against the Cookbook.

- `internal/publish` gains
  `type Target interface { Name() string; Publish(ctx context.Context, r PublishableRace) error }`.
- A `regattacentralTarget` implements it. `Publish` calls `Client.Upload` with
  the `lane:` records (draw + per-entry status) **then** the `result:` records —
  never the other order.
- The **outbox** — a bounded channel plus a retry-with-backoff worker, the
  [`internal/applog`](../../../internal/applog/) async-writer pattern — drains
  the queue. A per-race **Register** button only enqueues. Never on a click,
  never on the timing path; a failure is a `WARN` plus a non-blocking in-panel
  notice.
- **Field mapping** `store.RaceResult` → RegattaCentral:
  - race number + lane number are the keys on every `result:` record;
  - per-lane finish time → `result:` with `timingMilestoneId = 4`, `time`
    cumulative; mid-splits (if ever collected) → milestones `1`–`3`;
  - `DQ` / `DNF` / `DNS` → the `lane:` entry `status` (`DSQ` / `DNF` / `DNS`);
  - `Approved` (the publish signal) → race `status: "Official"`, optionally
    `"flush": true` for that race followed by a single full re-upload
    (Cookbook §10); `ApprovedAt` is available, `Envelope.Machine` for
    provenance. Note: no approver **name** is stored anywhere today.
- Published-state `{raceNumber: revision}` per
  [sidecar-personas.md](sidecar-personas.md#what-to-reserve-now-no-code) /
  [future-result-driven-persona.md](future-result-driven-persona.md), so a
  later edit to an approved race re-flags its row for re-publish.

## Non-disruption rules (first outbound HTTP)

regattaClock has no outbound network today. These rules apply to every phase and
generalise to Social Post:

- Every call is `context`-scoped. The client's background work — token refresh,
  roster fetch, outbox worker — runs on **its own goroutine under its own
  `context.CancelFunc`**, tied to the window / app lifecycle, the way
  `startWatcher` and the clock ticker already do. UI updates go through
  `fyne.Do`.
- All network I/O is **best-effort and off every click handler**.
- Failures are a `WARN` line plus a non-blocking notice — **never a modal, never
  on the timing path**.
- Retry with backoff, honouring `Retry-After`. Offline degrades gracefully: a
  read retries on the next poll; a write waits in the outbox.
- The session token travels in `Authorization`; TLS 1.2+ is mandatory; a
  registered client id may require an `Origin` header.
- [`internal/timesync`](../../../internal/timesync/) is untouched — the
  RegattaCentral session token has nothing to do with NTP.

## Phasing

| Phase | Scope | New surface |
|-------|-------|-------------|
| **A** | `internal/regattacentral` (auth, transport, typed JSON model, read + `Upload` methods), `internal/secretstore`, `personacfg` `RegattaCentral` config group, and `cmd/rcprobe`. No UI. | `internal/regattacentral`, `internal/secretstore`, `cmd/rcprobe`, `personacfg.Config` field |
| **B** | Roster adapter → in-app view model; a read-only "roster from RegattaCentral" reference panel. RD still authors the heat sheet in Excel; the Excel ingest is unchanged. | a roster adapter, a panel under `internal/regatta` |
| **C** *(own persona doc)* | In-app heat-sheet authoring from the roster → `store.Schedule` (and `RegattaData` via `regattaDataFromSchedule` / `reader.NewRegattaData`), plus a best-effort `Upload` of events / races / lane draws and `PreDraw` → `Draw` status. New `store.Origin{Type: "heatsheet"}` on the generalised `ScheduleOrigin` interface, dispatched in `loader.go` / `origin.go`. Retires Excel. Defines the Heat Sheet Author persona. | `ScheduleOrigin` interface, RC id/UUID fields on the schedule model, an authoring UI, a new `persona.Role` |
| **D** | Register Results: `Client.Upload` of `lane:` then `result:` records + status lifecycle + `flush`, wired through `internal/publish` `Target` + outbox, as [sidecar-personas.md](sidecar-personas.md) Increment 1. Depends on A. | `internal/publish` `Target`, `regattacentralTarget`, an outbox |

Phases A and B retire nothing; the Excel origin stays untouched. B, C and D each
depend only on A for the client and secret store.

## Tests to require

Phase A (no live API in CI):

- `httptest`-backed: token acquisition via the password grant, refresh via the
  refresh token, refresh-on-`401`.
- `429` / `503` retry with a fake clock; `Retry-After` respected; `ctx`
  cancellation stops the retry loop.
- JSON unmarshal goldens (captured with `cmd/rcprobe --out`) for `/bulk`,
  `Entry` + athletes, `Organization`.
- `Upload` serialises events / races / `lane` / `result` into one combined
  body; a `flush` flag round-trips at regatta and per-race scope.
- `internal/secretstore` get / set / delete round-trip, `t.Skip` where no
  keyring backend is present.

Phase B:

- The roster adapter against an `httptest` server produces the expected in-app
  view model (athletes grouped by entry, eligibility and team carried through).
- The panel renders an empty roster without panicking; a fetch failure shows the
  notice, not a modal.

Phase C:

- `store.Origin{Type: "heatsheet"}` round-trips through
  `scheduleFromRegattaData` / `regattaDataFromSchedule`; `pollOrigin` and
  `setRegattaData` dispatch on the type; `ContentHash()` is stable across a
  no-op re-author.
- The authored schedule carries RegattaCentral ids for existing entities and
  UUIDs for new ones; the `Upload` payload references each correctly.
- Lane records precede result records in any generated upload.

Phase D:

- Outbox retries then succeeds against an `httptest` server; a `4xx` is
  surfaced non-fatally and does not wedge the queue.
- The `store.RaceResult` → `result:` / `lane:` field map, table-driven,
  including `DQ` / `DNF` / `DNS` → `DSQ` / `DNF` / `DNS` and `Approved` →
  `status: "Official"`.
- `timingMilestoneId = 4` is the finish; `time` is cumulative; `time: 0`
  clears.

## What already exists to build on

- [`internal/timesync`](../../../internal/timesync/) — a self-contained
  process-level service with a background re-query goroutine and its own config:
  the structural model for a session-token client.
- [`internal/applog`](../../../internal/applog/)'s async writer — the reference
  for a bounded, non-blocking queue (the outbox).
- [`internal/personacfg`](../../../internal/personacfg/personacfg.go) —
  `Config` + `Load(path)`, the home for non-secret RegattaCentral config.
- `reader.NewRegattaData()` in
  [`internal/reader/regattaData.go`](../../../internal/reader/regattaData.go) —
  the **only** constructor for `*reader.RegattaData`; every origin builds
  through it.
- `scheduleFromRegattaData` / `regattaDataFromSchedule` in
  [`internal/regatta/schedule.go`](../../../internal/regatta/schedule.go) — the
  `reader.RegattaData` ↔ `store.Schedule` projection, already type-agnostic on
  `Origin.Type`, and already the `reader.NewRegattaData` call site for a
  non-Excel origin.
- `setRegattaData` in
  [`internal/regatta/loader.go`](../../../internal/regatta/loader.go) — the
  single caller of `reader.ReadExcelFile`; the dispatch point for a second
  origin.
- `pollOrigin` / `applyPendingOrigin` / `pendingOrigin` /
  `dismissedContentHash` in
  [`internal/regatta/origin.go`](../../../internal/regatta/origin.go) — the RD's
  detect → load → compare → Apply / Dismiss flow, reused verbatim for any
  origin.
- `store.Schedule.ContentHash()` in
  [`internal/persona/store/schedule.go`](../../../internal/persona/store/schedule.go)
  — the canonical schedule fingerprint (excludes `Origin`).
- `startWatcher` in
  [`internal/regatta/persona_startup.go`](../../../internal/regatta/persona_startup.go)
  and the per-race clock window — precedents for an independent sub-context that
  survives `switchPersona`.

## Open items

Resolve before Phase A code (confirm against a live staff account):

- Exact `Authorization` value format (raw token vs `Bearer <token>`); whether a
  desktop client needs the `Origin` / referer header, and how that referer is
  registered.
- Whether the `/bulk` payload carries **per-seat athlete eligibility / rowing
  certification** and **team affiliation** at field level (the RD expects both;
  the Cookbook does not spell the fields out).
- **Rate limits / quotas** — undocumented.
- Whether an **API-Key** (vs the password grant) is viable for a
  mostly-unattended race-day service, and how it is provisioned.
- **Keyring library** choice and the headless-Linux fallback (`RC_*` env vars
  vs operator file).

Phase C / later:

- Where RegattaCentral ids / UUIDs live on the schedule model (fields on
  `store.ScheduleRace` / `store.ScheduleEntry` vs a sidecar map).
- Heat-sheet authoring UX, and whether the in-app schedule must still be
  exportable to the RD's Excel format during the transition.
- Whether a dedicated Standalone "Heat Sheet Author" / "Publisher" machine is
  ever wanted (ties to the Standalone-persona question in
  [sidecar-personas.md](sidecar-personas.md#open-decisions)).
- **Follow-up doc edit:** [persona-plan.md](persona-plan.md) still describes
  RegattaCentral as "a roster source … not a schedule origin" (§ "Non-Excel
  schedule origin" and the `ScheduleOrigin` scope note). Reconcile it with this
  doc's two-way framing.
