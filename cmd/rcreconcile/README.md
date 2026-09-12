# rcreconcile

An investigation tool: it balances what [RegattaCentral](https://www.regattacentral.com/)
has on file for a regatta against the `.xlsm` heat-sheet / results workbook the
RD actually produced, and writes a plain-language HTML report for a
non-technical reader (a regatta executive, not a developer). See
[the investigation doc](../../docs/features/personas/heatsheet-rc-pivot-investigation.md)
for the goal and ground rules this tool exists to serve.

**`shape` and `reconcile` never call RegattaCentral's write API - `publish-schedule`
and `publish-results` are the one deliberate exception**, built only after the
RD explicitly approved publishing one real, completed regatta's schedule and
results for real. See
[`publish-schedule` and `publish-results`](#publish-schedule-and-publish-results-live-write)
below before assuming this tool is read-only end to end.

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

### `shape` — learn a real schema, safely

`internal/regattacentral`'s read side used to be entirely raw JSON, since
nobody had confirmed the exact field names. `shape` walks a captured file and
prints its key-path structure — field names and JSON types **only, never
values** — so it is safe to paste back into a conversation or an issue:

```sh
go run ./cmd/rcreconcile shape --bulk-file internal/regattacentral/testdata/bulk.json
```

```text
data.events[].entries[].entryId: number
data.events[].entries[].organizationId: number
data.organizations[].name: string
```

This is how `Entry`/`Organization`/`Event`
(`internal/regattacentral/readmodel.go`) went from guessed to confirmed - see
[the investigation doc](../../docs/features/personas/heatsheet-rc-pivot-investigation.md)'s
Milestone 4 entry. `Race`/`Lane`/`Result` are still unconfirmed (every real
capture seen so far has an empty `races[]` on every event), so `shape`
remains the tool to reach for if that ever needs confirming, or if
`reconcile` reports a decode error on a file shaped differently than
expected (see Troubleshooting, below).

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

An entry does not carry its organization's name inline — confirmed real: an
`Entry` (`internal/regattacentral/readmodel.go`) only has an `OrganizationID`.
`entriesFromDir` in [`rcmodel.go`](rcmodel.go) resolves that id against every
`Organization` decoded from an organizations-shaped file across `--rc-dir`;
an id that never resolves still shows up (as "Unknown organization
(RegattaCentral id …)") rather than silently vanishing. `rcprobe walk`
fetches `organizations.json` automatically now, so a `--rc-dir` populated by
`walk` already has what this join needs.

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
  using each entry's `EventID` (the confirmed-real `eventId` field, decoded
  directly - see `internal/regattacentral/readmodel.go`); (2) event *label*
  text (`matchingEventIDs`, matching an `events.json`-built label against the
  race's BoatClass/FlightInfo); (3) the old plain boat-class filter. Roster
  overlap is
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
- **`EventID` used to need special handling across a merge; it no longer
  does.** `entriesFromDir`'s dedupe is first-occurrence-wins in
  sorted-filename order — `bulk.json` sorts ahead of every `entries-<id>.json`
  file, and nests the same entries every `entries-<id>.json` file does. Back
  when `EventID` was derived from the capture's *filename*, `bulk.json`'s
  copy (no filename to derive one from) always won the merge with a blank
  `EventID`, silently starving `bestMatchingEvent` for nearly every entry -
  roster overlap had no observable effect at all. Now that `EventID` decodes
  directly from each entry's own confirmed-real `eventId` field
  (`internal/regattacentral/readmodel.go`), every occurrence of the same real
  entry already agrees on the same value regardless of which file found it
  first, so plain first-occurrence-wins is correct for every field,
  `EventID` included - confirmed empirically against a real capture (zero
  blank `EventID`/`OrganizationID` across every entry found).
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
- **`--guess-ties`** (off by default). A real regatta can still have a lane
  where every signal above - event scoping, label, rower name, boat class,
  even widening past the race-scoped pool - is exhausted and more than one
  candidate remains: typically two boats from the same school in the same
  class, which RegattaCentral's own data genuinely doesn't distinguish. With
  this flag, `matchRaces` picks the first untaken candidate as a last resort
  (`statusGuessed`) instead of leaving the lane ambiguous - specifically so
  an otherwise-fully-resolved regatta can feed a real `EntryID` into the
  upload preview (Milestone 2) for every lane, rather than a placeholder
  UUID for the couple of ties that are genuinely unresolvable from data
  alone. Two safeguards keep this from silently hiding uncertainty or
  mis-assigning entries:
  - If two different lanes end up with the *same* tied candidate set (the
    exact scenario this exists for), the second lane skips whatever the
    first already picked and takes the next untaken one instead - tracked
    across the whole run, so no entry is ever guessed for two lanes. If
    there are more tied lanes than distinct candidates left to give them,
    the extra lane stays honestly `statusAmbiguous` rather than reusing an
    id - a duplicate assignment would be outright wrong, not just imprecise.
  - The report still labels a guessed lane "Best guess (RegattaCentral
    doesn't distinguish these)" - distinct from a confident "Matches
    RegattaCentral" - and shows which alternative it was picked over, so a
    human reviewer always knows to double check it. The upload preview
    likewise flags it in the "needs a human's judgment" section even though
    it carries a real `EntryID`.
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
- **Organizations, events and entries are each decoded into their own
  confirmed type, by file shape - never guessed from a generic object's
  fields.** `entriesFromDir` (`rcmodel.go`) dispatches each file by name:
  `bulk.json` decodes as `regattacentral.BulkResponse` (nested events, each
  with its own entries, plus the regatta's organizations); a dedicated
  `organizations.json` / `events.json` decodes as a flat
  `OrganizationsResponse` / `EventsResponse`; anything else is tried as an
  `EntriesResponse`, and simply skipped if it doesn't decode as one (e.g.
  `rcprobe`'s `token.json`, `active-races.json`). Since organizations only
  ever come from `Organization`-typed fields and events only from
  `Event`-typed fields, RegattaCentral ids restarting at 1 per entity kind (an
  org, an event and an entry can all legitimately be id "1") can no longer
  cause one kind to shadow another the way generic per-object shape-guessing
  once risked.

## Troubleshooting: "found 0 RegattaCentral entries"

Two independent, non-exclusive causes:

1. **The capture is missing per-event entries.** Run:

   ```sh
   go run ./cmd/rcprobe walk <regattaID> --out internal/regattacentral/testdata
   ```

   which pulls `/bulk`, `organizations.json`, and then follows bulk with a
   per-event entries call for every event id it finds — one command instead of
   hand-running `entries <eventID>` per event. Re-run `reconcile` against the
   same `--rc-dir` afterward.
2. **A capture file fails to decode as its expected shape.** `Entry` /
   `Organization` / `Event` (`internal/regattacentral/readmodel.go`) are
   confirmed against a real regatta, but RegattaCentral's schema could still
   differ for a different regatta, a different account tier, or a future API
   version - `entriesFromDir` returns that decode error directly (it does not
   silently swallow a `bulk.json` / `organizations.json` / `events.json` that
   fails to parse, only files it can't otherwise identify as one of the four
   known shapes). Run `shape` (below) against the file named in the error and
   compare its key-path output to `readmodel.go`'s struct tags - the (PII-free)
   output is exactly what's needed to widen or correct them.

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
  reports as OK - unless the Heat Sheet's row-2 text for that lane carries
  the "Exhibition" decorator (see boat-class disambiguation, above), in
  which case it reports `LaneExhibition` ("EXH") instead. `LaneStatus`
  (`internal/regattacentral/model.go`) is backed by the schema's
  `ResultStatusType` enum, confirmed directly against
  `api.regattacentral.com/v4/xsd_doc/resultstatustype.html` while adding
  `EXH`: `LaneDisqualified` was also corrected from an earlier PROVISIONAL
  `"DSQ"` guess to the schema's actual `"DQ"`.
- `DisplayNumber` reuses `extractBoatLabel` ("A"/"B" from `AdditionalInfo`) -
  PROVISIONAL like everything else guessing at RegattaCentral's own field
  meanings, and known to occasionally false-positive when `AdditionalInfo`
  instead holds a small boat's rower name (see `reader.RaceEntry`'s doc
  comment) rather than a boat label.

Same PII rule as `--report-out`: this names real people once run against a
real capture, so keep it outside the repo or under a gitignored path.

### `publish-schedule` and `publish-results` (live write)

```sh
go run ./cmd/rcreconcile publish-schedule --xlsm PATH --rc-dir DIR --regatta ID
go run ./cmd/rcreconcile publish-results  --xlsm PATH --rc-dir DIR --regatta ID
```

**These two commands are the one deliberate exception to "never calls the
write API."** They exist only because the RD explicitly approved publishing
one real, completed regatta's schedule and results to RegattaCentral for
real - see
[the investigation doc's Findings](../../docs/features/personas/heatsheet-rc-pivot-investigation.md#findings).
`shape` and `reconcile` remain permanently read-only.

Both commands reuse `reconcile`'s exact matching pipeline
(`entriesFromDir` / `readHeatSheet` / `matchRaces`, with `--guess-ties`
always on - the author's decision was to include best-guess picks in a real
push rather than only confident matches) so the write path can never
disagree with what `reconcile` already showed. Only a lane with a
confidently resolved, numeric RC `EntryID` (`statusMatched` or
`statusGuessed`) is ever included - unlike `--upload-preview-out`, a lane
with no resolvable entry is **excluded, not given a placeholder UUID**:
inventing a new RC registration on a live regatta was never asked for.
`classifyForPublish` (`publish.go`) does this split.

- **`publish-schedule`** sends one `LaneRecord` per included lane
  (`EntryID`, `DisplayNumber`, and `Status` via the existing `laneStatus` -
  `SCR`/`EXH` are lane-level facts already known from the historical xlsm)
  and a `StatusDraw` `RaceRecord` for every race that has one. No results.
  `Client.Upload(..., assumeLanesUploaded: false)`.
- **`publish-results`**, run only after `publish-schedule` and verifying the
  schedule on RegattaCentral's own site, sends one `ResultRecord` per
  included lane with a parseable finish time (`AddFinish`) and a
  `StatusOfficial` `RaceRecord` for every race that gets one. No lanes -
  those were already sent by `publish-schedule`.
  `Client.Upload(..., assumeLanesUploaded: true)`. **Use the exact same
  `--xlsm`/`--rc-dir` as the `publish-schedule` run** - results are keyed by
  race + lane, and depend on that same pair having already been sent.
- **`--confirm` gates everything.** Without it, both commands only print a
  dry-run summary (races/lanes counted, guessed lanes and their picked
  entry id, excluded lanes and why) and touch nothing - not even the
  network, since the summary is built and printed before any client is
  created. With `--confirm`, the summary is followed by an interactive
  prompt requiring the operator to type the regatta id back exactly before
  `Client.Upload` is ever called - two independent gates before a real
  write happens. Credentials/config work exactly like `cmd/rcprobe`
  (`--secrets-file` or `RC_*` env vars, optional `--config`, optional
  `--origin` - RegattaCentral documents this as required for a client id
  with a registered referer; reads have never needed it against this
  regatta, so it's PROVISIONAL whether a write does either, but the flag
  exists in case).
- **A real 404 from `Client.Upload` isn't necessarily a bug in this tool.**
  The URL/path construction is identical to every already-working GET call
  against the same regatta id; a 404 whose response body is RegattaCentral's
  own API envelope (`{"success":false,"messages":[...]}`, not a bare
  proxy/gateway 404 page) means the request reached RC's real application
  code and was deliberately rejected - most likely a permission/entitlement
  the account lacks for this specific regatta, or a business rule (e.g. a
  completed regatta's upload window). Confirm the `LaneStatus` values being
  sent match the Cookbook's own quoted table exactly first (see the
  `model.go` doc comment - a wrong enum literal in even one lane could be
  rejected as a whole-request failure), then check with RegattaCentral
  support about account/regatta-level write entitlement before assuming the
  request shape itself is wrong.

## Not yet built

- A live `--regatta` pull, if the offline `--rc-dir` workflow turns out not to
  be enough.
