# Personas

High-level requirements for multi-persona operation of Regatta Clock.

**Related docs in this directory**

- [persona-plan.md](persona-plan.md) — implementation plan
- [logging-options.md](logging-options.md) — JSON event logging design
- [shared-storage-options.md](shared-storage-options.md) — SMB / spare-PC vs cloud sync

## Goal

Several operators share one `regattaData` directory. Each operator acts as one **persona** on one **team**, with clear privileges over what they may read, write, and see in the UI. Personas are defined in source so new ones can be added later without redesigning the model.

## Teams

| Team | Code | Who |
|------|------|-----|
| Executive | `executive` | Regatta Director (and future non-timing officials) |
| Primary | `primary` | Primary Start Timer + Primary Finish Timer |
| Secondary | `secondary` | Secondary Start Timer + Secondary Finish Timer |

Primary and secondary are independent ST/FT pairings for the same regatta. Timing data is keyed by team.

## Personas

| Persona | ID | Team | Challenge (example) |
|---------|----|------|---------------------|
| Regatta Director | `rd` | Executive | `rc-rd` |
| Primary Start Timer | `pst` | Primary | `rc-pst` |
| Secondary Start Timer | `sst` | Secondary | `rc-sst` |
| Primary Finish Timer | `pft` | Primary | `rc-pft` |
| Secondary Finish Timer | `sft` | Secondary | `rc-sft` |

### Regatta Director (RD)

- **Does:** Load / refresh schedule from Excel into shared data; establish `regattaData`; view live progress (restarts, start time, winning time, approval); export (e.g. lane images); read all timing data.
- **Does not:** Time races; write start times or finish results.
- **Entry:** Separate director entry point (not the timer picker).

### Start Timer (ST)

- **Does:** Load race tree from RD schedule; record start time per race; clear (with confirm) and restore cleared times; write only that team’s start data.
- **Does not:** Open the race clock; see or use **Time Race**.
- **Sees:** Race list, own start times, Clear / Restore when applicable.

### Finish Timer (FT)

- **Does:** Load race tree from RD schedule; see ST start times (live updates); open **Time Race**; collect laps / OOF / winning time; save on Referee Approval or Save; reopen a race with prior results restored.
- **Does not:** Record or clear start times; see **Start Time**.
- **Sees:** Race list, ST start times, own progress (saved / approved), **Time Race**.

## Shared data constraints

- One shared `regattaData` root (LAN SMB preferred; cloud-synced folder supported — OneDrive or Google Drive, same app mode).
- **One writer per file** — no shared write targets across personas.
- Watch shared timing files and refresh UI when they change.
- On restart, hydrate each persona’s view from its already-saved data.
- Do not auto-restore the last session from preferences alone; choose persona (timers) and confirm the regatta directory each launch.

## Timer startup (high level)

1. Choose persona (primary/secondary × start/finish).
2. Pass that persona’s simple challenge code (or return to step 1).
3. Select `regattaData` and confirm title / date / schedule.
4. Show the role-specific race tree.

## Privilege summary

| Action | RD | ST | FT |
|--------|----|----|----|
| Write schedule (`director` data) | yes | no | no |
| Write start times (own team) | no | yes | no |
| Write finish results (own team) | no | no | yes |
| Read schedule | yes | yes | yes |
| Read start times | yes | own | own team |
| Read finish results | yes | no* | own |
| Start Time / Clear / Restore UI | no | yes | no |
| Time Race / clock UI | no | no | yes |
| Progress-only race tree | yes | — | — |

\*ST does not need finish results for its job; RD reads both teams for oversight.
