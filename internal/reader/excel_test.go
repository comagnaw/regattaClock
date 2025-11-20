package reader

import (
	"os"
	"testing"
)

func TestReadExcelFile(t *testing.T) {
	testFile := "testdata/Example Regatta Input Table.xlsx"

	// Verify test file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Fatalf("Test file not found: %s", testFile)
	}

	data, err := ReadExcelFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read Excel file: %v", err)
	}

	if data == nil {
		t.Fatal("ReadExcelFile returned nil data")
	}

	// Verify regatta name and date were loaded
	if data.Name == "" {
		t.Error("Regatta name should not be empty")
	}

	if data.Date == "" {
		t.Error("Regatta date should not be empty")
	}

	// Verify races were loaded
	if len(data.Races) == 0 {
		t.Fatal("No races were loaded")
	}

	t.Logf("Loaded regatta: %s on %s", data.Name, data.Date)
	t.Logf("Number of races: %d", len(data.Races))

	// Verify races are sorted by race number
	for i := 1; i < len(data.Races); i++ {
		if data.Races[i].RaceNumber <= data.Races[i-1].RaceNumber {
			t.Errorf("Races are not sorted: Race %d comes after Race %d",
				data.Races[i].RaceNumber, data.Races[i-1].RaceNumber)
		}
	}
}

func TestReadExcelFile_InvalidPath(t *testing.T) {
	_, err := ReadExcelFile("nonexistent_file.xlsx")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

func TestReadExcelFile_InvalidFile(t *testing.T) {
	// Create a temporary non-Excel file
	tmpFile := "testdata/invalid.txt"
	err := os.WriteFile(tmpFile, []byte("not an excel file"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(tmpFile)

	_, err = ReadExcelFile(tmpFile)
	if err == nil {
		t.Error("Expected error for invalid Excel file, got nil")
	}
}

func TestReadExcelFile_RaceData(t *testing.T) {
	testFile := "testdata/Example Regatta Input Table.xlsx"

	data, err := ReadExcelFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read Excel file: %v", err)
	}

	if len(data.Races) == 0 {
		t.Fatal("No races loaded")
	}

	// Test first race in detail
	firstRace := data.Races[0]

	// Verify basic race properties
	if firstRace.RaceNumber == 0 {
		t.Error("Race number should not be zero")
	}

	// Verify RawData structure
	if firstRace.RawData == nil {
		t.Fatal("RawData should not be nil")
	}

	if len(firstRace.RawData) != 5 {
		t.Errorf("Expected 5 rows in RawData, got %d", len(firstRace.RawData))
	}

	for i, row := range firstRace.RawData {
		if len(row) != 7 {
			t.Errorf("Expected 7 columns in row %d, got %d", i, len(row))
		}
	}

	// Verify lanes map is populated correctly
	if firstRace.Lanes == nil {
		t.Fatal("Lanes map should not be nil")
	}

	t.Logf("First race: Race %d with %d boats", firstRace.RaceNumber, firstRace.BoatCount)
	if firstRace.BoatClass != "" {
		t.Logf("  Boat class: %s", firstRace.BoatClass)
	}
	if firstRace.FlightInfo != "" {
		t.Logf("  Flight info: %s", firstRace.FlightInfo)
	}

	// Verify boat count matches actual lanes
	actualBoats := len(firstRace.Lanes)
	if firstRace.BoatCount != actualBoats {
		t.Errorf("BoatCount (%d) doesn't match actual lanes (%d)", firstRace.BoatCount, actualBoats)
	}

	// Verify lane entries
	for lane, entry := range firstRace.Lanes {
		if lane < 1 || lane > 6 {
			t.Errorf("Invalid lane number: %d", lane)
		}

		if entry.SchoolName == "" {
			t.Errorf("Lane %d has empty school name", lane)
		}

		t.Logf("  Lane %d: %s", lane, entry.SchoolName)
	}
}

func TestReadExcelFile_MultipleRaces(t *testing.T) {
	testFile := "testdata/Example Regatta Input Table.xlsx"

	data, err := ReadExcelFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read Excel file: %v", err)
	}

	// Test that multiple races can be read
	if len(data.Races) < 2 {
		t.Skip("Test file needs at least 2 races for this test")
	}

	// Verify each race has unique race number
	raceNumbers := make(map[int]bool)
	for _, race := range data.Races {
		if raceNumbers[race.RaceNumber] {
			t.Errorf("Duplicate race number found: %d", race.RaceNumber)
		}
		raceNumbers[race.RaceNumber] = true
	}

	// Test each race
	for _, race := range data.Races {
		t.Run("Race_"+string(rune(race.RaceNumber+'0')), func(t *testing.T) {
			// Verify basic properties
			if race.Lanes == nil {
				t.Error("Lanes map should be initialized")
			}

			if race.RawData == nil {
				t.Error("RawData should be initialized")
			}

			// If race has boats, verify lanes are valid
			if race.BoatCount > 0 {
				if len(race.Lanes) == 0 {
					t.Error("Race has BoatCount > 0 but no lanes")
				}

				for lane, entry := range race.Lanes {
					if lane < 1 || lane > 6 {
						t.Errorf("Invalid lane number: %d", lane)
					}

					if entry.SchoolName == "" {
						t.Errorf("Lane %d has empty school name", lane)
					}
				}
			}

			// Verify BoatCount consistency
			if race.BoatCount != len(race.Lanes) {
				t.Errorf("BoatCount (%d) doesn't match lanes count (%d)",
					race.BoatCount, len(race.Lanes))
			}
		})
	}
}

func TestReadExcelFile_RaceTitle(t *testing.T) {
	testFile := "testdata/Example Regatta Input Table.xlsx"

	data, err := ReadExcelFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read Excel file: %v", err)
	}

	for _, race := range data.Races {
		title := race.RaceTitle()
		if title == "" {
			t.Errorf("Race %d has empty title", race.RaceNumber)
		}

		// Title should at least contain the race number
		if len(title) < 6 {
			t.Errorf("Race %d has suspiciously short title: %q", race.RaceNumber, title)
		}

		t.Logf("Race %d title: %s", race.RaceNumber, title)
	}
}

