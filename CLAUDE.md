# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

See `AGENTS.md` (committed) for repo-wide agent guidance. In particular: run **markdownlint** (`npx -y markdownlint-cli2 "<changed files>"`, config `.markdownlint-cli2.jsonc`) on any Markdown you create or change before committing or opening a PR, and keep `docs/features/**/*.md` lint-clean.

## Overview

**regattaClock** is a cross-platform desktop GUI app (Go + [Fyne v2](https://fyne.io/)) for collecting and publishing race times at rowing regattas that lack a cohesive timing system. It is run at the finish line: an operator starts a clock as the first boat finishes, hits Lap for each subsequent boat, records the referee's official winning time, enters the order-of-finish, and the app back-calculates every boat's time from the collected splits. See `README.md` for the full operator workflow and screenshots.

Project status: alpha. Time collection works; publishing is a work in progress.

## Commands

All common tasks go through the `Makefile`:

- `make build` — `go build` all packages into `.build/bin/`
- `make run` — run the app (`go run cmd/regattaClock/main.go`)
- `make test` — `go test ./... -v -covermode=atomic`
- `make test-cover` — write `coverage.out` and open the HTML coverage report
- `make update-deps` — `go get -u ./...` then `go mod tidy`
- `make clean` — remove `.build/`, `fyne-cross/`, `regattaClock.app/`

Run a single test: `go test ./internal/regatta/ -run TestStartup_RestoresHistory -v`

CI (`.github/workflows/test.yml`), on every PR: `coverage` (`ubuntu-latest`) runs `go build ./...` + `go vet ./...` (the only thing that compiles `cmd/`), then `go test ./internal/...` with coverage (fails below 60%); `test-windows` (`windows-latest`) runs `CGO_ENABLED=0 go test` over the OS-portable packages only, no MinGW. Post-merge (`push` to develop/main) and on `workflow_dispatch`, `test-windows-full` runs the full `go test ./internal/...` with MinGW. See `docs/features/testing/` and CONTRIBUTING.md "Testing strategy". Fyne links GLFW via CGO, so a C toolchain and OpenGL dev libraries are required to build or test the Fyne packages (Linux test: `libgl-dev libxcursor-dev libxinerama-dev libxrandr-dev libxi-dev libxxf86vm-dev`; Linux `go build ./...` also needs `libwayland-dev libxkbcommon-dev wayland-protocols libegl-dev` — GLFW compiles X11 + Wayland backends; Windows: MinGW gcc). CI pins Go 1.27.1; `go.mod` declares 1.26.0.

Releases are tag-driven (`v*`) via `.github/workflows/release.yml` (also runnable on `workflow_dispatch` for artifact-only test builds). Its jobs — `setup → build-macos / build-windows → sign-windows → provenance → release` — produce a macOS universal `.dmg` (`fyne package` + `lipo`), a Windows portable `.zip` and a per-user Inno Setup installer (native `windows-latest` `go build` + `go-winres`, no `fyne-cross`), and `SHA256SUMS` + a build-provenance attestation; `sign-windows` is scaffolded for a Windows code-signing cert and skips until one is configured. Publishing uses the built-in `GITHUB_TOKEN` (`permissions: contents: write`). The version string is the repo-root `version` file (`export VERSION=...`), bumped only in the `develop` → `main` promotion PR; `make build` / `scripts/compile` and `release.yml` link it (plus branch, commit, build time, source URL) into the binary via `-ldflags -X` on `internal/version`. `regattaClock -v` / `-version` prints the `internal/version.Current` struct as JSON; the app menu's **Version** item opens an independent window with the same attributes as `key: value` plus a "View on GitHub" link. A plain `go run` build reports `version: dev`.

## Architecture

Entry point `cmd/regattaClock/main.go` creates a `fyne.App`, then `regatta.NewRegatta(app).Run()`. Everything else lives under `internal/`.

### Package responsibilities

- **`internal/regatta`** — the main application window and its lifecycle. Owns the startup flow (welcome view → set regatta directory → import Excel → race list), the app menu, the config view, theme switching, and session persistence. `Regatta` holds the loaded `*reader.RegattaData` and spawns a `clock.Clock` window per race.
- **`internal/clock`** — a separate window per race for timing (titled `Race N Clock — <persona role>`). `Clock` runs a 100ms ticker goroutine driving a `canvas.Text` stopwatch, collects laps into a fixed 6-row `laps` table (OOF lane / place / split / calculated time), applies the winning-time offset to derive each boat's finish time, feeds the `results` grid, and drives Referee Approval → Save. Timing state lives in `clockState` (`isRunning`, `isCleared`, `startTime`, `stopChan`). The window is laid out in banded zones (a "Timing" accent band carrying the race title + wordmark, over a "Results" band and a reverse-contrast lanes card — see `internal/uitheme`). `compare.go` is the primary FT's read-only, non-blocking **Compare Secondary** window: a visually parallel render of the secondary team's committed `RaceResult`, in amber bands, refreshed live off the watched `timing/secondary/finish.json`; it never writes.
- **`internal/reader`** — parse an `.xlsx`/`.xlsm` workbook into `RegattaData` / `RaceData` / `RaceEntry`. Excel parsing keys off merged cells: the `A1:I1` / `A2:I2` merges give title and date; each 5-row merge in column A is one race, with lane data read from columns C–I. `findRaceSheet` locates the "Results" worksheet (falls back to the first sheet). Extend to new formats by implementing the `sourceData` interface (`setNameAndDate`, `setSourceInfo`, `loadRaces`) and calling `load()`.
- **`internal/exporter`** — render each race with boats to a `race_NN_<Regatta>.png` using freetype and the embedded Verdana Bold font. `Export` returns an `ExportResult` (succeeded/failed counts + errors) rather than failing the whole batch.
- **`internal/filesystem`** — small `os` wrappers: `CreateDirs`, `DirExists`, `ReadDir`, `SaveJSONFile` / `ReadJSONFile`, `FileHash` (sha256).
- **`internal/text`** — `canvas.Text` factory helpers (`Header1`/`Header2`/`Header3`/`Cell`/`Bold`/`BoldLeading`) so type styling is defined once.
- **`internal/common`** — all user-facing strings, preference keys, format constants, and shared enums (`DQ`/`DNF`/`DNS`, etc.). Add UI strings here, not inline.
- **`internal/assets`** — everything `//go:embed`ded into the binary, split by kind: `fonts/VerdanaBold.ttf` (exporter typeface, via `fonts.go`) and `images/RegattaClockBanner.svg` (branding wordmark, a single-fill path wrapped in a themed resource at use time, via `images.go`). `images/RegattaClockBanner-onlight.svg` (dark-fill variant for the README light-mode `<picture>` source) also lives there but is not embedded.

### Data flow & persistence

Excel file → `reader.ReadExcelFile` → `*RegattaData` → held on `Regatta` → serialized to `<RegattaDir>/regattaData/data.json` (`common.RegattaDataDir` / `common.RegattaDataFile`). On next launch, if `PrefRegattaDir` is set and `data.json` exists, the session is restored and the race list shown directly; a missing file is a normal first run, not an error. Exported images go to a user-chosen directory; `results/` under the regatta dir is created up front (`common.ResultsDir`).

User config is stored in Fyne `Preferences` (keys in `common/consts.go`): `RegattaDir`, `Theme` (`Light`/`Dark`), `Debug`, `Logging`.

### Fyne conventions in this codebase

- **Never touch UI from a goroutine directly.** The clock ticker updates widgets inside `fyne.Do(...)`. Follow this for any background work.
- Window close handlers must clean up goroutines (`Clock` closes `clockState.stopChan` in `SetOnClosed`).
- Dialogs raised during `NewRegatta` have no canvas yet; defer them with `App.Lifecycle().SetOnStarted` (see `warnOnStarted`).
- Layout sizes are explicit constants (`internal/regatta/consts.go`, `internal/clock/consts.go`) with comments explaining why each value is what it is — preserve that reasoning when changing them.
- Tests use `fyne.io/fyne/v2/test` with `test.NewTempApp(t)`; drive flows by calling the callback closures directly (e.g. `r.changeCallBack()(lister, nil)`, `r.callback(false)(reader, nil)`) and assert on widget tree state. `internal/regatta/startup_test.go` is the reference pattern.

## In-progress: multi-persona operation

Active design work (branch `persona_stage_0`) lives in `docs/features/personas/`. The plan: several operators share one `regattaData` root, each acting as one **persona** (Regatta Director, Primary/Secondary Start Timer, Primary/Secondary Finish Timer) on a **team**, with a slim `regattaSchedule.json` decoupling timers from the Excel origin and one-writer-per-file separation of start/finish data. Key constraint for implementers: **timing button clicks are the highest-priority path** — wall-clock capture must never block on disk I/O, sync, NTP, or logging; background routines use async queues and best-effort I/O. Read `docs/features/personas/README.md` and `persona-plan.md` before working in this area.
