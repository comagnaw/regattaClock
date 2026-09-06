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

## What `regattaSchedule.json` should contain

### Keep (schedule SoT)

**Regatta**

- `Name`
- `Date`
- `SourceInfo` (`Type`, `URI`, fingerprint/`Hash`) — origin metadata for RD reload detection ([persona-plan §3b](persona-plan.md))

**Per race**

- `RaceNumber`
- `BoatClass`
- `FlightInfo`
- `BoatCount` (or derive from non-empty lanes on read)
- `Lanes` map: lane → entry **schedule fields only**

**Per lane (schedule entry)**

- `SchoolName`
- `AdditionalInfo` (rower / A–B boat, etc.)
- Optional later: explicit scratch flag if the origin encodes it separately from empty school name

### Remove from schedule (move / already in finish)

- `RaceEntry.Place`
- `RaceEntry.Split`
- `RaceEntry.Time`
- `RaceData.Saved`
- `RaceData.Approved`
- Result rows inside `RawData` (place / split / time), if `RawData` is retained at all

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
  "SourceInfo": {
    "Type": "excel",
    "URI": "C:\\Regatta\\SpringSprints.xlsx",
    "Hash": "…"
  },
  "Races": [
    {
      "RaceNumber": 12,
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
regattaSchedule.json          start.json                 finish.json
─────────────────────         ────────────               ─────────────
Name, Date, SourceInfo        Envelope.RegattaKey  ←──→  Envelope.RegattaKey
Races[].RaceNumber      ←──→  Races[raceNumber]    ←──→  Races[raceNumber]
Races[].Lanes[lane].School…   StartedAt, Display…        Rows (OOF, Place, Split, Time),
                                                         WinningTime, Approved…
```

UI composition examples:

- **ST row:** schedule title fields + `start.Races[n]`
- **FT row:** schedule + `start.Races[n]` + optional `finish.Races[n]` progress
- **RD row:** schedule + primary (fallback secondary) start/finish fields by `RaceNumber`
- **Clock open:** schedule lane/school seed + `finish.Races[n]` rehydration if present

When `regattaSchedule.json` changes under a race that already has timing data, **do not rewrite start/finish**. Refresh labels from the schedule; alert FT (OOF/lane map) more strongly than ST. See [persona-plan.md §3c](persona-plan.md).

## Impact on `internal/reader`

| Today | Under personas |
|-------|----------------|
| `RaceEntry` holds Place/Split/Time | Split into schedule entry vs finish row types (or clear result fields on schedule persist) |
| `ApproveRace` mutates `RegattaData` | Approval only in `finish.json` / FT store |
| Excel loader fills place/split/time from sheet | Import **ignores** result columns for schedule SoT (sheet may still have empty result rows) |
| Single `data.json` round-trip | Schedule write never includes FT/ST attributes |

Prefer a dedicated schedule type in `internal/persona/store` or a slimmed reader type used for persistence, rather than overloading `RegattaData` as both “imported schedule” and “session with results.”

## Migration

1. New writes: only schedule fields → `director/regattaSchedule.json`.
2. On first RD open of a legacy `data.json`: strip Place/Split/Time/Saved/Approved (and trim `RawData`) when migrating to `regattaSchedule.json`.
3. Do **not** invent finish.json from legacy Place/Split/Time in schedule — those fields were rarely persisted from the clock today anyway; treating them as schedule pollution to drop is safer than fabricating FT history.

## Open decisions (small)

- **Scratches:** encoded as empty `SchoolName`, text in `AdditionalInfo` (e.g. `SCRATCHED`), or a future `Status` field on the lane entry — pick one convention when Excel/API origin is normalized.
- **Whether `BoatCount` is stored or derived** from non-empty lanes.
- **JSON field tags** for stable lowercase keys vs current PascalCase defaults.
- **Parse diagnostics UX:** DEBUG log dump on import is enough for most Excel parse checks; add an RD “Save parse diagnostics” action only if log lines prove awkward in practice.

## Bottom line

`regattaSchedule.json` should shrink to **regatta metadata + raceNumber + class/flight + lane assignments (school / additional info)**. All OOF and FT-captured results belong only in `finish.json`; start times only in `start.json`. **`RaceNumber` (+ regatta key) is the join**; everything else has a single persona/team SoT.
