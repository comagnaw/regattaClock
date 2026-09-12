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

Four more things `reconcile` handles that came up on real regattas:

- **Spelling variations.** RegattaCentral writes "St." where an xlsm may write
  "Saint" (and vice versa) — `commonAbbrevExpansions` in `match.go` expands
  both directions before comparing, alongside the existing "HS"/"High School"
  style abbreviations. This is a short, named list, not general spell-checking
  — a variation outside it is left for the report's human reader, same as
  everything else this tool is unsure of.
- **Pools scoped by event, using roster overlap, not label text.**
  `entriesForRace` (`match.go`) scopes each race's candidate pool to one RC
  event, tried in order: (1) **roster overlap** (`bestMatchingEvent`) - which
  event's entries best match the *schools actually racing in this race*,
  using each entry's `EventID` (read straight from its capture's filename,
  e.g. `entries-42.json` → event `42` — no guessing needed there); (2) event
  *label* text (`matchingEventIDs`, matching an `events.json`-built label
  against the race's BoatClass/FlightInfo - see `asEvent`, PROVISIONAL like
  `asEntry`/`asOrg`); (3) the old plain boat-class filter. Roster overlap is
  what actually works: a real regatta's xlsm used short codes like "M-2-8+"
  that share no text with RegattaCentral's fuller event names, so label
  matching alone resolved nothing, and every school with more than one boat
  anywhere in the regatta showed up as "ambiguous" in *every* race it raced
  in - overlap sidesteps that by using something both sides already agree on
  (which schools race together) instead of needing the two sides' text to
  match. Every step only narrows further than the one before it produced
  nothing usable, so this can never regress to fewer matches than before
  event-scoping existed. Two boats from the *same* school in the *same* event
  still show up as "ambiguous, needs a quick check" — that is the correct
  answer when RegattaCentral's own data doesn't distinguish them, not a bug
  to chase.
- **`EventID` has to survive the entry merge even when `bulk.json` "wins."**
  `entriesFromDir`'s dedupe is first-occurrence-wins in sorted-filename order
  — `bulk.json` sorts ahead of every `entries-<id>.json` file, and once an
  entry can be recognized from an org-id reference alone (the org-id join
  above), `bulk.json` turned out to nest the same entries every
  `entries-<id>.json` file does. `bulk.json` has no filename to derive an
  `EventID` from, so its winning copy always carried a blank one — on a real
  regatta this silently zeroed `EventID` for nearly every entry, so
  `bestMatchingEvent` never had anything to count and roster overlap had no
  observable effect at all, identical to the pre-fix symptom. Fixed by
  exempting `EventID` alone from first-occurrence-wins: a later duplicate's
  non-blank `EventID` backfills an earlier blank one, while every other field
  (org name, etc.) keeps the original, tested behavior.
