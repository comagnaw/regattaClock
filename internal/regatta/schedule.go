package regatta

import (
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/reader"
)

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
