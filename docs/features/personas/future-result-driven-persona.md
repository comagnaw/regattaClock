# Result-driven publishing personas — assessment

Whether the **Finish Timer approval workflow** should change now to support
future personas that turn an approved race into published content, or whether
that is best rolled into the feature that adds those personas.

**Status:** design assessment, no code. Companion to
[reconciliation.md](reconciliation.md) — the personas here are *content
consumers*; the *results/publish persona* named there is the *producer* they
depend on. See also [persona-plan.md](persona-plan.md) §9 (Director progress
tree), §13 (open items), and [schedule-data-model.md](schedule-data-model.md).

**Recommendation up front: defer.** `timing/primary/finish.json` +
`director/regattaSchedule.json` already carry everything a results-driven persona
needs, including enough to detect a post-approval edit. Introduce a materialized
`results/` view only alongside the personas that consume it, owned by a single
writer. Reasoning below.

## The scenario

Two (or more) **read-only** personas on the Executive team, each with its own
challenge:

- **Social-text publisher** — renders one race's results as a plaintext table
  for posting to social media.
- **Social-image publisher** — renders the same table as a PNG (the existing
  `internal/exporter` freetype path already draws race images; this is the same
  idea with places and times instead of lane labels).

Both would:

- Load an RD-style, read-only progress tree (title + status per race).
- Watch for a race's **Referee Approval** (`RaceResult.Approved` flipping true).
- Carry a per-race action button ("Publish" / "Post") that runs their transform
  for that race number.
- Care about **accuracy**: if an approved result is later edited, flag the row
  and let the operator re-publish.

Target artifact (both personas render this same content, one as text, one as an
image):

```text
Race 1: W-N-4+ Heat 1 Results
1 - Robinson 07:26.8
2 - Gloucester 07:29.9
3 - Episcopal 07:32.1
4 - James Madison 07:36.1
5 - Battlefield 08:36.1
```

## The question

Should Referee Approval, in addition to writing `finish.json`, write a small
per-race file into a new `regattaData/results/` directory — so a downstream
watcher gets a precise per-race signal and payload instead of unmarshalling the
whole `finish.json`?

## Options

### A. FT approval also writes `results/race_NN.json`

A cheap, precise watch target: a file that appears/changes only on approval,
holding just that race's published-ready data.

Costs:

- **Second source of truth for results.** The same order-of-finish and times
  would live in `timing/primary/finish.json` *and* `results/race_NN.json`. The
  intent is that `finish.json` stays the editable record — so `results/` is a
  copy that must be regenerated in lockstep on every re-approval or it drifts.
  That is exactly the "multiple sources of truth to reconcile" problem to avoid.
- **Ownership.** One-writer-per-file is load-bearing. The FT clock's job is
  timing, not publishing; there are two FT personas and the secondary never
  approves, so an FT-written `results/` file is really "the primary FT's approved
  set", not "the published set".
- **Couples to an unbuilt model.** A publish-ready per-race file is a
  *materialization of the reconciliation verdict* (`primary` / `secondary` /
  `disputed` / `gap` + provenance). That model is design-only (phase 8e) with
  open questions — e.g. *does `disputed` block publish?* Writing the file now
  bakes in "primary-approved ⇒ publishable" before that is decided.
- **Reverses a deliberate cleanup.** `regattaData/results/` was removed in
  stage 3 specifically because an empty, unwritten directory in everyone's
  synced folder is misleading ([persona-plan.md](persona-plan.md) §3). Bringing
  the name back is a decision to call out, not a free slot.
- **Departs from a stated invariant.** [reconciliation.md](reconciliation.md)
  says plainly: *"No new file. The Regatta Director persists no reconciliation
  decision."* A materialized `results/` tree is a change to that.

### B. Content personas watch `timing/primary/finish.json` directly

They are already unmarshalling `finish.json` to see the approval signal, so
"that is a lot of data to unmarshal" costs the same either way — and it is the
cost the **Regatta Director already pays today**: `applyDirectorTimingEvent`
unmarshals the whole file on every hash-changed watcher event, then refreshes one
row. A regatta is tens of races of JSON — single-digit KB.

`store.RaceResult` already holds `WinningTime` and `Rows []LapRow{Lane, Place,
Split, Time}`. School names come from `regattaSchedule.json`
(`ScheduleRace.Lanes[lane].SchoolName` / `.AdditionalInfo`), joined by
`RegattaKey` + `RaceNumber` + lane — the same join the RD tree and the stale
lane-map check already do. The race title is reproducible from
`ScheduleRace.RaceNumber` / `BoatClass` / `FlightInfo` (the inputs to
`reader.RaceData.RaceTitle()`).

Post-approval edit detection already works: `RaceResult.UpdatedAt` moves on every
write, `ApprovedAt` is set once, so `UpdatedAt` after `ApprovedAt` means the race
changed after approval; `LaneMapHash` catches a lane reassignment.
`Envelope.Sequence` is per-file, not per-race.

Single source of truth, nothing new to keep in sync. Downside: each content
persona re-implements the schedule join and (eventually) the reconciliation
verdict, and the watch signal is whole-file rather than per-race.

