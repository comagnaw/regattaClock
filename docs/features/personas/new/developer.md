# Developer (DEV)

A **standalone**, Executive-team, read-only Admin persona for engineering and
support diagnostics: a more detailed regatta-status view than the Regatta
Director's, a global log viewer/follower across every running persona (with
an export/zip action for a support report), and a regatta-metadata viewer.
DEV owns log collection and visibility outright — see "Existing-code reuse
analysis" below for why the earlier, narrower Director-side idea was
retired in its favor.

Not a sidecar — DEV does not attach to another persona's session, and no
other persona depends on it. `sidecar-personas.md`'s classification table
already pre-reserves DEV as **Standalone**, not Lead, matching this.

## Does / Does not / Entry / Constraint

- **Does:** Show a race-tree view in the shape of the RD's, but more
  detailed — a scope decision to make explicit at implementation time is
  whether "detailed" means also surfacing the **secondary team's** timing
  state (RD and Awards both read primary-team data only; see below). Provide
  a globally-scoped **View Logs** button opening a table of every persona's
  log lines (across all teams/roles/hosts), filterable by column, with a
  live-follow mode and a "collect logs" export/zip action for handing off a
  support report. Provide a globally-scoped **Regatta Metadata** button
  showing the loaded regatta's name/date/source, its on-disk paths, and the
  deployment config's host-pinning/challenge overrides.
- **Does not:** Time races; write schedule, start-time, or finish data;
  approve a race; take any write action anywhere. Same fully read-only
  posture as the RD and Awards.
- **Entry:** The **Developer** button already exists as a disabled
  placeholder in the Admins tab (`persona-plan.md`, `personas/README.md`) —
  DEV is that placeholder becoming real, gated by its own challenge code.
- **Constraint:** Never points `internal/watcher` at `regattaData/logs/`.
  The watcher's conflict-copy scanner (`internal/watcher/conflict.go`)
  treats any file sharing a stem but a different suffix as a OneDrive
  conflict copy — exactly the shape of the intentional
  `<role>-<hostname>.log` per-host naming under `logs/<team>/`. The
  package's own code comment already states this directly: directories
  under `logs/` must never be passed to the shared `Watcher`. DEV's
  log-follow feature needs its own, separate, poll-only read mechanism.

## Existing-code reuse analysis

- **Persona registry** — same mechanism as every other persona:
  `internal/persona/persona.go`'s `Definition`/`Role`/`Team`. DEV needs a new
  `Role` (e.g. `RoleDeveloper`) in `TeamExecutive`, alongside the RD.
- **Logging is already exactly what DEV needs to read** —
  `internal/applog` already writes append-only NDJSON (one JSON object per
  line, via `slog.JSONHandler`) to a per-persona file at
  `regattaData/logs/<team>/<role>-<hostname>.log`, gated by the existing
  `PrefLogging`/`PrefDebug` preferences. Every line already carries
  `persona_id`, `team`, `role`, `machine`, plus call-site fields
  (`component`, `action`, `race`, `err`, …). Because this already lives
  inside the shared `regattaData` tree (the same share `start.json`/
  `finish.json` use, per `shared-storage-options.md`), **any machine's DEV
  persona can already read any other persona's log today** — no new
  write-side plumbing, no redirection, this is a pure read/UI feature over
  data that already exists in the right place.
- **Regatta metadata sources already exist, all read-only-safe**:
  `reader.RegattaData` (`Name`, `Date`, `SourceInfo{Type, URI, Hash}`);
  `persona.Session`'s `Root` and path helpers (`SchedulePath`/`StartPath`/
  `FinishPath`); and `persona-config-file.md`'s deployment config (host→
  persona pinning, per-persona challenge overrides) via
  `internal/regatta/persona_config.go`. DEV's metadata viewer assembles
  already-loaded structs; it introduces no new state.
