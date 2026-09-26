# Regatta operational state: schedule ingest + results publish + social platforms

Cross-cutting configuration, not a persona. Captured once by the Regatta
Director when `regattaData` is first created, and read by any feature or
persona whose behavior depends on where the schedule comes from or where
official output goes — today that's the Primary Finish Timer's results-
publish feature (see
[new/results-publisher.md](new/results-publisher.md)); later, a Social
Media persona (SOM, still deferred — see `docs/features/TODO.md`).

## Why this needs to exist

Two independent axes are currently either half-modeled or not modeled at
all:

1. **Schedule ingest source** — where `regattaSchedule.json` comes from.
   Already half-modeled: `reader.RegattaData.SourceInfo{Type, URI, Hash}`
   (`internal/reader/regattaData.go`) records this today, but only ever as
   `Type: "excel"` (set in `internal/reader/excel.go`). Phase C of
   [regattacentral-integration.md](regattacentral-integration.md#phase-c--in-app-heat-sheet-authoring)
   already anticipates a second origin type — a generalized
   `ScheduleOrigin` interface with `store.Origin{Type: "heatsheet"}` for an
   RC-based ingest — but that phase is explicitly gated behind
   [heatsheet-rc-pivot-investigation.md](heatsheet-rc-pivot-investigation.md)
   concluding, per that doc's own text: "Before committing to Phase C/D: an
   investigation is underway…" **Update (2026-09-16):** the persona
   behind this ingest source is now scoped —
   [new/heat-sheet-creator.md](new/heat-sheet-creator.md) (Heat Sheet
   Creator, HSC) — staged so its read-only v1 (RC pull, local working
   copy, RC-id fields added directly to `store.ScheduleRace`/
   `store.ScheduleEntry`) does not depend on the investigation; only
   HSC's own write-back v2, and the actual `heatsheet` origin wiring
   this axis describes, remain gated on it concluding. **Update
   (2026-09-20):** the RD's Excel ingest now reads the workbook's `Heat
   Sheet` worksheet instead of the old results-shaped `Results` worksheet,
   for reasons unrelated to RegattaCentral — see
   [schedule-data-model.md](closed/schedule-data-model.md#ingest-source-results-tab-vs-heat-sheet-tab).
   Deliberately **not** a fourth `SourceInfo.Type`: it's the same Excel
   origin, just a different sheet/row-layout choice inside it, so it
   doesn't add a new value to this axis, only a future internal branch
   within the existing `"excel"` one.
2. **Results publish destination** — where official, approved results go
   once a race is signed off. **Not modeled anywhere today.** The current
   real-world process is manual: copy/paste from a pre-formatted results
   worksheet into the same Excel workbook the RD loaded to build
   `regattaSchedule.json` in the first place. Two candidate automations:
   a new, standalone spreadsheet (same format, RegattaClock-authored,
   decoupled from the RD's source workbook), or RegattaCentral — also
   gated behind the same investigation.

A third, related axis the RD should also settle up front: **which social
platforms (if any) this regatta will publish to** (X, Instagram, Facebook,
…), for the still-deferred SOM persona to read later rather than needing
its own separate configuration step.

Today, nothing records the second or third axis at all, and the first axis
is implicit (always "excel") rather than an explicit choice the RD makes.
This doc proposes making all three an explicit, regatta-level choice,
recorded once, alongside the schedule data it already governs.

## Where this belongs — not `persona-config-file.md`

[persona-config-file.md](persona-config-file.md) is an **optional, external,
per-deployment** file (host→persona pinning, challenge-code overrides) — it
does not travel with `regattaData`, is not required, and is set up once per
*organization*, not per *regatta*. Operational state is the opposite shape:
**required-ish** (the RD is asked for it up front), **regatta-scoped** (it
should move with `regattaData` if the folder is copied or archived), and
read by in-app features rather than by startup/picker logic. It belongs
alongside `Name`/`Date`/`SourceInfo` in `director/regattaSchedule.json`
(see [schedule-data-model.md](closed/schedule-data-model.md)), not in a separate
file.

## Proposed shape (sketch, not final)

```go
// Alongside RegattaData.Name / .Date / .SourceInfo, in the same
// director/regattaSchedule.json the RD already owns.
type PublishConfig struct {
    ResultsDestination string // "spreadsheet" | "regattacentral"
    // Destination-specific parameters - e.g. an RC regatta id - shaped
    // once the RC investigation's outcome is further along. Never a
    // filesystem path: see the note below.
    SocialPlatforms []string // e.g. ["x", "instagram"]; empty/omitted = none enabled
}
```

**Update (2026-09-26): `ResultsDestination` has landed** as
`store.PublishConfig` on `store.Schedule` (`omitzero`, excluded from
`Schedule.ContentHash()`), defaulting to `"spreadsheet"` when unset; there
is no RD prompt for it yet. **The spreadsheet's output folder is
deliberately *not* a `PublishConfig` parameter.** The published drive is
separate from `regattaData` and mounts at a different path on every
machine, so a path recorded once by the RD would be wrong on the Primary
Finish Timer's host. The PFT confirms it and records it in its own
`finish.json` (regatta-scoped, so a new regatta never inherits the last
one's folder). `PublishConfig` holds regatta-wide choices
only (which *kind* of destination), never machine-local locations. See
[new/results-publisher.md](new/results-publisher.md).

`SourceInfo.Type` already carries the ingest-source axis; this doc does not
propose changing it, only formalizing that once Phase C's `ScheduleOrigin`
generalization lands, the RD's ingest choice and this `PublishConfig`
choice are asked **at the same moment** (regatta creation) as one coherent
"how is this regatta operating" prompt, not two unrelated settings screens.

## Existing-code reuse analysis

- `reader.RegattaData.SourceInfo` is the direct precedent for
  "regatta-level origin metadata persisted in `regattaSchedule.json`" —
  `PublishConfig` is structurally the same idea, for the output side
  instead of the input side.
- `schedule-data-model.md`'s existing guidance on what belongs in
  `regattaSchedule.json` (regatta metadata + schedule, never result data)
  already reasons about exactly this kind of "small, RD-owned, rarely-
  changing metadata" — `PublishConfig` fits that same bucket, not
  `finish.json`/`start.json`.
- Phase C's `ScheduleOrigin` interface (persona-plan.md §3b, referenced by
  regattacentral-integration.md) is the reuse target for the ingest axis —
  this doc does not duplicate that design, it just notes where
  `PublishConfig` should sit next to it once built.

## Dependencies and sequencing

- The **spreadsheet** results-destination and the **excel** ingest source
  (today's only real option) need no new dependency — `PublishConfig`
  could be added to `regattaSchedule.json` now, defaulted to
  `ResultsDestination: "spreadsheet"`, independent of the RC investigation.
- The **regattacentral** results-destination and the **heatsheet** ingest
  source both remain gated behind
  [heatsheet-rc-pivot-investigation.md](heatsheet-rc-pivot-investigation.md)
  concluding — unchanged from Phase C's existing gate, not a new blocker
  this doc introduces.
- This doc is a **dependency of**
  [new/results-publisher.md](new/results-publisher.md) (needs
  `ResultsDestination` to know where to publish) and of the still-deferred
  SOM persona (needs `SocialPlatforms`) — not the other way around. The
  `ResultsDestination` skeleton landed with the PFT results-publish
  feature (#127). Still open: the RD's regatta-creation prompt for it, the
  `SocialPlatforms` field, and the ingest-source choice.
- Same live process constraint as every other doc in this directory right
  now: `develop` is in feature-freeze (see `AGENTS.md`) — this is a design
  doc, unaffected; implementation waits for the freeze to lift.
