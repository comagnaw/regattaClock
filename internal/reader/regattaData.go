package reader

import (
	"fmt"
	"iter"
	"sort"

	"github.com/comagnaw/regattaClock/internal/common"
)

// sourceData - interface that needs to be satisfied in order to call the load function
type sourceData interface {
	// setNameAndDate - from source data, load the regatta Name and Date into RegattaData
	setNameAndDate()

	// loadRaces - from source data, load RaceData into RegattaData
	loadRaces()
}

// load - used to load RegattaData from implmented sourceData
func load(b sourceData) {
	b.setNameAndDate()
	b.loadRaces()
}

// RegattaData - represents the structure of the regatta data we'll read from Excel
type RegattaData struct {
	// Name - title of Regatta
	Name string

	// Date - date of Regatta
	Date string

	// Races - slice of RaceData
	Races []RaceData
}

// NewRegattaData - return pointer to initilized RegattaData
func NewRegattaData() *RegattaData {
	return &RegattaData{
		Races: make([]RaceData, 0),
	}
}

func (r *RegattaData) ApproveRace(raceNumber int) {
	for i := range r.Races {
		if r.Races[i].RaceNumber == raceNumber {
			r.Races[i].Approved = true
			break
		}
	}
}

// ScheduleRaces - count number of Races with more 1 or more Lanes with boats return integer
func (r *RegattaData) ScheduledRaces() int {
	scheduledRaces := 0
	for _, race := range r.Races {
		if len(race.Lanes) > 0 {
			scheduledRaces++
		}
	}
	return scheduledRaces
}

// SortedRaces - return RegttaData as sorted RaceData based on RaceNumber
func (r *RegattaData) SortedRaces() []RaceData {
	races := make([]RaceData, len(r.Races))
	copy(races, r.Races)
	sort.Slice(races, func(i, j int) bool {
		return races[i].RaceNumber < races[j].RaceNumber
	})
	return races
}

// RaceData represents the data for a single race
type RaceData struct {

	// RaceNumber - integer value of race number
	RaceNumber int

	// Lanes -  lane number (1-6) as key for each RaceEntry
	Lanes map[int]RaceEntry

	// Rawdata - can be used to store raw table of sourceData for a race
	RawData

	// Saved - whether the race data has been saved to disk
	Saved bool

	// Approved - has the race data been approved by referee
	Approved bool

	// BoatCount - how many boats are in the race
	BoatCount int

	// BoatClass - what class of boat is in this race
	BoatClass string

	// FlightInfo - what heat is this race in, if any.
	FlightInfo string
}

// RawData - 5 row x 7 column table which represents sourceData for a race
type RawData [][]string

// getBoatClass - position 0x0 of table holds BoatClass value
func (r RawData) getBoatClass() string {
	return r[0][0]
}

// getFlightInfo - position 1x0 of table holds FlightInfo value
func (r RawData) getFlightInfo() string {
	return r[1][0]
}

// getRaceEntryByLane - for given column (lane), pull raceEntry attributes from the repsective row
func (r RawData) getRaceEntryByLane(lane int) RaceEntry {
	raceEntry := RaceEntry{}

	raceEntry.SchoolName = r[0][lane]
	raceEntry.AdditionalInfo = r[1][lane]
	raceEntry.Place = r[2][lane]
	raceEntry.Split = r[3][lane]
	raceEntry.Time = r[4][lane]
	return raceEntry
}

// newRaceData - init RaceData with provided raceNum
func newRaceData(raceNum int) RaceData {
	// make 5 rows of empty entries
	rawData := make([][]string, 5)
	for i := range rawData {
		// make 7 columns of empty entries
		rawData[i] = make([]string, 7)
	}
	return RaceData{
		RaceNumber: raceNum,
		Lanes:      make(map[int]RaceEntry),
		RawData:    rawData,
	}
}

// RaceTitle create the race title text
func (r *RaceData) RaceTitle() string {
	titleText := fmt.Sprintf("Race %d - (%d Boats)", r.RaceNumber, r.BoatCount)
	if r.BoatClass != common.EmptyString {
		titleText = fmt.Sprintf("%s - %s", titleText, r.BoatClass)
	}
	if r.FlightInfo != common.EmptyString {
		titleText = fmt.Sprintf("%s - %s", titleText, r.FlightInfo)
	}
	return titleText
}

// HasBoats - returns true if row from RaceData has boats
func (r *RaceData) HasBoats() bool {
	return r.BoatCount > 0
}

// OrderedLanes returns an iterator that yields lane numbers and entries in ascending order by lane number
func (r *RaceData) OrderedLanes() iter.Seq2[int, RaceEntry] {
	return func(yield func(int, RaceEntry) bool) {
		keys := make([]int, 0, len(r.Lanes))
		for k := range r.Lanes {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		for _, k := range keys {
			if !yield(k, r.Lanes[k]) {
				return
			}
		}
	}
}

func (r *RaceData) SchoolNames() []string {
	schoolsByLane := []string{}
	lanes := []int{1, 2, 3, 4, 5, 6}
	for _, laneNum := range lanes {
		schoolsByLane = append(schoolsByLane, r.Lanes[laneNum].SchoolName)
	}
	return schoolsByLane
}

func (r *RaceData) AdditionalInfos() []string {
	additionalInfo := []string{}
	lanes := []int{1, 2, 3, 4, 5, 6}
	for _, laneNum := range lanes {
		additionalInfo = append(additionalInfo, r.Lanes[laneNum].AdditionalInfo)
	}
	return additionalInfo
}

// RaceEntry represents a single entry in a race
type RaceEntry struct {

	// SchoolName - what school is represented in this lane
	SchoolName string

	// AdditionalInfo - this may represent rower name or A vs B boat for school with multiple boats in race
	AdditionalInfo string

	// Place - what place did this boat finish in
	Place string

	// Split - what is the difference in time betwen this boat and the first place boat
	Split string

	// Time - what is the toal time for this boat to finish the race
	Time string
}

func (r RaceEntry) isEmptyEntry() bool {
	return r.SchoolName == common.EmptyString
}
