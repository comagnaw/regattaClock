# sampleingest

A developer tool that turns one real race-day workbook into the obfuscated
fixture behind regattaClock's hidden `-dev-sample-regatta` flag. See the design
in [sample-regatta.md](../../docs/features/testing/sample-regatta.md).

**Not shipped.** `release.yml` packages only `./cmd/regattaClock`. It imports
no Fyne, so it builds without CGO.

> **Never commit the source workbook.** It names real schools and athletes.
> Keep it in `sample-source/` or name it `*.source.xlsm`; both are gitignored.
> Commit only the fixture, and only after reviewing it.

## Usage

```sh
go run ./cmd/sampleingest -in "sample-source/<day>.xlsm"
```

| Flag | Default | Meaning |
| --- | --- | --- |
| `-in` | (required) | The workbook: a Heat Sheet plus a filled-in Results sheet. |
| `-out` | `internal/sample/data/regatta-day.json` | Where the fixture is written. |
| `-seed` | `1` | Obfuscation seed. The same seed gives the same fixture. |

## What it does

- Reads the **Heat Sheet** through the RD's own import (`reader.ReadExcelFile`).
  Rower names and advancement notes come from each block's third row. Break
  blocks (lunch) are recorded as `BreakAfter`.
- Reads the **Results sheet** through REP's layout
  (`internal/publish/spreadsheet`), joining each block to its race by the
  column-A race number. Split and Time are converted from Excel day fractions to
  the clock's `MM:SS.t`.
- **Obfuscates.** Each real school and rower name maps to one realistic fake,
  drawn from `internal/sample/ingest/words/` by a seeded hash:
  - A school's crews keep their designators (`A`, `B`, `2V`).
  - Bow numbers, `Heat n`, `Exhibition`, `SCR`, event codes such as
    `W-Jr-2x`, and notes such as `3 to Advance` are kept verbatim.
  - The regatta becomes `Sample Regatta Day`, and its date is dropped.
- **Leak check.** Every output string is scanned against the real names it saw.
  Any hit fails the run, and nothing is written.

## Reviewing the output

The report lists the race counts, which races had no results, the breaks, and
how many names were replaced. Before committing, skim the fixture's `School`,
`Rowers`, `AdditionalInfo`, and `Note` values for anything the leak check could
not know was a name, such as a coach's name typed into a note.

Check that it generates, too:

```sh
go run ./cmd/regattaClock -dev-sample-regatta /tmp/sample-check
```
