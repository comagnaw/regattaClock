// Package applog is the persona feature's structured event log. It wraps
// log/slog with a JSON handler writing append-only NDJSON to a per-persona file
// under regattaData/logs/, gated by the Logging preference and widened to DEBUG
// by the Debug preference.
//
// The package is a process-wide singleton configured once at startup and again
// whenever the preferences change. Every write goes through a non-blocking async
// writer so a timing button click never waits on disk or cloud sync: if the
// queue is full, lines are dropped and counted rather than blocking the caller.
//
// When Logging is off the logger is slog's discard handler, so call sites can
// log unconditionally without guarding on a preference.
package applog

import (
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
)

var (
	mu        sync.Mutex
	lvl       = new(slog.LevelVar) // shared with the live handler; Set is race-free
	loggingOn bool
	outPath   string
	writer    *asyncWriter // non-nil only while logging is on
	ident     identity

	// active is read on every Debug/Info/Warn/Error call with no lock. It points
	// at the discard logger whenever logging is off.
	active  atomic.Pointer[slog.Logger]
	discard = slog.New(slog.DiscardHandler)
)

func init() { active.Store(discard) }

type identity struct {
	personaID string
	team      string
	role      string
	machine   string
}

// attrs returns the identity as slog key/value pairs, omitting the fields that
// are not known yet (before the persona phases wire them).
func (i identity) attrs() []any {
	var a []any
	if i.personaID != "" {
		a = append(a, "persona_id", i.personaID)
	}
	if i.team != "" {
		a = append(a, "team", i.team)
	}
	if i.role != "" {
		a = append(a, "role", i.role)
	}
	if i.machine != "" {
		a = append(a, "machine", i.machine)
	}
	return a
}

// Init configures the logger from the Logging and Debug preferences. Call it
// once at process start, before any other package function. When loggingEnabled
// is false everything is discarded regardless of debugEnabled.
func Init(loggingEnabled, debugEnabled bool) {
	mu.Lock()
	defer mu.Unlock()
	if ident.machine == "" {
		if host, err := os.Hostname(); err == nil {
			ident.machine = host
		}
	}
	configureLocked(loggingEnabled, debugEnabled)
}

// SetLevel re-applies the preferences after the user toggles them mid-session.
// It has the same effect as Init but keeps the identity and output path already
// set: turning Logging off closes the file, turning it back on reopens it.
func SetLevel(loggingEnabled, debugEnabled bool) {
	mu.Lock()
	defer mu.Unlock()
	configureLocked(loggingEnabled, debugEnabled)
}

// configureLocked applies loggingEnabled/debugEnabled to the shared state. mu
// must be held.
func configureLocked(loggingEnabled, debugEnabled bool) {
	if debugEnabled {
		lvl.Set(slog.LevelDebug)
	} else {
		lvl.Set(slog.LevelInfo)
	}

	loggingOn = loggingEnabled
	if !loggingEnabled {
		if writer != nil {
			writer.close()
			writer = nil
		}
		active.Store(discard)
		return
	}

	if writer == nil {
		writer = newAsyncWriter()
		if outPath != "" {
			// Best effort: a WARN is emitted below via the live logger if this
			// fails, and buffered lines stay in memory until a later SetOutput.
			_ = writer.setFile(outPath)
		}
	}
	rebuildActiveLocked()
}

// rebuildActiveLocked points active at a fresh JSON logger over the current
// writer and identity. mu must be held and writer must be non-nil.
func rebuildActiveLocked() {
	l := slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: lvl}))
	if a := ident.attrs(); len(a) > 0 {
		l = l.With(a...)
	}
	active.Store(l)
}

// SetOutput points the logger at path (created, with parent directories, if
// absent) and replays anything buffered before now. Safe to call before Init or
// while Logging is off: the path is remembered and opened when logging turns on.
func SetOutput(path string) error {
	mu.Lock()
	defer mu.Unlock()
	outPath = path
	if !loggingOn || writer == nil {
		return nil
	}
	return writer.setFile(path)
}

// SetIdentity sets the fixed attributes stamped on every line. persona_id,
// team, and role are empty until the persona phases; machine defaults to the
// hostname from Init. Empty values are omitted.
func SetIdentity(personaID, team, role, machine string) {
	mu.Lock()
	defer mu.Unlock()
	ident = identity{personaID: personaID, team: team, role: role, machine: machine}
	if ident.machine == "" {
		if host, err := os.Hostname(); err == nil {
			ident.machine = host
		}
	}
	if loggingOn && writer != nil {
		rebuildActiveLocked()
	}
}

// Close flushes and closes the log file. Call it once on shutdown.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if writer != nil {
		writer.close()
		writer = nil
	}
	active.Store(discard)
	loggingOn = false
}

// With returns a child logger carrying attrs in addition to the identity
// fields, for a subsystem that adds the same keys to many lines.
func With(attrs ...any) *slog.Logger { return active.Load().With(attrs...) }

// Debug logs at DEBUG; it only reaches disk when both Logging and Debug are on.
func Debug(msg string, args ...any) { active.Load().Debug(msg, args...) }

// Info logs a normal operational event (button click, hydrate/save success,
// watcher content change, NTP measure).
func Info(msg string, args ...any) { active.Load().Info(msg, args...) }

// Warn logs a recoverable problem worth noticing (large NTP offset, conflict
// copy, non-fatal read failure).
func Warn(msg string, args ...any) { active.Load().Warn(msg, args...) }

// Error logs a failure that already surfaced to the user or aborted an
// operation. Include the underlying error as an "err" attribute.
func Error(msg string, args ...any) { active.Load().Error(msg, args...) }
