# Results Publisher (REP)

**Resolution: not a new persona.** REP started as a "new persona" request
(Executive team, Media UI, results-driven) but resolved, after clarifying
the architecture options with the author, to a **native feature added to
the existing Primary Finish Timer (PFT) persona** — a "Publish" button
alongside PFT's existing Referee Approval flow, not a new `persona.Role`,
not a sidecar capability, not a standalone login. This doc records that
resolution and the design that follows from it; there is no new entry to
add to `docs/features/personas/README.md`'s persona table when this is
built — instead, PFT's existing section there gains a line for it.

Kept under `personas/new/` (rather than deleted or moved) for traceability
of the request and the decision trail; referenced from
`docs/features/TODO.md` per `AGENTS.md`'s New personas workflow, even
though the "once implemented, fold into README.md" step now means "extend
PFT's existing entry," not "add a new row."

## Background

Originally assessed as a class of "result-driven publishing personas" in
[future-result-driven-persona.md](../future-result-driven-persona.md)
(recommendation there: defer, keep `finish.json` the single source of
truth). Later referred to as "Register Results" in
[sidecar-personas.md](../sidecar-personas.md) and
[docs/features/TODO.md](../../TODO.md), scoped there as an **automated
push to RegattaCentral only** (a sidecar capability, increment 1). REP as
now described is broader than that narrower "Register Results" sketch — it
covers a spreadsheet destination as well as RC, and resolves to a PFT
feature rather than a sidecar. `sidecar-personas.md` and
`future-result-driven-persona.md` are updated (see their own files) to
point here rather than duplicate this design.

## Decisions made (author, 2026-09-16)

