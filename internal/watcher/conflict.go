package watcher

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/comagnaw/regattaClock/internal/applog"
)

// scanConflictsFor scans the directory of one watched path, if the mode calls
// for it.
func (w *Watcher) scanConflictsFor(path string) {
	if w.mode != ModeCloud {
		return
	}
	w.scanConflicts(filepath.Dir(path), w.snapshotPaths())
}

// scanConflicts looks for sync-client conflict copies alongside the watched
// files in dir - siblings whose stem matches an expected file but whose name is
// not the exact expected name (OneDrive "start-DESKTOP-A1B2C3.json", Google
// Drive "start (1).json", trailing ".conflict", etc.). Each new one is logged
// once at WARN. Only meaningful under ModeCloud; SMB produces a sharing
// violation on write instead.
func (w *Watcher) scanConflicts(dir string, watched []string) {
	var stems []string
	expected := make(map[string]struct{})
	for _, p := range watched {
		if filepath.Dir(p) != dir {
			continue
		}
		base := filepath.Base(p)
		expected[base] = struct{}{}
		stems = append(stems, strings.TrimSuffix(base, filepath.Ext(base)))
	}
	if len(stems) == 0 {
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if _, ok := expected[name]; ok {
			continue
		}
		if name == "desktop.ini" || strings.HasSuffix(name, ".tmp") {
			continue
		}
		for _, stem := range stems {
			if !isConflictName(name, stem) {
				continue
			}
			full := filepath.Join(dir, name)
			if _, seen := w.conflicts[full]; seen {
				break
			}
			w.conflicts[full] = struct{}{}
			applog.Warn("conflict copy detected", "component", "watcher",
				"path", full, "expected", stem+".json")
			break
		}
	}
}

// isConflictName reports whether name looks like a conflict copy of
// "<stem>.json": it starts with stem and the character right after stem is not
// alphanumeric (so "start-x.json", "start (1).json", "start.json.conflict"
// match; "startlist.json" does not). The caller has already excluded the exact
// expected name.
func isConflictName(name, stem string) bool {
	if !strings.HasPrefix(name, stem) || len(name) == len(stem) {
		return false
	}
	c := name[len(stem)]
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return false
	default:
		return true
	}
}
