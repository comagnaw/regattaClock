# Closed decision docs

Decision and investigation docs from [../](../) whose central question is
fully resolved and built. Each one carries its own `Status: closed` (or
equivalent "implemented" / "fully and permanently resolved") marker near the
top. Kept as the historical record of the design and the reasoning behind
each decision — not live planning guidance. If you're looking for what's
still open or in progress, see [../README.md](../README.md) instead.

- [schedule-data-model.md](schedule-data-model.md) — the schedule/start/finish
  ownership split (`director/regattaSchedule.json` vs `start.json` vs
  `finish.json`, "one attribute, one writer"), the RD's Heat Sheet ingest
  pivot, scratch handling (`Status`), the legacy `data.json` migration, and
  parse diagnostics. Closed 2026-09-20.
- [race-state-machine.md](race-state-machine.md) — the canonical
  race-lifecycle state machine, attribute ownership, the in-memory-vs-watcher
  rule, and the pane-of-glass race-tree redesign. Implemented 2026-09-19
  (PRs #99-#103).
- [reconciliation.md](reconciliation.md) — how the primary FT manually
  reconciles the two finish teams by reviewing the read-only **Compare
  Secondary** window and re-keying by hand. Fully and permanently resolved
  2026-09-18 — no programmatic reconciliation feature is planned. Superseded
  for state-transition modeling by [race-state-machine.md](race-state-machine.md).
- [logging-options.md](logging-options.md) — the `internal/applog` design:
  JSON event logging format, severity levels, per-persona/per-host file
  layout under `regattaData/logs/`, and the non-blocking async writer.
  Closed 2026-09-20. Size-based rotation and remote syslog export are
  settled as not pursued; log collection/visibility (a "collect logs"
  export action, once floated as a Director-side button) is now scoped to
  the **Developer (DEV)** persona proposal instead — see
  `docs/features/TODO.md` and [developer.md](../new/developer.md).
