package regatta

import (
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/reader"
)

// scheduleChange records how one race's schedule row moved between two loads
// (persona-plan.md 3c). lanes holds the lane numbers whose school or scratch
// state changed, for the open-clock highlight.
type scheduleChange struct {
	scratch bool // a lane gained or lost its school (SCR)
	moved   bool // a school or its additional info changed within a lane
	meta    bool // boat class or flight changed
	lanes   map[int]bool
}

func (c scheduleChange) any() bool { return c.scratch || c.moved || c.meta }

// changedLanes returns the affected lane numbers in ascending order.
func (c scheduleChange) changedLanes() []int {
	out := make([]int, 0, len(c.lanes))
	for lane := 1; lane <= 6; lane++ {
		if c.lanes[lane] {
			out = append(out, lane)
		}
	}
	return out
}

// diffSchedule compares two RegattaData snapshots by race number and lane and
// returns only the races that materially changed. New or dropped races are left
// out - the race set changing is handled by a full tree rebuild, not a notice.
func diffSchedule(old, cur *reader.RegattaData) map[int]scheduleChange {
	out := map[int]scheduleChange{}
	if old == nil || cur == nil {
		return out
	}

	prev := make(map[int]reader.RaceData, len(old.Races))
	for _, race := range old.Races {
		prev[race.RaceNumber] = race
	}

	for _, race := range cur.Races {
		o, ok := prev[race.RaceNumber]
		if !ok {
			continue
		}
		ch := scheduleChange{lanes: map[int]bool{}}
		if o.BoatClass != race.BoatClass || o.FlightInfo != race.FlightInfo {
			ch.meta = true
		}
		for lane := 1; lane <= 6; lane++ {
			ob, nb := o.Lanes[lane], race.Lanes[lane]
			if ob.SchoolName == nb.SchoolName && ob.AdditionalInfo == nb.AdditionalInfo {
				continue
			}
			ch.lanes[lane] = true
			if (ob.SchoolName == "") != (nb.SchoolName == "") {
				ch.scratch = true
			} else {
				ch.moved = true
			}
		}
		if ch.any() {
			out[race.RaceNumber] = ch
		}
	}
	return out
}

// scheduleFromRegattaData projects a freshly imported RegattaData onto the slim
// schedule that is persisted: regatta metadata, lane assignments, class and
// flight only. Places, splits, times, approval flags, and the raw Excel grid
// are dropped - the finish timer owns those in finish.json
// (schedule-data-model.md).
func scheduleFromRegattaData(rd *reader.RegattaData) *store.Schedule {
	sch := &store.Schedule{
		Name: rd.Name,
		Date: rd.Date,
		Origin: store.Origin{
			Type: rd.Type,
			URI:  rd.URI,
			Hash: rd.Hash,
		},
		Races: make([]store.ScheduleRace, 0, len(rd.Races)),
	}

	for _, race := range rd.Races {
		out := store.ScheduleRace{
			RaceNumber: race.RaceNumber,
			BoatClass:  race.BoatClass,
			FlightInfo: race.FlightInfo,
			BoatCount:  race.BoatCount,
			Lanes:      make(map[int]store.ScheduleEntry, len(race.Lanes)),
		}
		for lane, entry := range race.Lanes {
			out.Lanes[lane] = store.ScheduleEntry{
				SchoolName:     entry.SchoolName,
				AdditionalInfo: entry.AdditionalInfo,
			}
		}
		sch.Races = append(sch.Races, out)
	}
	return sch
}

// raceLaneMapHash is store.ScheduleRace.LaneMapHash for an in-memory race - the
// value the clock stamped onto a RaceResult when results were committed, so the
// RD and FT trees can spot a result that no longer matches the live lane map
// (persona-plan.md 3c item 4).
func raceLaneMapHash(rd reader.RaceData) string {
	sr := store.ScheduleRace{
		RaceNumber: rd.RaceNumber,
		Lanes:      make(map[int]store.ScheduleEntry, len(rd.Lanes)),
	}
	for lane, e := range rd.Lanes {
		sr.Lanes[lane] = store.ScheduleEntry{SchoolName: e.SchoolName, AdditionalInfo: e.AdditionalInfo}
	}
	return sr.LaneMapHash()
}

// regattaDataFromSchedule rebuilds the in-memory RegattaData the race tree and
// clock render from. Result fields (place/split/time/approved) come back zero;
// they now live only in finish.json.
func regattaDataFromSchedule(sch *store.Schedule) *reader.RegattaData {
	rd := reader.NewRegattaData()
	rd.Name = sch.Name
	rd.Date = sch.Date
	rd.SourceInfo = reader.SourceInfo{
		Type: sch.Origin.Type,
		URI:  sch.Origin.URI,
		Hash: sch.Origin.Hash,
	}

	for _, race := range sch.Races {
		out := reader.RaceData{
			RaceNumber: race.RaceNumber,
			BoatClass:  race.BoatClass,
			FlightInfo: race.FlightInfo,
			BoatCount:  race.BoatCount,
			Lanes:      make(map[int]reader.RaceEntry, len(race.Lanes)),
		}
		for lane, entry := range race.Lanes {
			out.Lanes[lane] = reader.RaceEntry{
				SchoolName:     entry.SchoolName,
				AdditionalInfo: entry.AdditionalInfo,
			}
		}
		rd.Races = append(rd.Races, out)
	}
	return rd
}
