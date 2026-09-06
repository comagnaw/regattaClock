# Persona Event Logging

Analysis and recommendation for OS-agnostic, syslog-style troubleshooting logs gated by the existing **Logging** preference. Deliberately separate from [persona-plan.md](persona-plan.md); a short pointer belongs there once you adopt an approach.

## Short answer

Use Go's standard `log/slog` with a **JSON handler** writing **append-only files under `regattaData/logs/`**, one file per persona session path, gated by `PrefLogging`. Do **not** use the Windows Event Log. Keep the hot path non-blocking with a buffered async writer. Every line carries stable identity fields (`persona`, `team`, `role`, `machine`, …). Log user actions, NTP measure/drift, and watcher *content changes* — not every poll tick.

## 1. What already exists

[internal/regatta/config.go](internal/regatta/config.go) already exposes a Logging checkbox bound to `common.PrefLogging`. Nothing reads that boolean yet. The only `log.Printf` usage today is in the lane-image exporter. That preference should remain the **sole enable switch**: when false, the logger is a no-op (or `slog.DiscardHandler`); when true, file logging starts after the persona has a resolved `regattaData` root.

`PrefDebug` stays separate: Logging = write troubleshooting events to disk; Debug = optional noisier detail (and whatever UI/dev behaviour you already intend). Do not overload one checkbox for both.

## 2. Why not Windows Event Log

You already dislike it, and it also fails the OS-agnostic requirement. Event Log is Windows-only, needs different APIs, is awkward to collect from a spare timing PC mid-regatta, and is a poor fit for "email me the log from the Finish Timer laptop." A plain text file next to the race data is what officials can actually attach to a bug report.

## 3. JSON lines (syslog *ideas*, JSON *shape*)

"Syslog style" here means the operational habits — one event per line, severity, timestamp, component — not RFC5424 text or a remote syslog daemon. For analysis, **JSON Lines** (NDJSON: one JSON object per line) is the better on-disk format: `jq`, Python, Excel Power Query, and most log tools ingest it cleanly.

**`log/slog` supports JSON natively** via `slog.JSONHandler`. No extra dependency. Example line:

```json
{"time":"2026-09-06T14:03:11.452-04:00","level":"INFO","msg":"button click","persona":"pst","team":"primary","role":"start","machine":"DESKTOP-A1B2C3","component":"race_tree","action":"start_time","race":12,"display":"14:03:11.4"}
```

```json
{"time":"2026-09-06T14:05:00.120-04:00","level":"INFO","msg":"ntp measure","persona":"pst","team":"primary","role":"start","machine":"DESKTOP-A1B2C3","component":"timesync","source":"192.168.1.10","offset_ms":12,"rtt_ms":3}
```

Shipping to a remote syslog server (UDP 514) remains optional later and is a poor default on race day.

Prefer `slog.JSONHandler` over `slog.TextHandler`. Text is nicer for eyeballing a single file; JSON is nicer once you are correlating four personas' logs after a bad race — which is the actual troubleshooting workflow.

## 4. Where the files should live

Your gut — under `regattaData` — is right for **troubleshooting a shared regatta**, with one important constraint from the persona design: **one writer per file**, and the watcher must ignore log paths.

### Proposed layout

```
regattaData/
├── director/
├── timing/
└── logs/
    ├── primary/
    │   ├── start-DESKTOP-A1B2C3.log      # Primary ST on that host
    │   └── finish-LAPTOP-XYZ.log        # Primary FT on that host
    ├── secondary/
    │   ├── start-DESKTOP-A1B2C3.log
    │   └── finish-LAPTOP-XYZ.log
    └── director/
        └── director-DESKTOP-RD01.log
```

Pattern: `logs/<team>/<role>-<hostname>.log` (director: `logs/director/director-<hostname>.log`).

**Hostname in the filename and on every JSON line.** The file name prevents two machines that somehow claim the same persona/team from appending to one file (the same class of failure that produces OneDrive conflict copies on timing data). The JSON `machine` field still appears on every line so a concatenated or renamed file remains self-describing for analysis.

Sanitize the hostname the same way as other Windows filenames (`sanitizeForFilename`) so characters illegal in paths cannot break log creation.

