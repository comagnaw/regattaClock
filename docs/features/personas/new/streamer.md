# Streamer (STM)

A **standalone persona** (Executive team, Media UI) — not a sidecar, not
attached to any Lead session. STM runs on its own dedicated machine, the
one also running OBS Studio for a live YouTube feed, and its whole job is
to keep that stream supplied with up-to-date graphics: lane-assignment
images, a running wall clock, and results images — without ever doing
anything that could stall or stutter the OBS capture.

First named in the SOM request as the persona that would eventually
transform a shared text result into a PNG (see
[social-media.md](social-media.md)'s "Text format / shared package"
decision); originally sketched, unscoped, as the "social-image
publisher" in
[future-result-driven-persona.md](../future-result-driven-persona.md)
(now retired there — see that doc's update note — since STM supersedes
it directly).

## Decisions made (author, 2026-09-16)

- **Race-switch behavior:** if "run clock" is pressed for a different
  race while one is already running, STM shows a confirm dialog
  ("Race 5 is still running — switch to Race 8?") before retargeting.
  Silent switching risks an operator fat-fingering the wrong row while
  live; a hard block is more friction than needed for a legitimate
  early-switch.
- **Wall clock lifecycle: fully live.** While tracking a race, STM keeps
  watching that race's `start.json` and `finish.json` and reacts:
  - A start-time correction for the tracked race updates the running
    elapsed-time calculation live (no restart needed).
  - The tracked race's result flipping to `Approved` auto-stops the
    clock, blanks the window, and reverts the tree's button back to
    "run clock" — the same event that gates STM's own results-PNG
    generation for that race (below), so one watcher event drives both.
- **Results-PNG gating: same `RaceResult.Approved` gate as SOM/REP.**
  Consistent with every other publish-shaped feature in this codebase —
  STM never broadcasts an unofficial time on the stream. A later
  hybrid (auto on Approved, plus a manual force-early button) was
  considered and explicitly deferred — start with the simple gate, add
  a force button only if real operation shows a need for it.
- **Output directory: `regattaData/stream/`**, STM's own, with
  `stream/lanes/` and `stream/results/` subfolders for the two image
  kinds. This also resolves a related open question: the
  `regattaData/results/` directory
  [future-result-driven-persona.md](../future-result-driven-persona.md)
  reserved for a "materialized published results" cache is **retired,
  not just avoided** — see that doc's update. Nothing built (REP, SOM,
  now STM) ever needed it; each reads `finish.json` /
  `regattaSchedule.json` directly via `internal/publish` instead.

## Does / Does not / Entry / Constraint

