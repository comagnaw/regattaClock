# Contributing to regattaClock

How to build, test, and contribute to regattaClock. AI coding agents should also
read [AGENTS.md](AGENTS.md).

## Prerequisites

- **Go** — the module targets `1.26`; CI builds and tests on `1.27.1`.
- **A C toolchain and OpenGL/X11 development libraries.** fyne links GLFW via
  CGO, so these are required to build or run:
  - **Linux:** `libgl-dev libxcursor-dev libxinerama-dev libxrandr-dev libxi-dev
    libxxf86vm-dev` (package names vary by distro).
  - **Windows:** MinGW gcc.
  - **macOS:** the Xcode command-line tools (`xcode-select --install`).

## Build, run, test

Common tasks go through the `Makefile`:

- `make build` — build every package into `.build/bin/`
- `make run` — run the app (`go run ./cmd/regattaClock`)
- `make test` — `go test ./... -v -covermode=atomic`
- `make test-cover` — write `coverage.out` and open the HTML report
- `make update-deps` — `go get -u ./...` then `go mod tidy`
- `make clean` — remove build output

Run a single test:

```sh
go test ./internal/regatta/ -run TestStartup_RestoresHistory -v
```

CI (`.github/workflows/test.yml`) runs `go test ./internal/...` on Linux and
Windows on every pull request and fails if line coverage drops below 60%.

## Project layout

- `cmd/regattaClock` — entry point; creates the fyne app and starts `regatta`.
- `internal/regatta` — the main window and its lifecycle: startup / persona
  picker, the Regatta Director setup and race tree, the Configuration screen,
  session persistence, the app menu.
- `internal/clock` — the per-race timing window (stopwatch, laps, order-of-finish,
  winning-time offset, Referee Approval).
- `internal/reader` — parse an `.xlsx` / `.xlsm` workbook into the race schedule.
- `internal/persona` and `internal/persona/store` — the persona model (roles,
  teams, challenges) and the one-writer-per-file layout of the shared
  `regattaData` tree.
- `internal/exporter` — render per-race lane images (freetype + embedded font).
- `internal/watcher` — watch the shared folder (cloud or SMB mode) and surface
  changes.
- `internal/timesync` — measure NTP clock offset (never applied to the system
  clock).
- `internal/applog` — structured JSON logging on a non-blocking async writer.
- `internal/common` — all user-facing strings, preference keys, and shared enums.

## Branching and pull requests

- Branch off `develop`; open pull requests against `develop`.
- `main` is the release branch — `develop` is merged into `main` when a release is
  cut.
- Keep pull requests focused. Before opening one, `go build ./...`,
  `go vet ./...`, and `go test ./internal/...` must pass; run
  `go test -race ./internal/<pkg>/` for anything that touches concurrency.
- User-facing strings belong in `internal/common`, not inline.
- End commit messages with `Co-Authored-By:` trailers as appropriate.

## Documentation

- Design notes go under `docs/features/`.
- Run **markdownlint** on any Markdown you create or change before opening a pull
  request: `npx -y markdownlint-cli2 "<files>"` (config `.markdownlint-cli2.jsonc`
  at the repo root). `docs/features/**/*.md` is expected to stay lint-clean. See
  [AGENTS.md](AGENTS.md) for the full rule.

## Releases

Releases are cut from `main`. Pushing a `v*` tag triggers
`.github/workflows/release.yml`, which builds the macOS `.dmg` and Windows
`.exe.zip` artifacts and publishes a GitHub release. The full step-by-step
procedure — version choice, pre-release handling, and the release notes — is in
[AGENTS.md](AGENTS.md) under **Releases**.
