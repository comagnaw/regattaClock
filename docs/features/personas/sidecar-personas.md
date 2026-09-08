# Sidecar personas — publishing sub-tasks

How an operator who is already timing or directing a regatta also runs a **publishing
sub-task** — post a race's results to social media, register results with an external
system — without a second login and without disturbing their primary job.

Companion to [future-result-driven-persona.md](future-result-driven-persona.md) (assesses
*standalone* read-only publisher personas — this doc refines that into an attach-to-a-lead
model), [reconciliation.md](reconciliation.md) (the future *producer* of the published
set), and [persona-plan.md](persona-plan.md) §1 (the timer-priority rule a sidecar
inherits) and §13.

**Status:** design assessment + phased build plan. No code yet.

**Recommendation up front.** Build a **sidecar capability** seam: a publishing task
attached to whichever **lead** persona is logged in (Regatta Director or a Timer), never
its own `persona.Definition` / `Team` / `Challenge` / `Session`. The first version
**renders an artifact for a human** — a results text table to the clipboard or a file, a
PNG via `internal/exporter` — with **no network and no credentials**. Automated posting,
and a dedicated *standalone* "Publisher" persona for a machine that does nothing else, are
later increments that reuse the same seam. Defer the materialized `regattaData/results/`
view (unchanged from [future-result-driven-persona.md](future-result-driven-persona.md)).

## The scenario

In our regatta operations these are real duties, done by the person already at the
keyboard:

- The **Secondary Finish Timer** also posts each race's results to X.
- The **Primary Finish Timer** also registers the results with the timing/results service.
- The **Regatta Director** (or any lead) could do either.

They are **side-duties of one operator on one machine**, not separate operators. And
unlike timing, they need technical configuration the timing personas never had: an
endpoint URI, an account handle, and — for automated posting — credentials.

## Persona classification

Every persona is a **Lead** or a **Standalone**. A **Sidecar capability** is a third
thing that is *not* a persona.

| Class | Examples | Hosts a sidecar? | Is a sidecar? |
|-------|----------|------------------|---------------|
| **Lead** | Regatta Director; Primary/Secondary Start & Finish Timer | yes — at most one | no |
| **Standalone** *(non-lead)* | Streaming *(unscoped)*; a dedicated "Publisher" persona *(optional, later)* | no | no |
| **Sidecar capability** *(not a persona)* | `social-post`, `register-results` | — it *is* the attachment | onto a **Lead** only |

- A **Lead** has a primary duty that is timing or directing. It may additionally enable
  **one** publish sidecar. It is never itself a sidecar.
- A **Standalone** persona is chosen at startup and is the entire job for that app
  instance. It **does not participate in the sidecar mechanism in either direction** — it
  neither hosts a sidecar nor is one. **Streaming** is the worked example: whenever it is
  scoped, it manages its own external surface (stream / overlay) and hosts nothing. A
  dedicated **Publisher** persona (below) is also Standalone — its job *is* one publish
  capability, run alone.
- A **Sidecar capability** never runs alone and never nests.

**Invariant:** one persona per process; a Lead may carry one sidecar; Standalone personas
and sidecar capabilities never combine — at most one publish task per process, and a
publisher never side-cars another publisher. Every future persona (Streaming, Developer, a
Publisher) is classified Lead or Standalone when it is scoped, and **Standalone is the
default** for a new self-contained persona. That single choice settles, once, whether the
persona touches the sidecar mechanism at all.

## What a sidecar is

A capability attached to a **Lead** session, with four parts:

