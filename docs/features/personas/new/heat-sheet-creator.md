# Heat Sheet Creator (HSC)

A **standalone persona** (Executive team, Admin UI), not a sidecar to any
other persona. Formalizes the "Heat Sheet Author" persona
[regattacentral-integration.md](../regattacentral-integration.md)'s Phase
C already named and left as a bare seam (`store.Origin{Type:
"heatsheet"}`, the generalized `ScheduleOrigin` interface) without ever
specifying who runs it, what its workflow is, or how it stages relative
to that phase's own still-open items. This doc is that specification.

**This is a genuinely risky, unproven transition** — it touches the RD's
own view of regatta data and depends in part on a live RegattaCentral
write API this project is still reverse-engineering (see
[heatsheet-rc-pivot-investigation.md](../heatsheet-rc-pivot-investigation.md),
currently blocked on an unresolved `HTTP 400` from RC, awaiting RC
support). Per the author, HSC does **not** block implementation of
Awards, Developer, Results Publisher, SOM, or Streamer — none of their
own scope depends on it — but its eventual framework (an RC-sourced
schedule origin, RC ids living on the schedule model) is worth keeping
in view for those docs' own later, RC-dependent slices. See "Dependencies
and sequencing" below.

## Decisions made (author, 2026-09-16)

- **Shape:** standalone persona, not a sidecar — matches the author's
  explicit instruction and resolves
  [sidecar-personas.md](../sidecar-personas.md)'s own open "whether a
  dedicated Standalone 'Heat Sheet Author' machine is ever wanted"
  question, and
  [regattacentral-integration.md](../regattacentral-integration.md)'s
  matching Phase C open item, the same way.
