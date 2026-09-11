# rcreconcile

An investigation tool: it balances what [RegattaCentral](https://www.regattacentral.com/)
has on file for a regatta against the `.xlsm` heat-sheet / results workbook the
RD actually produced, and writes a plain-language HTML report for a
non-technical reader (a regatta executive, not a developer). It never calls
RegattaCentral's write API. See
[the investigation doc](../../docs/features/personas/heatsheet-rc-pivot-investigation.md)
for the goal and ground rules this tool exists to serve.

Like [`cmd/rcprobe`](../rcprobe/README.md), it is **not shipped** —
`release.yml` packages only `./cmd/regattaClock` — and it builds without CGO.

> **Never commit rcreconcile output.** Once run against a real capture, the
> report names real athletes, schools, and clubs. `--report-out` is required
> precisely so nothing defaults into the repo; point it under `/out/`, which is
> **gitignored**, or anywhere outside this checkout. The same goes for
> `--upload-preview-out` (Milestone 2) and any `--rc-dir` capture directory you
> keep around locally — see [`cmd/rcprobe`'s PII warning](../rcprobe/README.md),
> which applies here identically.

## Commands

### `shape` — learn the real `/bulk` schema, safely

`internal/regattacentral`'s `/bulk` read model is deliberately raw JSON (see
[regattacentral-integration.md](../../docs/features/personas/regattacentral-integration.md)) —
nobody has confirmed the exact field names yet. `asEntry` in
[`rcmodel.go`](rcmodel.go) is a **provisional**, best-effort guess at where an
"Entry" lives in that JSON and what its fields are called.

`shape` walks a captured `/bulk` file and prints its key-path structure —
field names and JSON types **only, never values** — so it is safe to paste back
into a conversation or an issue:

```sh
go run ./cmd/rcreconcile shape --bulk-file internal/regattacentral/testdata/bulk.json
```

```text
regattas[].events[].entries[].id: number
regattas[].events[].entries[].organization.name: string
regattas[].regatta.name: string
```

If `reconcile` (below) reports "found 0 RegattaCentral entries", this is the
first thing to run — the output tells you (and whoever fixes `rcmodel.go`)
exactly where the real fields are, with zero risk of leaking PII. Run it
against both `bulk.json` and one `entries-<eventID>.json` — the two responses
may not be shaped the same way.

### `reconcile` — the comparison report (Milestone 1)

```sh
go run ./cmd/rcreconcile reconcile \
  --xlsm internal/reader/testdata/example.xlsm \
  --rc-dir internal/regattacentral/testdata \
  --report-out out/reconciliation.html
```

- `--xlsm` — the regatta's workbook. Read with the existing
  [`reader.ReadExcelFile`](../../internal/reader/excel.go) — no new Excel
  parsing. Its "Results" tab already carries both the lineup (school, boat)
  and, once the regatta has happened, the outcome (place/split/time); the
  "Heat Sheet" tab is not read (see the comment on `findRaceSheet`).
- `--rc-dir` — a directory of captured RegattaCentral JSON files, e.g. from
  [`rcprobe walk --out`](../rcprobe/README.md#walk-command) (or the individual
  `bulk` / `entries` commands pointed at the same `--out` directory). Every
  `*.json` file directly inside it is read and merged; there is no live-pull
  flag here — this tool is offline-only, so iterating on the matching logic
  never re-hits the network.
- `--report-out` — where the HTML report is written. Required; see the PII
  warning above.

For each lane in the xlsm, `reconcile` looks for a RegattaCentral entry whose
organization name/short-name/abbreviation matches the xlsm's school name
(normalized, with a short list of common suffix abbreviations like "HS" → "high
school" expanded — see `commonAbbrevExpansions` in [`match.go`](match.go)),
disambiguating multiple boats from the same school with an "A"/"B" label if
one is present. Every lane is classified **matched** / **needs a quick check**
/ **not found**, and any RegattaCentral entry never referenced by a lane is
listed separately (a possible scratch). Nothing is inferred silently — anything
uncertain is left for the report's human reader to judge, the same
"read-only, human reviews and decides" pattern as
[reconciliation.md](../../docs/features/personas/reconciliation.md).

An entry does not have to carry its organization's name inline — a real
capture had none at all, only an id reference. `entriesFromDir` in
[`rcmodel.go`](rcmodel.go) resolves that id against every `organizations.json`
/ org-shaped object found across `--rc-dir` (see `asOrg`); an id that never
resolves still shows up (as "Unknown organization (RegattaCentral id …)")
rather than silently vanishing. `rcprobe walk` fetches `organizations.json`
automatically now, so a `--rc-dir` populated by `walk` already has what this
join needs.

Two more things `reconcile` handles that came up on the first real regatta:

- **Spelling variations.** RegattaCentral writes "St." where an xlsm may write
  "Saint" (and vice versa) — `commonAbbrevExpansions` in `match.go` expands
  both directions before comparing, alongside the existing "HS"/"High School"
  style abbreviations. This is a short, named list, not general spell-checking
  — a variation outside it is left for the report's human reader, same as
  everything else this tool is unsure of.
- **Pools scoped by event, not by a field on the entry.** A school entered in
  more than one boat class at the same regatta used to show up as an
  "ambiguous" candidate in *every* race, because an entry's own boat class has
  never turned out to be reliably inline. `entriesForRace` (`match.go`) scopes
  the candidate pool to the RC event whose label matches a race's boat class,
  using each entry's `EventID` (read straight from its capture's filename,
  e.g. `entries-42.json` → event `42` — no guessing needed there) and an
  `eventID -> label` index built from `events.json` (see `asEvent` —
  PROVISIONAL, same as `asEntry`/`asOrg`). A race whose class can't be matched
  to any event falls back to the old behavior, so this can only narrow a pool,
  never lose a match that worked before. Two boats from the *same* school in
  the *same* event still show up as "ambiguous, needs a quick check" — that is
  the correct answer when RegattaCentral's own data doesn't distinguish them,
  not a bug to chase.

## Troubleshooting: "found 0 RegattaCentral entries"

Three independent, non-exclusive causes, roughly in the order they turned out
to matter on the first real regatta tried:

1. **The capture is missing per-event entries.** It is unconfirmed whether
   `/bulk` nests full entries per event or just event/regatta metadata. Run:

   ```sh
   go run ./cmd/rcprobe walk <regattaID> --out internal/regattacentral/testdata
   ```

   which pulls `/bulk`, `organizations.json`, and then follows bulk with a
   per-event entries call for every event id it finds — one command instead of
   hand-running `entries <eventID>` per event. Re-run `reconcile` against the
   same `--rc-dir` afterward.
2. **An entry references its organization by id, not by name.** Confirmed on a
   real capture (grep the entries file yourself - no school/org name strings
   at all). `entriesFromDir` resolves this automatically as long as
   `organizations.json` is in the same `--rc-dir` (see above); if `reconcile`
   still shows entries with an "Unknown organization" label, the id-field name
   or the organizations shape doesn't match `asEntry`'s / `asOrg`'s guesses —
   run `shape` (below) against both `entries-<id>.json` and
   `organizations.json`.
3. **`rcmodel.go`'s field-name guesses don't match reality.** Run `shape`
   (above) against `bulk.json`, an `entries-<id>.json`, `organizations.json`,
   and `events.json`, and share the (PII-free) output so `asEntry` / `asOrg` /
   `asEvent` / `firstString` / `firstOrgName`'s candidate key lists can be
   widened.

## Not yet built

- `--upload-preview-out` (Milestone 2): a dry-run preview of what a
  heat-sheet-and-results "publish" to RegattaCentral would look like, built
  from the xlsm plus this tool's matches. Still never calls the write API.
- A live `--regatta` pull, if the offline `--rc-dir` workflow turns out not to
  be enough.
