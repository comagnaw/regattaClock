package watcher

import (
	"context"
	"path/filepath"

	"github.com/fsnotify/fsnotify"

	"github.com/comagnaw/regattaClock/internal/applog"
)

// startNotify sets up the fsnotify backend for ModeSMB: it watches the parent
// directories of the registered files and, on a relevant event for one of those
// files, nudges the run loop to re-check it. The periodic poll in run remains as
// a backstop. Returns nil (poll only) if fsnotify cannot be started or none of
// the parent directories can be added yet - a team's timing directory may not
// exist until its first write.
func (w *Watcher) startNotify(ctx context.Context) *fsnotify.Watcher {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		applog.Warn("watcher notify unavailable; polling only", "component", "watcher", "err", err)
		return nil
	}

	dirs := make(map[string]struct{})
	for _, p := range w.snapshotPaths() {
		dirs[filepath.Dir(p)] = struct{}{}
	}

	added := 0
	for d := range dirs {
		if err := fsw.Add(d); err != nil {
			applog.Warn("watcher notify add failed; polling covers it", "component", "watcher", "dir", d, "err", err)
			continue
		}
		added++
	}
	if added == 0 {
		fsw.Close()
		return nil
	}

	go w.notifyLoop(ctx, fsw)
	return fsw
}

const notifyOps = fsnotify.Write | fsnotify.Create | fsnotify.Rename | fsnotify.Remove

func (w *Watcher) notifyLoop(ctx context.Context, fsw *fsnotify.Watcher) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-fsw.Events:
			if !ok {
				return
			}
			if ev.Op&notifyOps == 0 {
				continue
			}
			clean := filepath.Clean(ev.Name)
			if !w.isWatched(clean) {
				continue
			}
			select {
			case w.trig <- clean:
			case <-ctx.Done():
				return
			}
		case err, ok := <-fsw.Errors:
			if !ok {
				return
			}
			applog.Warn("watcher notify error", "component", "watcher", "err", err)
		}
	}
}
