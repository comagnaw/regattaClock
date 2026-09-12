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

- **Real matches are expected** as of the sixth real run (after the org-id
  join, the "St."/"Saint" fix, the short-abbreviation guard, file-scoped
  org/event extraction, and - the fix that actually made event-scoping work -
  roster-overlap event resolution). Exact match-rate, remaining "needs a
  quick check" / "not found" causes, whether RC already had any
  `Event`/`Race`/`Lane` data for this regatta, and RD/executive feedback:
  still to be recorded here.
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
  fetches automatically).
- **Third real run: real matches, plus two quality issues, both confirmed and
  fixed.** (1) RegattaCentral spells some organizations "St." where the xlsm
  spells them "Saint" — `commonAbbrevExpansions` (`cmd/rcreconcile/match.go`)
  now expands both, checked by hand against the two real schools involved so
  the fix doesn't newly confuse them with each other. (2) Schools entered in
  more than one boat class showed up as "ambiguous" in _every_ race, not just
  their own, because an entry's boat class was never reliably inline —
  confirmed by the author (schools do have boats across multiple events, and
  some have two boats in the _same_ event too). Fixed by scoping the candidate
  pool to the RC event whose label matches a race's boat class
  (`entriesForRace`, using each entry's filename-derived `EventID` and a new
  `events.json`-built label index, `asEvent`) rather than guessing an inline
  field, falling back to the old full-pool behavior when no event resolves.
  Two boats from the _same_ school in the _same_ event are expected to remain
  "ambiguous" - RegattaCentral's data may not distinguish them at all, and
  that is the correct, human-reviews-it answer, not a bug.
- **Fourth real run: event-scoping alone wasn't enough - two more false-match
  causes found and fixed.** Still-ambiguous lanes included a case with 8
  candidates for one school, most of them a _different_ real school
  ("Osbourn Park") that shares no real similarity with the xlsm's "Bishop
  Ireton". Two causes, both structural, neither needing new field-name
  guesses:
  1. `matchOrgName`'s substring comparison had no minimum length - "Bishop
     Ireton" normalizes to contain "op" (the tail of "bishop"), which
     coincidentally matched Osbourn Park's short abbreviation "OP". Fixed
     with a minimum length below which only an exact match counts
     (`minSubstringMatchLen`, `cmd/rcreconcile/match.go`).
  2. `asOrg`/`asEvent` were walking every file in `--rc-dir`, not just the
     dedicated organizations/events listings. Since RC ids very likely
     restart at 1 per entity kind, an unrelated id-plus-name object elsewhere
     (e.g. in `bulk.json`) could collide with a real organization's id and
     silently shadow it. Fixed by extracting orgs only from a file whose name
     contains "organization" and events only from a file named like
     `events.json` (`isOrganizationsFile` / `isEventsFile`,
     `cmd/rcreconcile/rcmodel.go`).
- **Fifth real run: the Bishop Ireton fix worked, but now _every_ school
  showed the same pattern** - one xlsm school resolving to 4-6 duplicate
  copies of its own correct name (and the "unused" list ballooning to
  essentially every entry in the regatta, since nothing was ever confidently
  "matched"). Root cause: event-_label_ matching (Milestone 1.7's original
  `entriesForRace`) never actually resolved anything, because this regatta's
  xlsm records boat class as a short code ("M-2-8+", "W-Jr-4x") that shares no
  text with RegattaCentral's fuller event names - so every race silently fell
  back to the full, unscoped pool, and any school with more than one boat
  anywhere in the whole regatta appeared as an "ambiguous" duplicate in every
  race it raced in. Fixed with a different signal that does not depend on the
  two sides' text agreeing at all: **roster overlap** (`bestMatchingEvent`,
  `cmd/rcreconcile/match.go`) picks the RC event whose entries best match the
  _set of schools actually racing in this specific race_, requiring a clear
  lead over every other event before scoping to it. Event-label matching is
  kept as a second-opinion fallback behind it, in case a future regatta's
  labels do line up with the xlsm's codes.
- The real `/bulk` / `entries` / `organizations` / `events` schema — confirm
  with `rcreconcile shape` against the author's local capture; correct
  `asEntry` / `asOrg` / `asEvent` in `cmd/rcreconcile/rcmodel.go` accordingly.
  This is also what Milestone 4 of the swimlane promotes into
  `internal/regattacentral`'s read model, once confirmed.
- Whether the abbreviation-expansion heuristic in `cmd/rcreconcile/match.go`
  ("HS"/"MS"/"JV"/"RC"/"BC"/"St.") needs to grow (or shrink) once tested
  against more real organization names.
- Whether a live `--regatta` pull is worth adding to `rcreconcile`, or the
  offline `--rc-dir` workflow is sufficient.
