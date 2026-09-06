package reader

import (
	"testing"

	"github.com/comagnaw/regattaClock/internal/common"
)

func TestNewRegattaData(t *testing.T) {
	rd := NewRegattaData()

	if rd == nil {
		t.Fatal("NewRegattaData returned nil")
	}

	if rd.Name != "" {
		t.Errorf("Expected empty Name, got %q", rd.Name)
	}

	if rd.Date != "" {
		t.Errorf("Expected empty Date, got %q", rd.Date)
	}

	if rd.Races == nil {
		t.Error("Expected Races to be initialized, got nil")
	}

	if len(rd.Races) != 0 {
		t.Errorf("Expected empty Races slice, got length %d", len(rd.Races))
	}
}

func TestRegattaData_ApproveRace(t *testing.T) {
	rd := NewRegattaData()
	rd.Races = []RaceData{
		{RaceNumber: 1, Approved: false},
		{RaceNumber: 2, Approved: false},
		{RaceNumber: 3, Approved: false},
	}

	// Approve race 2
	rd.ApproveRace(2)

	if !rd.Races[1].Approved {
		t.Error("Race 2 should be approved")
	}

	if rd.Races[0].Approved {
		t.Error("Race 1 should not be approved")
	}

	if rd.Races[2].Approved {
		t.Error("Race 3 should not be approved")
	}
}

func TestRegattaData_ApproveRace_NonExistent(t *testing.T) {
	rd := NewRegattaData()
	rd.Races = []RaceData{
		{RaceNumber: 1, Approved: false},
		{RaceNumber: 2, Approved: false},
	}

	// Try to approve non-existent race
	rd.ApproveRace(99)

	// Verify no races were approved
	for i, race := range rd.Races {
		if race.Approved {
			t.Errorf("Race %d should not be approved", i+1)
		}
	}
}

func TestRegattaData_ScheduledRaces(t *testing.T) {
	tests := []struct {
		name     string
		races    []RaceData
		expected int
	}{
		{
			name:     "no races",
			races:    []RaceData{},
			expected: 0,
		},
		{
			name: "all races have boats",
			races: []RaceData{
				{RaceNumber: 1, Lanes: map[int]RaceEntry{1: {SchoolName: "School A"}}},
				{RaceNumber: 2, Lanes: map[int]RaceEntry{1: {SchoolName: "School B"}}},
				{RaceNumber: 3, Lanes: map[int]RaceEntry{1: {SchoolName: "School C"}}},
			},
			expected: 3,
		},
		{
			name: "some races have no boats",
			races: []RaceData{
				{RaceNumber: 1, Lanes: map[int]RaceEntry{1: {SchoolName: "School A"}}},
				{RaceNumber: 2, Lanes: map[int]RaceEntry{}},
				{RaceNumber: 3, Lanes: map[int]RaceEntry{1: {SchoolName: "School C"}}},
			},
			expected: 2,
		},
		{
			name: "no races have boats",
			races: []RaceData{
				{RaceNumber: 1, Lanes: map[int]RaceEntry{}},
				{RaceNumber: 2, Lanes: map[int]RaceEntry{}},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rd := NewRegattaData()
			rd.Races = tt.races
			result := rd.ScheduledRaces()
			if result != tt.expected {
				t.Errorf("Expected %d scheduled races, got %d", tt.expected, result)
			}
		})
	}
}

func TestRegattaData_SortedRaces(t *testing.T) {
	rd := NewRegattaData()
	rd.Races = []RaceData{
		{RaceNumber: 5},
		{RaceNumber: 1},
		{RaceNumber: 3},
		{RaceNumber: 2},
		{RaceNumber: 4},
	}

	sorted := rd.SortedRaces()

	if len(sorted) != 5 {
		t.Fatalf("Expected 5 races, got %d", len(sorted))
	}

	// Check if sorted
	for i := 0; i < len(sorted); i++ {
		if sorted[i].RaceNumber != i+1 {
			t.Errorf("Expected race number %d at position %d, got %d", i+1, i, sorted[i].RaceNumber)
		}
	}

	// Verify original slice is unchanged
	if rd.Races[0].RaceNumber != 5 {
		t.Error("Original slice should not be modified")
	}
}

func TestNewRaceData(t *testing.T) {
	raceNum := 5
	rd := newRaceData(raceNum)

	if rd.RaceNumber != raceNum {
		t.Errorf("Expected race number %d, got %d", raceNum, rd.RaceNumber)
	}

	if rd.Lanes == nil {
		t.Error("Lanes map should be initialized")
	}

	if len(rd.Lanes) != 0 {
		t.Errorf("Expected empty Lanes map, got length %d", len(rd.Lanes))
	}

	if rd.RawData == nil {
		t.Fatal("RawData should be initialized")
	}

	if len(rd.RawData) != 5 {
		t.Fatalf("Expected 5 rows in RawData, got %d", len(rd.RawData))
	}

	for i, row := range rd.RawData {
		if len(row) != 7 {
			t.Errorf("Expected 7 columns in row %d, got %d", i, len(row))
		}
	}
}

