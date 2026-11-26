# Heat Sheet metadata: from the Heat Sheet to the published results

**Status: proposed (2026-11-26).** A design, not yet built. It plans how the
app captures everything the Heat Sheet says, how the Regatta Director (RD)
and Primary Finish Timer (PFT) carry it, and how the published web results
change. Tracked in [TODO.md](TODO.md).

## Why

Converting 115 past regattas into the web results format
(`sample-source/converted/`, built by the local `tools/archive-import` tool)
showed how much the Heat Sheet says that the app throws away. Every piece of
information the archive now publishes was printed on the Heat Sheet all along:
the advancement rule under each heat, each boat's seed, exhibition boats, and
the rowers' names in singles and doubles.

Today the app reads most of it and drops it before results are written. Live
results therefore carry less than the archive does.

**The principle of this design: capture everything the Heat Sheet says,
change none of its wording, and publish it.**

- **The app stays neutral about wording.** Events, flights, advancement rules
  and school names are kept and published exactly as the Heat Sheet prints
  them.
- **Normalizing that wording is a later step.** The archive needed one wording
  (`M-1-4+`, `Petite Final`, `Top 2 to Final`, canonical school names). Live
  data will get it when the [Heat Sheet Creator](personas/new/heat-sheet-creator.md)
  persona writes the standard forms at the source.
- **One exception, and it's optional:** a **school list** that the person
  running the app can configure. When one is configured, the published web
  results use the list's school names and add each renamed school's former
  name. Without one, names publish as printed.

## What the Heat Sheet carries, and what the app keeps today

Each race is a 3-row block. The reader finds it by the 3-row merged race
number in column A ([excel.go](../../internal/reader/excel.go)
`loadRaces` / `loadLanes`) and reads columns C–I into `RawData`
([regattaData.go](../../internal/reader/regattaData.go)). The RD's schedule is
built from that by `store.ScheduleFromRegattaData`
([schedule.go](../../internal/persona/store/schedule.go)).

| Cells | Content | Today |
| --- | --- | --- |
| A, B | race number, start time | kept |
| C row 0 | event, e.g. `W-1-4+` | kept as `BoatClass` |
| C row 1 | flight, e.g. `Heat 1` | kept as `FlightInfo` |
| C row 2 | advancement rule, e.g. `3 to Semi Finals` | read into `RawData`, **dropped** when the schedule is saved |
| D–I row 0 | school, with any A/B suffix | kept as `SchoolName` |
| D–I row 1 | seed (`17`), `Exhibition`, source heat in a final (`Heat 2`), another event (`W-Jr-2x`), `SCRATCHED` | kept as one raw `AdditionalInfo` string; only a scratch is recognized (`Status` `SCR`) |
| D–I row 2 | rowers' names in a 1x or 2x, sometimes `Exhibition` | read into `RawData`, **dropped** when the schedule is saved |
| Break blocks | lunch break | dropped (a separate TODO) |

A real block, race 1 of the 2025 VSRC Day 2 Heat Sheet:

| | C | Lane 1 | Lane 2 | Lane 3 | Lane 4 | Lane 5 |
| --- | --- | --- | --- | --- | --- | --- |
| row 0 | `W-1-4+` | Saint John Paul | James River | Robinson | Norfolk Academy | West Potomac |
| row 1 | `Heat 1` | `17` | `9` | `1` | `8` | `16` |
| row 2 | `3 to Semi Finals` | | | | | |

Today the advancement rule never reaches the results, and the seeds publish
only as bare numbers in `additionalInfo`.

## Heat Sheet import (`internal/reader`): capture, don't reword

- **C row 2** becomes `RaceData.Progression`: the advancement rule exactly as
  printed (`3 to Semi Finals`).
- **Lanes, row 2** become `RaceEntry.Athletes`, as printed.
- **Lanes, row 1** are read into parts. `AdditionalInfo` keeps the full
  printed text, so nothing published today changes:
  - `Seed`: a leading number (`17`, or `8 Exhibition` → 8);
  - exhibition: `Exhibition` / `Ex` sets the entry's status (below);
  - `SourceHeat`: `Heat 2` in a final, where the boat came from;
  - `EntryEvent`: a boat class that differs from the race's (`W-Jr-2x` in a
    `W-2x` race);
  - a scratch keeps today's exact-match rule (`detectStatus`).
- **No rewording.** The event, flight and school text are stored as printed.
- **A layout variant to detect:** the 2025 Heat Sheets put a single's event in
  row 1 and its flight in row 2. The archive tool already handles this
  (`tools/archive-import/heatsheets.go`, `readHeatSheet`); the app reader would
  read the flight as the event.

