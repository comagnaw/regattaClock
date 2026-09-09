# Reconciling primary and secondary finish results

How the two finish-timer teams' results are combined into one published result
set. Companion to [persona-plan.md](persona-plan.md) (§9 Director, §2.1 clock
skew, §3c lane-map hash) and [schedule-data-model.md](schedule-data-model.md).

## Purpose and scope

The primary and secondary Start/Finish pairs time the **same** regatta
independently, each writing its own `timing/<team>/finish.json`
([schedule-data-model.md](schedule-data-model.md), one-writer-per-file). Somebody
has to turn those two files into the single set of race results that gets
published.

**In scope:** the model for choosing, per race, which team's `RaceResult` is
authoritative, and how disagreements surface.

**Out of scope:** the export / publish mechanism itself. `internal/exporter`
today renders lane images from the schedule only; the results-publishing surface
(places, times, a disputed-race resolution screen) will land with a future
results/publish persona that does not exist yet. This document is that persona's
spec. [future-result-driven-persona.md](future-result-driven-persona.md) assesses
the *content* personas downstream of it (a social-media text table, the same as a
PNG) and whether Referee Approval should materialize a per-race `results/` file —
recommendation: defer, keep `finish.json` the only source of truth.

## Operating model

- **The secondary pair is a backup.** In practice it is brought online in case
  the primary pair misses races; most regattas it stays untouched. The historical
  workflow was to read the secondary's numbers off and hand-type them into the
  primary's spreadsheet.
- **Primary is canonical.** When both teams have a committed result for a race
  and they agree, the primary's is used.
- **Disagreements are surfaced, never auto-merged.** The app never blends two
  results or silently prefers one over the other when they conflict — a human
  decides.
- **Common case is trivial.** Secondary empty ⇒ every race is `primary` or a
  `gap`; there is nothing to reconcile.
- **The secondary FT never approves.** Its race clock has no Referee Approval
  step — the approval panel is a single **Save and Close** button, and it writes
  `RaceResult.Approved = false` then closes the window. So `Approved == false` in
  `timing/secondary/finish.json` is *definitive*: the results are complete, they
  are simply not a refereed outcome. The only path to `approved` is the **primary
  FT** re-entering the secondary's numbers and presenting them.
- **The primary FT has no un-refereed Save.** Its clock commits a race **only**
  through Referee Approval (`Approved = true`, `ApprovedAt` stamped); its other
  control is **Close**, which is disabled until the race is approved and persists
  nothing. So a `RaceResult` in `timing/primary/finish.json` that carries a
  `WinningTime` and `Rows` always has `Approved = true` — an unapproved primary
  entry is only ever the automatic in-progress record (Start clicked,
  `FirstFinishAt` set, `WinningTime` empty).

## Per-race timing state

A race moves through the same milestone ladder for each team. Reconciliation
reasons in these states, not in raw field values.

| State | Reached when | `RaceResult` evidence |
|-------|--------------|-----------------------|
| `none` | nothing recorded | no entry in `finish.json` for `RaceNumber` |
| `start-recorded` | ST captured a start time | `start.json` `StartRecord.StartedAt` (not a finish state, but the RD tree shows it) |
| `in-progress` | FT clicked **Start** on the clock | `FirstFinishAt` set, `WinningTime` empty, `Approved` false |
| `saved` | **Secondary FT** pressed **Save and Close** (the primary FT has no Save action — it commits only through Referee Approval) | `WinningTime` non-empty, `Rows` populated, `Approved` false |
| `approved` | Referee Approval (**primary FT only** — the secondary FT has no approval step) | `Approved` true, `ApprovedAt` set |

This mirrors the RD progress tree's existing status vocabulary — *timing in
progress* / *saved* / *approved* — in
[`internal/regatta/director_tree.go`](../../../internal/regatta/director_tree.go)
`directorFinishCells`. With the split above, a `saved` row can only ever be a
**secondary**-team row.

## The reconciliation verdict

Per race, from `(primary state, secondary state)`. The secondary column tops out
at `saved` — it has no `approved` state; the primary column starts at `approved`
— with no Save action the primary FT never produces a complete-but-unapproved
result, so its only pre-`approved` states are `none` / `start-recorded` /
`in-progress`.

| Primary | Secondary | Verdict | Notes |
|---------|-----------|---------|-------|
| `approved` | `none` / `start-recorded` / `in-progress` | **primary** | normal; publish primary, tag "no secondary confirmation" |
| `approved` | `saved`, values match | **primary** | agreement |
| `approved` | `saved`, values differ | **disputed** | human resolves; nothing auto-published |
| `none` / `start-recorded` / `in-progress` | `saved` | **secondary** | primary pair missed this race; publish secondary, provenance `secondary` |
| `in-progress` | `in-progress` | `not-yet` | revisit — neither committed |
| `none` | `none` | `gap` | race not timed by anyone; already visible as an empty RD row |

