package reader

import (
	"os"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestFindRaceSheet(t *testing.T) {
	tests := []struct {
		name   string
		sheets []string
		want   string
	}{
		{
			name:   "results is the only sheet",
			sheets: []string{"Results"},
			want:   "Results",
		},
		{
			name:   "results sits at the regatta workbook position",
			sheets: []string{"Change Log", "Regatta Attributes", "Heat Sheet", "Results", "Referee Heat Sheet"},
			want:   "Results",
		},
		{
			name:   "heat sheet is not mistaken for the results sheet",
			sheets: []string{"Heat Sheet", "Referee Heat Sheet", "Results"},
			want:   "Results",
		},
		{
			name:   "results name differs in case and padding",
			sheets: []string{"Heat Sheet", " results "},
			want:   " results ",
		},
		{
			name:   "no results sheet falls back to the first sheet",
			sheets: []string{"Entries", "Lineups"},
			want:   "Entries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := excelize.NewFile()
			defer f.Close()

			for _, sheet := range tt.sheets {
				if _, err := f.NewSheet(sheet); err != nil {
					t.Fatalf("Failed to create sheet %q: %v", sheet, err)
				}
			}
			// NewFile() seeds a default "Sheet1" that is not part of the fixture
			if err := f.DeleteSheet("Sheet1"); err != nil {
				t.Fatalf("Failed to delete default sheet: %v", err)
			}

			got, err := findRaceSheet(f)
			if err != nil {
				t.Fatalf("findRaceSheet returned error: %v", err)
			}

			if got != tt.want {
				t.Errorf("findRaceSheet() = %q, want %q", got, tt.want)
			}
		})
	}
}

// raceFixture is the expected shape of one scheduled race - a race that carries
// lane data on the Results worksheet.
type raceFixture struct {
	number int
	boats  int
	class  string
	flight string
}

// workbookFixtures are the sample regattas under testdata/, one per supported
// Excel extension. Both files carry a "Results" worksheet; the reader tests
// exercise only that sheet's contents - the regatta title and date and the races
// that have lane data. The .xlsm additionally holds macros and formulas, which
// the reader ignores and which are not tested here.
var workbookFixtures = []struct {
	name        string // subtest label
	path        string
	regattaName string
	regattaDate string
	totalRaces  int
	scheduled   int
	races       []raceFixture // the scheduled races, in race-number order
}{
	{
		name:        "xlsx",
		path:        "testdata/Example Regatta Input Table.xlsx",
		regattaName: "Test Name",
		regattaDate: "March 13, 2025",
		totalRaces:  65,
		scheduled:   4,
		races: []raceFixture{
			{number: 1, boats: 4, flight: "M-1x"},
			{number: 2, boats: 5, flight: "W-JR-1x"},
			{number: 3, boats: 6, flight: "M-2x"},
			{number: 4, boats: 5, flight: "W-2x"},
		},
	},
	{
		name:        "xlsm",
		path:        "testdata/Example Heat Sheets and Results With Macros.xlsm",
		regattaName: "Charlie Brown Classic",
		regattaDate: "Saturday, May 01, 2027",
		totalRaces:  120,
		scheduled:   2,
		races: []raceFixture{
			{number: 1, boats: 4, class: "M-1x"},
			{number: 2, boats: 5, class: "M-Jr-4+", flight: "Heat 1"},
		},
	},
}

func mustReadWorkbook(t *testing.T, path string) *RegattaData {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("fixture not found: %s", path)
	}
	data, err := ReadExcelFile(path)
	if err != nil {
		t.Fatalf("ReadExcelFile(%s): %v", path, err)
	}
	if data == nil {
		t.Fatalf("ReadExcelFile(%s) returned nil", path)
	}
	return data
}

func raceByNumber(races []RaceData, n int) (RaceData, bool) {
	for _, r := range races {
		if r.RaceNumber == n {
			return r, true
		}
	}
	return RaceData{}, false
}

func assertSortedByRaceNumber(t *testing.T, races []RaceData) {
	t.Helper()
	for i := 1; i < len(races); i++ {
		if races[i].RaceNumber <= races[i-1].RaceNumber {
			t.Errorf("races not sorted: race %d follows race %d",
				races[i].RaceNumber, races[i-1].RaceNumber)
		}
	}
}