## Schedule model (`regattaSchedule.json`, written by the RD)

In [schedule.go](../../internal/persona/store/schedule.go):

- **`ScheduleRace`** gains `Progression`.
- **`ScheduleEntry`** gains `Seed *int`, `Athletes`, `SourceHeat` and
  `EntryEvent`.
- **`ScheduleEntryStatus`** gains an exhibition value, `EXH`, beside `SCR`.
  The type's comment already reserves room for it.
- **Compatibility:** every new field is optional (`omitempty`). Older schedule
  files load unchanged; a **Reload Schedule** fills the new fields from the
  Heat Sheet.
- **`ContentHash`** covers the new fields, so a changed advancement rule or
  seed on the Heat Sheet raises the RD's reload banner like any other edit.
- **`LaneMapHash`** keeps covering school, info and status. School names
  aren't rewritten, so upgrading in the middle of a regatta doesn't mark any
  committed result stale (†). The one change is that a lane newly recognized
  as an exhibition boat changes its status, which is a real change in that
  lane.

## RD persona

- **Import confirmation:** the dialog that shows the name, date and race count
  adds a short summary of what was captured: how many advancement rules,
  seeds, exhibition boats and athlete names.
- **RD tree:** can show the advancement rule next to the flight.
- **Schools:** nothing changes. The schedule always holds the Heat Sheet's
  names.

## PFT persona

- **Exhibition boats:** a lane with status `EXH` gets no place in the order of
  finish; the other boats close up (1, 2, 3, … without a gap). It keeps its
  time and split. This is the rule the archive follows, and it changes how the
  PFT assigns places.
- **Race tree and results window:** can show the advancement rule.
- **School list setting:** an optional **School list** file chosen in the
  PFT's **Publish Destinations** settings (`publish_settings.go`), stored with
  the other publish settings in `finish.json`. See the next section.