- **What's genuinely new, not reuse:**
  - A **watcher-free log-follow mechanism** — periodic re-stat + seek-to-
    last-offset + parse-new-lines, since the existing shared `Watcher` type
    is explicitly off-limits for `logs/` (see Constraint above).
  - A **filterable, column-sortable table UI** — the app has exactly one
    `widget.Table` usage today (`internal/clock/content.go`'s live results
    grid), and it is a fixed-shape, unsorted, unfiltered 6-lane grid. There
    is no header-row, sort, search-box, or filter pattern anywhere in
    `internal/` to build on. DEV's log table is new UI investment, not a
    quick reuse — worth scoping honestly rather than assuming a component
    already exists.
  - If DEV's "detailed" status view includes the **secondary team**, that's
    more than RD/Awards read today. The primitive to do it already exists
    (`teamPathSession(root, team)` in `internal/regatta/director_tree.go` is
    already generic over `Team`) but is only ever invoked for
    `TeamPrimary` outside one guard-only exception — extending it to also
    hydrate/watch `TeamSecondary` for DEV specifically is real, scoped work,
    not a reuse of something already wired for two teams.
- **Resolved overlap with a narrower idea, in DEV's favor**:
  `logging-options.md` (now closed) had speculated an unbuilt Director
  "collect logs" (clipboard/zip) action as a later nicety. That bullet has
  been retired from `persona-plan.md` and `docs/features/TODO.md` in favor
  of DEV, whose broader, standalone log viewer supersedes it outright — log
  collection and visibility across every persona's log file is DEV's
  responsibility, not the Director's, per its own **Does** entry above.

## High-level implementation plan

1. **Registry**: add `RoleDeveloper` and a `Definition` (`Team:
   TeamExecutive`, a challenge code, `Label: "Developer"`, `File: ""`) —
   self-contained, same shape as every other persona addition.
2. **Detailed status tree**: decide and document explicitly whether DEV
   hydrates both teams' timing data or stays primary-only like RD/Awards.
   If both: extend the hydration/watch pattern via the already-generic
   `teamPathSession(root, team)` to also cover `TeamSecondary`, reusing the
   RD tree's per-row refresh pattern for both teams rather than duplicating
   it.
3. **Log viewer**:
   - Enumerate `regattaData/logs/<team>/*.log` across all teams — a plain
     directory listing, no new architecture.
   - Build the watcher-free follow mechanism described above.
   - Parse each NDJSON line into a row (timestamp, level, `persona_id`,
     `team`, `role`, `machine`, `component`, `action`, message, …) and
     render it in a new filterable, sortable table view — the one piece of
     net-new UI work this persona requires.
   - Add a "collect logs" export action (clipboard / zip) over the same
     `regattaData/logs/**` enumeration, for handing off a support report.
4. **Regatta metadata viewer**: a read-only window assembling
   `reader.RegattaData`, `persona.Session` paths, and the deployment
   config's host/challenge state — straightforward, all already-loaded
   data.
5. **Startup wiring**: replace the disabled Developer placeholder button in
   the Admins tab with a real entry, gated by its challenge code.
6. **Docs** (once built, per `AGENTS.md`'s New personas workflow): fold
   into `personas/README.md`, update `docs/features/TODO.md` (this bullet),
   delete this file.

## Dependencies and sequencing

No item in `docs/features/TODO.md` blocks this persona — implementing it
resolves the TODO's own "entirely unscoped" Developer placeholder in the
Admins-tab bullet. Two things are worth sequencing awareness around, not
blockers:

- The log-follow mechanism (step 3) is genuinely new, unshared work — it
  cannot lean on the existing shared `Watcher` type, by explicit design
  constraint, not an oversight to "just fix."
- The filterable/sortable table (also step 3) is new UI investment with no
  existing pattern in the codebase to shorten it.

**No architectural blockers — implementation could start as soon as
engineering capacity is available.** As with Awards, the live process
constraint as of this writing is `develop`'s feature-freeze while the
author functional-tests the current release and clears bugs via patch
releases (see `AGENTS.md`). Writing/refining this proposal is unaffected;
implementation should wait for the freeze to lift or land on a feature
branch that doesn't merge until it does.