- **A lane combined into a different event's race, by the RD, for lack of
  entries.** Real example: a regatta's only Junior Men's 1x entry had no one
  else to race, so the RD sent it down the course as an extra lane in a
  Men's 2x race instead. That lane's real RegattaCentral entry belongs to a
  completely different event than `bestMatchingEvent` correctly resolves for
  the rest of the race - the race-scoped pool is *supposed* to exclude it.
  `matchLane` (`match.go`) handles this structurally, not by trying to read
  the xlsm's boat-class text (a dead end - see roster overlap above):
  whenever a lane's race-scoped candidates aren't already exactly one - zero
  (nothing matched) *or* more than one (a real regatta shape: a school raced
  three boats in one event, but only two of its RC entries actually belong
  there, so all three lanes' org-name match returned the same wrong pair) -
  it retries against every entry in the regatta. That widened pool is only
  ever used if the Heat Sheet's rower name or boat class (next) narrows it to
  exactly one; otherwise the original, already-bounded scoped result is kept
  rather than reporting a much larger, unnarrowed candidate list.
- **Disambiguating a widened lane by the rower's name and boat class, from
  the Heat Sheet tab.** The xlsm's "Heat Sheet" tab (a separate tab from
  "Results" - `internal/reader` never reads it; see its `findRaceSheet`
  comment) is a 3-row block per race, confirmed against a real RD-authored
  template (`Heat Sheet Input Examples.xlsx`'s Instructions tab): row 1 is
  the nominal boat class and per-lane school names; row 2 is a per-lane
  annotation that is an alternate boat class exactly when the RD combined a
  different class into this race (it can just as easily be "A"/"B",
  "SCRATCHED", or an advancement note - see `disambiguateByBoatClass` below
  for how that's told apart); row 3 is the stroke/rower's last name, listed
  only for 1x/2x boats. `readHeatSheet` (`heatsheet.go`) is a small,
  standalone parser scoped to `cmd/rcreconcile` on purpose - not
  `internal/reader` - since this investigation isn't meant to grow the
  shipped app's Excel-parsing surface for a one-off disambiguation signal; a
  workbook with no tab named exactly "Heat Sheet" (so "Referee Heat Sheet" is
  never mistaken for it) simply yields no data, and reconcile still works
  without it. Two independent narrowing steps run on a widened lane's
  candidates:
  - `disambiguateByRowerLastName` narrows when the row-3 name appears as a
    whole token in any of a candidate's `ParticipantNames` (`rcEntry`, from a
    confirmed-real `entryParticipants` array) - PROVISIONAL like every other
    name-shape guess in this tool, since the real API's name format
    ("First Last" vs "Last, First") is unconfirmed.
  - `disambiguateByBoatClass` narrows when the row-2 text exactly matches
    (normalized, not substring - the same false-positive risk a short code
    runs as any other) a candidate's own `BoatClass` field or its resolved
    event's label. A real example needed one more step first: an RD can
    write a local annotation into the same cell as the real class (e.g.
    "Exhibition M-1-4x") - RegattaCentral's own text never carries that
    annotation, so `normalizeBoatClass` strips a short, named list of known
    decorator words (`heatSheetClassDecorators`, same "named list, not
    general NLP" approach as `commonAbbrevExpansions`) before comparing, so
    "Exhibition M-1-4x" still exact-matches a plain "M-1-4x".
  - Anything that isn't really a name or a class landing in either cell
    (blank, an advancement note, "SCRATCHED", "A"/"B") simply won't match
    and is a harmless no-op in both.
- **`--debug-race N`.** Prints a step-by-step trace, to stderr, of how one
  race's lanes were resolved - the race-scoped pool size, then per lane:
  candidate counts before/after widening, before/after the rower-name check,
  before/after the boat-class check. Only school/organization names, entry
  ids, and event ids ever print - never an athlete's name - so a specific
  lane's non-match can be diagnosed against a real capture without sharing
  anything sensitive.
- **Short abbreviations don't substring-match.** A real mismatch: "Bishop
  Ireton" (xlsm) was showing "Osbourn Park" as a candidate, because
  normalize("Bishop Ireton") happens to contain "op" (the tail end of
  "bishop") and "OP" is a plausible abbreviation for "Osbourn Park" - purely
  coincidental. `matchOrgName` (`match.go`) only allows the substring side of
  a comparison when both normalized strings are at least
  `minSubstringMatchLen` (4) characters; a short name can still match, but
  only exactly. This trades a few legitimate short-abbreviation matches for
  far fewer coincidental false ones - again, erring toward "ask a human"
  rather than a confident wrong answer.
- **Organizations and events are read only from their own dedicated capture
  file.** `asOrg` / `asEvent` used to walk every file in `--rc-dir`, but
  RegattaCentral ids very likely restart at 1 per entity kind - an org, an
  event and an entry can all legitimately be id "1" - so an unrelated
  id-plus-name object elsewhere (in `bulk.json`, say) could coincidentally
  collide with a real organization's or event's id and silently shadow it.
  `entriesFromDir` now only extracts organizations from a file whose name
  contains "organization" and events from a file named like `events.json`
  (see `isOrganizationsFile` / `isEventsFile`), so that collision can't
  happen within one coherent, single-endpoint listing.

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

### `--upload-preview-out` — dry-run upload preview (Milestone 2)

```sh
go run ./cmd/rcreconcile reconcile \
  --xlsm internal/reader/testdata/example.xlsm \
  --rc-dir internal/regattacentral/testdata \
  --report-out out/reconciliation.html \
  --upload-preview-out out/upload-preview.txt
```

An optional flag on `reconcile` (not a separate command, since it reuses the
same matches): renders a local, human-readable preview of the
`regattacentral.UploadRequest` (`internal/regattacentral/model.go`) this
data would produce if it were ever PUT to `/regattas/{id}/upload` - **it never
is**; `Client.Upload` is not called anywhere in `cmd/rcreconcile`
(`grep -rn "\.Upload(" cmd/rcreconcile/` finds nothing). One output file holds
both a per-race/lane rendering (`Race 1 Lane 1 - RC entry #4821 - 6:12.5`) and
the raw JSON payload underneath it, so both a quick read and the exact wire
shape are in one place.

`buildUploadPreview` (`preview.go`) adapts each `laneMatch` from Milestone 1:

- **Matched** lanes get the real, already-confirmed RegattaCentral `EntryID`.
- **Ambiguous** or **unmatched** lanes get a locally-generated placeholder
  UUID instead (the same shape RegattaCentral documents for a brand-new
  entry) and are listed in a "needs a human's judgment" section - nothing is
  silently guessed into a real id.
- A result (`AddFinish`) is only added for a lane with a parseable finish time
  (`parseRaceTime`, expects the xlsm's own `"M:SS.s"` format) - a race that
  hasn't happened yet, or a bye lane, still gets a lane record but no result.
- `Place`'s `"DQ"`/`"DNF"`/`"DNS"` (what `internal/clock`'s live timing UI
  writes) and `"SCR"`/`"SCRATCHED"` (not part of that app's vocabulary - a
  scratch is known before the race even starts, at the Heat Sheet stage, not
  something the finish-line clock marks, but an RD can still hand-type it
  into the post-race Results tab for a boat that never rowed) map to the
  matching `LaneStatus`; anything else (a real finish place, or blank)
  reports as OK.
- `DisplayNumber` reuses `extractBoatLabel` ("A"/"B" from `AdditionalInfo`) -
  PROVISIONAL like everything else guessing at RegattaCentral's own field
  meanings, and known to occasionally false-positive when `AdditionalInfo`
  instead holds a small boat's rower name (see `reader.RaceEntry`'s doc
  comment) rather than a boat label.

Same PII rule as `--report-out`: this names real people once run against a
real capture, so keep it outside the repo or under a gitignored path.

## Not yet built

- A live `--regatta` pull, if the offline `--rc-dir` workflow turns out not to
  be enough.