- **Shape:** native PFT feature, not a persona, not a sidecar. Considered
  and rejected: a sidecar capability attachable to any Lead (would have
  formalized `sidecar-personas.md`'s existing `register-results` sketch);
  a Standalone persona with its own login (would have formalized that same
  doc's sketched-but-unbuilt "Publisher" persona). Both remain on the
  table for `social-post`/SOM, which this decision does not resolve.
- **Delivery sequencing:** ship the spreadsheet destination first, add
  RegattaCentral later as a second destination once
  [heatsheet-rc-pivot-investigation.md](../heatsheet-rc-pivot-investigation.md)
  concludes. The feature is not blocked on that investigation; only its RC
  destination option is.
- **Destination configuration:** lives in the new cross-cutting
  [operational-state.md](../operational-state.md) doc, not duplicated
  here — the RD picks the results destination once, at regatta creation,
  and PFT's publish button reads that choice.

## Does / Does not / Entry / Constraint

- **Does:** Add a per-race **Publish** button to the PFT's existing race
  view (the same tree PFT already sees, not a new RD-style read-only tree —
  since this is now a PFT feature, not a separate read-only persona). The
  button is enabled only when `store.CanPublish(res)`
  (`internal/persona/store/state.go`, race-state-machine.md) is true, the
  same named helper the RD/Awards proposal already gate on. Pressing it
  renders that race's result into the
  configured destination: initially, a new standalone results spreadsheet
  (same format as the current manual "results worksheet," but a dedicated
  file RegattaClock owns and writes, decoupled from the RD's source
  workbook); later, RegattaCentral, once available.
- **Does not:** Publish an unapproved race. Write to the RD's own source
  workbook (the current manual process's destination) — the whole point is
  to stop that copy/paste into a file the RD is also actively using.
  Introduce a new persona, login, or challenge code.
- **Entry:** No new entry point — it's part of the PFT session the
  operator is already in.
- **Constraint:** Only the Primary Finish Timer publishes (not the
  Secondary) — matches the existing pattern where only the primary FT's
  `Approved` result is the official one (`reconciliation.md`); the
  secondary FT's terminal action never sets `Approved: true`.

## Existing-code reuse analysis

- **Approval gating** — `store.CanPublish(res)`
  (`internal/persona/store/state.go`, race-state-machine.md), already what
  `raceProgressStatus` and the Awards proposal both call. Same named check
  here: no new state to invent.
- **PFT's own race view already exists** — this is an addition to
  `internal/clock`/`internal/regatta`'s existing PFT flow, not a new tree
  to build (unlike Awards/Developer, which had to build a read-only tree
  from scratch). The "publish" button most naturally sits near the
  existing Referee Approval control, since it only makes sense once that
  race is approved.
- **Spreadsheet writing** — no existing code writes `.xlsx` today;
  `internal/reader` only reads Excel workbooks
  (`internal/reader/excel.go`). A results-spreadsheet writer is genuinely
  new work — check whether the Excel library already in `go.mod` for
  reading also supports writing before assuming a new dependency is
  needed.
- **RegattaCentral destination (later)** — reuses whatever
  `internal/regattacentral` client shape the
  `regattacentral-heatsheet-investigation` branch lands on
  (`Client.Upload`/`CreateRaces`/`AssignLanes`, or whatever it resolves
  to) — not designed further here, since that investigation is still
  open.
- **Not reused, by decision**: `sidecar-personas.md`'s `openSidecar`/
  capability-attachment machinery, and its `{raceNumber: revision}`
  re-publish tracking pattern. The revision-tracking *idea* (detect an
  approved result edited after publish, prompt re-publish) is still
  relevant here even though the sidecar machinery isn't — worth carrying
  forward as a requirement, implemented directly in PFT rather than via
  the sidecar seam.

## High-level implementation plan

1. **Land [operational-state.md](../operational-state.md)'s
   `PublishConfig`** (or at minimum its `ResultsDestination` field) in
   `regattaSchedule.json` — this feature has nothing to read otherwise.
   Can default to `"spreadsheet"` with no RD prompt yet if that survey UI
   isn't ready first; the field existing is what unblocks this feature.
2. **Spreadsheet writer**: build the standalone-results-spreadsheet
   output — same visual/column format as the current results worksheet.
   **The destination location is a deliberate open question, not
   `regattaData`-adjacent by default** — today's real publish location is
   a *separate* OneDrive/SharePoint folder, distinct from the shared
   `regattaData` tree, that a public-facing website reads from. That's
   context for the eventual answer, not a decision made here: how
   RegattaClock reaches that location (a configured path, a picked folder,
   something SharePoint-API-specific, or a different mechanism entirely)
   is an **implementation-time discovery**, deliberately left open per the
   author. `operational-state.md`'s `PublishConfig` already reserves a
   generic "destination-specific parameters" slot for whatever that turns
   out to be — nothing here should be taken as pre-deciding it.
3. **Publish button + gating**: add to PFT's existing race view, enabled
   on `RaceResult.Approved`, calls the spreadsheet writer for that race.
4. **Re-publish detection**: track a per-race revision/hash so an
   approved-then-edited result is flagged for re-publish rather than
   silently going stale (carrying forward the idea from
   `sidecar-personas.md`'s deferred revision-tracking design, applied
   directly rather than via a sidecar).
5. **RegattaCentral destination** (later, once the investigation
   concludes): a second branch in the same publish path, reusing whatever
   client the investigation lands on.
6. **Docs** (once built): extend PFT's section in
   `docs/features/personas/README.md` with the Publish button; update
   `docs/features/TODO.md`; delete this file (per `AGENTS.md`'s New
   personas workflow — even though the outcome isn't a new persona, the
   same "proposal doc → fold into real docs → delete" lifecycle applies).

## Dependencies and sequencing

- **Hard dependency**: [operational-state.md](../operational-state.md)'s
  `ResultsDestination` field, at least in skeleton form (step 1 above).
  Not yet built, but not gated on anything external — see that doc's own
  sequencing note.
- **Not a blocker, but a later branch**: RegattaCentral as a destination
  waits on `heatsheet-rc-pivot-investigation.md`. The spreadsheet
  destination does not.
- **No blocker** from the now-moot sidecar-vs-standalone question — that
  design surface in `sidecar-personas.md` remains open for `social-post`/
  SOM, not for this feature.

**No architectural blockers for the spreadsheet-destination slice.**
Sequencing: `operational-state.md`'s config field first, then this
feature. As with the other proposals in this directory, the live process
constraint as of this writing is `develop`'s feature-freeze (see
`AGENTS.md`) — implementation waits for that to lift.