1. **Input** — a read-time "publishable races" view, built from `finish.json` +
   `regattaSchedule.json` (see [What it consumes](#what-it-consumes)).
2. **Transform** — render text / render an image now; call an external API later.
3. **Target config** — endpoint URIs and options now; credentials once automated.
4. **Per-race action + published-state** — a *Publish* / *Post* button per race, plus a
   `{raceNumber: revision}` record of what was published, so a later edit to an approved
   race flags the row for re-publish.

It is opt-in, toggled from a menu on the lead's window, and lives in its **own panel or
window**. It is not a `persona.Definition`, not a `Team`, not a challenge, not a second
`persona.Session`.

## Why a capability, not a new persona (yet)

The runtime is strictly single-persona-per-process. `Regatta.session` is one slot read
unconditionally throughout `internal/regatta`; `internal/applog` and `internal/timesync`
are process singletons with one identity and one output file; there is one shared-file
watcher with one `stopWatcher` closure and one `SetMaster()` window
(`internal/regatta/regatta.go`, `internal/regatta/persona_startup.go`). Attaching the
publish work to the lead's existing session avoids a second identity, a second log stream,
and a new `persona.Session.WritePath()` branch.

The **Standalone "Publisher" persona** — Executive team, read-only, its own `Role` and
challenge; the shape sketched in
[future-result-driven-persona.md](future-result-driven-persona.md) and reserved by the
greyed-out **Media** tab buttons (`Register Results`, `Social Media`) — stays a
**documented later option**: an RD-style read-only oversight tree plus exactly one publish
capability chosen at launch, for a machine that only publishes. It is a Standalone per the
classification above and attaches nothing further. Framework first; that persona later, if
a publish-only machine is actually wanted.

## Non-disruption rules

A sidecar sharing a process with a Finish Timer inherits [README.md](README.md) "Timer
priority — Collecting times from button clicks is the highest-priority work". Concretely:

- Its background work runs on **its own goroutine(s)** under **its own
  `context.Context`**, cancelled when its window closes or the app shuts down — the
  pattern of `startWatcher` and the clock ticker. UI updates go through `fyne.Do`.
- All I/O is **best-effort and off every click handler**. The per-race action
  **enqueues**; a worker drains. The model is `internal/applog`'s async writer: a bounded
  queue that drops / counts overflow rather than blocking.
- Failures are a `WARN` line plus a **non-blocking in-panel notice** — never a modal, and
  never on the timing path.
- It has **its own window or panel** and never joins the race-tree refresh cycle. The
  per-race clock window is the precedent: an independent sub-context carrying its own
  copied `session`, which already survives `switchPersona`.
- (Automated increment) a network call is **never** on a click; it is the worker's job,
  with retry and backoff.

## What it consumes

Reuse the analysis in [future-result-driven-persona.md](future-result-driven-persona.md)
§B:

- Watch `timing/primary/finish.json` (and `timing/secondary/finish.json` later) plus
  `director/regattaSchedule.json`.
- Join by `RegattaKey` + `RaceNumber`. `store.RaceResult.Approved` flipping true is the
  publish signal.
- `RaceResult.UpdatedAt` after `RaceResult.ApprovedAt`, or a changed `RaceResult.LaneMapHash`,
  means "approved result changed — re-publish".
- Build the view the way the Director tree already does:
  `internal/regatta/director_tree.go` (`hydrateDirectorLogs`, `applyDirectorTimingEvent`),
  with the lane → school join in `internal/regatta/schedule.go`.

**Defer `regattaData/results/` materialization.** For a single-pair regatta — the common
case — "approved primary rows" is a sufficient publishable set. Full reconciliation
(primary vs secondary, `disputed`, provenance) stays with the future producer persona in
[reconciliation.md](reconciliation.md); a sidecar consumes whatever that persona
eventually publishes, or `finish.json` directly until then.

## Config and secrets

Two kinds, kept apart:

- **Non-secret config** — endpoint URIs, account handles, message templates, "which
  platform". Per-machine, **not** in the synced `regattaData/` tree. Options: extend the
  deployment [persona-config-file.md](persona-config-file.md) JSON, a new small local
  JSON, or `Preferences` keys. The render-only first version needs essentially none.
- **Secrets** — API keys, OAuth tokens. Only once automated push exists. **Never in the
  synced tree; never in Fyne `Preferences`** — Fyne preferences are an unencrypted JSON
  file at `~/Library/Preferences/fyne/<bundleID>/preferences.json` (and the XDG / AppData
  equivalents), per-machine and world-readable-by-owner.

| Secrets option | For | Against |
|----------------|-----|---------|
| **OS keyring** *(recommended)* — macOS Keychain / Windows Credential Manager / libsecret, via a cross-platform library (e.g. `zalando/go-keyring`) | encrypted at rest, OS-managed, no plaintext | first dependency of its kind; headless Linux needs a keyring daemon |
| Local `0600` file outside the tree, path in a preference | simple, matches the `personacfg` precedent | plaintext at rest |
| App never stores — read from an env var or an operator-provided file the app never writes | least secret-handling code in the app | the deploying org must wire up secret delivery |

Automated publishing would be the codebase's **first outbound HTTP dependency and first
secret storage** — call that out in the increment that introduces it.

## Recommended increments

| # | Scope | New surface |
|---|-------|-------------|
| **0** | Framework + **render-only**: capability seam, a menu toggle on a **Lead** session, per-race render to clipboard/file (text) and PNG (`internal/exporter`, extended to draw places + times), `{raceNumber: revision}` tracking. **No network, no secrets.** | `internal/publish`, `internal/regatta/sidecar.go`, `internal/exporter` addition, `menu.go` |
| **1** | **Register Results** (automated) — push to the results service. Forces the config + secrets decision, an outbox/retry design, and a field-mapping spec. Own follow-up doc. | `internal/publish` `Target`, `internal/secretstore`, an outbox, `net/http` |
| **2** | **Social Post** (automated) — X API: OAuth, media upload, rate limits. Own doc. | `golang.org/x/oauth2`, media upload |
| later | **Standalone "Publisher" persona** (Media tab) for a publish-only machine — one capability, chosen at launch, attaches nothing. | a new `persona.Role`, a `Definition`, a picker entry, `startPublisherFlow` |
| deferred | `regattaData/results/` materialization, reconciliation verdict, `disputed` handling — unchanged. **Streaming** and **Developer** — Standalone; each scoped in its own later doc. | — |

## Implementation steps

Phased like [persona-plan.md](persona-plan.md) §10. **Increment 0 in detail**; 1–2 and the
Standalone persona sketched.

### Phase 0a — `internal/publish` leaf package

Imports `internal/persona/store`, `internal/reader`, `internal/filesystem`. No Fyne, no
`internal/regatta`.

```go
type Row struct {
	Place, Lane, School, AdditionalInfo, Time string
}

type PublishableRace struct {
	RaceNumber                       int
	Title, RegattaKey, WinningTime   string
	Rows                             []Row
	ApprovedAt, SourceUpdatedAt      time.Time
	LaneMapHash, Revision            string
}

func BuildView(sch *store.Schedule, fin *store.FinishLog) []PublishableRace
func Revision(r PublishableRace) string
func RenderText(r PublishableRace) string
```

- `BuildView` joins by `RaceNumber`, keeps `RaceResult.Approved`, joins
  `RaceResult.Rows []store.LapRow` to `ScheduleRace.Lanes` by lane for school /
  additional-info, derives `Title` from `ScheduleRace` (`RaceNumber` / `BoatClass` /
  `FlightInfo`) and `RegattaKey` from `store.RegattaKey(sch.Name, sch.Date)`.
- `Revision` hashes only the **visible** fields — ordered `(place, lane, school, time)`
  per row plus `winningTime` and `title` — per the re-publish contract in
  [future-result-driven-persona.md](future-result-driven-persona.md). Reuse
  `filesystem.HashBytes`. An incidental `UpdatedAt`-only write must not move it.
- `RenderText` produces the plaintext table (the "Target artifact" in
  [future-result-driven-persona.md](future-result-driven-persona.md)).

#### Tests to require

- Table-driven `BuildView`: join hit / miss, approved-filter, a race missing from the
  schedule, `DQ` / `DNF` / `DNS` rows, unassigned lane (`Lane == 0`).
- `Revision` stable across an `UpdatedAt`-only re-write; changes on a place / time /
  school edit; order-independent inputs give the same hash.
- `RenderText` golden strings.

### Phase 0b — image render in `internal/exporter`

Add `func RenderResult(r publish.PublishableRace) (image.Image, error)` (or a sibling
`results.go`) reusing `newDrawingContext` / `drawText`. Leave `Export` (the lane-sheet
path) untouched.

#### Tests to require

- Non-nil image of the expected dimensions; no panic on an empty `Rows`.

### Phase 0c — sidecar lifecycle, `internal/regatta/sidecar.go`

```go
type sidecar struct {
	capability string
	cancel     context.CancelFunc
	win        fyne.Window
	rows       map[int]*sidecarRow
	published  map[int]string // raceNumber -> last-published Revision
}
```

- New `Regatta.sidecar *sidecar` field — nil unless one is open; **at most one**.
- `func (r *Regatta) isLead() bool` — `r.mode == modeDirector || r.mode == modeTimer`.
- `func (r *Regatta) openSidecar(capability string)` — guarded by `r.isLead()`; derives the
  read root from `r.session.Root` (or `directorSession()`); starts **its own**
  `context.WithCancel` and an `internal/watcher` on `SchedulePath()` and the primary
  `FinishPath()`. Each event, via `fyne.Do`: reload `store.LoadSchedule` / `LoadFinish` →
  `publish.BuildView` → refresh rows. Each row carries **Copy**
  (`r.window.Clipboard().SetContent(publish.RenderText(pr))`), **Save image…**
  (folder pick → `exporter.RenderResult` → PNG), and a re-publish mark when
  `published[n] != publish.Revision(pr)`.
- `func (r *Regatta) closeSidecar()` — `cancel()`, `win.Close()`, nil the field. Called
  from the sidecar `win.SetOnClosed`, from `switchPersona` (extend it to close an open
  sidecar), and on main-window close.
- Published-state: preference key `common.PrefPublishedFormat` (`"Published:%s:%s"` with
  `regattaKey`, `capability`) holding JSON `map[int]string`. Load in `openSidecar`, save
  after a Copy / Save action.

#### Tests to require

- `openSidecar` from a Timer or Director session builds one row per approved race;
  it is a no-op when `!isLead()`.
- A Copy updates the preference and clears that row's re-publish mark.
- A later `finish.json` edit that bumps `Revision` re-flags the row.
- `closeSidecar` cancels the context with no `-race` fallout; `switchPersona` closes an
  open sidecar.

### Phase 0d — menu + strings

`internal/regatta/menu.go` gains `publishMenu()` — items shown only when `r.isLead()`, one
per capability (`common.SidecarSocialLabel`, `common.SidecarRegisterLabel`) calling
`openSidecar`, plus "Close publish panel" when `r.sidecar != nil`. New `internal/common`
constants: capability IDs, menu labels, `PrefPublishedFormat`.

#### Tests to require

- The publish items are present for `NewDirector` / `NewTimer`, absent on the bare picker.

### Phase 0e — docs

Update this file's status to "phase 0 shipped". `README.md` and the picker are unchanged —
no new persona.

### Increment 1 sketch — Register Results

- `internal/publish`: `type Target interface { Name() string; Publish(ctx context.Context, r PublishableRace) error }`;
  an HTTP `Target` (`net/http`; `httptest` in tests).
- An **outbox**: a bounded channel plus a retry-with-backoff worker (the `internal/applog`
  async-writer pattern), so a per-race *Register* click only enqueues.
- `internal/secretstore`: a thin wrapper over an OS-keyring library for the API key.
- Non-secret config (endpoint URI + a field map) via an extended `personacfg` file or a
  new local JSON. The field map covers race number, school, lane, place, time,
  `DQ`/`DNF`/`DNS`, the approved flag, `ApprovedAt`, and `Envelope.Machine` — note no
  approver *name* is stored anywhere today.

#### Tests to require

- Outbox retries then succeeds against an `httptest` server; a `4xx` is surfaced
  non-fatally and does not wedge the queue.
- `secretstore` get / set round-trips (skipped where no keyring is available).

### Increment 2 sketch — Social Post

`golang.org/x/oauth2`, media upload for the PNG, rate-limit backoff on the same outbox.
Own doc.

### Standalone "Publisher" persona sketch (optional)

Add a new `persona.Role` (name TBD) and a `Definition`; `Session.WritePath()` returns
`""` / an error for it; wire a Media-tab picker button; `startPublisherFlow` shows an
RD-style read-only oversight tree plus exactly one panel for the capability chosen at
launch. It is Standalone — it never calls `openSidecar` on top of another persona. Reuses
all of `internal/publish`.

## What already exists to build on

- `internal/exporter` — folder-pick → build artifact → summary dialog; freetype PNG.
  Consumes the schedule today; `RenderResult` extends it to `store.RaceResult`.
- The watcher + join + per-row refresh in `internal/regatta/director_tree.go`
  (`hydrateDirectorLogs`, `applyDirectorTimingEvent`, `directorWatchPaths`) and
  `startWatcher` in `internal/regatta/persona_startup.go`.
- The lane → school join in `internal/regatta/schedule.go`.
- `store.RaceResult` / `store.LapRow` in `internal/persona/store/log.go`.
- `internal/applog`'s async writer as the reference for a non-blocking queue.
- The per-race clock window as the precedent for an independent sub-context that survives
  `switchPersona`; and `switchPersona` itself for teardown.

## What to reserve now (no code)

- **The classification** — Lead / Standalone / Sidecar capability — applied to every
  persona, present and future.
- A **capability-id vocabulary**: `social-post`, `register-results` (and room for more).
- **Published-state** location: a local preference keyed by `regattaKey` + capability,
  holding `{raceNumber: revision}` — mirrors
  [future-result-driven-persona.md](future-result-driven-persona.md).
- **Log lines** `component:"publish"` / `action:"publish"` on the lead persona's stream
  (a sidecar has no identity of its own).
- **Invariant:** a sidecar **never writes** `timing/`, `director/`, or anything in the
  synced tree — it only reads and emits outward.
- **Invariant:** a Lead may carry one sidecar; a Standalone persona carries none and is
  none.
- The Media-tab `Register Results` / `Social Media` buttons are sidecar capabilities (and,
  standalone, the Publisher persona) — **not** new personas yet. `Streaming` (Standalone,
  hosts no sidecar) and `Developer` stay unscoped.

## Open decisions

- **Which Leads may toggle a sidecar** — any Lead, or Finish-Timers only to match today's
  duties. (The Lead / Standalone / Sidecar split itself is settled.)
- Non-secret config location — extend `personacfg`, a new local JSON, or preferences.
- Keyring library choice and the headless-Linux fallback.
- Automated push: a synchronous outbox with retry, or fire-and-forget.
- Whether the Standalone "Publisher" persona is ever built.
- Per-machine vs per-regatta config.
- Render-for-a-human as the standing v1 line (recommended) vs going straight to automated
  push.