### Why not a single shared `regattaClock.log`

Multiple personas appending to one file on OneDrive/SMB is exactly the multi-writer problem the plan spent pages avoiding: partial lines, conflict copies, and interleaved garbage. Per-persona files keep the same ownership rule as `start.json` / `finish.json`.

### Why not only local AppData

Local-only logs are safer for I/O and do not sync, but when the RD is diagnosing "why didn't FT get race 12's start time?", the useful trail is on *another* laptop. Putting logs under `regattaData` means the RD (or anyone with the share) can collect them after the fact without chasing four machines. That benefit is large enough to accept the sync cost **if logging is opt-in via `PrefLogging`** and volume stays low.

### Watcher and sync interactions

- The watcher must **never** watch `logs/` (exact-filename allowlist already required for timing files — keep logs off that list).
- Under OneDrive, log appends will sync; that is acceptable at low volume, noisy if you log every 2s poll. See section 6.
- Under SMB, appends are cheap; still ignore `logs/` in the watcher.
- Do **not** rotate into dozens of dated files by default; one append file per persona per regatta is enough. Optional size-based rotate (e.g. at 5 MiB keep `start.log.1`) is a later nicety, not day-one.

### Before `regattaData` is known

Persona picker and challenge failures happen *before* the directory is chosen. Options:

1. Buffer early events in memory and flush once the root is known (and logging is on).
2. Write a short-lived local file under the Fyne app data dir, then stop using it once `regattaData/logs/...` is available.

Prefer (1) for simplicity: a ring buffer of the last N startup events, discarded if the user never enables logging or never opens a directory.

## 5. What to log (and what not to)

### Log when `PrefLogging` is true

| Area | Events | Level |
|------|--------|-------|
| UI / persona | Persona selected, challenge pass/fail, directory chosen/rejected, confirm accept/decline | INFO |
| Race tree | `Start Time`, `Clear`, `Restore`, `Time Race` clicks; race number and resulting display value | INFO |
| Clock | Start / Lap / Stop / Clear; Winning Time entered or auto-filled; Referee Approval; Save | INFO |
| NTP / timesync | Measure success (source, offset, RTT); measure failure; drift beyond threshold; source=`none` | INFO (WARN if \|offset\| > ~1s) |
| Watcher | Content change detected (path, new sequence / hash short id); conflict-copy spotted; read failure | INFO / WARN |
| Store | Atomic write success/failure; hydrate on startup; RegattaKey mismatch; parse failure blocking writes | INFO / ERROR |

### Do **not** log by default

- Every watcher poll with "no change" (that is the hot path every ~2s × 4 files).
- Every clock UI refresh tick (100ms).
- Full file contents or PII beyond race numbers and school names already on the schedule (keep messages small).

If both `PrefLogging` and `PrefDebug` are on, you may add DEBUG lines for poll stats (e.g. once a minute: files watched, last change ages). Never enable per-tick DEBUG from Logging alone.

## 6. Keep it off the critical path

Timing clicks must not wait on disk or OneDrive.

**Pattern:**

```go
// conceptual
type Logger interface {
    Info(msg string, args ...any)
    Warn(msg string, args ...any)
    Error(msg string, args ...any)
}

// When PrefLogging is false: discard implementation (no allocations in hot path if possible).
// When true: slog -> channel -> single goroutine -> os.File.Write append.
```

- One dedicated writer goroutine per process; UI and watcher only enqueue a small struct or call `slog` backed by an async handler.
- Drop or count-overflow if the queue is full rather than blocking the Start button (a few lost log lines beat a delayed start time).
- Open the log file once at session start; flush periodically (e.g. 1s) and on shutdown — not `Sync()` on every line (that would fight OneDrive and SMB).
- Failures to write logs must never surface as fatal dialogs on the timing path; at most a one-time WARN in-process and continue.

This matches your "not too intensive to the application's primary function" constraint.

## 7. Suggested package shape

New package `internal/applog` (avoid the name `log` — clashes with stdlib):

```go
package applog

func Init(enabled bool)                          // from PrefLogging at startup / on toggle
func SetOutput(path string) error                // after persona + regattaData resolved
func SetIdentity(persona, team, role, machine string) // fixed attrs on every subsequent line
func With(attrs ...any) *slog.Logger             // extra attrs for a subsystem
func Info(msg string, args ...any)
func Warn(msg string, args ...any)
func Error(msg string, args ...any)
func Close()                                     // flush on shutdown
```