- **Does:**
  - Show a **race tree** modeled on the RD's read-only progress tree
    (title + status per race), with two additional terse-timestamp
    columns: **lanes PNG last updated** and **results PNG last
    updated** (blank until the first render for that race).
  - Auto-generate a **lane-assignment PNG** per race into
    `regattaData/stream/lanes/` whenever `regattaSchedule.json` changes
    in a way that affects that race's lane map, watched via
    `internal/watcher` on `SchedulePath()`. Also expose a per-race
    **force regenerate** button for an ad-hoc re-render outside a
    schedule change.
  - Run a **wall clock**, in its own separate, fixed-size window, driven
    by a per-race **"run clock"** button on the tree (enabled by
    `store.CanTrackWallClock(start)`, `internal/persona/store/state.go`,
    race-state-machine.md — a recorded `StartRecord.StartedAt`). Running, the button
    reads "clear clock" and the window title carries the race's
    metadata (title/boat class); the elapsed time is `MM:SS.m`,
    computed from the start record's own captured offset and STM's own
    machine's live NTP offset (see reuse analysis). "Clear clock" blanks
    the window (title included) but leaves it open — OBS keeps
    capturing an existing window; closing it would drop the source from
    the scene.
  - Auto-generate a **results PNG** per race into
    `regattaData/stream/results/` once `store.CanPublish(res)`
    (`internal/persona/store/state.go`, race-state-machine.md — that
    race's `RaceResult.Approved` becomes true), watched via
    `internal/watcher` on the primary team's `FinishPath()`. Format
    matches SOM's shared text table (`internal/publish.RenderText`),
    rendered as an image.
- **Does not:** Write any persona-owned timing file (`start.json`,
  `finish.json`, `regattaSchedule.json`) — STM is read-only over all
  three, the same posture as Awards/Developer/RD. Attach as a sidecar to
  a Start/Finish Timer or the Director — it is its own persona with its
  own login. Run more than one wall clock at a time — only one race may
  be tracked at once (see the confirm-before-switch decision above).
  Generate a results PNG before `Approved` (see gating decision above).
  Do anything synchronously on a UI/render thread that could compete
  with OBS for CPU/GPU — see the performance constraint below.
- **Entry:** New persona picker entry (own `Role`, `Definition`,
  challenge code), same mechanism as every other persona — not an
  add-on to an existing session.
- **Constraint:** STM's own machine is presumed to also run OBS; every
  background duty (watcher-driven PNG regeneration, the wall clock's
  ticker) must follow the same non-blocking discipline the codebase
  already requires of timing captures — background work never runs on
  the UI goroutine outside a `fyne.Do` update, and none of it should be
  CPU-heavy enough to compete with OBS's own capture/encode loop. This
  reads as "keep doing what this codebase already does for timing
  clicks," not a new mechanism — see reuse analysis.

## Existing-code reuse analysis

- **Race tree** — the RD's read-only progress tree
  (`internal/regatta/director_tree.go`: `hydrateDirectorLogs`,
  `refreshDirectorRow`, `teamPathSession`, `directorWatchPaths`,
  `startWatcher` in `persona_startup.go`) is the direct template, same
  as it was for Awards/Developer. STM adds two columns instead of an
  action button per row (lanes/results status) plus the "run clock"
  button column.
- **New `persona.Role` + `Definition`** — same registry gap Awards hit
  first (`docs/features/personas/new/awards.md`'s reuse analysis):
  `RoleStreamer`, `Team: TeamExecutive`, `File: ""`, a challenge code,
  appended alongside `DirectorDefinition` (or however AWD/DEV's own
  additions end up structured — this is now the third Executive-team
  standalone persona hitting the same `All()`/registry shape, worth
  landing them together or at least consistently).
- **`Session.WritePath()` gap** — same gap Awards/Developer already
  flagged for a read-only, non-director Role. STM's own code should
  simply never call `store.Save*`; the guard is a safety net, not
  something STM's normal operation exercises.
- **Lane-map change detection** — `ScheduleRace.LaneMapHash()`
  (`internal/persona/store/schedule.go`) already exists and is exactly
  the per-race fingerprint STM's "in-memory tally, don't rewrite every
  PNG on every schedule change" requirement needs: keep
  `map[int]string` (raceNumber → last-rendered `LaneMapHash`), skip a
  race whose hash hasn't changed. No new hashing to design.
- **Lane-assignment PNG rendering** — `internal/exporter`'s existing
  `Export`/`renderImage`/`buildRaceText`/`drawText`/`saveImage`
  (`internal/exporter/exporter.go`) already do exactly this
  transform (lane label + boat text → PNG) for the whole regatta in one
  batch, driven by a manual menu action today. STM needs the same
  per-race rendering logic reachable as a single-race entry point,
  driven by a watcher instead of a menu click — the existing low-level
  drawing helpers are reused as-is; only the batch-vs-single-race entry
  point is new, and `Export` itself is left untouched (existing manual
  export keeps working for whoever still uses it).
- **Results-PNG rendering — this is Phase 0b from
  `sidecar-personas.md`, previously unbuilt.** That doc already sketches
  `func RenderResult(r publish.PublishableRace) (image.Image, error)` in
  `internal/exporter`, reusing the same `newDrawingContext`/`drawText`
  primitives, specifically for this shape of work. It was scoped as
  sidecar-adjacent infrastructure (a "Save image" button inside a
  sidecar panel) back when the image consumer was still an unnamed
  future concept; SOM's own resolution explicitly excluded the image
  path, so that sidecar "Save image" button currently has no real
  consumer. STM is Phase 0b's actual first consumer, and — being a
  standalone persona, not a sidecar — does **not** need Phase 0c
  (`internal/regatta/sidecar.go`) to land first; it only needs Phase 0a
  (`internal/publish`) and the Phase 0b rendering function itself.
  `sidecar-personas.md` is updated to note this.
- **Shared text format with SOM** — `internal/publish`'s `BuildView`,
  `PublishableRace`, `RenderText` (built, race-state-machine.md) are
  reused as-is: STM's results PNG is `RenderResult` applied to the same
  `PublishableRace` SOM's `RenderText` renders as plain text. One join,
  one revision hash, two renderers.
- **Results-PNG change detection** — `publish.IsStale(published, pr)`
  (the same re-publish-detection check SOM already uses) is reused
  directly for STM's own "don't re-render an unchanged result" tally:
  `map[int]string` (raceNumber → last-rendered `Revision`).
- **NTP-corrected wall clock** — `internal/timesync.Ref()` (already
  measuring STM's own machine's offset from app startup,
  `internal/regatta/bootstrap.go`'s `timesync.Start`, no persona-specific
  wiring needed) plus `StartRecord.Clock` (the ST's own `ClockRef`,
  already stored per race in `start.json`) give exactly what the wall
  clock needs: `trueNow := timesync.Ref().Corrected(time.Now())`,
  `elapsed := trueNow.Sub(startRecord.Clock.Corrected(*startRecord.StartedAt))`.
  Both halves already exist; STM writes no new offset-measurement code.
- **Stopwatch rendering loop** — `internal/clock.Clock`'s existing
  100ms-ticker-driving-a-`canvas.Text` pattern
  (`internal/clock/clock.go`) is the template for STM's own wall clock
  loop — same `fyne.Do`-wrapped update discipline, much simpler content
  (one big elapsed-time label, no laps/results/writes).
- **Terse relative-timestamp columns** — `docs/features/TODO.md`
  already lists an unbuilt, generically-useful "Timer-side staleness
  indicator... from the watcher's last-change time" item
  (`persona-plan.md` §12). STM's "PNG last updated" columns are the same
  kind of display; worth building that shared formatter once rather
  than STM inventing its own, though STM does not strictly block on it
  (a small local helper is an acceptable fallback if that TODO item
  hasn't landed first).
- **Non-blocking background work** — the project-level constraint
  already stated in `AGENTS.md`'s "In-progress: multi-persona operation"
  section ("timing button clicks are the highest-priority path...
  background routines use async queues and best-effort I/O") is the
  existing discipline STM's performance constraint is really asking for
  — no new pattern, just applying the same rule to STM's watcher-driven
  PNG regeneration and its own render ticker.
- **Watcher usage** — `internal/watcher` already supports watching
  `regattaSchedule.json` and a team's `finish.json`/`start.json`
  (`directorWatchPaths`, `startWatcher` in `persona_startup.go`); STM
  watches all three (schedule for lane images; start + finish for the
  wall clock's live behavior; finish again for results images) using
  the exact same mechanism, no new watcher capability needed. (The
  watcher's standing prohibition on watching `logs/` is unrelated — STM
  never touches that directory.)

## High-level implementation plan

1. **Registry**: add `RoleStreamer`, a `Definition` (`ID: "stm"`,
   `Team: TeamExecutive`, `File: ""`, challenge code, `Label:
   "Streamer"`), a picker entry. Coordinate with however Awards/
   Developer's own registry additions land, since all three hit the
   same `WritePath()` gap and the same `All()` shape.
2. **Race tree**: build from the Compare-Secondary/RD-tree template,
   read-only, primary team only, two new terse-timestamp columns plus
   the run-clock button column.
3. **Lane-assignment PNGs**: a new single-race entry point in
   `internal/exporter` (reusing existing drawing helpers), a
   `LaneMapHash`-keyed in-memory tally, a schedule watcher wired to
   regenerate only the changed races, written to
   `regattaData/stream/lanes/`, plus the manual force-regenerate button.
4. **Results PNGs**: build Phase 0b's `RenderResult` (previously only
   sketched) in `internal/exporter`, gated on `RaceResult.Approved`, a
   `Revision`-keyed in-memory tally, a finish watcher, written to
   `regattaData/stream/results/`.
5. **Wall clock window**: fixed-size, blank by default; "run clock"
   button per row (enabled once `StartRecord.StartedAt != nil`),
   confirm-dialog before retargeting a different race, elapsed time
   driven by `timesync.Ref()` combined with `StartRecord.Clock`, live
   start-time corrections, auto-stop/blank on that race's `Approved`
   flip, "clear clock" blanks without closing.
6. **Docs** (once built): add STM's section to
   `docs/features/personas/README.md`; update `docs/features/TODO.md`;
   delete this file.

## Dependencies and sequencing

- **No hard blocker.** Unlike SOM (which needs Phase 0c, the sidecar
  lifecycle, to land first), STM only needs Phase 0a
  (`internal/publish`) plus its own Phase 0b rendering work — both are
  self-contained additions STM can build directly, without waiting on
  `sidecar-personas.md`'s sidecar-lifecycle work at all.
- **Loosely coupled to Awards/Developer**: no functional dependency, but
  all three are new Executive-team standalone personas hitting the same
  registry gap (`WritePath()`, `All()` shape) — sequencing them together
  avoids three near-identical small registry PRs.
- **Soft dependency, not a blocker**: `docs/features/TODO.md`'s
  timer-staleness-indicator item (a generic terse-relative-time
  formatter) would save STM writing its own, but STM can ship with a
  small local helper if that item hasn't landed first.
- **No blocker** from `sidecar-personas.md`'s open decisions (which
  Leads may host a sidecar, etc.) — none of that applies to STM, a
  standalone persona.

**No architectural blockers.** As with the other proposals in this
directory, the live process constraint as of this writing is
`develop`'s feature-freeze (see `AGENTS.md`) — implementation waits for
that to lift.
