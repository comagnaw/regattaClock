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

	if len(rd.RawData) != 3 {
		t.Fatalf("Expected 3 rows in RawData, got %d", len(rd.RawData))
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

func TestRaceData_RaceDetail(t *testing.T) {
	tests := []struct {
		name     string
		race     RaceData
		expected string
	}{
		{
			name:     "no boat class or flight info",
			race:     RaceData{RaceNumber: 1, BoatClass: common.EmptyString, FlightInfo: common.EmptyString},
			expected: "",
		},
		{
			name:     "boat class only",
			race:     RaceData{RaceNumber: 2, BoatClass: "Varsity 8", FlightInfo: common.EmptyString},
			expected: "Varsity 8",
		},
		{
			name:     "flight info only",
			race:     RaceData{RaceNumber: 3, BoatClass: common.EmptyString, FlightInfo: "Heat 1"},
			expected: "Heat 1",
		},
		{
			name:     "both boat class and flight info",
			race:     RaceData{RaceNumber: 4, BoatClass: "JV 4", FlightInfo: "Final"},
			expected: "JV 4 - Final",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.race.RaceDetail()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestRaceData_ScheduledTimeDisplay(t *testing.T) {
	tests := []struct {
		name     string
		race     RaceData
		expected string
	}{
		{
			name:     "scheduled time set",
			race:     RaceData{ScheduledTime: "09:00 AM"},
			expected: "09:00 AM",
		},
		{
			name:     "24h scheduled time set",
			race:     RaceData{ScheduledTime: "14:30"},
			expected: "14:30",
		},
		{
			name:     "no scheduled time",
			race:     RaceData{ScheduledTime: common.EmptyString},
			expected: common.NoStartTimeText,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.race.ScheduledTimeDisplay()
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
	}

	flightInfo := rawData.getFlightInfo()
	expected := "Heat 1"

	if flightInfo != expected {
		t.Errorf("Expected flight info %q, got %q", expected, flightInfo)
	}
}

// TestRawData_getRaceEntryByLane - the Heat Sheet worksheet's 3-row block
// carries no result data; row 2 (a rower's last name, for 1x/2x boats) is
// captured in RawData but has no RaceEntry field.
func TestRawData_getRaceEntryByLane(t *testing.T) {
	rawData := RawData{
		{"Class", "School 1", "School 2", "School 3", "School 4", "School 5", "School 6"},
		{"Flight", "Info 1", "Info 2", "Info 3", "Info 4", "Info 5", "Info 6"},
		{"Rower", "Rower 1", "Rower 2", "Rower 3", "Rower 4", "Rower 5", "Rower 6"},
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
			},
		},
		{
			lane: 3,
			expected: RaceEntry{
				SchoolName:     "School 3",
				AdditionalInfo: "Info 3",
			},
		},
		{
			lane: 6,
			expected: RaceEntry{
				SchoolName:     "School 6",
				AdditionalInfo: "Info 6",
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
		})
	}
}

// TestDetectStatus - the Excel importer's only scratch-recognition rule:
// exact-match against "scratched"/"scratch"/"scr" via a lowercase
// normalization (so any case combination matches), never a substring
// match, so a school name or boat-class code that happens to contain "scr"
// is never mistaken for a scratch.
func TestDetectStatus(t *testing.T) {
	tests := []struct {
		name           string
		additionalInfo string
		want           RaceEntryStatus
	}{
		{"scratched, exact", "SCRATCHED", StatusScratched},
		{"scratch, exact", "SCRATCH", StatusScratched},
		{"scr, exact", "SCR", StatusScratched},
		{"lowercase", "scratched", StatusScratched},
		{"mixed case, scratch", "Scratch", StatusScratched},
		{"padded", "  SCRATCHED  ", StatusScratched},
		{"mixed case, scr", "Scr", StatusScratched},
		{"empty", "", StatusOK},
		{"unrelated note", "A", StatusOK},
		{"alternate class, not a scratch", "M-Jr-1x", StatusOK},
		{"substring, not a scratch", "Descriptive text", StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detectStatus(tt.additionalInfo); got != tt.want {
				t.Errorf("detectStatus(%q) = %q, want %q", tt.additionalInfo, got, tt.want)
			}
		})
	}
}

// TestRawData_getRaceEntryByLane_Scratched - AdditionalInfo carries the raw
// "SCRATCHED" text unchanged; Status is the derived, structured signal
// alongside it. SchoolName survives - a scratch is not an empty entry.
func TestRawData_getRaceEntryByLane_Scratched(t *testing.T) {
	rawData := RawData{
		{"M-Jr-4+", "School 1", "School 2", "", "", "", ""},
		{"Heat 2", "", "SCRATCHED", "", "", "", ""},
		{"3 to Advance", "", "", "", "", "", ""},
	}

	entry := rawData.getRaceEntryByLane(2)
	if entry.SchoolName != "School 2" {
		t.Errorf("SchoolName = %q, want %q (a scratch keeps its school)", entry.SchoolName, "School 2")
	}
	if entry.AdditionalInfo != "SCRATCHED" {
		t.Errorf("AdditionalInfo = %q, want the raw %q text preserved", entry.AdditionalInfo, "SCRATCHED")
	}
	if entry.Status != StatusScratched {
		t.Errorf("Status = %q, want %q", entry.Status, StatusScratched)
	}
	if entry.isEmptyEntry() {
		t.Error("a scratched entry with a preserved SchoolName must not be considered empty")
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
			},
			expected: true,
		},
		{
			name: "has school name",
			entry: RaceEntry{
				SchoolName:     "School A",
				AdditionalInfo: common.EmptyString,
			},
			expected: false,
		},
		{
			name: "has only additional info",
			entry: RaceEntry{
				SchoolName:     common.EmptyString,
				AdditionalInfo: "Some info",
			},
			expected: true,
		},
		{
			name: "has all fields",
			entry: RaceEntry{
				SchoolName:     "School B",
				AdditionalInfo: "Info",
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