func TestReadExcelFile_ScheduledRaces(t *testing.T) {
	testFile := "testdata/Example Regatta Input Table.xlsx"

	data, err := ReadExcelFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read Excel file: %v", err)
	}

	scheduled := data.ScheduledRaces()
	total := len(data.Races)

	if scheduled < 0 {
		t.Error("Scheduled races count should not be negative")
	}

	if scheduled > total {
		t.Errorf("Scheduled races (%d) exceeds total races (%d)", scheduled, total)
	}

	t.Logf("Scheduled races: %d out of %d total", scheduled, total)

	// Verify scheduled count matches races with boats
	racesWithBoats := 0
	for _, race := range data.Races {
		if len(race.Lanes) > 0 {
			racesWithBoats++
		}
	}

	if scheduled != racesWithBoats {
		t.Errorf("ScheduledRaces() returned %d but counted %d races with boats",
			scheduled, racesWithBoats)
	}
}

func TestReadExcelFile_SchoolNames(t *testing.T) {
	testFile := "testdata/Example Regatta Input Table.xlsx"

	data, err := ReadExcelFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read Excel file: %v", err)
	}

	for _, race := range data.Races {
		if race.BoatCount == 0 {
			continue
		}

		names := race.SchoolNames()
		if len(names) != 6 {
			t.Errorf("Race %d: Expected 6 school names, got %d", race.RaceNumber, len(names))
		}

		// Count non-empty names
		nonEmpty := 0
		for _, name := range names {
			if name != "" {
				nonEmpty++
			}
		}

		if nonEmpty != race.BoatCount {
			t.Errorf("Race %d: BoatCount is %d but found %d non-empty school names",
				race.RaceNumber, race.BoatCount, nonEmpty)
		}
	}
}

func TestReadExcelFile_AdditionalInfos(t *testing.T) {
	testFile := "testdata/Example Regatta Input Table.xlsx"

	data, err := ReadExcelFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read Excel file: %v", err)
	}

	for _, race := range data.Races {
		if race.BoatCount == 0 {
			continue
		}

		infos := race.AdditionalInfos()
		if len(infos) != 6 {
			t.Errorf("Race %d: Expected 6 additional infos, got %d", race.RaceNumber, len(infos))
		}
	}
}

