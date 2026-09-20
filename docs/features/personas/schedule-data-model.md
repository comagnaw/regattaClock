# Schedule data model under personas

Assessment of what belongs in `director/regattaSchedule.json` once personas own timing results. Companion to [persona-plan.md](persona-plan.md) and [README.md](README.md).

## Problem

Today’s persisted schedule (`data.json` → planned `regattaSchedule.json`) is shaped like a **single source of truth for everything**: schedule *and* race results. The on-disk / in-memory types in [`internal/reader/regattaData.go`](../../../internal/reader/regattaData.go) already carry:

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
- `Origin` (`Type`, `URI`, fingerprint/`Hash`) — origin metadata for RD reload detection ([persona-plan §3b](persona-plan.md)). Named `Origin` on the persisted `store.Schedule` (`internal/persona/store/schedule.go`); `reader.SourceInfo` is a same-shaped but distinct in-memory-only type at the reader layer, not what's on disk.

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
- Scratches: settled as an empty `SchoolName`, optionally noted in `AdditionalInfo`
  (`store.ScheduleEntry`'s own doc comment) — not yet an explicit `Status` field;
  that remains a possible future addition if the origin ever encodes it separately.

### Remove from schedule (move / already in finish)

- `RaceEntry.Place`
- `RaceEntry.Split`
- `RaceEntry.Time`
- `RaceData.Saved`
- `RaceData.Approved`
- Result rows inside `RawData` (place / split / time), if `RawData` is retained at all

**Status:** done on the persisted side — `store.ScheduleRace`/`ScheduleEntry`
(the actual `regattaSchedule.json` type) never had these fields to begin
with. **Not done on the reader side** — `reader.RaceData`/`RaceEntry` still
declare `Saved`/`Approved`/`Place`/`Split`/`Time`, and
`RegattaData.ApproveRace()` still writes `RaceData.Approved = true` from the
live referee-approval flow (`internal/clock/buttons.go`) — but nothing
reads it back; the canonical approval state has been `store.RaceResult.Approved`
(via `store.DeriveTeamState`/`store.CanPublish`) since race-state-machine.md
landed. Tracked as a small cleanup —
[docs/features/TODO.md](../TODO.md#personas--feature-follow-ups).

### `RawData` recommendation

`RawData` is a 5×7 mirror of the Excel sheet used during import. Historically it doubled as a **parse-debug aid** (“did we read the right cells?”). That remains useful, but it must not ride along as schedule SoT.

**Do not persist `RawData` in `regattaSchedule.json`.** Result-shaped rows (place / split / time) reintroduce FT fields into the RD file, and even schedule-only raw cells bloat what timers load.

#### Keep the troubleshooting corner case without abusing PrefDebug

`PrefDebug` is a **log severity filter** (together with `PrefLogging`), not a directive to change on-disk schemas. Turning Debug on mid-regatta should mean noisier logs — not a different `regattaSchedule.json` shape.

Prefer (in order):

1. **DEBUG log dump on import (recommended).** When Logging and Debug are both on, during Excel (or future API) load emit a DEBUG `applog` event per race with the raw grid (or a compact form: race number + non-empty cells). Same troubleshooting intent as today, JSON-line greppable, zero impact on schedule SoT. Only on RD Apply/Reload — not on timer hot paths.

2. **Explicit “Save parse diagnostics” (optional RD action).** If a full grid on disk is easier to diff than log lines, write a **sidecar** under e.g. `regattaData/logs/executive/parse-<hostname>-<timestamp>.json` (watcher ignores `logs/`). Triggered by an RD menu item, not by flipping Debug alone.

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
        "1": { "SchoolName": "School A", "AdditionalInfo": "" },
        "2": { "SchoolName": "School B", "AdditionalInfo": "A" },
        "3": { "SchoolName": "", "AdditionalInfo": "" },
        "4": { "SchoolName": "School C", "AdditionalInfo": "SCRATCHED" }
      }
    }
  ]
}
```

(Exact JSON key casing can stay Go-default or gain tags later; the ownership split matters more than tags.)

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

When `regattaSchedule.json` changes under a race that already has timing data, **do not rewrite start/finish**. Refresh labels from the schedule; alert FT (OOF/lane map) more strongly than ST. See [persona-plan.md §3c](persona-plan.md).

## Impact on `internal/reader`

| Today | Under personas |
|-------|----------------|
| `RaceEntry` holds Place/Split/Time | Split into schedule entry vs finish row types (or clear result fields on schedule persist) |
| `ApproveRace` mutates `RegattaData` | Approval only in `finish.json` / FT store |
| Excel loader fills place/split/time from sheet | Import **ignores** result columns for schedule SoT (sheet may still have empty result rows) |
| Single `data.json` round-trip | Schedule write never includes FT/ST attributes |

Prefer a dedicated schedule type in `internal/persona/store` or a slimmed reader type used for persistence, rather than overloading `RegattaData` as both “imported schedule” and “session with results.” This table's aspiration is realized for the **persisted** schedule (`store.Schedule`, built exactly this way) but not yet for `internal/reader`'s own types — see the "Status" note under "Remove from schedule" above: `ApproveRace` still mutates `RegattaData`, it just no longer matters, since nothing reads that mutation back.

## Migration

1. New writes: only schedule fields → `director/regattaSchedule.json`.
2. On first RD open of a legacy `data.json`: strip Place/Split/Time/Saved/Approved (and trim `RawData`) when migrating to `regattaSchedule.json`.
3. Do **not** invent finish.json from legacy Place/Split/Time in schedule — those fields were rarely persisted from the clock today anyway; treating them as schedule pollution to drop is safer than fabricating FT history.

## Resolved since this doc was written

- **`BoatCount` is stored**, not derived from non-empty lanes — a real
  `store.ScheduleRace.BoatCount` field, round-tripped and part of
  `ContentHash()`.
- **Scratches:** settled as an empty `SchoolName`, optionally noted in
  `AdditionalInfo` — `store.ScheduleEntry`'s own doc comment confirms this
  is the working convention today. A future explicit `Status` field
  remains possible if an origin ever encodes scratches separately, but
  that's a "could," not an open pick blocking anything.
- **`ScheduledTime`** was added as a new stored per-race field (not
  anticipated when this doc was first written) — the workbook's Time
  column, shown as its own race-tree column, part of `ContentHash()`. See
  "What `regattaSchedule.json` should contain" above.

## Open decisions (small)

- **JSON field tags** for stable lowercase keys vs current PascalCase defaults.
- **Parse diagnostics UX:** still unbuilt. DEBUG log dump on import (option 1
  above) is the recommended default; add an RD “Save parse diagnostics”
  action only if log lines prove awkward in practice.

## Bottom line

`regattaSchedule.json` should shrink to **regatta metadata + raceNumber + scheduled time + class/flight + lane assignments (school / additional info)**. All OOF and FT-captured results belong only in `finish.json`; start times only in `start.json`. **`RaceNumber` (+ regatta key) is the join**, with `LaneMapHash` as a secondary staleness check on top of it; everything else has a single persona/team SoT.
