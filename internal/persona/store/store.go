// Package store holds the on-disk types every persona reads and writes, and the
// functions that load and save them. It deliberately lives outside
// internal/reader: reader is about ingesting an origin (Excel today, an API
// later), store is about multi-writer state shared through regattaData/.
//
// store imports persona for the Role and Team types and for the Session path
// helpers; persona must never import store back. Loading and saving are package
// functions that take a persona.Session rather than methods on Session, which is
// what keeps that dependency one-directional.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/comagnaw/regattaClock/internal/filesystem"
)

// SchemaVersion is written into every persona-owned file's Envelope so a future
// reader can tell which layout it is looking at.
const SchemaVersion = 1

// ErrCorrupt wraps a file that exists but does not parse. Callers must treat it
// as fatal for that persona - never replace a corrupt history with a fresh
// empty one (persona-plan.md section 8).
var ErrCorrupt = errors.New("store: file is corrupt")

// RegattaKey is the cross-file join key for "which regatta is this": a short
// hash of the schedule's name and date. It is stamped on every timing
// envelope so a start.json from last weekend cannot attach to this weekend's
// schedule.
func RegattaKey(name, date string) string {
	h := filesystem.HashBytes([]byte(strings.TrimSpace(name) + "\x00" + strings.TrimSpace(date)))
	return h[:12]
}

// loadJSON reads path into v. A missing file returns an error satisfying
// errors.Is(err, fs.ErrNotExist); a file that does not parse returns one
// satisfying errors.Is(err, ErrCorrupt).
func loadJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%w (%s): %v", ErrCorrupt, filepath.Base(path), err)
	}
	return nil
}

// saveJSONAtomic creates path's parent directory and writes v through the
// atomic writer, so a concurrent reader never sees a half-written file.
func saveJSONAtomic(path string, v any) error {
	if err := filesystem.CreateDirs(filepath.Dir(path)); err != nil {
		return err
	}
	return filesystem.SaveJSONFileAtomic(v, path)
}