func TestReadExcelFile_ApprovalWorkflow(t *testing.T) {
	testFile := "testdata/Example Regatta Input Table.xlsx"

	data, err := ReadExcelFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read Excel file: %v", err)
	}

	if len(data.Races) == 0 {
		t.Fatal("No races loaded")
	}

	// Verify all races start as unapproved
	for _, race := range data.Races {
		if race.Approved {
			t.Errorf("Race %d should start as unapproved", race.RaceNumber)
		}
	}

	// Test approval
	firstRaceNum := data.Races[0].RaceNumber
	data.ApproveRace(firstRaceNum)

	// Verify only the first race is approved
	for _, race := range data.Races {
		if race.RaceNumber == firstRaceNum {
			if !race.Approved {
				t.Errorf("Race %d should be approved", firstRaceNum)
			}
		} else {
			if race.Approved {
				t.Errorf("Race %d should not be approved", race.RaceNumber)
			}
		}
	}
}

func TestReadExcelFile_SavedStatus(t *testing.T) {
	testFile := "testdata/Example Regatta Input Table.xlsx"

	data, err := ReadExcelFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read Excel file: %v", err)
	}

	// Verify all races start as not saved
	for _, race := range data.Races {
		if race.Saved {
			t.Errorf("Race %d should start as not saved", race.RaceNumber)
		}
	}
}

func TestReadExcelFile_EmptyLanes(t *testing.T) {
	testFile := "testdata/Example Regatta Input Table.xlsx"

	data, err := ReadExcelFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read Excel file: %v", err)
	}

	// Verify that empty lanes are not included
	for _, race := range data.Races {
		for lane, entry := range race.Lanes {
			if entry.SchoolName == "" {
				t.Errorf("Race %d, Lane %d: Empty entry should not be in Lanes map",
					race.RaceNumber, lane)
			}
		}
	}
}

func TestReadExcelFile_RawDataIntegrity(t *testing.T) {
	testFile := "testdata/Example Regatta Input Table.xlsx"

	data, err := ReadExcelFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read Excel file: %v", err)
	}

	for _, race := range data.Races {
		// Verify RawData structure
		if len(race.RawData) != 5 {
			t.Errorf("Race %d: Expected 5 rows in RawData, got %d", race.RaceNumber, len(race.RawData))
			continue
		}

		// Verify each row has 7 columns
		for rowIdx, row := range race.RawData {
			if len(row) != 7 {
				t.Errorf("Race %d, Row %d: Expected 7 columns, got %d",
					race.RaceNumber, rowIdx, len(row))
			}
		}

		// Verify boat class matches RawData
		if race.BoatClass != race.RawData.getBoatClass() {
			t.Errorf("Race %d: BoatClass mismatch: %q vs %q",
				race.RaceNumber, race.BoatClass, race.RawData.getBoatClass())
		}

		// Verify flight info matches RawData
		if race.FlightInfo != race.RawData.getFlightInfo() {
			t.Errorf("Race %d: FlightInfo mismatch: %q vs %q",
				race.RaceNumber, race.FlightInfo, race.RawData.getFlightInfo())
		}
	}
}

func TestReadExcelFile_HasBoats(t *testing.T) {
	testFile := "testdata/Example Regatta Input Table.xlsx"

	data, err := ReadExcelFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read Excel file: %v", err)
	}

	for _, race := range data.Races {
		hasBoats := race.HasBoats()
		expectedHasBoats := race.BoatCount > 0

		if hasBoats != expectedHasBoats {
			t.Errorf("Race %d: HasBoats() = %v, but BoatCount = %d",
				race.RaceNumber, hasBoats, race.BoatCount)
		}
	}
}

func TestReadExcelFile_SortedRaces(t *testing.T) {
	testFile := "testdata/Example Regatta Input Table.xlsx"

	data, err := ReadExcelFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read Excel file: %v", err)
	}

	sorted := data.SortedRaces()

	// Verify length matches
	if len(sorted) != len(data.Races) {
		t.Errorf("SortedRaces length (%d) doesn't match Races length (%d)",
			len(sorted), len(data.Races))
	}

	// Verify sorting
	for i := 1; i < len(sorted); i++ {
		if sorted[i].RaceNumber <= sorted[i-1].RaceNumber {
			t.Errorf("SortedRaces not properly sorted at index %d: %d follows %d",
				i, sorted[i].RaceNumber, sorted[i-1].RaceNumber)
		}
	}
}

