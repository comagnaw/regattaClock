# Known issues

Upstream bugs that constrain how the tests are written, and the rules that keep
the suite from tripping over them.

## Fyne's global text shaper is not goroutine-safe

Fyne PR [#6254](https://github.com/fyne-io/fyne/pull/6254) (shipped in v2.8.0)
made `internal/painter/font.go` use a **package-global**
`var shaper = &shaping.HarfbuzzShaper{}`. `HarfbuzzShaper` reuses one internal
`harfbuzz.Buffer` and is not safe for concurrent use, but `walkString` /
`shapeCallback` call `shaper.Shape` with no lock (the `runBufferMut` mutex only
guards a slice, and its unlock is not deferred). Two goroutines measuring text at
once corrupt the shared buffer.

With `go-text/typesetting >= v0.3.4` that corruption is a hard `index out of
range` panic in `harfbuzz.(*Buffer).cur`
([go-text/typesetting#250](https://github.com/go-text/typesetting/issues/250),
closed as a Fyne threading issue). Because the mutex unlock is not deferred, the
recovered panic also orphans that mutex and **deadlocks every later text
measurement in the process** — under `go test` that surfaces as a 10-minute
package timeout, not a single failed test.

Pinned versions: `fyne.io/fyne/v2 v2.8.1`, `github.com/go-text/typesetting
v0.3.5` — both the latest releases; there is no upstream fix to bump to.

**This is only reproducible under Fyne's `test` driver.** That driver runs
`fyne.Do` inline on the calling goroutine, so a background goroutine's `fyne.Do`
render can run concurrently with a foreground one. The production GLFW driver
serialises every `fyne.Do` onto one thread, so the shared shaper is never
touched concurrently there.

## Testing rule

**A test must not let a background goroutine drive a Fyne render while the test
goroutine also renders.** The two background render sources in this codebase:

- the `internal/regatta` schedule watcher — `consumeWatcher` → `applyWatchEvent`
  → `fyne.Do(onScheduleChanged)` → `showRaceTree` / `refreshAllRows`;
- the `internal/clock` stopwatch ticker — `startClockUpdate` →
  `fyne.Do(c.clock.Refresh())` every 100 ms while the clock is running.

Quiesce the background source before the test measures text itself:

- `quiesceWatcher(t, r)` (in `internal/regatta`) — stops the watcher and its
  consumer goroutine now. `directorAt` already does this; call it directly in any
  other test that constructs a director/timer with `PrefRegattaDir` set and then
  writes a watched file and re-renders (e.g. `applyPendingOrigin`, a
  schedule-guard `proceed()`).
- `stopClockTicker(t, c)` (in `internal/clock`) — stops the ticker goroutine.
  Call it after `OpenRaceClock` + Start when the test then refreshes or measures
  text. (No `internal/clock` test currently hits the overlap — the ticker only
  re-renders the short digit string, which does not exercise the crashing GSUB
  ligature path — but the helper is there if one is added.)

Timer tests that call `store.SaveStart` / `SaveFinish` / `SaveSchedule` **before**
`startSession` are safe: the watcher seeds its hashes from the already-written
content and never re-emits, so its consumer stays idle.

## If it recurs

`stopWatch` now runs the watcher shutdown with a 10-second deadline
(`stopWithTimeout`). A failure reading `watcher shutdown did not complete within
10s` means a new render path is racing the watcher — add a `quiesceWatcher(t, r)`
call to that test rather than raising the timeout.