func TestReadExcelFile(t *testing.T) {
	for _, wb := range workbookFixtures {
		t.Run(wb.name, func(t *testing.T) {
			data := mustReadWorkbook(t, wb.path)

			if data.Name != wb.regattaName {
				t.Errorf("Name = %q, want %q", data.Name, wb.regattaName)
			}
			if data.Date != wb.regattaDate {
				t.Errorf("Date = %q, want %q", data.Date, wb.regattaDate)
			}
			if len(data.Races) != wb.totalRaces {
				t.Errorf("race count = %d, want %d", len(data.Races), wb.totalRaces)
			}
			if got := data.ScheduledRaces(); got != wb.scheduled {
				t.Errorf("ScheduledRaces() = %d, want %d", got, wb.scheduled)
			}
			assertSortedByRaceNumber(t, data.Races)
		})
	}
}

// TestReadExcelFile_ScheduledRaceContents is the focus of the fixture coverage:
// for each workbook, the races that carry lane data on the Results worksheet
// parse into the expected boat count, class, flight, and lane assignments.
func TestReadExcelFile_ScheduledRaceContents(t *testing.T) {
	for _, wb := range workbookFixtures {
		t.Run(wb.name, func(t *testing.T) {
			data := mustReadWorkbook(t, wb.path)

			if len(wb.races) != wb.scheduled {
				t.Fatalf("fixture lists %d scheduled races but scheduled = %d", len(wb.races), wb.scheduled)
			}

			for _, want := range wb.races {
				race, ok := raceByNumber(data.Races, want.number)
				if !ok {
					t.Fatalf("race %d not found", want.number)
				}
				if race.BoatCount != want.boats {
					t.Errorf("race %d: BoatCount = %d, want %d", want.number, race.BoatCount, want.boats)
				}
				if race.BoatClass != want.class {
					t.Errorf("race %d: BoatClass = %q, want %q", want.number, race.BoatClass, want.class)
				}
				if race.FlightInfo != want.flight {
					t.Errorf("race %d: FlightInfo = %q, want %q", want.number, race.FlightInfo, want.flight)
				}
				if len(race.Lanes) != want.boats {
					t.Errorf("race %d: %d lanes, want %d", want.number, len(race.Lanes), want.boats)
				}
				for lane, entry := range race.Lanes {
					if lane < 1 || lane > 6 {
						t.Errorf("race %d: invalid lane %d", want.number, lane)
					}
					if entry.SchoolName == "" {
						t.Errorf("race %d, lane %d: empty school name", want.number, lane)
					}
				}
				if !race.HasBoats() {
					t.Errorf("race %d: HasBoats() = false", want.number)
				}
			}
		})
	}
}

func TestReadExcelFile_RaceData(t *testing.T) {
	for _, wb := range workbookFixtures {
		t.Run(wb.name, func(t *testing.T) {
			data := mustReadWorkbook(t, wb.path)
			if len(data.Races) == 0 {
				t.Fatal("no races loaded")
			}

			first := data.Races[0]
			if first.RaceNumber == 0 {
				t.Error("race number should not be zero")
			}
			if first.RawData == nil {
				t.Fatal("RawData should not be nil")
			}
			if len(first.RawData) != 5 {
				t.Errorf("RawData rows = %d, want 5", len(first.RawData))
			}
			for i, row := range first.RawData {
				if len(row) != 7 {
					t.Errorf("RawData row %d has %d columns, want 7", i, len(row))
				}
			}
			if first.Lanes == nil {
				t.Fatal("Lanes map should not be nil")
			}
			if first.BoatCount != len(first.Lanes) {
				t.Errorf("BoatCount (%d) != lanes (%d)", first.BoatCount, len(first.Lanes))
			}
			for lane, entry := range first.Lanes {
				if lane < 1 || lane > 6 {
					t.Errorf("invalid lane number: %d", lane)
				}
				if entry.SchoolName == "" {
					t.Errorf("lane %d has empty school name", lane)
				}
			}
		})
	}
}