### C. Defer: one results/publish owner materializes the set; content personas consume that (recommended)

The FT approval workflow does **not** change now. When the content personas are
built:

- The **results/publish persona** (the one [reconciliation.md](reconciliation.md)
  already anticipates) — or, until it exists, the **Regatta Director**, which
  already reads both `finish.json` files and the schedule and runs the progress
  tree — materializes `regattaData/results/` as a **derived, single-writer**
  view: one `results/race_NN.json` per race, regenerated from `finish.json` +
  `regattaSchedule.json` + the reconciliation verdict.
- `results/` is explicitly **not** a peer timing record — it is a cache of "what
  would be published", reproducible at any time from the timing files. That
  keeps one-writer-per-file intact and adds no second *timing* SoT.
- The content personas are pure consumers: watch `results/`, react per race, run
  their transform, track what they published.

## Recommendation

**Defer.** Nothing blocks building these personas later against `finish.json` +
`regattaSchedule.json` as they stand. The materialized `results/` file is a
reconciliation artifact whose shape depends on decisions
[reconciliation.md](reconciliation.md) deliberately leaves open. Adding a writer
to the FT approval path is the expensive, invariant-touching change; adding a
consumer later is cheap and isolated. Roll `results/` into the same feature that
adds the personas, and treat this doc as the placeholder for that decision.

## What to reserve now (no code)

- **Directory:** `regattaData/results/`, reserved for the materialized published
  set — per-race `results/race_NN.json` (matches "one persona targets one race
  number"; a re-publish touches exactly one file). Re-introducing this name is a
  reversal of the stage-3 cleanup and should be a conscious part of the later
  feature.
- **Single writer:** the results/publish persona, or the RD in the interim.
  Whichever it is, exactly one persona writes `results/`.
- **Schema sketch** for `results/race_NN.json` — every field reproducible from
  `finish.json` + schedule + verdict, so the file is a cache, never an
  authority:

  | Field | Source |
  |-------|--------|
  | `raceNumber` | join key |
  | `title` | `ScheduleRace` `RaceNumber` / `BoatClass` / `FlightInfo` |
  | `regattaKey` | `RegattaKey(schedule.Name, schedule.Date)` |
  | `provenance` | `primary` / `secondary` / `disputed:<reason>` (verdict table) |
  | `winningTime` | `RaceResult.WinningTime` |
  | `rows` | `[{place, lane, school, additionalInfo, time}]` — `RaceResult.Rows` ⋈ `ScheduleRace.Lanes` by lane |
  | `approvedAt` | `RaceResult.ApprovedAt` |
  | `sourceUpdatedAt` | the `RaceResult.UpdatedAt` this file was built from |
  | `laneMapHash` | `RaceResult.LaneMapHash` |
  | `revision` | hash of the publish-relevant fields only (see below) |
  | `builtAt` | when the view was materialized |

- **Re-publish contract:** `revision` is a hash over just what a reader would
  *see* — ordered `(place, lane, school, time)` plus `winningTime` and `title`.
  A content persona stores `{raceNumber: revision}` for what it last published; a
  mismatch flags the row "re-publish". Incidental writes that do not change the
  visible result do not bump `revision`, so the operator is not nagged over
  nothing.
- **Content-persona shape** (description, not spec): RD-style read-only tree;
  one action button per row; watches `results/`; renders text (to clipboard /
  file) or PNG (via `internal/exporter`); records the published `revision`.
  Never writes `timing/` or `results/`.

## What already exists to build on

- `store.RaceResult` / `LapRow` fields (`internal/persona/store/log.go`).
- The watcher + `matchesRegatta` + per-row refresh pattern in
  `internal/regatta/director_tree.go` (`applyDirectorTimingEvent`,
  `onDirectorTeamChanged`, `refreshDirectorRow`, `directorWatchPaths`,
  `hydrateDirectorLogs`) and `startWatcher` in
  `internal/regatta/persona_startup.go`.
- The lane-number → school join used by the RD tree and the stale lane-map
  check (`internal/regatta/schedule.go`).
- `internal/exporter`'s freetype/PNG rendering for the image persona.
- The persona registry (`internal/persona`): `Definition`, `Role`, `Team`,
  `All()`. Adding these personas needs a new `Role` (the current three are
  director / start / finish) and, since they are read-only, `File: ""`;
  `Session.WritePath()` has no branch for them today.

## Open questions

- One `results/race_NN.json` per race vs a single `results/published.json`.
- Does the RD own `results/` until the dedicated publish persona exists, or does
  the whole thing wait for that persona?
- Does a `disputed` (or `gap`) race get a `results/` file at all, or is its
  absence the "not publishable yet" signal?
- Where a content persona records what it has published — a local preference, or
  a small per-persona file (which would itself be shared state to reason about).
- Should `revision` live on the `RaceResult` in `finish.json` (written by the FT,
  so any consumer sees it without a schedule join) or only on the derived
  `results/` file. Putting it on `RaceResult` is the one change that would touch
  the FT write path — worth weighing when the personas are specced.
