# Sample regattaData

A proposed developer flag that generates a **full-day, realistic
`regattaData`** — one of our larger real regatta days, obfuscated — so every
persona can be exercised against production-sized files on the multi-machine
Windows setup. It generates **every race-day artifact**, not just
`regattaData`, all into one directory the user provides:

- the Heat Sheet workbook the RD imports;
- the published Results workbook;
- the full `regattaData` tree.

Load testing then covers the RD's import path, the PFT's publish path, and
every persona's files together. Nothing here is built yet; this is the design
to work from.

**Unblocked (2026-09-26):** its hard dependency, the spreadsheet writer from
[Results Publisher (REP)](../personas/new/results-publisher.md), has landed
(#126, #127). See [Dependencies and sequencing](#dependencies-and-sequencing).

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
- **Every artifact under one user-provided directory** (added 2026-09-26).
  `<Dir>` receives the Heat Sheet workbook, the Results workbook, and
  `regattaData/`, so a test run starts with everything a race day would have
  at that point. The Results workbook goes in its own `<Dir>/results/`
  folder, separate from `regattaData` as it is on race day (a different
  drive). The primary `finish.json` records that folder as its confirmed
  `ResultsDir`, the state a real PFT is in mid-day.

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
  **shared Results-layout definition REP's writer exports**
  (`internal/publish/spreadsheet`: `BlockOrigin`, `LaneCol`, the `Row*`
  offsets, `HeaderRows` / `BlockRows`). Read cells through those constants,
  not `spreadsheet.Read`: `Read` only accepts RegattaClock-written workbooks
  (it refuses one without its hidden ledger sheet as `ErrForeignWorkbook`),
  and the real `.xlsm` is hand-kept. It stays out of `internal/reader`,
  whose contract is to read the Heat Sheet only. Races
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
`<Dir>` is the **artifact root**. Everything the generator writes lands under
it, and nothing is written anywhere else:

```text
<Dir>/
  Sample Heat Sheet.xlsx                      RD's source workbook (Heat Sheet layout)
  results/
    Sample Regatta Day - <Date> Results.xlsx  REP workbook: raced races published, last N blank
  regattaData/
    director/regattaSchedule.json             Origin.URI -> absolute path of the heat sheet
    timing/primary/start.json
    timing/primary/finish.json                ResultsDir -> absolute path of <Dir>/results
    timing/secondary/start.json
    timing/secondary/finish.json
```

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
   **REP's `spreadsheet.Write`** to `<Dir>/results/` (created by the
   generator). This is the same file the PFT regenerates when the last five
   races are published on test day. REP rewrites the whole workbook rather
   than appending, so for the PFT to treat the file as its own and extend
   it:
   - Name it `spreadsheet.FileName(name, date)` inside `<Dir>/results/`.
   - Set `Meta.RegattaKey` to `store.RegattaKey(name, date)`. A workbook
     with another regatta's key is refused (`ErrOtherRegatta`).
   - Pass a `Ledger` holding each raced race's `publish.Revision`, taken
     from `publish.BuildView` over the generated schedule and finish.json.
     Those races then show as **Published**.

   The primary `finish.json`'s `ResultsDir` is set to the absolute path of
   `<Dir>/results` (`filepath.Abs`). The PFT then opens with the raced
   races showing **Published** and publishes the last five into the same
   workbook with no folder prompt, provided the share mounts at the same
   path on the PFT machine. See
   [Constraints and gotchas](#constraints-and-gotchas).

Timing logs are written with `filesystem.SaveJSONFileAtomic` to
`persona.Session.WritePath()`, **bypassing the journal**: `store.SaveStart` /
`SaveFinish` stage through a local write-ahead journal under
`os.UserCacheDir()`, which a generator must not touch; no Fyne preferences are
written either. The generator **creates `<Dir>` if it is absent and refuses
unless it is empty**, so one run never mixes artifacts from two samples.
It then prints a report: races total / raced / un-raced, the first un-raced
race, and each artifact's path and size.

### Command line

In `cmd/regattaClock/main.go`:

- `-dev-sample-regatta <dir>` — the artifact root; receives the heat sheet,
  `results/`, and `regattaData/`
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
- **Absolute paths are from the generating machine.** Two generated paths
  are absolute: `Origin.URI` (the heat sheet) and the primary `finish.json`'s
  `ResultsDir`.
  - If another machine can't reach `Origin.URI`, the RD's origin poll only
    logs at debug level (`pollOrigin`, `internal/regatta/origin.go`) and
    shows no banner. Harmless.
  - If the PFT can't reach `ResultsDir`, it gets the designed "results
    folder can't be reached — choose a folder" prompt (`ensureResultsDir`,
    `internal/regatta/publish_results.go`).

  To skip that prompt, generate on a machine that maps the share at the
  same path as the test machines (the same drive letter or UNC path).
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
- **REP** (`internal/publish/spreadsheet`, `internal/publish`) — the
  Results-layout definition (ingest reads it), `spreadsheet.Write` and
  `FileName` (sample results workbook), `publish.BuildView` / `Revision`
  (its ledger), and the excelize write-path groundwork that
  `Sample Heat Sheet.xlsx` builds on.
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
- **Generator tests**:
  - Every artifact is under `<Dir>` — heat sheet, `results/` workbook,
    `regattaData/` — and nothing is written outside it.
  - The tree loads through `store.LoadSchedule` / `LoadStart` / `LoadFinish`,
    and `RegattaKey` matches across files.
  - `DeriveTeamState` is Approved (primary) and Saved (secondary) for the
    raced races and NotStarted for the last five; no `†`.
  - `reader.ReadExcelFile("Sample Heat Sheet.xlsx")` gives the same
    `ContentHash` as the written schedule.
  - `spreadsheet.Read(<Dir>/results/…)` succeeds and `CheckRegatta` passes,
    and its ledger holds exactly the raced races.
  - `LoadFinish` (primary) has `ResultsDir` set to the absolute
    `<Dir>/results`.
  - A non-empty `<Dir>` is refused.
- **CLI test** — argument parsing for the three flags.
- **Manual, Mac** — `go run ./cmd/regattaClock -dev-sample-regatta <tmp>`,
  launch as RD: every race shows, the last five are Pending Start, the rest
  Official, no origin or stale banners.
- **Manual, Windows domain** — generate onto the share; bring up RD, PST/SST,
  PFT/SFT, and AWD on separate machines. The PFT should show the raced races
  as **Published** with no folder prompt. Time, approve, and publish the last
  five races into the same `<Dir>/results/` workbook. Watch load latency,
  watcher churn, and memory.

## Dependencies and sequencing

- **Hard dependency, satisfied: REP's local `.xlsx` results write** (#126,
  #127). REP owns the Results-layout definition and the first `.xlsx` writer
  in the repo (`internal/publish/spreadsheet`); this feature reads the one
  and calls the other.
- **Not blocked on** REP's deferred RegattaCentral destination, or on any
  [integration-testing.md](integration-testing.md) item.

**No blockers — can start now.**
