# Deployment persona config file

An optional JSON file that lets an established organisation pre-assign machine roles
and set its own challenge codes, so operators do not have to pick a persona and type
a code every launch. It is best-effort: any problem loading it is non-fatal and the
app falls back to the normal [persona picker](persona-plan.md).

The file's name does not matter — only its contents and the preference that points at it.

## Setting the path

Configuration screen → **Persona Config:** row:

- **Change Persona Config** — pick a `.json` file. It is parsed and validated immediately;
  the path is saved to the `PersonaConfigFile` preference **only if it validates**, and a
  summary dialog reports how many host assignments and challenge overrides were loaded.
  A rejected file changes nothing.
- **Clear** — forget the file. Challenge overrides stop applying at once; a host assignment
  was only ever consulted at launch.

The path persists in user preferences, so a configured machine picks it up on every start.

## File format

```json
{
  "hosts": {
    "start-tent-pc": "pst",
    "finish-tower.regatta.local": "pft",
    "director-laptop": "rd"
  },
  "challenges": {
    "pst": "houston-start",
    "pft": "houston-finish",
    "rd":  "houston-director"
  }
}
```

Both sections are optional (a file may carry one, both, or neither).

### `hosts` — skip the picker on this machine

Maps a hostname to a persona ID (`pst`, `sst`, `pft`, `sft`, `rd`). When the running
machine's hostname matches, the persona picker and its challenge dialog are skipped
entirely:

- A **timer** lands on a small view naming the assigned role with a single
  **Select regatta folder** button — the shared regatta folder is the one thing still
  required. From there the normal folder → confirm → date-check → session flow runs.
- The **Regatta Director** goes straight to the two-step Director Setup view (choose
  folder, load Excel). This never auto-restores the previous regatta — the same as
  choosing "Regatta Director" from the picker; it takes precedence over the picker's
  "Resume as Regatta Director" shortcut on an assigned machine.

Hostname matching is case-insensitive and whitespace-trimmed. An FQDN also matches an
entry for its first label (`finish-tower.regatta.local` matches a `finish-tower` entry),
because `os.Hostname()` reports the short name on some platforms and the FQDN on others.
Use the exact value your OS's `hostname` command prints; if it is wrong, **Switch Persona**
(below) is the recovery path.

A host assignment takes effect at the **next launch** or via **Switch Persona** — loading
the file from the Configuration menu does not yank the current session into the assigned flow.

### `challenges` — replace the built-in codes

Maps a persona ID to the challenge code that **replaces** its built-in `rc-*` code. Once a
persona is overridden, its built-in code (published in [persona-plan.md](persona-plan.md))
no longer works; personas with no entry keep their `rc-*` code. Values must be non-empty.

`challenges` only matters when the picker is shown — an un-pinned machine, or a pinned
operator who used **Switch Persona**. It is inert on a machine that `hosts` resolves at
launch (no challenge is asked there at all).

## Validation and fallback

Every failure — file missing, unreadable, empty, invalid JSON, an unknown persona ID, or a
blank challenge value — is non-fatal: a notice appears once the window is up and the normal
persona picker runs. "File not there" versus "file there but bad" is only a log-level
distinction. A malformed file loaded from the Configuration menu is reported and **not**
saved, so it can never wedge the next launch.

## Switch Persona

The **Regatta Clock** application menu carries **Switch Persona…** in every mode. It closes
the current session (stopping the shared-file watcher), resets in-memory timing state, and
re-opens the persona picker — the way for a pinned-host operator to step into a different
role. Open race-clock windows are left as they are; they keep their own bound session.

## Where this lives in the code

- `internal/personacfg` — leaf package: `Config`, `Load`, `AssignmentFor`, `MatchesChallenge`.
- `internal/regatta/persona_config.go` — `loadPersonaConfig` / `assignedPersona` /
  `startAssignedPersona` / `showAssignedPersona` (startup), `matchesChallenge` (picker),
  `switchPersonaItem` / `switchPersona` (menu), and the `hostName()` seam.
- `internal/regatta/config.go` — the Configuration screen row.
- `PrefPersonaConfigFile` and the user-facing strings are in `internal/common/consts.go`.
