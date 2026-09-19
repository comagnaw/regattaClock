# Awards (AWD)

A **standalone**, Executive-team, read-only persona for the awards table. AWD
emulates the Regatta Director's race tree, adding a **View Results** button
per race so the operator can transcribe 1st/2nd/3rd place without touching a
timing clock or the schedule.

Not a sidecar — AWD does not attach to another persona's session, and no
other persona depends on it.

## Does / Does not / Entry / Constraint

- **Does:** Show the same read-only, primary-team progress tree the RD sees
  (race number, restarts, start time, winning time, approval status); add a
  **View Results** button per row; open a read-only results window for that
  race, showing the full lap/placing grid, with a single **Close** button and
  no other controls.
- **Does not:** Time races; write schedule, start-time, or finish data;
  approve a race; read or reconcile secondary-team data (AWD inherits the
  RD tree's primary-team-only scoping, not a reconciled view).
- **Entry:** A picker button in the **Admins** tab (alongside the Regatta
  Director and the placeholder Developer persona), gated by its own challenge
  code (e.g. `rc-awd`).
- **Constraint:** Reads only `regattaSchedule.json` plus the **primary
  team's** `start.json`/`finish.json` — the same data and the same
  primary-only invariant the RD tree already enforces (`reconciliation.md`),
  not a new read path. AWD never calls a `store.Save*` write path, full stop.

**"View Results" button gating:** enabled only when that race's primary-team
finish result has `Approved == true` — not merely "has a winning time."
`raceProgressStatus` (`internal/regatta/timer_races.go:187-198`) already
distinguishes `RaceApprovedText` from `RaceSavedText` on exactly this field;
AWD's button reuses the same check (`res.Approved`), not a new status
computation.

## Existing-code reuse analysis

- **Persona registry** — `internal/persona/persona.go`'s `Definition` /
  `Role` / `Team` / `Registry` / `DirectorDefinition` / `All()`. `TeamExecutive`'s
  own doc comment already reserves this team for "the director and any
  future non-timing officials" — AWD is that case. Adding AWD is a new
  `Definition` (own `Role`, `Team: TeamExecutive`, a challenge code, `File:
  ""`), per the file's own comment: "New personas are added by appending a
  struct literal to Registry; nothing else has to change."
- **Read-only progress tree** — `internal/regatta/director_tree.go`'s
  watcher/join/per-row-refresh pattern (`hydrateDirectorLogs`,
  `applyDirectorTimingEvent`, `onDirectorTeamChanged`, `refreshDirectorRow`,
  `directorWatchPaths`) plus `persona_startup.go`'s `startWatcher` is the
  reusable seam both `future-result-driven-persona.md` and
  `sidecar-personas.md` already name for a future read-only Executive
  persona. AWD reuses it as-is rather than building a second watcher.
- **Race-tree row shape** — `internal/regatta/races.go` /
  `timer_races.go`'s `raceRow`/`newRaceRow`/`refreshRow` switch on
  `r.session.Role`. Today `RoleDirector` falls into the `default` case, so
  adding AWD means promoting RD to an explicit `case persona.RoleDirector:`
  and adding a sibling `case` for AWD's role — `default` can no longer
  silently mean "director."
- **Read-only results window** — model this on `internal/clock/compare.go`'s
  `compareBody`/`compareLapGrid`/`compareWinningLine` and its window
  lifecycle (`compareWindow` field, `SetOnClosed` clearing it), **not** on
  `referee_window.go`'s actual Referee Approval implementation. The latter is
  entangled with a live, writable `clock.Clock` (it reads live `laps`/
  `results` state, always shows Approve/Cancel, and blocks the clock while
  open) — reusing it in a "read-only mode" would mean constructing a full
  writable clock for a persona that must never write, then suppressing
  machinery AWD doesn't need. Compare Secondary already proves the exact
  shape AWD wants: a standalone window built directly from a
  `store.RaceResult`, labels only, no inputs, nothing to block. AWD's window
  swaps Compare's automatic styling for a plain title + Close button, and
  reads the **primary** team's own approved result rather than a peer team's.
  (If the visual similarity to the actual Referee Approval grid matters more
  than the code-reuse path, `internal/clock/approval.go`'s `scalingGridLayout`
  is the closer visual template — but it is built from live laps via
  `asApprovals`, so reusing it means re-deriving it from a `store.RaceResult`
  first; Compare's pattern needs no such translation.)
- **Approval gating** — `internal/persona/store/log.go`'s
  `RaceResult.Approved`/`ApprovedAt`, the same field `raceProgressStatus` and
  `director_tree.go`'s `directorFinishCells` already branch on.
- **Startup wiring** — the RD's picker button already lives in the `admins`
  `VBox` in `internal/regatta/persona_startup.go`; AWD's button belongs
  alongside it, not a new tab.
- **Doc shape** — the Regatta Director section of
  [personas/README.md](../README.md) is the template voice (Does / Does not
  / Entry / Constraint) this doc follows above, and that AWD's eventual
  README.md entry should match.

## High-level implementation plan

1. **Registry**: add a `Role` (e.g. `RoleAwards`) and a `Definition` (ID
   `"awd"`, `Team: TeamExecutive`, a challenge code, `Label: "Awards"`,
   `File: ""`) — a self-contained change to `internal/persona/persona.go`,
   no other package needs to change for this step alone.
2. **`Session.WritePath()` gap**: `WritePath()` has no branch today for a
   read-only, non-director Role — it either returns the schedule path
   (director) or joins `s.File` (timer personas), and `s.File` is empty for
   AWD. This is the **same gap** `docs/features/TODO.md`'s deferred
   "Result-publishing personas" bullet already names ("Needs a new
   `persona.Role` with `File: ""`, a `Session.WritePath()` branch"). AWD
   would be the first implementation to actually hit it. Decision to make at
   implementation time: add a minimal, generic guard (e.g. `WritePath()`
   returns an error/empty for any Role that never writes) so the
   still-deferred publisher personas can reuse it too, or resolve it
   narrowly for AWD alone and leave the shared fix for whichever persona
   lands second. Either way, AWD's own code path should simply never call
   `store.Save*` — the guard is a safety net, not something AWD's normal
   operation should ever exercise.
3. **Race tree**: promote `RoleDirector` out of `default` in
   `raceRow`/`newRaceRow` (`timer_races.go`) and the header
   (`races.go`), add a sibling case for AWD reusing the same four read-only
   columns (restarts / start time / winning time / approval status) plus a
   **View Results** button, enabled via `res.Approved`.
4. **Results window**: new function(s) modeled on `compareBody`/
   `compareLapGrid`/`compareWinningLine`, consuming a `store.RaceResult`
   directly, with a Close-only footer and the same window-lifecycle pattern
   (a tracked window field, `SetOnClosed` clearing it).
5. **Startup**: add AWD's picker button to the `admins` `VBox` in
   `persona_startup.go`, gated by its challenge code.
6. **Docs** (once built, per `AGENTS.md`'s New personas workflow): fold this
   persona into `personas/README.md`'s team table, persona table, privilege
   table, and a new "Awards (AWD)" section matching the shape above; update
   `docs/features/TODO.md`; delete this file.

## Dependencies and sequencing

No item in `docs/features/TODO.md` blocks this persona architecturally:

- The registry change (step 1) has no prerequisite.
- The read-only tree/watcher pattern (step 3) is already built and stable —
  reused, not extended.
- The RD tree's recent **primary-team-only** simplification
  (`docs/features/TODO.md`, "Personas — feature follow-ups") is a constraint
  AWD must inherit (read primary only, no secondary reconciliation UI), not
  something AWD is blocked on.
- The `Session.WritePath()` gap (step 2) is shared with the still-deferred
  "Result-publishing personas" TODO bullet, but that bullet marks its own
  entry **deferred** — it does not gate AWD; if anything, AWD implementing a
  fix here first narrows that bullet's own remaining scope.
- RegattaCentral's planned "Register Results" write path
  (`regattacentral-integration.md`, Phase D) is a separate, automated,
  outbound-write path with no functional overlap with AWD's manual,
  read-only, human-transcription use case.

**No blockers — implementation could start as soon as engineering capacity is
available.** The one live process constraint as of this writing: `develop` is
in feature-freeze while the author functional-tests the current release on a
real Windows domain and clears bugs via patch releases (see `AGENTS.md`).
Writing and refining this proposal doc is unaffected by that freeze; actual
implementation (steps 1-5 above) should wait for the freeze to lift, or land
on a feature branch that doesn't merge until it does.
