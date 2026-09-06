package store

import (
	"errors"
	"io/fs"

	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/persona"
)

// Schedule is the slim race program the director owns and every persona reads.
// It carries regatta metadata and lane assignments only - no places, splits,
// times, or approval flags. Those belong to the finish timer in finish.json
// (schedule-data-model.md).
type Schedule struct {
	Name   string
	Date   string
	Origin Origin
	Races  []ScheduleRace
}

// Origin describes where the schedule was ingested from, for the director's
// reload-detection (persona-plan.md section 3b). It mirrors reader.SourceInfo
// without coupling store to reader.
type Origin struct {
	Type string // "excel"
	URI  string // source path or identifier
	Hash string // origin fingerprint at the last accepted load
}

// ScheduleRace is one race's schedule row.
type ScheduleRace struct {
	RaceNumber int
	BoatClass  string
	FlightInfo string
	BoatCount  int
	Lanes      map[int]ScheduleEntry // lane number (1-6) -> entry
}

// ScheduleEntry is one lane's schedule assignment. A scratched lane is an empty
// SchoolName, optionally noted in AdditionalInfo, until the origin encodes it
// explicitly.
type ScheduleEntry struct {
	SchoolName     string
	AdditionalInfo string
}

// Key returns the RegattaKey for this schedule.
func (s *Schedule) Key() string { return RegattaKey(s.Name, s.Date) }

// LoadSchedule reads director/regattaSchedule.json. A missing file returns an
// error satisfying errors.Is(err, fs.ErrNotExist), which the caller treats as a
// normal first run; a file that does not parse returns ErrCorrupt.
func LoadSchedule(s persona.Session) (*Schedule, error) {
	path := s.SchedulePath()
	var sch Schedule
	if err := loadJSON(path, &sch); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			applog.Error("schedule load failed", "component", "store", "file", path, "err", err)
		}
		return nil, err
	}
	applog.Info("schedule hydrated", "component", "store", "file", path, "races", len(sch.Races))
	return &sch, nil
}

// SaveSchedule atomically writes the schedule to director/regattaSchedule.json,
// creating director/ if needed.
func SaveSchedule(s persona.Session, sch *Schedule) error {
	path := s.SchedulePath()
	if err := saveJSONAtomic(path, sch); err != nil {
		applog.Error("schedule write failed", "component", "store", "file", path, "err", err)
		return err
	}
	applog.Info("schedule written", "component", "store", "file", path, "races", len(sch.Races))
	return nil
}