- **Cancelling a race** is already planned as its own TODO ("PFT: cancel a
  race, with a reason"). It supplies `races[].cancelled` below.
- **A race note** (`races[].note`) has no author yet; see the open questions.

## The school list (optional runtime configuration)

- **Who uses it:** whoever runs the PFT and wants published school names to
  line up across years. With no list configured, names publish exactly as the
  Heat Sheet prints them.
- **Format:** the shape the archive tool already uses for `schools.json`:
  canonical name → aliases, plus a `renamed` table of
  `{name, until}` entries for schools that changed their name.
- **Matching:** ignores case, periods, apostrophes, hyphens and extra spaces. A
  trailing boat designator (`A`, `B`, `C`, `D`) stays with the entry.
- **Code:** a small package, for example `internal/schools`, loads and matches
  the list. It is the archive tool's `schoolKey`, `canonSchool` and
  `formerNameOn` logic moved into the app.
- **Shipped data:** none. The archive's list can be offered as an example file.
- **Unknown names** publish as printed, and the publish report lists them so
  the list's owner can add aliases.
- **Where it applies:** the web results only. The Excel results workbook and
  the app's own screens keep the Heat Sheet's names.

## How the published results change

### Web results (`results-*.json`)

The web results file gains the fields below. Every addition is optional, so
the format stays `schemaVersion` 1: readers ignore fields they don't know,
which is the format's existing rule.

| Level | Field | Source |
| --- | --- | --- |
| Race | `progression` | the Heat Sheet's advancement rule, as printed |
| Race | `cancelled` | the PFT's cancel-race TODO |
| Race | `note` | not decided yet (open question) |
| Race | `status` | `scheduled` for a race not yet raced; see [Every scheduled race](#every-scheduled-race-in-the-web-results) |
| Lane | `seed` | the number from row 1 |
| Lane | `status` | `SCR`, or the new `EXH` |
| Lane | athletes | the rowers' names from row 2, if they are published (open question) |
| Lane and result | `school`, `formerName` | the school list's names, only when a list is configured |
| Result | an exhibition boat's row | an empty `place`, with its time and split |

Unchanged: `boatClass`, `flightInfo` and `additionalInfo` stay as printed.

Race 1 of 2025 VSRC Day 2 today and after (two lanes shown):

```json
{
  "raceNumber": 1, "title": "Race 1 - W-1-4+ - Heat 1", "scheduledTime": "09:00",
  "boatClass": "W-1-4+", "flightInfo": "Heat 1", "winningTime": "06:16.3",
  "lanes": {
    "3": { "school": "Robinson", "additionalInfo": "1", "status": "" },
    "4": { "school": "Norfolk Academy", "additionalInfo": "8", "status": "" }
  },
  "results": [
    { "place": "1", "lane": "4", "school": "Norfolk Academy", "additionalInfo": "8", "split": "", "time": "06:16.3" },
    { "place": "2", "lane": "3", "school": "Robinson", "additionalInfo": "1", "split": "00:05.5", "time": "06:21.8" }
  ]
}
```

```json
{
  "raceNumber": 1, "title": "Race 1 - W-1-4+ - Heat 1", "scheduledTime": "09:00",
  "boatClass": "W-1-4+", "flightInfo": "Heat 1", "winningTime": "06:16.3",
  "progression": "3 to Semi Finals",
  "lanes": {
    "3": { "school": "Robinson", "additionalInfo": "1", "status": "", "seed": 1 },
    "4": { "school": "Norfolk Academy", "additionalInfo": "8", "status": "", "seed": 8 }
  },
  "results": [
    { "place": "1", "lane": "4", "school": "Norfolk Academy", "additionalInfo": "8", "split": "", "time": "06:16.3" },
    { "place": "2", "lane": "3", "school": "Robinson", "additionalInfo": "1", "split": "00:05.5", "time": "06:21.8" }
  ]
}
```

Fields the example leaves out are unchanged (`revision`, `publishedAt`,
`approvedAt`, the other lanes).

### Every scheduled race in the web results

**The idea.** A parent or coach opens the results page in the morning, types
their event or school into the filters, and sees what amounts to their own
slice of the Heat Sheet: every race they care about, with lanes and entries,
and results appearing race by race as the PFT publishes them. To do that, the
web results file has to list **every scheduled race**, not only the ones with
results.

**Where each output stands today:**

| Output | Unraced races |
| --- | --- |
| Excel results workbook | already written: `render` in `internal/publish/spreadsheet/write.go` writes a block for every scheduled race, with blank place, split and time. It already reads like the Heat Sheet. **No change.** |
| Web results | left out: `webjson.Write` writes only races in the publish ledger, i.e. published ones |
| RegattaCentral | must stay results-only. Sending unraced races would waste API calls and isn't what RC's results are for. |

**The change:** the web results file lists every race on the schedule. A race
without results yet is written with its lanes and entries, an empty `results`
list, and `"status": "scheduled"`. It has no `revision`, `publishedAt` or
`winningTime`. Once its results are approved and published, it becomes an
ordinary published race, and `status` is left out. Leaving it out keeps
the archive's races and published races reading as they do today.

**How it stays current:** the file is regenerated whole on every Publish
(that's already how it works), from the schedule as it stands. Each publish
therefore also refreshes every unraced race: a scratch, a lane change or a
final's lanes filled in from the heats. Two additions make it useful before
the first result:

- **Publish Schedule:** a PFT action that writes the file with every race
  scheduled, so the regatta appears on the results page before racing starts.
  It needs no approval, since no results are involved.
- **Refresh on schedule change:** when the RD's schedule changes (the PFT
  already sees schedule reloads), the PFT is offered a refresh, or does it
  automatically. Which one is an open question.

**Risks, and how the design handles each:**

- **A scheduled race taken for a published one.** `webjson.Read` builds the
  publish ledger from every race in the file. If it counted scheduled races,
  the Publish button would show **Published** for races never raced, and
  carry-forward would keep empty results. **Read skips `scheduled` races**;
  only races with a revision enter the ledger. A test pins this.
- **Out-of-date entries between publishes.** An unraced race only changes when
  the file is rewritten. Every Publish rewrites it, and the refresh-on-change
  addition above closes the gap.
- **Finals before their heats are done.** A final's lanes may still hold
  placeholders (`1st`, `2nd`). They publish as printed until the RD fills the
  draw. The results page should show them as "to be decided", not as schools
  (see the open question on placeholders).
- **Telling scheduled from "no results recorded".** The archive has a few
  races that were raced but have no results (`entries only`). The explicit
  `scheduled` status keeps an upcoming race apart from those, and from a
  cancelled race (`cancelled`).
- **Earlier exposure of names.** Entries, and athletes' names if they are
  published, appear before racing instead of after. Heat Sheets are usually
  public already, but this makes the rowers'-names question more pressing.
- **The page's wording.** Today the page says "47 races published". With every
  race listed, it should say how many are official ("12 of 47 official"), mark
  each scheduled race ("Scheduled · 09:00", no results table) and, for the
  sake of the race tree, not count scheduled races as published.
- **Size and sync.** A whole day's schedule is a few hundred kilobytes at most,
  and the file is already rewritten whole on every publish, so Drive sync and
  the Google Sites cache behave as today.

**Which outputs take the whole schedule** becomes a property of each publish
destination. Web results: yes (new). Excel workbook: yes (as today).
RegattaCentral: no, published results only.

### One definition of the format

- **The app's struct is the definition.** `internal/publish/webjson` gains the
  new optional fields, and becomes the single definition of the format. The
  archive tool keeps its own copy today (`tools/archive-import/format.go`); it
  should use the app's.
- **A contract test** decodes every archive file with unknown fields
  disallowed. If the archive and the app ever drift apart, the test fails
  instead of a field silently disappearing.

### Revision and republishing

`publish.Revision` ([publish.go](../../internal/publish/publish.go)) decides
when a race shows **Re-publish**. Today it covers the title, winning time and
result rows only, so a scratch on its own doesn't ask to be republished.
It should cover everything the results page shows: progression, cancelled,
note, lane status, seed and additionalInfo. The first publish after the
upgrade therefore republishes each race once.

### Protecting the archive

Archive files carry `"generator": "regattaClock"`, like the app's own, so the
PFT would take one for its own file and rewrite it if it ever published into
the archive folder for a regatta and season the archive holds. `webjson.Read`
should refuse a file whose `source` starts with `import:`, the same way it
refuses a foreign file, and report it instead.

### Live results and the archive will read differently, for now

Until the Heat Sheet Creator writes standard forms at the source, a live file
says what its Heat Sheet says: `M-1-4` where the archive says `M-1-4+`, and
`3 to Semi Finals` where the archive says `Top 3 to Semi Final`. The results
pages' Event filter matches on text within the event, so it mostly bridges the
gap; the progression badge simply shows each file's own wording.

### What does not change

- **Excel results workbook:** layout, code and content unchanged.
- **RegattaCentral:** its results format is still unknown. Nothing here is
  committed for it beyond the note that it would read the same enriched data.
- **Results pages:** they already show `progression`, `cancelled`,
  `formerName` and Exhibition (via `additionalInfo`). An `EXH` status and the
  seed are small additions to `rowsFor`.

### Docs to update when this is built

- The format table in
  [`examples/web-results/README.md`](../../examples/web-results/README.md):
  `seed`, the `EXH` status, athletes if published, and the archive's
  `source` / `sourceFile`.
- [`publish-destinations.md`](publish-destinations.md): its web JSON example
  already shows a `note` the code doesn't write, and its interface sketch
  predates `Read(path)` / `ByID`.

## Phases

Each phase is its own PR, in this order.

1. **Format contract.** The new optional fields on the webjson struct, the
   contract test against the archive, the archive guard in `webjson.Read`, and
   the README format table. Nothing changes on race day.
2. **The whole day's schedule.** Web results list every scheduled race
   (`status: scheduled`), `Read` keeps them out of the ledger, a per-destination
   "takes the whole schedule" property (web results and Excel yes,
   RegattaCentral no), **Publish Schedule**, and the results page's scheduled
   races and "N of M official". Independent of capture, so it can come early.
3. **Capture.** The reader and schedule model: progression, athletes, the
   row-1 parts and the `EXH` status; `ContentHash`; the RD's import summary.
4. **Publish.** `PublishableRace` and web results fields, `Revision`
   coverage, the PFT's exhibition placing, and the results-page additions.
5. **School list.** `internal/schools`, the PFT setting, and the school names
   and `formerName` in the web results.
6. **Cancel race.** Already a TODO; it adds `races[].cancelled`.
7. **Later: normalization**, with the Heat Sheet Creator persona.

## Open questions

- **Rowers' names:** are they published? They are students' names; the
  archive already publishes coxswains' names.
- **Seeds:** does the results page show them, or only the data carry them?
- **Race notes:** who can add one: the RD, the PFT, or both?
- **Advancement placeholders:** how unfilled lanes in a final (`1st`, `2nd`
  waiting for a heat's result) are shown.
- **Refreshing the schedule:** when the RD's schedule changes, does the PFT's
  app refresh the web results automatically, or offer a one-click refresh?
- **Publish Schedule:** a separate button, or done automatically when the PFT
  starts the regatta with a web results destination chosen?
- **The archive tool's normalization code** (`canonEvent`, `progressionOf`,
  the heat-sheet rules, `canonSchool`) lives on a local, unpushed branch. It
  should be kept somewhere the Heat Sheet Creator work can reuse it.