func TestReadExcelFile_MultipleRaces(t *testing.T) {
	for _, wb := range workbookFixtures {
		t.Run(wb.name, func(t *testing.T) {
			data := mustReadWorkbook(t, wb.path)
			if len(data.Races) < 2 {
				t.Fatalf("fixture %s has %d races, expected at least 2", wb.name, len(data.Races))
			}

			seen := make(map[int]bool)
			for _, race := range data.Races {
				if seen[race.RaceNumber] {
					t.Errorf("duplicate race number: %d", race.RaceNumber)
				}
				seen[race.RaceNumber] = true

				if race.Lanes == nil {
					t.Errorf("race %d: Lanes map should be initialized", race.RaceNumber)
				}
				if race.RawData == nil {
					t.Errorf("race %d: RawData should be initialized", race.RaceNumber)
				}
				if race.BoatCount > 0 && len(race.Lanes) == 0 {
					t.Errorf("race %d: BoatCount > 0 but no lanes", race.RaceNumber)
				}
				if race.BoatCount != len(race.Lanes) {
					t.Errorf("race %d: BoatCount (%d) != lanes (%d)",
						race.RaceNumber, race.BoatCount, len(race.Lanes))
				}
				for lane, entry := range race.Lanes {
					if lane < 1 || lane > 6 {
						t.Errorf("race %d: invalid lane number: %d", race.RaceNumber, lane)
					}
					if entry.SchoolName == "" {
						t.Errorf("race %d, lane %d: empty school name", race.RaceNumber, lane)
					}
				}
			}
		})
	}
}

func TestReadExcelFile_RaceTitle(t *testing.T) {
	for _, wb := range workbookFixtures {
		t.Run(wb.name, func(t *testing.T) {
			data := mustReadWorkbook(t, wb.path)
			for _, race := range data.Races {
				title := race.RaceTitle()
				if title == "" {
					t.Errorf("race %d has empty title", race.RaceNumber)
				}
				if len(title) < len("Race 1") {
					t.Errorf("race %d has suspiciously short title: %q", race.RaceNumber, title)
				}
			}
		})
	}
}

func TestReadExcelFile_ScheduledRaces(t *testing.T) {
	for _, wb := range workbookFixtures {
		t.Run(wb.name, func(t *testing.T) {
			data := mustReadWorkbook(t, wb.path)

			scheduled := data.ScheduledRaces()
			if scheduled < 0 {
				t.Error("scheduled race count should not be negative")
			}
			if scheduled > len(data.Races) {
				t.Errorf("scheduled (%d) exceeds total (%d)", scheduled, len(data.Races))
			}

			withBoats := 0
			for _, race := range data.Races {
				if len(race.Lanes) > 0 {
					withBoats++
				}
			}
			if scheduled != withBoats {
				t.Errorf("ScheduledRaces() = %d, counted %d races with boats", scheduled, withBoats)
			}
		})
	}
}

func TestReadExcelFile_SchoolNames(t *testing.T) {
	for _, wb := range workbookFixtures {
		t.Run(wb.name, func(t *testing.T) {
			data := mustReadWorkbook(t, wb.path)
			for _, race := range data.Races {
				if race.BoatCount == 0 {
					continue
				}
				names := race.SchoolNames()
				if len(names) != 6 {
					t.Errorf("race %d: %d school names, want 6", race.RaceNumber, len(names))
				}
				nonEmpty := 0
				for _, name := range names {
					if name != "" {
						nonEmpty++
					}
				}
				if nonEmpty != race.BoatCount {
					t.Errorf("race %d: BoatCount %d, %d non-empty school names",
						race.RaceNumber, race.BoatCount, nonEmpty)
				}
			}
		})
	}
}

func TestReadExcelFile_AdditionalInfos(t *testing.T) {
	for _, wb := range workbookFixtures {
		t.Run(wb.name, func(t *testing.T) {
			data := mustReadWorkbook(t, wb.path)
			for _, race := range data.Races {
				if race.BoatCount == 0 {
					continue
				}
				if infos := race.AdditionalInfos(); len(infos) != 6 {
					t.Errorf("race %d: %d additional infos, want 6", race.RaceNumber, len(infos))
				}
			}
		})
	}
}

