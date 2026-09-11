# Heat Sheet ↔ RegattaCentral pivot: investigation

Whether regattaClock becoming the thing that populates RegattaCentral's heat
sheet and results — instead of the RD's manual `.xlsm` — is viable, tested
against one real, already-completed regatta before any of it becomes a real
feature. Companion to
[regattacentral-integration.md](regattacentral-integration.md) (Phase C/D,
which this investigation's findings will confirm or revise) and
[`cmd/rcreconcile`](../../../cmd/rcreconcile/README.md) (the tool this doc
describes the use of).

**Status:** investigation, in progress. No product code changes; a dev tool
and its findings only.

## The question

`regattacentral-integration.md` established that RegattaCentral models a race
schedule, lane draws, and results — a "heat sheet" capability — and expects a
timing system to populate it. The RD for this regatta never used that
capability: they built the lineup by hand in an `.xlsm`, copying the roster off
regattacentral.com, and that same workbook's Results tab (hand-updated after
the race) was the public results, shared via SharePoint. Nobody knows what
fit-and-finish would look like if regattaClock pivoted that workflow onto
RegattaCentral, and guessing wrong risks the trust of people who are well
served by the current manual process.

## Ground rules

- **Read + local preview only.** RegattaCentral's write API is never called
  against this real, already-public, already-completed regatta. Any "this is
  what would be published" artifact is a local dry-run — a file on disk, never
  transmitted.
- **The audience is not technical.** The RD is the hands-on operator; the
  regatta executives evaluating this are not developers. Anything meant for
  them to look at is plain language, not JSON — see `cmd/rcreconcile`'s HTML
  report.
- **Real data never leaves the author's machine.** The `.xlsm` and the RC
  `/bulk` capture both carry athlete PII. Neither is committed, pasted into a
  conversation, or published anywhere (an AI assistant's hosted "artifact"
  included — that's an external service). `internal/regattacentral/testdata/`
  and `/out/` are gitignored for exactly this reason (see
  [`cmd/rcprobe/README.md`](../../../cmd/rcprobe/README.md)). Where structural
  help is needed (e.g. confirming the real `/bulk` schema), only
  field-names-and-types are shared — via `rcreconcile shape` — never values.

## The tools: `cmd/rcprobe walk` and `cmd/rcreconcile`

Full detail in [`cmd/rcprobe`'s README](../../../cmd/rcprobe/README.md) and
[`cmd/rcreconcile`'s README](../../../cmd/rcreconcile/README.md). Summary:

1. **`rcprobe walk <regattaID> --out DIR`** pulls `/bulk` and
   `organizations.json`, then structurally discovers event ids in the bulk
   response (no fixed path assumed — it was unconfirmed whether `/bulk` nests
   full entries per event) and calls `entries <eventID>` for each one it
   finds, saving everything into `DIR`. One command instead of hand-running
   `entries` per event and remembering `orgs` separately.
2. **`rcreconcile shape`** walks a captured JSON file and prints its key-path
   structure (field names + JSON types, never values) — how the real schema
   gets confirmed without exposing PII.
3. **`rcreconcile reconcile --rc-dir DIR`** compares the xlsm's lineup (via the
   existing `reader.ReadExcelFile` — no new Excel parsing) against every RC
   entry found across all the JSON files in `DIR` (i.e., everything `walk`
   captured) — resolving an entry that only references its organization by id
   against `organizations.json` in the same directory — and writes a
   plain-language HTML report: which boats matched RegattaCentral
   automatically, which need a human's judgment, and which RC entries the
   lineup never used (possible scratches).
4. **Upload preview** (not yet built): a `--dry-run` rendering of what a
   heat-sheet-and-results "publish" would look like, built from the xlsm and
   `reconcile`'s matches, using the already-typed, already-tested
   `regattacentral.UploadRequest`. `Client.Upload` is never called.

## Findings

_(filled in as the investigation proceeds)_

- Match-rate observed on the real regatta: —
- Common causes of "needs a quick check" / "not found": —
- Did RegattaCentral already have any `Event`/`Race`/`Lane` data for this
  regatta (i.e., was the unused heat-sheet capability truly empty)?: —
- RD / regatta-executive feedback on the report: —
- Go / no-go recommendation for a real Phase C/D: —

## Open items

- **First real run found 0 entries.** The author's real `bulk.json` (984 KB)
  and real xlsm produced 0 recognized RegattaCentral entries and 104/104
  unmatched lanes — the designed degrade-gracefully path did its job (no
  crash, no false match), but it meant the real schema still wasn't confirmed.
- **Second real run (after `rcprobe walk`): event ids were found, entries were
  not.** `walk` correctly discovered real event ids in `bulk.json` and
  captured `entries-<id>.json` for each, but `reconcile` still found 0
  entries. The author inspected `entries-4.json` directly and confirmed **no
  school/org name string appears anywhere in it** — an entry references its
  organization by id only. `asEntry` now accepts an id-only reference and
  `entriesFromDir` resolves it against `organizations.json` (which `walk` now
  fetches automatically); not yet confirmed against the real data whether the
  guessed reference field name (`organizationId` et al.) is right.
- The real `/bulk` / `entries` / `organizations` schema — confirm with
  `rcreconcile shape` against the author's local capture; correct
  `bulkEntries` / `asEntry` / `asOrg` in `cmd/rcreconcile/rcmodel.go`
  accordingly. This is also what Milestone 4 of the swimlane promotes into
  `internal/regattacentral`'s read model, once confirmed.
- Whether the "HS"/"MS"/"JV"/"RC"/"BC" abbreviation-expansion heuristic in
  `cmd/rcreconcile/match.go` needs to grow (or shrink) once tested against real
  organization names.
- Whether a live `--regatta` pull is worth adding to `rcreconcile`, or the
  offline `--rc-dir` workflow is sufficient.