"Values match" means: same winning time (see [clock skew](#clock-skew-between-the-two-ft-machines))
**and** the same order of finish and per-boat splits (`Rows` compared by lane).

## Where the authoritative set lives — read-time auto-selection

**No new file. The Regatta Director persists no reconciliation decision.**
One-writer-per-file stays intact; the only cross-file identity is `RegattaKey` +
`RaceNumber` ([schedule-data-model.md](schedule-data-model.md)).

The future results/publish consumer:

1. Reads `director/regattaSchedule.json` and both `timing/<team>/finish.json`.
2. Joins by `RegattaKey` + `RaceNumber` (a team's file whose `Envelope.RegattaKey`
   does not match the schedule is excluded, as it already is at RD hydration —
   `matchesRegatta` in `director_tree.go`).
3. Applies the [verdict table](#the-reconciliation-verdict) per race.
4. Tags each published race with **provenance**: `primary`, `secondary`, or
   `disputed:<reason>`. A `disputed` or `gap` race is not published until resolved.

**Gaps the primary missed are closed by the primary FT reopening the race and
entering the secondary's numbers.** That is the digital form of the old
transcription step, and it is the only path that keeps a team's results in one
file with one writer: the primary FT writes the primary `finish.json`. After that
the race is a normal `primary` verdict.

### Alternatives considered and rejected

- **An RD-owned `director/reconciliation.json`** recording explicit per-race
  decisions (use primary / use secondary / override values). It would give the RD
  a first-class "choose the authoritative set" action, but it is more machinery
  than a backup-only secondary warrants for v1, and it introduces a fourth file
  to keep consistent. Revisit only if a regatta routinely runs both pairs as
  co-equal.
- **§3b's "safe merge"** (auto-apply only where there is no timing data) — that
  is about applying a schedule change without clobbering timing; it does not
  apply to combining two result sets.

## Concern-cases

### Clock skew between the two FT machines

The winning time is `FT Start click − ST Start time`, captured on two laptops
whose clocks may differ by seconds ([persona-plan.md §2.1](persona-plan.md)). The
primary and secondary FT machines are a *third* pair of clocks. Two winning-time
strings from different teams are **not comparable at face value**.

Before an equality test, recompute a corrected winning time from the per-record
`ClockRef`s each result carries:
`FirstFinishClock.Corrected(FirstFinishAt) − StartedAtClock.Corrected(StartedAt)`.
Or compare with a tolerance no smaller than the measured offset delta between the
two machines. The RD skew banner (persona-plan.md §10 8b-2) already warns when
the four timing files' stamped offsets diverge by more than
`timesync.SkewWarnThreshold`.

### Different `LaneMapHash` between primary and secondary

If the two teams' `RaceResult.LaneMapHash` for the same race differ, they timed
against **different lane maps** — one applied a schedule change the other did not.
`Rows` key by lane number, so the order of finish cannot be compared or merged
mechanically. Verdict: **disputed**, needs a human. (8d flags each team's result
against the *live* schedule; this case is primary-vs-secondary.)

### OOF / split disagreement with a matching winning time

Equal winning time is not sufficient. Compare `Rows` by lane: `Place`, `Split`,
`Time`. Any difference ⇒ **disputed**.

### `RegattaKey` mismatch

A team's `finish.json` left over from a different event. Already excluded at RD
hydration and never entered into reconciliation.

### Staleness

A committed result whose `Envelope.WrittenAt` is hours old while racing continues
is probably abandoned. The RD staleness banner (8b-2) already warns; the consumer
should treat a stale `approved` primary against a fresh `saved` secondary as
*flag for RD review*, not automatic.

### Partial secondary

A backup pair brought online mid-regatta only has the later races. Expected, not
a conflict — those races are `secondary` verdicts, earlier races are `primary`.

### Neither team committed

`(in-progress, in-progress)` or `(start-recorded, none)` ⇒ `not-yet` / `gap`.
Nothing to publish; the RD tree already shows the state.

## What the Director sees today vs. later

**Shipped (RD oversight):**

- Per-value primary→secondary fallback in the progress tree with a `·2nd`
  marker and legend (8b-2).
- Clock-skew and staleness banners across the four timing files (8b-2).
- `†` mark on races whose committed result no longer matches the live lane map
  (8d).

**Deferred to the results/publish persona:**

- A per-race **verdict** column (`primary` / `secondary` / `disputed` / `gap`).
- A `disputed`-race resolution screen.
- The actual export / publish with places and times and the provenance tag.

## Data available for reconciliation

Per race, from each team's [`store.RaceResult`](../../../internal/persona/store/log.go):

| Field | Use |
|-------|-----|
| `WinningTime` | referee time (auto-filled from ST, editable) |
| `Rows []LapRow` | `Lane` (0 = unassigned), `Place`, `Split`, `Time` — the order of finish |
| `Approved` / `ApprovedAt` | referee approval — set by the primary FT only; always `false` in `timing/secondary/finish.json`, and in `timing/primary/finish.json` any `RaceResult` carrying `WinningTime` / `Rows` has `Approved = true` (no unapproved-save path) |
| `UpdatedAt` | last write time for this race |
| `LaneMapHash` | lane map the result was committed against (`ScheduleRace.LaneMapHash`) |
| `StartedAt` / `StartedAtClock` | ST start actually used + its offset |
| `FirstFinishAt` / `FirstFinishClock` | FT Start click + its offset |

From `FinishLog.Envelope`: `Machine` (which laptop), `WrittenAt`, `Sequence`
(monotonic per writer), `Clock` (writer's offset at write time), `RegattaKey`.

Join: `RegattaKey` + `RaceNumber` only.

## Open questions for the results persona

- **Winning-time comparison tolerance.** Recompute from `ClockRef`s (precise) or
  compare strings with a fixed tolerance? A policy is needed before "values
  match" is well-defined.
- **Does `disputed` block publish** entirely, or publish with a visible
  provenance annotation and let the downstream officials sort it out?
- **A lightweight RD acknowledgement** — "yes, race 14 is secondary-sourced, I
  know" — so the consumer can distinguish an unreviewed `secondary` from an
  accepted one, without a full `reconciliation.json`.
