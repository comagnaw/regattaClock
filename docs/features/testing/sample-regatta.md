# Sample regattaData

A developer flag that generates a **full-day, realistic
`regattaData`** — one of our larger real regatta days, obfuscated — so every
persona can be exercised against production-sized files on the multi-machine
Windows setup. It generates **every race-day artifact**, not just
`regattaData`, all into one directory the user provides:

- the Heat Sheet workbook the RD imports;
- the published Results workbook;
- the full `regattaData` tree.

Load testing then covers the RD's import path, the PFT's publish path, and
every persona's files together.

**Built (2026-09-26):** the generator (`internal/sample`), the one-time ingest
(`internal/sample/ingest`, `cmd/sampleingest`), and the hidden flag. The
embedded fixture is a real 69-race day (65 raced, a lunch break after race
36), obfuscated: by default the sample has races 1–60 raced and 61–65
un-raced.

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
  does not map (`internal/reader/regattaData.go`). **Break blocks** — a lunch
  break: a 3-row column-A merge with no race number and `Break` in column B —
  are found by a merge scan of their own (the reader skips them) and recorded
  as the race each one follows.
- **Results sheet** — the 5-row-per-race block (schools / flight+info /
  **Place** / **Split** / **Time**, lanes in columns D–I), read through the
  **shared Results-layout definition REP's writer exports**
  (`internal/publish/spreadsheet`: `BlockOrigin`, `LaneCol`, the `Row*`
  offsets, `HeaderRows` / `BlockRows`). Read cells through those constants,
  not `spreadsheet.Read`: `Read` only accepts RegattaClock-written workbooks
  (it refuses one without its hidden ledger sheet as `ErrForeignWorkbook`),
  and the real `.xlsm` is hand-kept. It stays out of `internal/reader`,
  whose contract is to read the Heat Sheet only.
  - Each block is joined to its race by its **column-A race number, not its
    position**. The Results sheet has no break blocks, so from the first break
    on, block *n* on the two sheets is a different race.
  - Split and Time are stored as **Excel day fractions** formatted `mm:ss.0`.
    The raw number is converted and rounded to the tenth into the clock's
    `MM:SS.t`, rather than trusting Excel's display rounding.
  - Races with boats but no results (an all-scratched race, say) are reported,
    and the generator leaves them un-timed.
- **Obfuscation:**
  - One real→fake map for the whole day, so a school keeps the same fake
    everywhere and crews-per-school stays realistic.
  - The fake is chosen by `sha256(seed + normalized name)` from embedded word
    lists (`internal/sample/ingest/words/`), probing forward past fakes
    already taken or containing a real name. Names are assigned in sorted
    order, so the result is deterministic for a seed.
  - Candidates come in tiers, the realistic one first. A school gets a place
    stem no other school has, plus a suffix (High School, Academy, Crew,
    Rowing Club, Prep, …). A rower gets a plain surname. Only when a tier
    runs out does a name fall back to any stem × suffix, then hyphenated
    surname pairs.
  - Only names are replaced. Non-name tokens are kept: boat designators (`A`,
    `B`, `2V`), `SCRATCHED` / `SCR`, bow numbers, `Heat n`, `Exhibition`,
    event codes (`W-Jr-2x`, `M-2x Exhibition`), and advancement notes. Rower
    and `AdditionalInfo` cells are split on `/`, and each piece is judged on
    its own: vocabulary is kept, and anything else is mapped as a rower
    name.
  - The regatta name becomes `Sample Regatta Day`; the date is dropped and
    applied at generate time.
- **Leak check** — before writing, every output string is scanned against the
  real names collected, plus each distinctive word of them (generic words such
  as *High*, *School*, *Crew* are exempt). A name or word of four or more
  letters matches as a case-insensitive substring; a shorter one (`Lee`) only
  as a whole word, so it cannot trip on an unrelated fake. Any hit fails the
  ingest, and nothing is written.
- **`.gitignore` guard** — `*.source.xlsm`, `/sample-source/`, and any
  root-level `/*.xlsm`, so the real workbook cannot be committed by accident.

### Fixture

`internal/sample/data/regatta-day.json`, embedded with `//go:embed`:

```go
type Fixture struct {
    Name       string
    BreakAfter []int // races a Heat Sheet break block follows
    Races      []Race
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
   3-row race blocks whose third row carries the fake rower names and notes,
   and a `Break` block after each race in `BreakAfter`). `Schedule.Origin`
   then points at a real file, so the RD's 45 s origin poll
   (`internal/regatta/origin.go`) stays quiet, and testers can also run the
   **RD's import path** at full size.
2. **`regattaData/director/regattaSchedule.json`** — made the way the RD's
   import makes it: the generated Heat Sheet is read back through
   `reader.ReadExcelFile` and projected by `store.ScheduleFromRegattaData`.
   That gives `Origin{Type: "excel", URI: <abs xlsx path>, Hash: …}`, and the
   Heat Sheet and schedule agree by construction. It is saved with
   `store.SaveSchedule` under an RD `persona.Session`.
3. **Timing logs for every race with boats except the last `Unraced` of
   them** (by race number). Numbered-but-empty races, where the RD sized the
   sheet for more races than ran, are never timed. Neither is a race with
   boats but no fixture results; the report lists it.
   - `timing/primary/start.json` and `timing/secondary/start.json` —
     `StartedAt` = date + `ScheduledTime` + a deterministic 0–90 s jitter,
     secondary a few tenths off; `Display` in `15:04:05.0`; a plausible
     `timesync.ClockRef{Source: "ntp:sample"}`.
   - `timing/primary/finish.json` — `StartedAt`, `FirstFinishAt` (start +
     winner's time), `StoppedAt` (start + slowest time), `WinningTime`, `Rows`
     from the fixture, `LaneMapHash` from `ScheduleRace.LaneMapHash()`,
     `Approved: true`. These show as **Official**.
   - `timing/secondary/finish.json` — the same finish order, every Time
     shifted a few tenths (splits kept), `WinningTime` set, not approved.
     These show as **Saved**.
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
It then prints a report: races total / with boats / raced / un-raced, the
first un-raced race, and each artifact's path and size.

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
- **Lunch breaks are not represented in the app.** The reader skips a break
  block, so a break shows up only as a gap in scheduled times. The sample
  reproduces the block, so the RD's import at full size includes one; showing
  breaks in the app is a separate item in [TODO.md](../TODO.md).
- **Use a fresh folder per run.** A new root also gives each persona machine a
  fresh local journal namespace, so stale journal entries from a previous
  sample never replay into a new one.

## Existing-code reuse analysis

- **`internal/reader`** — `ReadExcelFile` and `RawData` for the Heat Sheet;
  the Heat Sheet round trip is the generator's own test oracle.
- **`internal/persona/store`** — every on-disk type (`Schedule`, `StartLog`,
  `FinishLog`, `RaceResult`, `LapRow`, `Envelope`), plus `RegattaKey`,
  `LaneMapHash`, `ContentHash`, `SaveSchedule`, and `DeriveTeamState` for
  assertions. `ScheduleFromRegattaData`, the reader → schedule projection,
  moved here from `internal/regatta`, so the RD and the generator share it
  without the generator importing a Fyne package.
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

1. **Done.** Built on `feature/sample-regatta`: `internal/sample`,
   `internal/sample/ingest`, `cmd/sampleingest`, and the CLI flag.
2. **Done.** The author ran `sampleingest` locally against the real `.xlsm`
   (kept in `sample-source/`) and reviewed the JSON. Only the fixture is
   committed. To replace it with another day, see
   [cmd/sampleingest/README.md](../../../cmd/sampleingest/README.md).
3. Merge through a PR.

## Verification

- **Ingest tests** — the public example `.xlsm` has no results filled in, so
  the tests write their own workbook: a Heat Sheet with a break, plus a
  hand-kept-style Results sheet with invented "real" names.
  - Place/Split/Time come through, from day fractions and from text.
  - Results are joined by race number across the break.
  - The same seed gives the same output; one school maps to one fake.
  - Vocabulary is kept verbatim.
  - A planted real name trips the leak check.
  - The example workbook ingests with the reader's race count.
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

**Built**, with the real fixture embedded.
