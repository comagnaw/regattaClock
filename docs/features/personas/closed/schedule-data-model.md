# Schedule data model under personas

**Status: closed (2026-09-20).** Every item this doc raised is resolved and
built. Kept as the historical record of the design (the "one attribute, one
writer" principle, the three-file join, and the reasoning behind each
decision) — not live planning guidance. See "Resolved since this doc was
written" below for the full list and what shipped.

Assessment of what belongs in `director/regattaSchedule.json` once personas own timing results. Companion to [persona-plan.md](../persona-plan.md) and [README.md](../README.md).

## Problem

Today’s persisted schedule (`data.json` → planned `regattaSchedule.json`) is shaped like a **single source of truth for everything**: schedule *and* race results. The on-disk / in-memory types in [`internal/reader/regattaData.go`](../../../../internal/reader/regattaData.go) already carry:

| Field | Layer | Problem under personas |
|-------|--------|-------------------------|
| `RaceEntry.Place` / `Split` / `Time` | Per lane | FT SoT — must not live in RD-owned schedule |
| `RaceData.Saved` / `Approved` | Per race | FT SoT — same |
| `RawData` rows for place/split/time | Per race | Excel results template leftover; duplicates FT fields |
| Schedule fields (name, date, lanes, class, …) | Regatta / race | Legitimate RD / origin SoT |

If FT writes results into `timing/<team>/finish.json` **and** anything still writes Place/Split/Time into `regattaSchedule.json`, there are two conflicting sources of truth for the same attributes. Personas make that conflict structural, not accidental.

## Principle

**One attribute, one writer.**

| Concern | SoT file | Writer |
|---------|----------|--------|
| Regatta meta + race entries (who is in which lane) | `director/regattaSchedule.json` | RD (from origin) |
| Start times | `timing/<team>/start.json` | ST |
| Finish results (laps, OOF, winning time, approval) | `timing/<team>/finish.json` | FT |

**Join keys** (and only these for cross-file identity):

1. **Regatta identity** — e.g. `RegattaKey` derived from schedule name + date (and/or origin fingerprint), carried on timing envelopes so a file cannot attach to the wrong regatta.
2. **`RaceNumber`** — primary key for a race across schedule, start, and finish.

No other timing attribute should be copied back into the schedule file.

A related but distinct mechanism, added later (persona-plan.md §3c, phase
8d): **`ScheduleRace.LaneMapHash()`** — not an identity key (it doesn't
join records), but a fingerprint of one race's lane assignments, stamped
onto `RaceResult.LaneMapHash` when a result is written. Comparing it to
the *live* schedule's current hash detects a result committed against a
lane map that has since changed (a scratch, a lane swap) — the race
tree's `†` stale-lane-map mark. See "How the three files join" below.

## What `regattaSchedule.json` should contain

### Keep (schedule SoT)

**Regatta**

- `Name`
- `Date`
- `Origin` (`Type`, `URI`, fingerprint/`Hash`) — origin metadata for RD reload detection ([persona-plan §3b](../persona-plan.md)). Named `Origin` on the persisted `store.Schedule` (`internal/persona/store/schedule.go`); `reader.SourceInfo` is a same-shaped but distinct in-memory-only type at the reader layer, not what's on disk.

**Per race**

