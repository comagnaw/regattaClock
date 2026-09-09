// Package watcher reports content changes to a fixed set of JSON files under
// regattaData/, so a persona's in-memory mirror of the files it does not own
// (schedule, peer start/finish) stays current without re-reading on a timer in
// the UI.
//
// The event shape is the same for both storage modes; the mode only selects how
// changes are detected. ModeCloud polls with a stat-then-hash short circuit,
// because cloud clients apply remote changes through placeholder hydration that
// fsnotify either misses or spams. ModeSMB prefers fsnotify (with a slow poll
// as a backstop), because the Windows SMB client caches directory metadata for
// ~10s and a fast poller alone would miss updates.
//
// The watcher never blocks a timing click: it runs on its own goroutine keyed
// to the app session's context, and delivers through a channel the caller
// drains into fyne.Do.
package watcher

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/filesystem"
)

// Mode selects the change-detection backend.
type Mode string

const (
	ModeCloud Mode = "cloud" // poll with stat-then-hash (vendor-synced local folder)
	ModeSMB   Mode = "smb"   // prefer fsnotify; fall back to poll
)

// DefaultInterval is the poll period when New is given a non-positive interval.
// Under ModeSMB the caller should pass something at least as long as the SMB
// client's directory-cache lifetime (~10s); fsnotify carries the responsiveness.
const DefaultInterval = 2 * time.Second

// Event is one file whose content changed (or was first seen). Data is the whole
// file; the caller unmarshals it.
type Event struct {
	Path string
	Data []byte
}

// Watcher observes the files registered with Add. The zero value is not usable;
// call New.
type Watcher struct {
	mode     Mode
	interval time.Duration

	mu    sync.Mutex
	paths map[string]struct{} // watched file paths, cleaned

	events chan Event
	trig   chan string // "" = re-check everything; else one path
	cancel context.CancelFunc
	done   chan struct{}

	// touched only by the run goroutine
	state     map[string]fileState
	warned    map[string]struct{}
	conflicts map[string]struct{}
}

type fileState struct {
	present bool
	modTime time.Time
	size    int64
	hash    string
}

// New returns an unstarted Watcher. interval <= 0 uses DefaultInterval.
func New(mode Mode, interval time.Duration) *Watcher {
	if interval <= 0 {
		interval = DefaultInterval
	}
	return &Watcher{
		mode:     mode,
		interval: interval,
		paths:    make(map[string]struct{}),
	}
}

// Add registers a file to watch. Safe before or after Start. Directories and
// paths under logs/ must never be passed - the watcher only ever touches exactly
// these paths and their parent directory (for conflict-copy scanning in cloud
// mode).
func (w *Watcher) Add(path string) {
	clean := filepath.Clean(path)
	w.mu.Lock()
	w.paths[clean] = struct{}{}
	w.mu.Unlock()

	select {
	case w.trig <- clean:
	default:
	}
}

// Start begins watching and returns the channel changes are delivered on. The
// channel is closed when ctx is cancelled or Stop is called. A second call is a
// no-op and returns the same channel.
func (w *Watcher) Start(ctx context.Context) <-chan Event {
	w.mu.Lock()
	if w.cancel != nil {
		ch := w.events
		w.mu.Unlock()
		return ch
	}
	runCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.done = make(chan struct{})
	w.events = make(chan Event, 32)
	w.trig = make(chan string, 64)
	w.mu.Unlock()

	go w.run(runCtx)
	return w.events
}

// Stop cancels the watch and waits for the goroutine to exit. Safe without Start
// and safe to call more than once.
func (w *Watcher) Stop() {
	w.mu.Lock()
	cancel, done := w.cancel, w.done
	w.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (w *Watcher) run(ctx context.Context) {
	defer close(w.done)
	defer close(w.events)

	w.state = make(map[string]fileState)
	w.warned = make(map[string]struct{})
	w.conflicts = make(map[string]struct{})

	var fsw *fsnotify.Watcher
	if w.mode == ModeSMB {
		fsw = w.startNotify(ctx)
	}
	if fsw != nil {
		defer fsw.Close()
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.checkAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.checkAll(ctx)
		case p := <-w.trig:
			if p == "" {
				w.checkAll(ctx)
			} else {
				w.checkPath(ctx, p)
				w.scanConflictsFor(p)
			}
		}
	}
}

// snapshotPaths returns a copy of the watched set so the run goroutine never
// holds w.mu across I/O.
func (w *Watcher) snapshotPaths() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]string, 0, len(w.paths))
	for p := range w.paths {
		out = append(out, p)
	}
	return out
}

func (w *Watcher) isWatched(clean string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, ok := w.paths[clean]
	return ok
}

func (w *Watcher) checkAll(ctx context.Context) {
	paths := w.snapshotPaths()
	for _, p := range paths {
		if ctx.Err() != nil {
			return
		}
		w.checkPath(ctx, p)
	}
	if w.mode == ModeCloud {
		seen := make(map[string]struct{})
		for _, p := range paths {
			dir := filepath.Dir(p)
			if _, done := seen[dir]; done {
				continue
			}
			seen[dir] = struct{}{}
			w.scanConflicts(dir, paths)
		}
	}
	applog.Debug("watcher poll", "component", "watcher", "files", len(paths))
}

// checkPath does the stat-then-hash comparison for one file and emits an Event
// when the content differs from what was last delivered.
func (w *Watcher) checkPath(ctx context.Context, path string) {
	info, err := os.Stat(path)
	if err != nil {
		st := w.state[path]
		if os.IsNotExist(err) {
			// Not created yet, or removed. Not an error; a later create emits.
			st.present = false
			w.state[path] = st
		} else {
			w.warnOnce("watcher stat failed", path, err)
		}
		return
	}

	st := w.state[path]
	if st.present && st.size == info.Size() && st.modTime.Equal(info.ModTime()) {
		return // cheap check: nothing moved
	}

	// os.Stat is usually safe on a streaming placeholder; os.ReadFile may
	// trigger hydration and can fail offline. A read failure is "no change".
	data, err := os.ReadFile(path)
	if err != nil {
		w.warnOnce("watcher read failed", path, err)
		return
	}
	delete(w.warned, "watcher read failed"+path)
	delete(w.warned, "watcher stat failed"+path)

	hash := filesystem.HashBytes(data)
	next := fileState{present: true, modTime: info.ModTime(), size: info.Size(), hash: hash}

	if st.present && st.hash == hash {
		// Cloud sync can rewrite a file with an older mtime and identical
		// bytes; record the new stat but do not re-emit.
		w.state[path] = next
		return
	}

	w.state[path] = next
	applog.Info("watcher change", "component", "watcher", "path", path, "bytes", len(data))
	select {
	case w.events <- Event{Path: path, Data: data}:
	case <-ctx.Done():
	}
}

func (w *Watcher) warnOnce(msg, path string, err error) {
	key := msg + path
	if _, done := w.warned[key]; done {
		return
	}
	w.warned[key] = struct{}{}
	applog.Warn(msg, "component", "watcher", "path", path, "err", err)
}