func TestRaceData_RaceTitle(t *testing.T) {
	tests := []struct {
		name     string
		race     RaceData
		expected string
	}{
		{
			name: "basic race title",
			race: RaceData{
				RaceNumber: 1,
				BoatCount:  4,
				BoatClass:  common.EmptyString,
				FlightInfo: common.EmptyString,
			},
			expected: "Race 1",
		},
		{
			name: "race with boat class",
			race: RaceData{
				RaceNumber: 2,
				BoatCount:  6,
				BoatClass:  "Varsity 8",
				FlightInfo: common.EmptyString,
			},
			expected: "Race 2 - Varsity 8",
		},
		{
			name: "race with flight info",
			race: RaceData{
				RaceNumber: 3,
				BoatCount:  5,
				BoatClass:  common.EmptyString,
				FlightInfo: "Heat 1",
			},
			expected: "Race 3 - Heat 1",
		},
		{
			name: "race with both class and flight",
			race: RaceData{
				RaceNumber: 4,
				BoatCount:  4,
				BoatClass:  "JV 4",
				FlightInfo: "Final",
			},
			expected: "Race 4 - JV 4 - Final",
		},
		{
			name: "race with zero boats",
			race: RaceData{
				RaceNumber: 5,
				BoatCount:  0,
				BoatClass:  "Novice 8",
				FlightInfo: "Semi-Final",
			},
			expected: "Race 5 - Novice 8 - Semi-Final",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.race.RaceTitle()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestRaceData_HasBoats(t *testing.T) {
	tests := []struct {
		name      string
		boatCount int
		expected  bool
	}{
		{"zero boats", 0, false},
		{"one boat", 1, true},
		{"multiple boats", 6, true},
		{"negative boats", -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rd := RaceData{BoatCount: tt.boatCount}
			result := rd.HasBoats()
			if result != tt.expected {
				t.Errorf("Expected HasBoats() to be %v for boat count %d", tt.expected, tt.boatCount)
			}
		})
	}
}

func TestRaceData_SchoolNames(t *testing.T) {
	rd := RaceData{
		Lanes: map[int]RaceEntry{
			1: {SchoolName: "School A"},
			2: {SchoolName: "School B"},
			4: {SchoolName: "School D"},
			6: {SchoolName: "School F"},
		},
	}

	names := rd.SchoolNames()

	if len(names) != 6 {
		t.Fatalf("Expected 6 school names, got %d", len(names))
	}

	expectedNames := []string{"School A", "School B", "", "School D", "", "School F"}
	for i, expected := range expectedNames {
		if names[i] != expected {
			t.Errorf("Expected school name at position %d to be %q, got %q", i, expected, names[i])
		}
	}
}

func TestRaceData_AdditionalInfos(t *testing.T) {
	rd := RaceData{
		Lanes: map[int]RaceEntry{
			1: {AdditionalInfo: "Info A"},
			3: {AdditionalInfo: "Info C"},
			5: {AdditionalInfo: "Info E"},
		},
	}

	infos := rd.AdditionalInfos()

	if len(infos) != 6 {
		t.Fatalf("Expected 6 additional infos, got %d", len(infos))
	}

	expectedInfos := []string{"Info A", "", "Info C", "", "Info E", ""}
	for i, expected := range expectedInfos {
		if infos[i] != expected {
			t.Errorf("Expected additional info at position %d to be %q, got %q", i, expected, infos[i])
		}
	}
}

func TestRawData_getBoatClass(t *testing.T) {
	rawData := RawData{
		{"Varsity 8", "col1", "col2", "col3", "col4", "col5", "col6"},
		{"row1", "", "", "", "", "", ""},
		{"row2", "", "", "", "", "", ""},
		{"row3", "", "", "", "", "", ""},
		{"row4", "", "", "", "", "", ""},
	}

	boatClass := rawData.getBoatClass()
	expected := "Varsity 8"

	if boatClass != expected {
		t.Errorf("Expected boat class %q, got %q", expected, boatClass)
	}
}

func TestRawData_getFlightInfo(t *testing.T) {
	rawData := RawData{
		{"row0", "", "", "", "", "", ""},
		{"Heat 1", "col1", "col2", "col3", "col4", "col5", "col6"},
		{"row2", "", "", "", "", "", ""},
		{"row3", "", "", "", "", "", ""},
		{"row4", "", "", "", "", "", ""},
	}

	flightInfo := rawData.getFlightInfo()
	expected := "Heat 1"

	if flightInfo != expected {
		t.Errorf("Expected flight info %q, got %q", expected, flightInfo)
	}
}

func TestRawData_getRaceEntryByLane(t *testing.T) {
	rawData := RawData{
		{"Class", "School 1", "School 2", "School 3", "School 4", "School 5", "School 6"},
		{"Flight", "Info 1", "Info 2", "Info 3", "Info 4", "Info 5", "Info 6"},
		{"Place", "1", "2", "3", "4", "5", "6"},
		{"Split", "0.0", "0.5", "1.0", "1.5", "2.0", "2.5"},
		{"Time", "6:00.0", "6:00.5", "6:01.0", "6:01.5", "6:02.0", "6:02.5"},
	}

	tests := []struct {
		lane     int
		expected RaceEntry
	}{
		{
			lane: 1,
			expected: RaceEntry{
				SchoolName:     "School 1",
				AdditionalInfo: "Info 1",
				Place:          "1",
				Split:          "0.0",
				Time:           "6:00.0",
			},
		},
		{
			lane: 3,
			expected: RaceEntry{
				SchoolName:     "School 3",
				AdditionalInfo: "Info 3",
				Place:          "3",
				Split:          "1.0",
				Time:           "6:01.0",
			},
		},
		{
			lane: 6,
			expected: RaceEntry{
				SchoolName:     "School 6",
				AdditionalInfo: "Info 6",
				Place:          "6",
				Split:          "2.5",
				Time:           "6:02.5",
			},
		},
	}

	for _, tt := range tests {
		t.Run("Lane_"+string(rune(tt.lane+'0')), func(t *testing.T) {
			entry := rawData.getRaceEntryByLane(tt.lane)

			if entry.SchoolName != tt.expected.SchoolName {
				t.Errorf("Expected SchoolName %q, got %q", tt.expected.SchoolName, entry.SchoolName)
			}
			if entry.AdditionalInfo != tt.expected.AdditionalInfo {
				t.Errorf("Expected AdditionalInfo %q, got %q", tt.expected.AdditionalInfo, entry.AdditionalInfo)
			}
			if entry.Place != tt.expected.Place {
				t.Errorf("Expected Place %q, got %q", tt.expected.Place, entry.Place)
			}
			if entry.Split != tt.expected.Split {
				t.Errorf("Expected Split %q, got %q", tt.expected.Split, entry.Split)
			}
			if entry.Time != tt.expected.Time {
				t.Errorf("Expected Time %q, got %q", tt.expected.Time, entry.Time)
			}
		})
	}
}

func TestRaceEntry_isEmptyEntry(t *testing.T) {
	tests := []struct {
		name     string
		entry    RaceEntry
		expected bool
	}{
		{
			name: "completely empty",
			entry: RaceEntry{
				SchoolName:     common.EmptyString,
				AdditionalInfo: common.EmptyString,
				Place:          common.EmptyString,
				Split:          common.EmptyString,
				Time:           common.EmptyString,
			},
			expected: true,
		},
		{
			name: "has school name",
			entry: RaceEntry{
				SchoolName:     "School A",
				AdditionalInfo: common.EmptyString,
				Place:          common.EmptyString,
				Split:          common.EmptyString,
				Time:           common.EmptyString,
			},
			expected: false,
		},
		{
			name: "has only additional info",
			entry: RaceEntry{
				SchoolName:     common.EmptyString,
				AdditionalInfo: "Some info",
				Place:          common.EmptyString,
				Split:          common.EmptyString,
				Time:           common.EmptyString,
			},
			expected: true,
		},
		{
			name: "has all fields",
			entry: RaceEntry{
				SchoolName:     "School B",
				AdditionalInfo: "Info",
				Place:          "1",
				Split:          "0.0",
				Time:           "6:00.0",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.entry.isEmptyEntry()
			if result != tt.expected {
				t.Errorf("Expected isEmptyEntry() to be %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetRowNumber(t *testing.T) {
	tests := []struct {
		cellRef  string
		expected int
	}{
		{"A1", 1},
		{"B10", 10},
		{"C100", 100},
		{"AA25", 25},
		{"Z999", 999},
		{"A5", 5},
		{"ZZ1234", 1234},
	}

	for _, tt := range tests {
		t.Run(tt.cellRef, func(t *testing.T) {
			result := getRowNumber(tt.cellRef)
			if result != tt.expected {
				t.Errorf("Expected row number %d for %q, got %d", tt.expected, tt.cellRef, result)
			}
		})
	}
}
