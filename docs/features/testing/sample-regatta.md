# Sample regattaData

A proposed developer flag that generates a **full-day, realistic
`regattaData`** — one of our larger real regatta days, obfuscated — so every
persona can be exercised against production-sized files on the multi-machine
Windows setup. Nothing here is built yet; this is the design to work from.

**Sequenced after [Results Publisher (REP)](../personas/new/results-publisher.md)**
— see [Dependencies and sequencing](#dependencies-and-sequencing).

## Motivation

End-to-end testing of `develop` across several Windows machines in one domain
(RD, PST/SST, PFT/SFT, AWD on a shared folder) passes, but only ever with small
regattas. A real race day produces a `regattaSchedule.json` with every race and
lane, plus four timing logs (`timing/{primary,secondary}/{start,finish}.json`)
that grow with each race. Load time, watcher churn, tree rendering, and memory
at that size are untested.

Two things are needed:

1. **A full-size load.** A regattaData shaped like a real, large day — race
   count, boat classes, lane fill, scratches, name lengths — not a synthetic
   three-race fixture.
2. **An end-of-day scenario.** The **last five races left un-raced**, so testers
   can load the sample into a test environment and run the final races of the
   day end to end on real machines.

## Decisions made (author, 2026-09-26)

- **Ingest once, never commit the source.** The real `.xlsm` (Heat Sheet plus a
  filled-in Results sheet) is read one time on a developer machine. Only the
  **obfuscated fixture** it produces is committed; the workbook stays local.
- **Realistic, deterministic fakes.** School and rower names are replaced by
  realistic-looking fakes drawn from built-in word lists with a fixed seed —
  not `School 07` / `Rower 0142` placeholders, whose uniform short lengths
  would hide truncation and layout issues. Re-running the ingest with the same
  seed gives the same output.
- **A hidden flag in the main binary.** `regattaClock -dev-sample-regatta <dir>`
  writes the sample and exits, so a Windows test machine needs nothing beyond
  the normal build. Not shown in any UI or help text.
- **Last five races un-raced** by default, adjustable by flag.

## Design

### One-time ingest

`go run ./cmd/sampleingest -in <day.xlsm> -out internal/sample/data/regatta-day.json [-seed N]`

A developer-only command (`flag.NewFlagSet`, following `cmd/rcprobe`), with its
logic in `internal/sample/ingest` so it can be tested.

- **Heat Sheet** — reuse `reader.ReadExcelFile` (`internal/reader/excel.go`)
  for race numbers, scheduled times, boat class, flight, and per-lane
  `SchoolName` / `AdditionalInfo` / `Status`. Rower last names (1x/2x) and
  advancement notes come from `RawData` row 2, which the reader parses but
  does not map (`internal/reader/regattaData.go`).
- **Results sheet** — the 5-row-per-race block (schools / flight+info /
  **Place** / **Split** / **Time**, lanes in columns D–I), read through the
  **shared Results-layout definition REP's writer introduces**. It stays out
  of `internal/reader`, whose contract is to read the Heat Sheet only. Races
  with no results (an all-scratched race, say) are recorded as un-raced and
  reported.
- **Obfuscation:**
  - One real→fake map for the whole day, so a school keeps the same fake
    everywhere and crews-per-school stays realistic.
  - The fake is chosen by `sha256(seed + normalized name)` from embedded word
    lists — place-name stems × {High School, Academy, Crew, Rowing Club, Prep,
    …} for schools, surnames for rowers — with deterministic collision
    resolution.
  - Only names are replaced. Non-name tokens are kept: boat designators (`A`,
    `B`, `2V`), `SCRATCHED` / `SCR`, event codes in `AdditionalInfo`, and
    advancement notes. `Smith/Jones` doubles are split on `/` and mapped per
    name.
  - The regatta name becomes `Sample Regatta Day`; the date is dropped and
    applied at generate time.
- **Leak check** — before writing, every output string is scanned
  (case-insensitive substring) against the set of real names collected. Any
  hit fails the ingest.
- **`.gitignore` guard** — `*.source.xlsm` and `/sample-source/`, so the real
  workbook cannot be committed by accident.

### Fixture

`internal/sample/data/regatta-day.json`, embedded with `//go:embed`:

```go
type Fixture struct {
    Name  string
    Races []Race
}

type Race struct {
    RaceNumber                                  int
    ScheduledTime, BoatClass, FlightInfo, Note string
    Lanes                                       map[int]Lane
}

type Lane struct {
    School, AdditionalInfo, Rowers string
    Status                         store.ScheduleEntryStatus
    Place, Split, Time             string // from the Results sheet; "" when un-raced
}
```

### Generator

`internal/sample.Generate(opts Options) (Report, error)`, with
`Options{Dir; Date /*default today, local*/; Unraced /*default 5*/; Now}`.
Into `<Dir>` it writes:

1. **`Sample Heat Sheet.xlsx`** — the obfuscated Heat Sheet in the layout
   `reader.ReadExcelFile` expects (merged A1:I1 name + " Heat Sheet", A2 date,
   3-row race blocks, row 2 carrying the fake rower names). `Schedule.Origin`
   then points at a real file, so the RD's 45 s origin poll
   (`internal/regatta/origin.go`) stays quiet, and testers can also run the
   **RD's import path** at full size.
2. **`regattaData/director/regattaSchedule.json`** — a `store.Schedule` with
   `Origin{Type: "excel", URI: <abs xlsx path>, Hash: filesystem.FileHash(…)}`,
   saved with `store.SaveSchedule` under an RD `persona.Session`.
3. **Timing logs for every race except the last `Unraced`** (by race number):
   - `timing/primary/start.json` and `timing/secondary/start.json` —
     `StartedAt` = date + `ScheduledTime` + a deterministic 0–90 s jitter,
     secondary a few tenths off; `Display` in `15:04:05.0`; a plausible
     `timesync.ClockRef{Source: "ntp:sample"}`.
   - `timing/primary/finish.json` — `StartedAt`, `FirstFinishAt` (start +
     winner's time), `StoppedAt` (start + slowest time), `WinningTime`, `Rows`
     from the fixture, `LaneMapHash` from `ScheduleRace.LaneMapHash()`,
     `Approved: true`. These show as **Official**.
   - `timing/secondary/finish.json` — the same rows lightly perturbed,
     `WinningTime` set, not approved. These show as **Saved**.
   - Envelopes are stamped directly: `store.SchemaVersion`, role/team,
     `store.RegattaKey(name, date)`, `Machine: "SAMPLE-PFT"` and so on.
4. **A sample results workbook** for the raced races, written through
   **REP's spreadsheet writer** — the same file REP appends to when the last
   five races are published on test day.

Timing logs are written with `filesystem.SaveJSONFileAtomic` to
`persona.Session.WritePath()`, **bypassing the journal**: `store.SaveStart` /
`SaveFinish` stage through a local write-ahead journal under
`os.UserCacheDir()`, which a generator must not touch. The generator
**refuses if `<Dir>/regattaData` already exists**, and prints a report: races
total / raced / un-raced, the first un-raced race, and each file's size.

### Command line

In `cmd/regattaClock/main.go`:

- `-dev-sample-regatta <dir>`
- `-dev-sample-unraced <n>` — default 5
- `-dev-sample-date <YYYY-MM-DD>` — default today

Parsed by a hand `os.Args` scan beside `versionRequested`, for the same reason
(the `flag` package would claim Fyne's `-fyne*` flags). It runs before
`app.NewWithID`, so no window opens, and exits non-zero on error.

## Constraints and gotchas

- **Date the sample today.** The past-date gate
  (`internal/regatta/date_guard.go`) blocks loading a regatta dated before the
  host's local date, and the regatta date feeds `RegattaKey`, which every
  timing envelope must match.
- **Un-raced is absence.** Race state is derived by `store.DeriveTeamState`
  from the timing files; a race missing from every `Races` map is Pending
  Start. There is no marker to write.
- **`LaneMapHash` must match** the schedule's, or the race carries the stale
  lane-map `†`.
- **Use a fresh folder per run.** A new root also gives each persona machine a
  fresh local journal namespace, so stale journal entries from a previous
  sample never replay into a new one.

## Existing-code reuse analysis

- **`internal/reader`** — `ReadExcelFile` and `RawData` for the Heat Sheet;
  the Heat Sheet round trip is the generator's own test oracle.
- **`internal/persona/store`** — every on-disk type (`Schedule`, `StartLog`,
  `FinishLog`, `RaceResult`, `LapRow`, `Envelope`), plus `RegattaKey`,
  `LaneMapHash`, `ContentHash`, `SaveSchedule`, and `DeriveTeamState` for
  assertions.
- **`internal/persona`** — `Session` path helpers
  (`SchedulePath` / `StartPath` / `FinishPath` / `WritePath`).
- **`internal/filesystem`** — `SaveJSONFileAtomic`, `FileHash`.
- **REP** — the Results-layout definition (ingest reads it), the `.xlsx`
  writer (sample results workbook), and the excelize write-path groundwork
  that `Sample Heat Sheet.xlsx` builds on.
- **Genuinely new** — the obfuscator and its word lists, the leak check, the
  fixture, the timing synthesizer, and the CLI scan.

## Implementation plan

1. Branch from `develop`; build `internal/sample`, `internal/sample/ingest`,
   `cmd/sampleingest`, and the CLI flag with a **small synthetic fixture**,
   tested against `examples/Example Heat Sheets and Results With Macros.xlsm`
   (already public).
2. Merge through a PR.
3. The author runs `sampleingest` locally against the real `.xlsm`, reviews
   the JSON, and commits **only** the fixture.

## Verification

- **Ingest tests** — the example `.xlsm` yields the expected race count and
  Place/Split/Time; the same seed gives the same output; one school maps to
  one fake; a planted real name trips the leak check.
- **Generator tests** — the tree loads through `store.LoadSchedule` /
  `LoadStart` / `LoadFinish`; `RegattaKey` matches across files;
  `DeriveTeamState` is Approved (primary) and Saved (secondary) for the raced
  races and NotStarted for the last five; no `†`;
  `reader.ReadExcelFile("Sample Heat Sheet.xlsx")` gives the same
  `ContentHash` as the written schedule; an existing `regattaData` is refused.
- **CLI test** — argument parsing for the three flags.
- **Manual, Mac** — `go run ./cmd/regattaClock -dev-sample-regatta <tmp>`,
  launch as RD: every race shows, the last five are Pending Start, the rest
  Official, no origin or stale banners.
- **Manual, Windows domain** — generate onto the share; bring up RD, PST/SST,
  PFT/SFT, and AWD on separate machines; time, approve, and publish the last
  five races. Watch load latency, watcher churn, and memory.

## Dependencies and sequencing

- **Hard dependency: REP's local `.xlsx` results write** (its
  spreadsheet-writer slice, itself after
  [operational-state.md](../personas/operational-state.md)'s
  `ResultsDestination`). REP owns the Results-layout definition and the first
  `.xlsx` writer in the repo; this feature reads the one and calls the other.
- **Not blocked on** REP's later RegattaCentral destination, or on any
  [integration-testing.md](integration-testing.md) item.

**Start once REP's local results-spreadsheet write has landed.**
