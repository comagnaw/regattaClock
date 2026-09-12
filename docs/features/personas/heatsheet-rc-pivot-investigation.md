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
4. **`rcreconcile reconcile --upload-preview-out`** (Milestone 2) renders a
   local, human-readable preview of what a heat-sheet-and-results "publish"
   would look like, built from the xlsm and `reconcile`'s matches, using the
   already-typed, already-tested `regattacentral.UploadRequest`. `Client.Upload`
   is never called - see [`cmd/rcreconcile`'s README](../../../cmd/rcreconcile/README.md)
   for exactly what it does and does not resolve confidently.

## Findings

- **The majority of boats match RegattaCentral automatically** once the
  fixes through the ninth real run (below) were applied: the org-id join,
  "St."/"Saint" and short-abbreviation handling, file-scoped org/event
  extraction, roster-overlap event resolution, the `EventID` merge-priority
  fix, and the Heat Sheet tab's rower-name and boat-class disambiguation for
  RD-combined lanes.
- **What's left unresolved falls into two kinds, and only one of them is a
  real gap:**
  1. Two boats from the same school in the same event, where RegattaCentral's
     own data may not distinguish them at all - correctly flagged
     "ambiguous, needs a quick check" rather than guessed, per this tool's
     whole design philosophy (see the fifth real run). Not a gap to close;
     this is the intended, honest outcome.
  2. A small-boat race with few schools, each already racing in several
     other events, where roster overlap can tie and neither the rower name
     (absent for boats bigger than a double) nor the boat-class text (no
     override present, since the boat wasn't actually combined in) can
     rescue it (the ninth real run). This is a genuine algorithmic
     limitation - fixing it means reworking roster overlap's tie-breaking
     for thin-roster races, not something the Heat Sheet tab can supply.
  Note, from the ninth real run: the boat-class signal is a data-quality
  dependency, not just a code path - it only works when the RD filled in
  the Heat Sheet's per-lane class completely (including the gender prefix),
  confirmed by the author correcting a real workbook and re-running.
- **`--debug-race N`** (added during the eighth/ninth real runs) turned out
  to be essential for diagnosing exactly which of these two categories a
  remaining ambiguous lane falls into, without ever needing to share real
  data - every future round of tuning this tool should reach for it first
  rather than guessing again.
- Exact match-rate (N of M boats), whether RC already had any
  `Event`/`Race`/`Lane` data for this regatta, and RD/executive feedback
  beyond the author's own: still to be recorded here if gathered.
- **Carrying this forward:** this swimlane's commits live on
  `regattacentral-heatsheet-investigation`, not `develop` - how (or
  whether) any of it lands there is undecided. The author's own framing:
  possibly cherry-picking or otherwise carrying forward specific documented
  findings (this file, and the reasoning already captured in
  `cmd/rcreconcile`'s commit history) rather than merging the branch
  wholesale, once a Phase C/D go/no-go is actually decided. Nothing about
  that mechanism is settled yet.
- Go / no-go recommendation for a real Phase C/D: leaning toward **go, with
  the tie-breaking limitation above named as a known, bounded gap** - the
  majority-automatic result plus a design that never guesses past a genuine
  ambiguity (real matches are trustworthy exactly because the tool would
  rather ask a human than be confidently wrong) suggests the pivot is
  viable, pending real RD/executive feedback and the schema-confirmation
  work below.

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
- **Sixth real run: roster overlap changed nothing - identical duplicate
  counts, before and after.** The exact same schools showed the exact same
  number of duplicate self-matches as before the roster-overlap fix, which
  ruled out a per-race coincidence (a real per-race tie would vary race to
  race, not reproduce an identical count everywhere). Root cause:
  `entriesFromDir`'s entry merge (`cmd/rcreconcile/rcmodel.go`) dedupes by
  entry ID with "first occurrence wins, sorted-filename order" -
  `bulk.json` sorts ahead of every `entries-<id>.json` file. Once the org-id
  relaxation (Milestone 1.6) let `asEntry` match an entry from its org-id
  reference alone, `bulk.json` - which nests the same entries every
  `entries-<id>.json` file does - started winning the merge for essentially
  every real entry. But `bulk.json` has no filename to derive an `EventID`
  from, so its (winning) copy always carried a **blank** `EventID`, and the
  `entries-<id>.json` copy that would have had the correct one was discarded
  as a duplicate. `bestMatchingEvent` had nothing to count for any race, so
  it silently returned `""` every time, falling all the way through to the
  same unscoped pool as before event-scoping existed - explaining why the fix
  had zero observable effect. Fixed by exempting `EventID` from "first
  occurrence wins": a later duplicate's non-blank `EventID` now backfills an
  earlier, blank one, while every other field (org name, etc.) keeps the
  existing, tested first-occurrence-wins behavior.
- **Seventh real run: confirmed on the real regatta - the majority of boats
  now match RegattaCentral automatically.** The only remaining "needs a quick
  check" cases: two boats from the same school in the same event (expected -
  see the fifth real run), and boat classes the RD combined into a race's
  open lanes for lack of entries (e.g. the regatta's only Junior Men's 1x
  raced as an extra lane in a Men's 2x).
- **Eighth real run: the rower-name signal alone parsed correctly (8 names
  found) but didn't move the ambiguous-lane count - row 2's boat class
  turned out to be needed too, in progress.** A combined lane's real
  RegattaCentral entry belongs to an event `bestMatchingEvent` correctly
  excludes from the rest of that race, so it's structurally unmatchable from
  the Results tab alone. The xlsm's separate "Heat Sheet" tab (never read by
  `internal/reader` — a different, 3-row block layout) carries two signals:
  row 3's rower last name (1x/2x boats only) and row 2's per-lane boat
  class, set exactly when the RD combined a different class into a race's
  open lanes. The rower name alone wasn't enough on the real regatta - a
  bigger boat has no rower name at all, and a school can have several other
  entries that don't help distinguish by name either.
  `cmd/rcreconcile/heatsheet.go` now captures both signals;
  `disambiguateByBoatClass` narrows a widened lane's candidates by matching
  row 2's text (exact normalized match only - the same false-positive risk
  as any short code) against a candidate's own `BoatClass` field or its
  resolved event's label. Separately, `asEntry`'s `BoatClass` guess was
  widened to include `division` and `alternateTitle` - confirmed-real field
  names on a live Entry object, traced structurally from a real `entryId`'s
  own keys (`entryLabel`, also confirmed-real, reads more like a boat's
  display label than its class, so it moved to the `Label` guess instead).
  A new `--debug-race N` flag prints a step-by-step trace of one race's
  matching (school/org names, entry ids, candidate counts - never an
  athlete's name), added specifically because the rower-name fix's "8 names
  found, 10 lanes still ambiguous, no change" result gave no way to tell
  which stage was failing without it. **Confirmed on the real regatta**: the
  traced lane (a Junior Men's 1x combined into a Men's 2x race) resolved
  from 3 candidates to 1 via the boat-class match. Separately: an RD can
  hand-type "SCR"/"SCRATCHED" into the Results tab's Place column for a
  scratched boat (not something `internal/clock`'s live timing ever writes)
  — `preview.go`'s `laneStatusForPlace` (Milestone 2) now recognizes it too.
- **Ninth real run: `--debug-race` on two more combined races found a real
  data-entry gap, and a separate, deeper limitation that's out of scope for
  now.** Tracing race 7 (`--debug-race 7`) showed one lane's Heat Sheet
  class text as `"Jr-4x"` where the race's own nominal class was `"W-4x"` -
  the RD had dropped the gender prefix, presumably since it's implied by
  the race itself, so it never exact-matched anything. The author corrected
  the source xlsm (added the missing "W") rather than have the tool guess
  at implied prefixes, and re-running confirmed it then resolved
  correctly - a data-quality dependency on the Heat Sheet being filled in
  completely, not a code fix. A second, different lane in the same race
  (two different schools, `"W-4x"` in the Heat Sheet matching the race's own
  nominal class - not actually combined at all) stayed ambiguous for a
  deeper reason: `bestMatchingEvent`'s roster overlap hit a 4-way tie -
  with only 3 schools in a small-boat race, each already spread across
  several other events, no one event's overlap count came out ahead, so
  the pool fell back to nearly the whole regatta. Rower name didn't help
  either (a 4-person boat gets no individual name per the established
  convention), leaving no signal at all to pick the right entry. This is a
  genuine data/algorithm limitation, not a bug - fixing it would mean
  reworking roster overlap's tie-breaking for races with few schools, a
  separate and riskier change not pursued in this swimlane.
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
