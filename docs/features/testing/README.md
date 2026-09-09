# Testing

How `regattaClock` is tested today, and the plan for the layer that is missing —
end-to-end coverage of the shared-folder, multi-operator workflow that the
persona feature is built on.

**Related docs in this directory**

- [integration-testing.md](integration-testing.md) — the proposed integration /
  functional test lane: scenarios, mechanics, and a CI job that is not built yet

## Where things stand

Everything is a single-process unit test. The `internal/**` suite runs on a
`t.TempDir()`, the pure-Go Fyne `test` driver, and dependency-injected NTP and
clock seams. `.github/workflows/test.yml` gates every pull request on Linux
(`coverage`, with a 60% line-coverage floor) and Windows (`test-windows`), and
runs a full native-Windows job after each merge.

| Layer | Scope | Where it runs |
|---|---|---|
| Unit | `go test ./internal/...` — one process, `t.TempDir()`, Fyne `test` driver, injected NTP/clock | local, plus CI `coverage` (Linux) and `test-windows` (fast, `CGO_ENABLED=0`) on every PR |
| Full native Windows | `go test ./internal/...` including the Fyne packages, with MinGW | CI `test-windows-full`, on merge to `develop`/`main` and via **Run workflow** |
| Integration | multi-persona, one shared directory, watcher round-trips, atomic-rename races | proposed — see [integration-testing.md](integration-testing.md) |
| Manual race-day smoke | `make run`, two personas on one folder, a real clock | maintainer, before a release |

## Why the Windows job is split

Windows-vs-POSIX divergence in this codebase is concentrated in two packages,
`internal/filesystem` (the `os.Rename` sharing-violation retry, its `//go:build
windows` reproduction test, and `dir_test.go`'s `runtime.GOOS` branches) and
`internal/watcher` (`fsnotify`'s `ReadDirectoryChangesW` backend and the
SMB/cloud "mtime goes backwards" handling). Both are pure Go, so the
PR-blocking `test-windows` job runs them with `CGO_ENABLED=0` and no MinGW
toolchain — fast. The Fyne GUI packages (`clock`, `regatta`, `text`) reach the
screen only through Fyne's platform-agnostic `test` driver, so a native-Windows
run of them is low value per PR; `test-windows-full` covers them after a merge
instead.

## Open items

Each is detailed in [integration-testing.md](integration-testing.md):

- **Multi-writer round-trip** — two persona processes writing `start.json` /
  `finish.json` into one root while a Regatta Director watcher runs.
- **Atomic rename vs a held handle** — `SaveJSONFileAtomic` racing a reader that
  holds the target file open.
- **`filepath.FromSlash` on a real Fyne `file:///C:/…` URI** — the folder-pick
  callbacks are only tested with `t.TempDir()` paths today.
- **mtime goes backwards** — a cloud client rewriting a watched file with an
  older timestamp and identical bytes.
- **Schedule change while a clock is open** — the stale-lane-map and
  schedule-conflict paths, end to end.
