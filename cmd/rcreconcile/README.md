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
> `--upload-preview-out` (Milestone 2) and any `--bulk-file` you keep around
> locally — see [`cmd/rcprobe`'s PII warning](../rcprobe/README.md), which
> applies here identically.

## Commands

### `shape` — learn the real `/bulk` schema, safely

`internal/regattacentral`'s `/bulk` read model is deliberately raw JSON (see
[regattacentral-integration.md](../../docs/features/personas/regattacentral-integration.md)) —
nobody has confirmed the exact field names yet. `bulkEntries` in
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
exactly where the real fields are, with zero risk of leaking PII.

### `reconcile` — the comparison report (Milestone 1)

```sh
go run ./cmd/rcreconcile reconcile \
  --xlsm internal/reader/testdata/example.xlsm \
  --bulk-file internal/regattacentral/testdata/bulk.json \
  --report-out out/reconciliation.html
```

- `--xlsm` — the regatta's workbook. Read with the existing
  [`reader.ReadExcelFile`](../../internal/reader/excel.go) — no new Excel
  parsing. Its "Results" tab already carries both the lineup (school, boat)
  and, once the regatta has happened, the outcome (place/split/time); the
  "Heat Sheet" tab is not read (see the comment on `findRaceSheet`).
- `--bulk-file` — a `/bulk` capture from `rcprobe bulk --out` (or `rcprobe
  bulk --out` pointed at the regatta the xlsm belongs to). There is no
  `--regatta` live-pull flag yet — this tool is offline-only for now, so
  iterating on the matching logic never re-hits the network.
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

## Not yet built

- `--upload-preview-out` (Milestone 2): a dry-run preview of what a
  heat-sheet-and-results "publish" to RegattaCentral would look like, built
  from the xlsm plus this tool's matches. Still never calls the write API.
- A `--regatta` live-pull flag, if offline capture-file iteration turns out not
  to be enough.