- **Editing UI: not decided by discussion — decided by a feasibility
  spike.** The author floated two shapes — an in-app table/draggable-cell
  authoring UI (Phase C's original sketch, retires Excel) vs. generating
  a formatted Excel Heat Sheet that executives keep editing by hand, with
  HSC reconciling hand-edits back against its RC-sourced record — and
  the choice hinges entirely on whether Fyne can actually deliver a
  usable draggable-cell table, which nobody has tried yet. Rather than
  guess, or find out mid-build, the author wants a small, throwaway Fyne
  mock-up built **first, separately from HSC itself**, specifically so
  HSC's own implementation is never blocked "figuring this out" — see
  "Fyne feasibility spike" below. This doc designs the RC-pull /
  local-storage / indexing / deadline-finalize layer **UI-agnostically**
  regardless of the spike's outcome — either editing UI sits behind it
  unchanged.
- **RC id / UUID indexing: fields directly on the schedule model.**
  Resolves [regattacentral-integration.md](../regattacentral-integration.md)'s
  own open item ("fields on `store.ScheduleRace` / `store.ScheduleEntry`
  vs a sidecar map") — new fields on those two types, not a separate
  index file. See "Existing-code reuse analysis" for what changes.
- **Staged: HSC v1 (read-only), HSC v2 (write-back), HSC v3
  (progressions) — three separate increments.** v3, added after a
  reference document surfaced heat→semi/final progression seeding (see
  "Progressions — heats to semis and finals" below), is **not** folded
  into v1 despite v1's own "continuously update the Heat Sheet, even on
  day of the Regatta" language — it needs live `finish.json` access v1
  never required, so it stays a distinct, later increment. It also does
  **not** depend on the RC write-path investigation the way v2 does
  (progressions never touch RC), so it can be sequenced independently of
  v2 — see "Dependencies and sequencing."
- **Staged: HSC v1 (read-only) then HSC v2 (write-back).** v1 — pulling
  RC's roster/event data, a local working copy, participant/waiver
  filtering, the deadline/finalize workflow, and a Heat Sheet artifact
  the RD can apply — uses only RC's already-working, already-built
  **read** methods (`Client.Bulk`/`Events`/`EventEntries`/… from Phase
  A) and has **zero dependency** on the still-open write-path
  investigation. v2 — pushing HSC's finalized Heat Sheet to RC as
  Races/Lanes — is hard-gated on that investigation concluding. This
  directly answers the author's own framing: "the question is timing of
  its implementation" — v1 can start as soon as the feature freeze
  lifts; v2 cannot start before the investigation does.
- **Post-finalize RC sync: fully manual.** Once HSC finalizes a Heat
  Sheet, no further RC state — including a scratch (SCR) an RC
  organization files during the coaches'-meeting window — flows in
  automatically. Every coaches'-meeting-era change (a lane move, an SCR,
  an Exhibition mark) is a manual HSC-operator edit into the finalized
  copy; RC's own SCR flag becomes something the operator manually checks
  and applies, not something HSC keeps polling for.
- **Progressions write mechanism: HSC proposes, RD applies.** Whenever
  v3 is built, it never writes `regattaSchedule.json` itself — same
  one-writer-per-file posture as v1. It computes a later race's lane
  assignments from an earlier heat's approved result and hands that off
  through the existing origin-apply (Apply/Dismiss) flow, the same seam
  v1's own Heat Sheet artifact already uses, just triggered mid-regatta
  instead of once beforehand.

## Background

RegattaCentral's own "heat sheet" capability
([regattacentral-integration.md](../regattacentral-integration.md), "The
RegattaCentral workflow (V4 Cookbook)") is what this project's real
regattas do **not** use today: the RD builds the lineup by hand in an
`.xlsm`, having copied the roster off regattacentral.com by hand. HSC is
the persona that replaces that manual export/copy/paste, and — per the
author — is the **foundational** persona for originating regatta data at
all: its output is what the RD would turn into `regattaSchedule.json`,
whether that happens through Phase C's `ScheduleOrigin` generalization
directly, or through a generated Excel file flowing through the
existing, completely unchanged Excel-import path. Either way, **HSC
never writes `regattaSchedule.json` itself** — the RD stays the sole
writer of that file (one-writer-per-file, the same invariant every other
persona in this codebase respects); HSC produces an *input* the RD
applies through machinery that already exists.

## Fyne feasibility spike — draggable table UI

**Runs before, and separately from, HSC itself.** The in-app-authoring
UI option only makes sense if Fyne can actually deliver a usable
table/grid layout with cell-level drag-and-drop (moving an entry from
one lane/race cell to another) at the scale of a real regatta (dozens of
races, six lanes each). Nobody has tried this in Fyne on this project
yet, and finding out mid-build would stall HSC's own implementation on
an open-ended UI-toolkit question — exactly what the author wants to
avoid.

- **What it is:** a small, disposable command, e.g.
  `cmd/hscuimockup/main.go` — its own `main` package, not part of
  `internal/regatta` or any persona, no RC calls, no `regattaData`, no
  persistence. Synthetic/hard-coded rows only. Its only job is layout and
  interaction iteration.
- **A concrete starting point exists**: `fyne.Draggable`
  (`fyne.io/fyne/v2`) —
  `type Draggable interface { Dragged(*DragEvent); DragEnd() }`. Any
  `fyne.CanvasObject` implementing it receives drag callbacks from the
  canvas directly; this is Fyne's own general-purpose mechanism for "an
  item the user drags across the screen," not something the spike has to
  invent from raw mouse events. `widget.Table` itself does not implement
  `Draggable` and has no built-in drag support, so the spike's real
  question is **not** "can Fyne do drag-and-drop at all" (it can, via
  this interface) but "does a `Draggable`-based custom cell widget,
  dropped into a table/grid layout, add up to a usable heat-sheet
  editor" — a layout and interaction-design question, not a raw-capability
  one.
- **What it needs to answer:**
  - Whether a custom cell widget (a small `fyne.CanvasObject`
    implementing `Draggable`, positioned inside a grid/table container)
    gives clean drag-and-drop between lane/race cells, including drop
    targeting (which cell is a valid drop, and how that's signaled) —
    `Draggable` only reports movement and drag-end, so *drop-target
    detection and the actual data swap* are on the spike to design, not
    something Fyne hands over for free.
  - Is the interaction usable by a non-technical operator during a live
    coaches' meeting (clear drag affordance, sane drop targets, no
    accidental drags)?
  - Does it hold up at realistic scale (a few dozen races × six lanes)
    without becoming sluggish or visually cluttered?
  - What's the resulting layout primitive — `widget.Table` hosting
    `Draggable` cells, a hand-rolled grid `fyne.Container`, or something
    else — since that choice shapes how much of HSC's later
    authoring-UI work is "use an existing widget" vs. "build one."
  - Whether the same layout primitive can also carry a race's round
    type + number (see "Progressions" below) and expose creating a
    placeholder race for a not-yet-run Semi-Final/Final slot — a
    lane-drag test alone under-scopes what this UI actually needs to
    do.
- **Explicitly out of scope for the spike:** RegattaCentral data, the
  schedule model, persistence, anything resembling the real HSC
  persona. It is a layout/interaction test only, thrown away or kept
  purely as a reference sketch once the question is answered — not
  grown into HSC's real UI in place.
- **Outcome, either way:**
  - **Go** — Fyne can do it well enough: HSC's in-app-authoring UI work
    can start against a validated approach, informed by whichever
    layout primitive the spike landed on.
  - **No-go** — Fyne can't do it well, or not without disproportionate
    custom-widget work: HSC defaults to the Excel-generate-and-reconcile
    path (see "Decisions made" above) without further UI R&D, and this
    doc's "editing UI" decision updates from "decided by a spike" to
    "resolved: Excel-generate-and-reconcile."
- **Dependencies:** none. The spike touches no RC client, no persona
  registry, no `regattaData`, nothing HSC v1's own data/index layer
  depends on — it can run in parallel with, before, or fully
  independently of that work. Per the author, it should run **first**,
  precisely so it's answered before HSC implementation reaches the point
  of needing it. Being new source code, it still follows the project's
  own docs-vs-source workflow — its own branch, not a direct commit —
  and, like all new development right now, waits for `develop`'s current
  feature-freeze to lift (see `AGENTS.md`) unless the author decides this
  narrowly-scoped, throwaway spike is worth an explicit exception.

## Progressions — heats to semis and finals (HSC v3, deferred)

**Reference:**
[reference/Progressions.pdf](../reference/Progressions.pdf) (VASRA,
"Standard Progression (Lane Assignment) From Heats to Finals"). Captured
here as a real, concrete requirement; **not** designed in detail or
scheduled in this pass — see the staging decision above. This addresses
the original spec's "the HSC will need to continuously update the Heat
Sheet, even on the day of the Regatta" bullet in a way v1 alone cannot:
v1 is entirely pre-regatta (roster/entries, deadline, finalize); a heat's
actual finish order isn't known until the regatta is under way, so
computing what it feeds into is necessarily a separate, later capability
that reads live `finish.json`.

- **What the reference document establishes:** a deterministic,
  field-size-driven structure — 7–12 entries run 2 heats straight to a
  Final (+ Petite Final for the non-advancers); 13–18 entries run 3
  heats to a Final; 19–24 entries run 4 heats into 2 Semi-Finals, whose
  top 3 finishers advance to the Final (6 boats) and next 3 to the
  Petite Final (6 boats) — boats that reach a Final/Petite Final race 3
  times total. Each transition has a **named seeding pattern**
  (`MiddleOutLow`, `MiddleOutHigh`, `MiddleOutLowAlternatingReverse`)
  mapping "Nth place in Heat X" to a specific lane in the next race, per
  two fixed rules: top finishers go to the inner lanes, and no boat is
  placed adjacent to a boat that was in its own previous heat. The
  Petite Final is explicitly flagged in the source as "only applicable
  during VSRC Regatta" — progression rules may vary by hosting
  organization, so this is not necessarily one universal, hardcoded
  table for every regatta this app serves.
- **What v3 would need to do, at a high level:** watch the primary
  team's `finish.json` for a heat's `RaceResult.Approved` flip, identify
  which later race(s) that heat feeds (a relationship the schedule model
  does not capture today — see below), apply the field-size-appropriate
  named pattern to compute that later race's lane assignments, and
  propose the result through the origin-apply flow (per the write-mechanism
  decision above) rather than writing it directly.
- **What this surfaces as new, unresolved design work** (left for v3's
  own design pass, not resolved here):
  - **A label already exists, but it's the wrong shape.**
    `reader.RaceData.FlightInfo` / `store.ScheduleRace.FlightInfo` is
    already a free-text field carrying exactly this kind of round label
    today — real Excel-sourced values include `"Heat 1"`, `"Semi-Final"`,
    `"Final"` (`internal/reader/regattaData_test.go`). So the gap isn't
    "no field exists"; it's that `FlightInfo` is an unstructured display
    string (good enough for a race title, e.g. `RaceData.RaceTitle()`),
    not a machine-readable **(round type, number)** pair — the author's
    own framing, "each race may be labeled as Heat/Flight with a
    numerical number" — nor does it capture *which later race a given
    race feeds into*. `store.ScheduleRace` needs new, structured
    progression metadata (round type + number, plus a feeds-into link)
    alongside, or replacing, the display-only `FlightInfo` string. It
    isn't obviously the same shape as the RC-id fields v1 adds (a
    progression link is a relationship between two *local* races, not an
    RC identifier).
  - **Placeholder races.** Per the author, HSC also needs to
    *pre-create* races for the Semi-Final/Final/Petite-Final slots a
    given entry count implies — empty of lane assignments (those aren't
    known until the feeding heat(s) finish), but real, numbered,
    Heat/Flight-labeled races that exist in the schedule from the start,
    so the full race list is complete before the regatta begins (timers
    and the RD tree need to know a "Final" race number exists even
    before any heat has run). v1 (or whichever increment authors the
    initial Heat Sheet) creates these placeholders from the progression
    table at entry-count time; v3 later fills in their lanes as the
    feeding heats complete, through the same propose/apply mechanism.
  - **This is a hard requirement for both branches of the still-open
    editing-UI decision, not just the in-app path.** Whichever wins —
    Fyne in-app authoring or Excel-generate-and-reconcile — needs to
    let the operator see/set a race's round type + number and its
    placeholder children, and (for the Excel path specifically) v2's
    "post back to RC" step needs to carry that same structure across,
    not just lane draws. Worth folding into the Fyne feasibility spike's
    own scope once it starts (a draggable cell is not the only UI
    surface a heat sheet needs).
  - Which seeding pattern applies is a function of entry count *per
    event*, not global — the same regatta can have some events at 2
    Heats→Final and others at 4 Heats→2 Semis, simultaneously.
  - Whether the Petite-Final-is-organization-specific caveat means the
    pattern table itself needs to be configurable, not hardcoded from
    this one reference document.

## Does / Does not / Entry / Constraint

### HSC v1 (read-only; buildable now)

- **Does:**
  - Pull a regatta's roster, events, and entries from RegattaCentral
    given an RC regatta id, using the already-built read-only
    `internal/regattacentral` client methods (`Bulk`, `Events`,
    `EventEntries`, `EventLanes`, `Organizations`,
    `SearchOrganizations`, `SearchParticipants` — all Phase A, all GET,
    all unaffected by the write-path investigation's open questions).
  - Maintain a **local working copy** of that data, converging toward
    `store.Schedule` shape, with the pulled RC event/entry ids carried
    as new fields directly on `store.ScheduleRace` /
    `store.ScheduleEntry`.
  - **Diff, not auto-apply**: re-pulling RC (operator-triggered, not a
    background poll — the spec describes this as something the HSC
    operator does deliberately in the run-up to the regatta, not
    continuous sync) compares the new snapshot against the last-indexed
    copy by RC id, and surfaces what changed (an entry moved boat class,
    a participant changed, a new registration appeared) for the operator
    to review — never applies silently.
  - **Participant/waiver filtering** — filter the pulled roster by
    participant name, organization, and waiver-signed status, so the HSC
    operator can track down anyone who hasn't signed.
  - **Deadline / finalize workflow**: before the entry deadline, the
    working copy re-syncs against RC on operator request. **Finalize**
    locks the working copy — after this point, nothing further arrives
    from RC automatically (see the fully-manual decision above). The
    day-before coaches'-meeting requests (move a lane, mark SCR, mark
    Exhibition) are then manual edits the HSC operator applies directly
    to the finalized copy, with explicit decorators for Exhibition/SCR
    status on an entry.
  - **Produce a Heat Sheet artifact** from the finalized working copy —
    a `store.Schedule`-shaped file for the RD to apply as a `heatsheet`
    origin, or a generated `.xlsx` for the RD to import through the
    existing Excel path, depending on which editing UI is eventually
    chosen (left open, above). Either way the contract is the same: an
    artifact the RD applies through machinery that already exists, not a
    direct write to `regattaSchedule.json`.
- **Does not (v1):** Write `regattaData/director/regattaSchedule.json`
  directly — the RD remains the sole writer. Push anything to RC — no
  write calls at all in v1. Auto-apply a re-pulled RC change without
  operator review. Poll RC automatically, before or after finalize —
  every RC pull in v1 is operator-initiated.
- **Entry:** New persona picker entry — own `persona.Role`,
  `Definition`, challenge code (Executive team, Admin UI, per the
  author's spec).
- **Constraint:** Every RC call follows the same non-disruption posture
  [regattacentral-integration.md](../regattacentral-integration.md)
  already establishes project-wide — its own goroutine and `context`,
  `fyne.Do` for UI updates, best-effort with a `WARN` line plus a
  non-blocking notice on failure, never a modal, never on a click path.

### HSC v2 (write-back; gated on the investigation)

- **Does:** Push the finalized Heat Sheet's races and lane draws to RC,
  once [heatsheet-rc-pivot-investigation.md](../heatsheet-rc-pivot-investigation.md)
  resolves the actual write-path shape (its current design pass is
  working through nested `events[]→races[]→lanes[]` payloads and
  dedicated race/lane creation endpoints — not settled yet). Reuses the
  same RC id fields v1 already carries on the schedule model. A later
  coaches'-meeting-era manual edit can optionally be re-pushed too, on
  request.
- **Does not:** Push automatically. A live write to a real, public RC
  record is a deliberate, reviewed, operator-confirmed action every
  time — the same posture REP and SOM already take (approve/review
  before anything leaves the machine), not a background sync.

## Existing-code reuse analysis

- **Phase A's read-only client** (`internal/regattacentral`: `Bulk`,
  `Events`, `EventEntries`, `EventLanes`, `Organizations`,
  `SearchOrganizations`, `SearchParticipants`) — v1's entire RC-pull
  layer. Nothing new to build here; these already exist and are
  unaffected by the write-path investigation's open questions.
- **`internal/secretstore` + `personacfg`'s RegattaCentral config
  group** — the credential/config wiring Phase A already built, reused
  as-is; no new secrets model for HSC.
- **`store.Schedule` / `store.ScheduleRace` / `store.ScheduleEntry`**
  (`internal/persona/store/schedule.go`) — the shape HSC's working copy
  converges toward. Per the author's decision, these two types gain new
  RC-id fields directly — the concrete resolution of
  [regattacentral-integration.md](../regattacentral-integration.md)'s
  own open item.
- **`FlightInfo`** (`reader.RaceData.FlightInfo` /
  `store.ScheduleRace.FlightInfo`, sourced from `RawData.getFlightInfo()`
  in `internal/reader/regattaData.go`) — already carries free-text round
  labels (`"Heat 1"`, `"Semi-Final"`, `"Final"`) today, and already flows
  into `RaceData.RaceTitle()`. The right starting point for v3's
  structured round-type + number field, not a green field — whether that
  means replacing `FlightInfo` with a structured type or adding a
  parallel field alongside it (keeping `FlightInfo` as the derived
  display string) is v3's own design question.
- **`reader.NewRegattaData` / `regattaDataFromSchedule`**
  (`internal/regatta/schedule.go`) — the single entry point Phase C
  already identifies for turning a HSC-produced schedule into the
  in-memory `RegattaData` the app renders from. Reused as-is, once a
  `heatsheet` origin is actually wired into the RD's loader — not new
  work HSC introduces.
- **`pollOrigin` / `applyPendingOrigin` / `pendingOrigin` /
  `dismissedContentHash`** (`internal/regatta/origin.go`) — the RD's
  existing detect → load → compare → Apply/Dismiss flow, already reused
  verbatim for the Excel origin's reload story and already flagged by
  Phase C as reusable for a heatsheet origin too. This is exactly the
  seam that lets HSC hand off a new schedule without ever writing
  `regattaSchedule.json` itself.
- **`store.Schedule.ContentHash()`** — usable as-is for HSC's own
  re-pull diffing, the same fingerprinting pattern the RD's
  origin-refresh detector already uses.
- **Non-disruption rules** — already codified project-wide in
  [regattacentral-integration.md](../regattacentral-integration.md)
  ("Non-disruption rules (first outbound HTTP)"): own goroutine/context,
  `fyne.Do`, `WARN` + notice, retry/backoff, never blocks a click. HSC's
  RC-pull work reuses this posture directly — no new discipline to
  invent.
- **New `persona.Role` + `Session.WritePath()` gap** — the same registry
  shape (and the same read-only-persona `WritePath()` gap) Awards,
  Developer, and Streamer already hit — see those docs' own reuse
  analyses. HSC becomes a fourth Executive-team standalone persona
  landing in the same spot; worth sequencing the registry change
  alongside theirs rather than four near-identical small PRs.
- **Confirm-before-action precedent** — SOM's "operator sees and can
  edit before anything goes out" posture and REP's `Approved` gating are
  the direct precedent for v2's "never auto-push" requirement.

## High-level implementation plan

**Step 0, before v1 (and before HSC has any UI to speak of): the Fyne
feasibility spike** (see above) — resolves the editing-UI fork so v1's
later authoring-UI work (whichever path it turns out to be) starts from
an answered question, not an open one.

**v1:**

1. **RC-pull layer**: wrap the existing `Bulk`/`Events`/`EventEntries`/
   `EventLanes` calls into a single "load roster for RC regatta ID"
   action, off the click path, best-effort.
2. **Local working-copy model**: add RC-id fields to
   `store.ScheduleRace`/`store.ScheduleEntry`; a persisted HSC-owned
   file, distinct from `regattaSchedule.json`.
3. **Diff/index**: compare a re-pull against the last-indexed copy by RC
   id; surface changes for operator review.
4. **Participant/waiver filter UI.**
5. **Deadline/finalize state machine** plus the manual post-finalize
   edit UI (lane move, SCR, Exhibition decorators).
6. **Heat Sheet artifact output** — exact shape (in-app schedule vs.
   generated Excel) resolved alongside the still-open UI decision; the
   contract itself ("something the RD applies through the existing
   origin-apply flow") holds either way.
7. **Registry**: new `Role`/`Definition`/picker entry.
8. **Docs** (once v1 ships): add HSC's section to
   `docs/features/personas/README.md`; update `docs/features/TODO.md`
   to reflect v1 done / v2 still pending; keep this file until v2 also
   ships (matching how a partially-resolved proposal like
   `results-publisher.md` stays in `new/` until its own later slice
   lands too), then delete per the standard workflow.

**v2** (once the investigation concludes): a push-back action reusing
whatever `internal/regattacentral` write client that investigation lands
on, gated behind an explicit operator confirm.

## Dependencies and sequencing

- **The Fyne feasibility spike has zero dependencies** and, per the
  author, should run **first** — before HSC v1's own authoring-UI work
  reaches the point of needing an answer. It does not block v1's
  data/index layer (RC-pull, working-copy model, diffing), which is
  UI-agnostic and can proceed independently if sequencing works out that
  way, but the author's stated intent is to avoid finding this out
  mid-build, so treat the spike as the practical first step regardless.
- **v1: no hard blocker.** Uses only already-built, already-working
  Phase A read methods — GET calls, not the write path the investigation
  is still debugging. Can start as soon as `develop`'s feature freeze
  lifts.
- **v2: hard-gated** on
  [heatsheet-rc-pivot-investigation.md](../heatsheet-rc-pivot-investigation.md)
  concluding — the same gate [results-publisher.md](results-publisher.md)'s
  RegattaCentral destination and
  [operational-state.md](../operational-state.md)'s schedule-ingest axis
  already carry. All three converge on the same underlying
  `internal/regattacentral` write client once that investigation
  resolves — worth building it once and sharing it, not three times.
- **v3 (progressions): no dependency on the RC investigation** — unlike
  v2, it never calls RegattaCentral; it reads local `finish.json` and
  proposes a local schedule update. Its real blocker is design work, not
  an external gate: the round/progression metadata question flagged
  above needs its own pass before implementation starts, and it can be
  sequenced independently of v1/v2 (before, after, or between them).
- **Does not block** Awards, Developer, Results Publisher, SOM, or
  Streamer — none of their own scope depends on HSC landing first;
  `internal/publish`, the schedule model, and `finish.json` are already
  origin-agnostic (every one of those personas/features reads the same
  `regattaSchedule.json` regardless of whether an `excel` or future
  `heatsheet` origin produced it).
- **Does shape scope, not sequencing**, for those docs' own later,
  RC-dependent slices: Results Publisher's step 5 (RegattaCentral
  destination) and `operational-state.md`'s ingest-source axis should
  keep HSC's RC-id-on-schedule-model decision in view when their own
  turn comes, so nobody re-invents a second, different indexing scheme
  for the same RC ids.

**No architectural blocker for HSC v1.** As with the other proposals in
this directory, the live process constraint as of this writing is
`develop`'s feature-freeze (see `AGENTS.md`) — implementation waits for
that to lift. HSC v2 additionally waits on the investigation, independent
of the freeze.