func TestReadExcelFile_ApprovalWorkflow(t *testing.T) {
	for _, wb := range workbookFixtures {
		t.Run(wb.name, func(t *testing.T) {
			data := mustReadWorkbook(t, wb.path)
			if len(data.Races) == 0 {
				t.Fatal("no races loaded")
			}

			for _, race := range data.Races {
				if race.Approved {
					t.Errorf("race %d should start unapproved", race.RaceNumber)
				}
			}

			first := data.Races[0].RaceNumber
			data.ApproveRace(first)
			for _, race := range data.Races {
				want := race.RaceNumber == first
				if race.Approved != want {
					t.Errorf("race %d: Approved = %v, want %v", race.RaceNumber, race.Approved, want)
				}
			}
		})
	}
}

func TestReadExcelFile_SavedStatus(t *testing.T) {
	for _, wb := range workbookFixtures {
		t.Run(wb.name, func(t *testing.T) {
			data := mustReadWorkbook(t, wb.path)
			for _, race := range data.Races {
				if race.Saved {
					t.Errorf("race %d should start not saved", race.RaceNumber)
				}
			}
		})
	}
}

func TestReadExcelFile_EmptyLanes(t *testing.T) {
	for _, wb := range workbookFixtures {
		t.Run(wb.name, func(t *testing.T) {
			data := mustReadWorkbook(t, wb.path)
			for _, race := range data.Races {
				for lane, entry := range race.Lanes {
					if entry.SchoolName == "" {
						t.Errorf("race %d, lane %d: empty entry should not be in Lanes",
							race.RaceNumber, lane)
					}
				}
			}
		})
	}
}

func TestReadExcelFile_RawDataIntegrity(t *testing.T) {
	for _, wb := range workbookFixtures {
		t.Run(wb.name, func(t *testing.T) {
			data := mustReadWorkbook(t, wb.path)
			for _, race := range data.Races {
				if len(race.RawData) != 5 {
					t.Errorf("race %d: RawData rows = %d, want 5", race.RaceNumber, len(race.RawData))
					continue
				}
				for rowIdx, row := range race.RawData {
					if len(row) != 7 {
						t.Errorf("race %d, row %d: %d columns, want 7", race.RaceNumber, rowIdx, len(row))
					}
				}
				if race.BoatClass != race.RawData.getBoatClass() {
					t.Errorf("race %d: BoatClass %q != RawData %q",
						race.RaceNumber, race.BoatClass, race.RawData.getBoatClass())
				}
				if race.FlightInfo != race.RawData.getFlightInfo() {
					t.Errorf("race %d: FlightInfo %q != RawData %q",
						race.RaceNumber, race.FlightInfo, race.RawData.getFlightInfo())
				}
			}
		})
	}
}

func TestReadExcelFile_HasBoats(t *testing.T) {
	for _, wb := range workbookFixtures {
		t.Run(wb.name, func(t *testing.T) {
			data := mustReadWorkbook(t, wb.path)
			for _, race := range data.Races {
				if race.HasBoats() != (race.BoatCount > 0) {
					t.Errorf("race %d: HasBoats() = %v, BoatCount = %d",
						race.RaceNumber, race.HasBoats(), race.BoatCount)
				}
			}
		})
	}
}

func TestReadExcelFile_SortedRaces(t *testing.T) {
	for _, wb := range workbookFixtures {
		t.Run(wb.name, func(t *testing.T) {
			data := mustReadWorkbook(t, wb.path)
			sorted := data.SortedRaces()
			if len(sorted) != len(data.Races) {
				t.Errorf("SortedRaces length %d != Races length %d", len(sorted), len(data.Races))
			}
			assertSortedByRaceNumber(t, sorted)
		})
	}
}

func TestReadExcelFile_InvalidPath(t *testing.T) {
	if _, err := ReadExcelFile("nonexistent_file.xlsx"); err == nil {
		t.Error("expected an error for a nonexistent file, got nil")
	}
}

func TestReadExcelFile_InvalidFile(t *testing.T) {
	tmpFile := "testdata/invalid.txt"
	if err := os.WriteFile(tmpFile, []byte("not an excel file"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer os.Remove(tmpFile)

	if _, err := ReadExcelFile(tmpFile); err == nil {
		t.Error("expected an error for an invalid Excel file, got nil")
	}
}
