# Contributing to regattaClock

How to build, test, and contribute to regattaClock. AI coding agents should also
read [AGENTS.md](AGENTS.md).

## Prerequisites

- **Go** — the module targets `1.26`; CI builds and tests on `1.27.1`.
- **A C toolchain and OpenGL/X11 development libraries.** fyne links GLFW via
  CGO, so these are required to build or run:
  - **Linux:** `libgl-dev libxcursor-dev libxinerama-dev libxrandr-dev libxi-dev
    libxxf86vm-dev libwayland-dev libxkbcommon-dev wayland-protocols libegl-dev`
    (package names vary by distro). Testing `./internal/...` needs only the X11
    set; building the binary (GLFW compiles an X11 and a Wayland backend) needs
    the Wayland and EGL packages too.
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

## Testing strategy

The suite is unit tests only — one process, a `t.TempDir()`, the pure-Go Fyne
`test` driver, and injected NTP/clock. That covers most of the code; the
shared-folder, multi-operator behaviour is not yet exercised end to end (see
**Not covered yet** below).

### Before a pull request

- `go build ./...`, `go vet ./...`, and `go test ./internal/...` must pass.
- `go test -race ./internal/<pkg>/` for anything touching concurrency —
  `filesystem`, `watcher`, `persona/store`, `timesync`.
- `gofmt -l internal/` is clean, and markdownlint passes on any Markdown you
  changed (`npx -y markdownlint-cli2 "<files>"`).

### What CI runs on every pull request

`.github/workflows/test.yml` gates each PR with two jobs:

- **`coverage`** (`ubuntu-latest`) — `go build ./...` and `go vet ./...` (so
  `cmd/` is compile-checked, which `go test ./internal/...` never does), then
  `go test ./internal/...` with coverage. Posts a PR comment and fails if line
  coverage drops below 60%. This is the fast feedback loop.
- **`test-windows`** (`windows-latest`) — `CGO_ENABLED=0 go test` over the
  OS-portable packages only, with no MinGW. It runs on Windows because
  `internal/filesystem` and `internal/watcher` have real Windows-vs-POSIX
  behaviour: the `os.Rename` sharing-violation retry and its `//go:build
  windows` test, and `fsnotify`'s `ReadDirectoryChangesW` backend. The Fyne GUI
  packages (`clock`, `regatta`, `text`) are platform-agnostic through the
  `test` driver, so they do not gate PRs.

### What CI runs after a merge

- **`test-windows-full`** (`windows-latest`) — MinGW plus the full
  `go test ./internal/...`, including the Fyne packages. It runs on merge to
  `develop`/`main` and on demand via **Actions → Run workflow**. It is
  detection insurance for a Windows-only Fyne or CGO regression, not a PR gate.

### Not covered yet

There are no cross-process or real-shared-folder integration tests: two
operators writing into one `regattaData` tree, an atomic rename racing a sync
client's file lock, a watcher round-trip across two personas. The plan for that
lane is in [docs/features/testing/](docs/features/testing/README.md).

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
- One `develop` and one `main` today. Keeping an older major line alive for fixes
  would use a `release/v<N>` branch, never a second long-lived `develop` — see
  [docs/features/releases.md](docs/features/releases.md).
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
[AGENTS.md](AGENTS.md) under **Releases**; the branching and versioning rationale
is in [docs/features/releases.md](docs/features/releases.md).

The version string lives in the repo-root `version` file and is bumped **only**
in the PR that merges `develop` → `main`; that value feeds both the git tag
(`v` + `VERSION`) and the release notes. `regattaClock -v` (or **Version** in the
app menu) shows the attributes compiled into a given build — version, branch,
commit, build time, source link. A build past its release tag reports
`full: <version>+<N>.g<sha>` (the release line plus commits-since-tag); a bare
`go run` build reports `version: dev`.

### The `RELEASE_TOKEN` secret

`release.yml` publishes the GitHub release and uploads the artifacts with
`softprops/action-gh-release`, authenticated by the repo Actions secret
**`RELEASE_TOKEN`** (both the `build-macos` and `build-windows` jobs pass it as
`GITHUB_TOKEN`). It must be a token that can create releases on this repo:

- **Fine-grained PAT** — Resource owner `comagnaw`; Repository access must
  include `comagnaw/regattaClock`; Repository permissions →
  **Contents: Read and write** (this is what grants the Releases API — creating
  the release and uploading assets). `Metadata: Read-only` comes along
  automatically. No other permission is needed.
- **Classic PAT** — the `repo` scope.

Fine-grained PATs expire, so this breaks periodically. Rotate it under
**Settings → Secrets and variables → Actions → `RELEASE_TOKEN`** (or
`gh secret set RELEASE_TOKEN`), then re-run the failed release run — secrets are
read at run time, so a re-run picks up the new value with no re-tag.

Failure signatures in the `Create Release` / `Upload … Release Asset` step:

- `Bad credentials` (HTTP 401) — the token is expired, revoked, or the secret
  value is wrong.
- `GitHub release failed with status: 403` → `Too many retries` — the token
  authenticates but lacks `Contents: write` (or `comagnaw/regattaClock` is not
  in its repository-access list).

An alternative that removes the rotation burden entirely: give the workflow
`permissions: contents: write` and use the built-in `${{ secrets.GITHUB_TOKEN }}`
instead — nothing consumes the `release` event, so the built-in token is
sufficient.
