# regattaClock

![GitHub Release](https://img.shields.io/github/v/release/comagnaw/regattaClock) ![GitHub License](https://img.shields.io/github/license/comagnaw/regattaClock) ![Go version](https://img.shields.io/github/go-mod/go-version/comagnaw/regattaClock)

<!-- markdownlint-disable MD033 -->
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="internal/assets/images/RegattaClockBanner.svg">
  <img alt="Regatta Clock" src="internal/assets/images/RegattaClockBanner-onlight.svg" width="520">
</picture>
<!-- markdownlint-enable MD033 -->

**regattaClock** is an open-source application for collecting and publishing race
times at rowing regattas run by organizations with limited timing infrastructure.
It is written in Go and uses the [fyne.io](https://fyne.io/) toolkit for its user
interface. The project is in alpha: time collection works; results publishing is a
work in progress.

## Why regattaClock exists

Some rowing organizations do not have a cohesive system for capturing start and
finish times across the race course. regattaClock is built for that environment.
It is run at the finish line: operators record when each boat crosses, the referee
provides the official winning time, and the application back-calculates every
boat's time and produces a result that officials can review, approve, and publish.

## How a regatta is run

Several operators share one regatta folder — on a local network share or a
cloud-synced folder — and each runs regattaClock as a single **persona**:

![Race tree](docs/img/race-tree.png)

- **Regatta Director** — sets up the shared regatta folder, imports and owns the
  race schedule, watches every team's live progress, and exports lane images. Does
  not time races.
- **Start Timer** — records each race's start time. Runs as a primary operator
  plus an independent **secondary** operator as a backup.
- **Finish Timer** — runs the finish-line clock and enters the referee's winning
  time and the order-of-finish. The **primary** Finish Timer's result becomes the
  official result on **Referee Approval**; the **secondary** Finish Timer keeps an
  unapproved backup used for reconciliation.

Every persona reads the one shared schedule and writes only its own file, so the
teams never overwrite each other's work. Additional personas — for media and
results publishing — are planned.

## The finish-line clock

The Finish Timer opens a race clock and selects **Start** when the first boat
crosses the line, **Lap** as each remaining boat crosses, and **Stop** once the
course is clear — capturing a split for every boat. When the Start Timer has
recorded the race's start time, regattaClock combines it with the finish-line
**Start** click to pre-fill an initial **winning time** (the elapsed time of the
first-place boat); the operator overwrites this with the referee's **official**
time when it is given. regattaClock then calculates every boat's finish time from
the winning time and the splits. The operator enters the **order-of-finish**, one
lane number per place. Every captured detail — the splits, the winning time, and
the order-of-finish — stays editable by the Finish Timer after the clock has
stopped, so the results can be corrected against feedback from the course before
the race is reviewed, approved, and published as an official result.

![Finish-line clock](docs/img/finish-line-clock.gif)

## The race schedule

The schedule input today is an Excel workbook (`.xlsx`, or macro-enabled `.xlsm`),
because the organization regattaClock was first built for organizes its race
information in spreadsheets; a structured or API-based input may come later.
regattaClock reads the worksheet named **Results**, or the first worksheet if the
workbook has no sheet by that name, and derives the regatta title, date, and
per-race lane assignments from its layout. The Regatta Director imports the
workbook once into the shared folder; timers then read the shared schedule, never
the workbook itself.

![Example schedule](docs/img/example-schedule.png)

Sample workbooks for both supported formats — `.xlsx` and macro-enabled `.xlsm`
— are in [examples/](examples/).

## Downloading and verifying

Builds are attached to each [GitHub release](https://github.com/comagnaw/regattaClock/releases):

- **macOS** — `regattaClock-<version>-macos-universal.dmg` (Intel + Apple
  Silicon). Not yet notarized, so Gatekeeper needs a one-time
  System Settings → Privacy & Security → **Open Anyway**.
- **Windows** — `regattaClock-<version>-windows-amd64-portable.zip` (unzip and
  run) or `regattaClock-<version>-windows-setup.exe` (per-user installer, Start
  Menu entry, uninstaller). Not yet code-signed, so SmartScreen shows a
  "Windows protected your PC" warning — **More info → Run anyway**.

`regattaClock -v` prints the running build's version, commit, and build date.

Every release also carries a `SHA256SUMS` file and a GitHub build-provenance
attestation. Verify a download before running it:

```sh
# macOS / Linux
shasum -a 256 -c SHA256SUMS

# any platform, with the GitHub CLI
gh attestation verify <file> --repo comagnaw/regattaClock
```

```powershell
# Windows
(Get-FileHash .\regattaClock-<version>-windows-amd64-portable.zip -Algorithm SHA256).Hash
```

## Getting started

![Persona picker](docs/img/persona-picker.png)

1. Each operator opens regattaClock pointed at the shared regatta folder — a
   local network share or a cloud-synced folder (OneDrive, Google Drive).
2. Choose your persona from the picker and enter its **challenge**, a short access
   code. The challenge is either regattaClock's built-in default or a value your
   organization has configured — **confirm it with the Regatta Director or a
   regatta executive before your first session.**
3. Alternatively, an organization can assign a specific computer to a persona by
   hostname. That machine skips the picker and the challenge entirely.
4. The Regatta Director selects the shared folder and imports the schedule; the
   timers confirm the regatta and open their race view. A timer is warned first if
   the regatta's date has already passed.

## Configuration

![Configuration screen](docs/img/configuration.png)

The Configuration screen covers:

- the shared regatta folder;
- storage mode: local network share or cloud-synced folder;
- an optional persona config file (see below);
- logging and debug output;
- time-sync (NTP) servers;
- light or dark theme.

### Persona config file

An organization can point regattaClock at a single JSON file that pre-assigns
personas and/or sets its own challenge codes. Configuring every operator's machine
to use the same file keeps roles and codes consistent across the regatta — it is
recommended but not required. Without it, operators simply pick a persona and
enter its default challenge each launch.

```json
{
  "hosts": {
    "start-tent-pc": "pst",
    "finish-tower": "pft",
    "director-laptop": "rd"
  },
  "challenges": {
    "pst": "spring-start",
    "pft": "spring-finish",
    "rd": "spring-director"
  }
}
```

- **`hosts`** maps a computer's hostname to a persona ID (`pst`, `sst`, `pft`,
  `sft`, `rd`). When the running machine matches, it skips the picker and the
  challenge entirely and goes straight to that persona.
- **`challenges`** replaces the built-in challenge code for a persona, so the
  picker accepts your organization's code instead.

Both sections are optional. If the file is missing or invalid, regattaClock falls
back to the normal persona picker. Full details are in
[docs/features/personas/persona-config-file.md](docs/features/personas/persona-config-file.md).

## Documentation

Design notes live under [docs/features/](docs/features/); the multi-persona model
is in [docs/features/personas/](docs/features/personas/). See
[CONTRIBUTING.md](CONTRIBUTING.md) to build and run from source, and
[AGENTS.md](AGENTS.md) for AI-agent guidance.
