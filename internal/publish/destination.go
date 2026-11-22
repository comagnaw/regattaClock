package publish

import (
	"errors"
	"fmt"
	"time"
)

// Destination renders the publish view into one external form - the results
// workbook, the web-results JSON file, later RegattaCentral
// (publish-destinations.md). Every destination renders from the same
// in-memory view (BuildView / ScheduleView) and records what it published in
// a ledger kept inside its own output, so it is a cache, never an authority.
//
// path is FileName(meta) joined to the destination's folder; a destination
// that publishes to no folder ignores it.
type Destination interface {
	// FileName is the output file's name within its folder.
	FileName(meta Meta) string
	// Read loads what path says has been published. A missing file is not an
	// error - nothing has been published yet - and returns an empty Published.
	Read(path string) (Published, error)
	// Write regenerates path in full. races is every scheduled race in order,
	// with Rows filled for the published ones; ledger is written as given.
	Write(path string, meta Meta, races []PublishableRace, ledger Ledger) error
}

// ErrLocked is returned (wrapped) when another program - typically Excel on
// Windows - holds the output open, or the folder is read-only, so the caller
// can ask the user to close it and retry. The previous file is left intact.
var ErrLocked = errors.New("results file is open in another program")

// ErrForeign is returned by Read for an existing file RegattaClock did not
// write (say, a hand-kept results file with the same name in the publish
// folder). A caller must not overwrite it: regenerating would silently
// discard someone else's work.
var ErrForeign = errors.New("not a RegattaClock results file")

// ErrOtherRegatta is returned when an existing results file belongs to a
// different regatta than the one being published - e.g. last year's regatta
// of the same name, in a folder carried over from it. Merging into it would
// republish that regatta's races under this one's, so a caller must refuse.
var ErrOtherRegatta = errors.New("results file belongs to a different regatta")

// Meta is the regatta-level text a destination prints, plus the
// store.RegattaKey the output belongs to - recorded in it so a later publish
// can refuse to merge into another regatta's file.
type Meta struct {
	Name, Date, RegattaKey string
}

// LedgerEntry records one published race: the Revision the public copy was
// rendered from, and when.
type LedgerEntry struct {
	Revision    string
	PublishedAt time.Time
}

// Ledger is the published-race record, keyed by race number.
type Ledger map[int]LedgerEntry

// Published is what an existing output says has been published: its ledger,
// plus each ledger race's lane rows as published. Rows lets a republish carry
// forward a race that is in the ledger but no longer approved, instead of
// blanking a result the public has already seen. Exists is false (and
// everything else empty) when there was no file.
type Published struct {
	Exists     bool
	RegattaKey string
	Ledger     Ledger
	Rows       map[int][]Row
}

// CheckRegatta returns ErrOtherRegatta when an existing output is not
// regattaKey's. An output with no recorded key is treated as another
// regatta's: its provenance cannot be confirmed.
func (p Published) CheckRegatta(regattaKey string) error {
	if p.Exists && p.RegattaKey != regattaKey {
		return fmt.Errorf("%w (file %q, this regatta %q)", ErrOtherRegatta, p.RegattaKey, regattaKey)
	}
	return nil
}