**Identity fields on every line** (set once via `Logger.With` / `SetIdentity` when the session is known — do not repeat them at each call site):

| Field | Example | Source |
|-------|---------|--------|
| `persona` | `pst`, `pft`, `sft`, `rd` | `persona.Definition.ID` |
| `team` | `primary`, `secondary`, `""` for RD | `persona.Team` |
| `role` | `start`, `finish`, `director` | `persona.Role` |
| `machine` | `DESKTOP-A1B2C3` | `os.Hostname()` |

Also useful as session defaults (same mechanism): `regatta_key` (short), `storage_mode` (`onedrive` \| `smb`).

Call sites stay thin and only add event-specific keys:

```go
applog.Info("button click", "component", "race_tree", "action", "start_time", "race", n, "display", display)
applog.Info("ntp measure", "component", "timesync", "source", src, "offset_ms", offset.Milliseconds(), "rtt_ms", rtt.Milliseconds())
applog.Info("watcher change", "component", "watcher", "path", rel, "sequence", seq)
```

Implementation sketch: `slog.New(slog.NewJSONHandler(asyncWriter, &slog.HandlerOptions{Level: slog.LevelInfo}))`, then `logger = logger.With("persona", id, "team", team, "role", role, "machine", host)`. That is exactly what `slog` is for — child loggers inherit attrs without per-call boilerplate.

## 8. Lifecycle relative to PrefLogging

1. App starts → read `PrefLogging`. If false, `Init(false)` and return.
2. If true, buffer to memory until `regattaData` + persona are known.
3. Create `logs/<team>/<role>-<hostname>.log` (or `logs/director/director-<hostname>.log`), `SetOutput`, flush buffer.
4. If the user toggles Logging off in config mid-session, close the file and switch to discard (no restart required).
5. If they toggle on mid-session and a root exists, open the file and continue.

Creating `logs/` is the persona's responsibility on first write, same as creating `timing/<team>/`.

## 9. Alternatives considered

| Approach | Verdict |
|----------|---------|
| Windows Event Log | Rejected — not OS-agnostic; hard to collect |
| Remote syslog (UDP/TCP) | Optional later; race-day network is already fragile |
| Third-party zap/zerolog | Unnecessary; `slog` + `JSONHandler` is enough |
| `slog.TextHandler` (key=value) | Fine for eyeballing; worse for multi-file analysis — prefer JSON |
| Console-only | Useless on a Fyne GUI release build |
| Logs only in AppData | Easy I/O, poor cross-machine troubleshooting |
| One shared log file in `regattaData` | Multi-writer hazard on OneDrive/SMB |
| Per-persona file without hostname | Safer, but two hosts claiming the same persona/team still collide |
| Per-persona **and** per-host files under `regattaData/logs/` | **Recommended** — filename isolates writers; JSON `machine` still on every line |

## 10. Recommendation

1. Gate everything on existing **`PrefLogging`**.
2. Implement **`internal/applog`** with `slog.JSONHandler` + async append writer.
3. Put **`persona`, `team`, `role`, and `machine` on every line** via `logger.With` at session start.
4. Write to **`regattaData/logs/<team>/<role>-<hostname>.log`** (plus `director-<hostname>.log`), so two hosts cannot share one log file even if they claim the same persona.
5. Log **clicks, NTP measure/drift, watcher content changes, store hydrate/write errors** — not poll heartbeats.
6. Ensure the **watcher ignores `logs/`**.
7. Keep logging **best-effort**: never block or fail the timing path because a log write failed.

## 11. Fit with the persona plan

This can land as a thin cross-cutting phase after storage mode prefs (or alongside watcher/timesync, since those are primary producers). It does not change the data model in [persona-plan.md](persona-plan.md) aside from creating the `logs/` tree and documenting that the watcher allowlist excludes it.

Open follow-ups (not required for a useful first version):

- Size-based rotation
- Optional "Copy logs to clipboard / zip for support" button on the Director
- True remote syslog export behind a second preference
