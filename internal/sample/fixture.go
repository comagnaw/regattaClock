// Package sample generates a full-size, realistic race day - the Heat Sheet
// workbook, the published Results workbook, and a regattaData tree - from an
// embedded, obfuscated fixture, so every persona can be load-tested against
// production-sized files (docs/features/testing/sample-regatta.md). The
// fixture itself is produced once, from a real workbook, by
// internal/sample/ingest.
//
// A developer tool only: reached through regattaClock's hidden
// -dev-sample-regatta flag, never from the UI. It imports no Fyne, writes no
// preferences, and bypasses the persona write-ahead journal.
package sample

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/comagnaw/regattaClock/internal/persona/store"
)

// BreakMarker is the column-B text of a Heat Sheet break block (a lunch
// break): a 3-row block with no race number. reader.ReadExcelFile skips it;
// the fixture records where it sat so the generated Heat Sheet reproduces it.
const BreakMarker = "Break"

// Fixture is one obfuscated race day: every scheduled race, in race-number
// order, with its lanes and (for a race that was run) the Results sheet's
// Place / Split / Time per lane.
type Fixture struct {
	Name string

	// BreakAfter lists the race numbers a break block follows on the Heat
	// Sheet, ascending.
	BreakAfter []int `json:",omitempty"`

	Races []Race
}

// Race is one Heat Sheet block. A race with no Lanes is a numbered-but-empty
// block - the RD sized the sheet for more races than were scheduled.
type Race struct {
	RaceNumber    int
	ScheduledTime string // as the Heat Sheet shows it, e.g. "09:07 AM"
	BoatClass     string
	FlightInfo    string
	Note          string       // block row 3, column C - e.g. "3 to Advance"
	Lanes         map[int]Lane `json:",omitempty"`
}

// Lane is one lane of a race: the Heat Sheet's school, additional info and
// rower names, and the Results sheet's finish for it ("" when un-raced).
type Lane struct {
	School         string
	AdditionalInfo string                    `json:",omitempty"`
	Rowers         string                    `json:",omitempty"` // block row 3 - "Smith" or "Smith/Jones"
	Status         store.ScheduleEntryStatus `json:",omitempty"`
	Place          string                    `json:",omitempty"`
	Split          string                    `json:",omitempty"`
	Time           string                    `json:",omitempty"`
}

// HasBoats reports whether any lane is assigned.
func (r Race) HasBoats() bool { return len(r.Lanes) > 0 }

// HasResults reports whether any lane carries a Place from the Results sheet.
func (r Race) HasResults() bool {
	for _, l := range r.Lanes {
		if l.Place != "" {
			return true
		}
	}
	return false
}

//go:embed data/regatta-day.json
var fixtureJSON []byte

// Embedded returns the fixture compiled into the binary.
func Embedded() (*Fixture, error) {
	return Parse(fixtureJSON)
}

// Parse decodes and validates a fixture: a name, at least one race, unique
// race numbers, and lanes 1-6. Races come back sorted by race number.
func Parse(b []byte) (*Fixture, error) {
	var fx Fixture
	if err := json.Unmarshal(b, &fx); err != nil {
		return nil, fmt.Errorf("sample fixture does not parse: %w", err)
	}
	if fx.Name == "" {
		return nil, fmt.Errorf("sample fixture has no regatta name")
	}
	if len(fx.Races) == 0 {
		return nil, fmt.Errorf("sample fixture has no races")
	}
	sort.Slice(fx.Races, func(i, j int) bool { return fx.Races[i].RaceNumber < fx.Races[j].RaceNumber })
	seen := make(map[int]bool, len(fx.Races))
	for _, r := range fx.Races {
		if r.RaceNumber < 1 || seen[r.RaceNumber] {
			return nil, fmt.Errorf("sample fixture: race number %d is invalid or repeated", r.RaceNumber)
		}
		seen[r.RaceNumber] = true
		for lane := range r.Lanes {
			if lane < 1 || lane > 6 {
				return nil, fmt.Errorf("sample fixture: race %d has lane %d, want 1-6", r.RaceNumber, lane)
			}
		}
	}
	sort.Ints(fx.BreakAfter)
	return &fx, nil
}

// Marshal encodes a fixture the way it is committed: indented, with a
// trailing newline, map keys (lanes) in ascending order.
func (fx *Fixture) Marshal() ([]byte, error) {
	b, err := json.MarshalIndent(fx, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