- `RaceNumber`
- `ScheduledTime` — the workbook's Time column, stored as read with no parsing into `time.Time` (`internal/reader/regattaData.go`'s own comment on the field); empty when the source has no value for this race. Shown as its own race-tree column, not folded into the title (`docs/features/TODO.md`'s now-closed Entries-column precedent).
- `BoatClass`
- `FlightInfo`
- `BoatCount` — stored, not derived (settled; see "Resolved since this doc was written" below for the history)
- `Lanes` map: lane → entry **schedule fields only**

**Per lane (schedule entry)**

- `SchoolName`
- `AdditionalInfo` (rower / A–B boat, etc.)
- `Status` — **Status: implemented (2026-09-20).** A scratch keeps its
  `SchoolName` and is marked by a small, store-owned `ScheduleEntryStatus`
  enum (`StatusOK`/`StatusScratched`) on `store.ScheduleEntry` and its
  reader-side mirror `reader.RaceEntryStatus`. The Excel importer's
  `detectStatus` recognizes a scratch by an exact, case-insensitive match
  against `AdditionalInfo` — `"scratched"`, `"scratch"`, or `"scr"` — not a
  substring match, so a school name or class code that happens to contain
  "scr" is never mistaken for a scratch. `SchoolName` is never blanked;
  "School C scratched from this race" and "this lane was never assigned a
  boat" are now distinguishable everywhere downstream (`BoatCount` still
  counts a scratched lane as an entry — it's still an entry, just one not
  expected to race — the race tree, `internal/exporter`'s exported race
  card, `diffSchedule`'s change detector). Deliberately **not** a reuse of
  `internal/regattacentral.LaneStatus`: that enum models RegattaCentral's
  own post-race `ResultStatusType` and was confirmed stale on this branch
  at the time (`"DSQ"` not `"DQ"`, missing `EXH`/`REL`) relative to a
  corrected version on a separate, unmerged branch — reconciling the two is
  a future concern, not blocking this fix. `ScheduleEntryStatus` is shaped
  so that reconciliation is easy later, but only `Scratched` is wired up
  today (schedule-time statuses only — DNF/DNS/DQ/NJ/REL are finish-time
  outcomes that belong to `finish.json`'s domain per this doc's own "one
  attribute, one writer" principle, not the schedule).

### Remove from schedule (move / already in finish)

- `RaceEntry.Place`
- `RaceEntry.Split`
- `RaceEntry.Time`
- `RaceData.Saved`
- `RaceData.Approved`
- Result rows inside `RawData` (place / split / time), if `RawData` is retained at all

**Status: implemented (2026-09-20).** Done on both sides now.
`store.ScheduleRace`/`ScheduleEntry` (the actual `regattaSchedule.json`
type) never had these fields to begin with. `reader.RaceData`/`RaceEntry`
no longer declare `Saved`/`Approved`/`Place`/`Split`/`Time`, and
`RegattaData.ApproveRace()` is gone along with its one call site
(`internal/clock/buttons.go`'s `refereeApprovalFunc`) — confirmed dead by a
full-repo grep before removal: nothing read that mutation back, since the
canonical approval state has been `store.RaceResult.Approved` (via
`store.DeriveTeamState`/`store.CanPublish`) since race-state-machine.md
landed.

### `RawData` recommendation

`RawData` is a 5×7 mirror of the Excel sheet used during import. Historically it doubled as a **parse-debug aid** (“did we read the right cells?”). That remains useful, but it must not ride along as schedule SoT.

**Do not persist `RawData` in `regattaSchedule.json`.** Result-shaped rows (place / split / time) reintroduce FT fields into the RD file, and even schedule-only raw cells bloat what timers load.

#### Keep the troubleshooting corner case without abusing PrefDebug

`PrefDebug` is a **log severity filter** (together with `PrefLogging`), not a directive to change on-disk schemas. Turning Debug on mid-regatta should mean noisier logs — not a different `regattaSchedule.json` shape.

Prefer (in order):

1. **DEBUG log dump on import.** **Status: implemented (2026-09-20).**
   `internal/regatta/loader.go`'s `debugLoader()` — already called only from
   RD Apply/Reload (`callback`, `reloadSchedule`), never the background
   origin-poller — emits one DEBUG `applog` event per race that has boats
   (`HasBoats()`; blank template rows are skipped so a 50-70-row workbook
   doesn't spam the log), with `race`, `class`, `flight`, `scheduledTime`,
   `boatCount`, and the raw 3-row grid. `applog.Debug` is fully self-gating
   (a no-op discard logger when Logging is off), so no caller-side guard was
   needed. Same troubleshooting intent as originally proposed here,
   JSON-line greppable, zero impact on schedule SoT.

2. **Explicit "Save parse diagnostics" (optional RD action).** Not built,
   and not needed — the DEBUG log dump above has proven sufficient. Left
   here only as a fallback option if log lines ever prove awkward in
   practice.

3. **In-memory only during load (always).** Keep `RawData` on the loader’s transient structs until `BoatClass` / `FlightInfo` / `Lanes` are normalized, then drop it before schedule persist.

Avoid: persisting `RawData` when Debug is true and omitting it when false (two schedule schemas), or putting diagnostics in watched SoT paths.

## Target shape (illustrative)

```json
{
  "Name": "Spring Sprints",
  "Date": "2026-04-12",
  "Origin": {
    "Type": "excel",
    "URI": "C:\\Regatta\\SpringSprints.xlsx",
    "Hash": "…"
  },
  "Races": [
    {
      "RaceNumber": 12,
      "ScheduledTime": "09:00 AM",
      "BoatClass": "Varsity 8",
      "FlightInfo": "Heat 1",
      "BoatCount": 4,
      "Lanes": {
        "1": { "SchoolName": "School A", "AdditionalInfo": "", "Status": "" },
        "2": { "SchoolName": "School B", "AdditionalInfo": "A", "Status": "" },
        "3": { "SchoolName": "", "AdditionalInfo": "", "Status": "" },
        "4": { "SchoolName": "School C", "AdditionalInfo": "SCRATCHED", "Status": "SCR" }
      }
    }
  ]
}
```

Lanes 3 and 4 above are two different situations — lane 3 never had a boat
assigned; lane 4 had **School C**, who scratched, with that fact recorded
rather than erased. **This is the real, built shape** — see "Scratches"
under "Per lane (schedule entry)" above.

(Exact JSON key casing stays Go-default PascalCase — see "JSON field tags,"
resolved below.)

## How the three files join

```text
regattaSchedule.json           start.json                 finish.json
──────────────────────         ────────────               ─────────────
Name, Date, Origin             Envelope.RegattaKey  ←──→  Envelope.RegattaKey
Races[].RaceNumber       ←──→  Races[raceNumber]    ←──→  Races[raceNumber]
Races[].ScheduledTime                                     Rows (OOF, Place, Split, Time),
Races[].Lanes[lane].School…    StartedAt, Display…        WinningTime, Approved…
Races[].LaneMapHash()    ─────────────────────────────→  RaceResult.LaneMapHash
                                                          (stamped on write; compared to
                                                          the live schedule to flag a
                                                          result committed against a
                                                          stale lane map)
```

UI composition examples:

- **ST row:** schedule title fields + `start.Races[n]`
- **FT row:** schedule + `start.Races[n]` + optional `finish.Races[n]` progress
- **RD row:** schedule + the **primary team's** start/finish fields by `RaceNumber` (no secondary fallback)
- **Clock open:** schedule lane/school seed + `finish.Races[n]` rehydration if present

When `regattaSchedule.json` changes under a race that already has timing data, **do not rewrite start/finish**. Refresh labels from the schedule; alert FT (OOF/lane map) more strongly than ST. See [persona-plan.md §3c](../persona-plan.md).

## Impact on `internal/reader`

| Before | Under personas |
|-------|----------------|
| `RaceEntry` holds Place/Split/Time | Removed entirely — `internal/reader`'s types no longer declare them |
| `ApproveRace` mutates `RegattaData` | Removed entirely; approval lives only in `finish.json` / FT store |
| Excel loader fills place/split/time from sheet | Import reads the Heat Sheet worksheet, which has no result columns at all — see "Ingest source" below |
| Single `data.json` round-trip | Schedule write never includes FT/ST attributes |

**Status: implemented (2026-09-20).** `store.Schedule` (the persisted
schedule type in `internal/persona/store`) was built exactly this way from
the start. `internal/reader`'s own types caught up: `RaceEntry.Place`/
`Split`/`Time`, `RaceData.Saved`/`Approved`, and `RegattaData.ApproveRace()`
are all removed, confirmed dead in every production path by a full-repo
grep before deletion.

## Ingest source: Results tab vs. Heat Sheet tab

Everything above is about what's persisted once read. This section is
about what's actually **read** — a related but separate concern, raised
alongside a question about the "Results Publisher" (REP, still unbuilt)
writing results to a spreadsheet: could the RD's read and REP's eventual
write ever collide on the same file?

**They can't, by existing design:** `results-publisher.md` already
decided REP writes to "a new, standalone results spreadsheet...
decoupled from the RD's source workbook," and explicitly rules out
writing to the RD's own file. Separately, `store.ScheduleRace`/
`ScheduleEntry` structurally have no Place/Split/Time fields to begin
with, and `Schedule.ContentHash()` — the RD's "did the schedule actually
change" check — never hashes a result cell even if one existed nearby in
the same file. See `results-publisher.md`'s own note on this for the
full evidence trail.

**Status: implemented (2026-09-20).** `internal/reader/excel.go`'s
`findRaceSheet` reads the worksheet named exactly `"Heat Sheet"`
(`common.HeatSheetName`, case/whitespace-insensitive), falling back to
the first worksheet as before when none matches. It parses a 3-row-per-
race block — row 0 is boat class (col C) and per-lane school name
(cols D-I); row 1 is flight/heat info for a team boat (col C) or a
per-lane note otherwise (A/B, an alternate class, `"SCRATCHED"`); row 2
is a per-lane rower's last name for 1x/2x boats, or an advancement note
for a team boat (col C) — captured into `RawData` but not mapped into
`RaceEntry`, since no field exists for it. **No result columns exist in
this shape at all** — `RaceEntry.Place`/`Split`/`Time` stay unset from
this source, which structurally removes the RD's exposure to a tab a
human might be hand-editing with results, rather than relying on
`ContentHash()` to filter that out after the fact (confirmed real-world
by [heatsheet-rc-pivot-investigation.md](../heatsheet-rc-pivot-investigation.md):
a real regatta's workbook has exactly this Results/Heat-Sheet split, and
the Results tab is hand-updated with results and shared as the public
record — the current manual process already does the exact thing REP is
designed not to automate).

Two decisions made when this shipped:

- **No new `Origin.Type`.** `Origin.Type = "heatsheet"` remains reserved
  (`persona-plan.md`'s "Keep Excel out of the long-term core",
  `regattacentral-integration.md` Phase C) for the *future, RC-authored,
  Excel-retiring* ingest path. This stays `Type: "excel"` — only which
  sheet/row-shape is parsed changed, the same
  name-match-else-fallback mechanism as before.
- **Sheet matching is exact, not fuzzy, and block height is hardcoded to
  3 rows — an intentional breaking change.** A workbook whose only sheet
  is Results-shaped (5-row blocks), or that has no sheet named exactly
  `"Heat Sheet"`, now imports **zero races** rather than falling back to
  the old 5-row parse. `examples/Example Regatta Input Table.xlsx` (the
  old minimal Results-only fixture) was retired for exactly this reason
  — see `examples/README.md`.

The old Results-tab layout (5-row blocks, rows 2-4 = Place/Split/Time)
isn't preserved as unused reading code — its exact shape is recorded in
[results-publisher.md](../new/results-publisher.md#existing-code-reuse-analysis)
instead, as the reference for REP's future spreadsheet writer, since that
doc's own decision already commits it to "the same format as the current
manual results worksheet."

**Longer-term goal, not this pivot:** the RD reading the `Heat Sheet` tab
is one piece of retiring reliance on the shared, multi-tab `.xlsm`
workbook format entirely (`Regatta Attributes` / `Heat Sheet` / `Results`
/ `Referee Heat Sheet`, macro-linked) — a stopgap from before the persona
model existed. See [releases.md](../../releases.md#declaring-10)'s
"Declaring 1.0" section for the full goal and what else has to be true
first (mainly: REP shipping a real destination for results).

## Migration

**Status: implemented.** `Regatta.migrateLegacyData` (`internal/regatta/regatta.go`),
called from `loadRegattaData`, is a one-time, no-op-once-migrated move:

1. New writes: only schedule fields → `director/regattaSchedule.json`.
2. On first RD open of a legacy `data.json`: read it into `reader.RegattaData`
   and convert via `scheduleFromRegattaData`, which structurally drops
   Place/Split/Time/Saved/Approved/RawData — `store.Schedule` has nowhere to
   put them, so they are silently dropped rather than explicitly stripped.
   The legacy file is renamed aside (`data.json.migrated`) once the new
   schedule file is written. Covered by `TestMigratesLegacyDataFile`
   (`internal/regatta/schedule_test.go`), which now unmarshals a hand-written
   raw-JSON fixture representing the true historical shape (including the
   old result-field keys) to prove they are dropped on migration — a
   stronger guarantee than a runtime assertion, since the current struct has
   nowhere to put them even if it tried.
3. No finish.json is ever invented from legacy Place/Split/Time — those
   fields were rarely persisted from the clock even before personas, and are
   simply discarded on migration rather than fabricated into FT history.

## Resolved since this doc was written

- **`BoatCount` is stored**, not derived from non-empty lanes — a real
  `store.ScheduleRace.BoatCount` field, round-tripped and part of
  `ContentHash()`.
- **`ScheduledTime`** was added as a new stored per-race field (not
  anticipated when this doc was first written) — the workbook's Time
  column, shown as its own race-tree column, part of `ContentHash()`. See
  "What `regattaSchedule.json` should contain" above.
- **Scratches preserve the school's identity (2026-09-20).** See "Per lane
  (schedule entry)" above — a scratch is now `Status: StatusScratched` with
  `SchoolName` intact, not an erased entry.
- **Reader-side dead code removed (2026-09-20).** `RaceEntry.Place`/`Split`/
  `Time`, `RaceData.Saved`/`Approved`, `RegattaData.ApproveRace()` — see
  "Impact on `internal/reader`" above.
- **Parse diagnostics UX shipped (2026-09-20).** `debugLoader()` emits a
  per-race DEBUG line — see "Keep the troubleshooting corner case" above.
- **JSON field tags — resolved by decision, not code (2026-09-20).** This
  codebase's only real tagging convention is external-API or hand-edited-
  config boundaries (`internal/regattacentral`'s wire format,
  `internal/personacfg`'s hand-edited config) — never internal
  program-to-itself persistence. `store.Schedule` and `reader.RegattaData`
  round-trip within the app only; adding tags would be a new pattern, not a
  continuation of an existing one. Keeping the Go-default PascalCase keys is
  the decision, not a placeholder for one.

## Bottom line

`regattaSchedule.json` holds **regatta metadata + raceNumber + scheduled time + class/flight + lane assignments (school / additional info / status)**. All OOF and FT-captured results live only in `finish.json`; start times only in `start.json`. **`RaceNumber` (+ regatta key) is the join**, with `LaneMapHash` as a secondary staleness check on top of it; everything else has a single persona/team SoT.
