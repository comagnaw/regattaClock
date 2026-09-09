# Integration testing

A proposed integration / functional test lane for the shared-folder workflow.
Nothing here is built yet; this is the design to work from.

## Motivation

Several operators share one `regattaData` directory over SMB or a cloud-synced
folder. Each writes only its own file; a watcher drives every other persona's
UI from those writes; a Regatta Director combines the two finish teams. That
design — concurrent writers, atomic replaces, a watcher that must not miss or
double-count a change — is the riskiest and least-tested part of the app.

The current suite never exercises it. Every test is one process against one
`t.TempDir()`. The Windows-vs-POSIX divergence that does exist is all in
`internal/filesystem` and `internal/watcher`, and even there the tests are
single-writer. There is no test where two personas write into one tree and a
third observes the result.

## Scenarios

1. **Multi-writer round-trip.** Two `persona.Session`s — a Primary Start Timer
   and a Primary Finish Timer, or a Primary and a Secondary pairing — write
   `start.json` / `finish.json` into one root while a Regatta Director `watcher`
   runs against the same tree.
   - *Why it matters:* this is the normal race-day topology and has zero
     coverage.
   - *Assert:* every committed write surfaces as exactly one watcher event; a
     reader never sees a partial file; a stray `start-DESKTOP-A1B2C3.json`
     conflict copy is detected once and only once.

2. **Atomic rename vs a held handle.** Loop `SaveJSONFileAtomic` on one file
   while another goroutine holds the target open for read. This promotes the
   idea in `internal/filesystem/file_atomic_windows_test.go` from a single unit
   to a cross-persona flow.
   - *Why it matters:* on Windows a sync client (or a peer reader) holding the
     file triggers `ERROR_SHARING_VIOLATION` on the rename; the retry loop is
     the only thing that keeps a save from failing.
   - *Assert:* the retry wins within the backoff budget; the reader only ever
     sees a whole, valid JSON document. Run on Linux and Windows.

3. **`filepath.FromSlash` on a real Fyne URI.** Feed a `file:///C:/regattas/2026`
   style URI through the folder-pick callbacks in
   `internal/regatta/config.go` and `internal/regatta/persona_startup.go`.
   - *Why it matters:* Fyne reports URI paths with forward slashes; the callbacks
     call `filepath.FromSlash` to restore the native form before persisting.
     Current tests only pass `t.TempDir()` paths, so the `/C:/drive/…` shape is
     never checked.
   - *Assert:* the persisted string is a path the app can `os.Stat` and
     `filepath.Join` against on Windows.

4. **mtime goes backwards.** Rewrite a watched file with an older modification
   time and identical bytes, then with an older mtime and changed bytes.
   - *Why it matters:* cloud clients hydrate remote changes with a stale
     timestamp; the watcher falls back to a content hash for exactly this.
   - *Assert:* the identical-bytes rewrite emits no event; the changed-bytes
     rewrite is detected despite the older mtime.

5. **Schedule change while a clock is open.** The Regatta Director imports a
   changed schedule (a lane swap, a scratch) while a Finish Timer has an open
   `clock` window for an affected race.
   - *Why it matters:* the stale-lane-map (`†`) mark and the schedule-conflict
     banner are the operator's only signal that a committed result no longer
     matches the lane assignments.
   - *Assert:* the mark and the banner appear, and the Finish Timer's committed
     result for that race is flagged.

## Mechanics

- **Build tag.** Put the lane behind `//go:build integration` in a
  `test/integration/` package (or `*_integration_test.go` beside the code it
  drives). The default `go test ./...` must not pick it up.
- **Fake SMB / cloud without infrastructure.** A `t.TempDir()` plus the existing
  `watcher.Mode` switch (`ModeCloud` / `ModeSMB`) and the package's test seams
  for injected latency covers most scenarios. Stand up a real Samba / `smbd`
  container only for a case that genuinely depends on share semantics (metadata
  caching, oplocks).
- **In-process first.** Run the personas as goroutines with separate
  `persona.Session` values against one root — fast and deterministic. Reserve a
  subprocess flavour (`os/exec` the built binary with a persona-config file) for
  the few cases where a real second process matters.
- **Helper.** A `spawnPersonas(t, root, defs...)` that returns the sessions and
  a single stop function, so a scenario reads as "given these operators on this
  folder, when X, then Y".

## CI lane

A new job in `.github/workflows/test.yml`:

- `strategy.matrix.os: [ubuntu-latest, windows-latest]`, `fail-fast: false`.
- `go test -tags integration ./test/integration/...`, `timeout-minutes: 20`.
- Triggers: `push` to `develop`/`main`, a nightly `schedule`, and
  `workflow_dispatch`. **Not** on `pull_request` — these tests are
  timing-sensitive and slower, and belong off the fast PR path.
- `macos-latest` is optional and off by default (10x runner billing); add it
  only if a macOS-specific filesystem or watcher issue is suspected.

## Risks and non-goals

- **Timing assertions are flaky.** Use generous timeouts and poll helpers of the
  `waitEvent(ch, 2*time.Second)` kind already in `internal/watcher`; never a
  fixed `time.Sleep` as the synchronisation point.
- **Keep every test hermetic.** One `t.TempDir()` per test, no shared global
  state, no reliance on wall-clock ordering between processes.
- **Do not test real cloud vendors or a real domain.** No Dropbox / OneDrive /
  Google Drive client and no production SMB server in CI — simulate the
  behaviours (atomic replace, stale mtime, metadata cache) instead.
