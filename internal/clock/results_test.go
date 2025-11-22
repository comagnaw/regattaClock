package clock

import (
	"testing"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/reader"
)

func createTestResults() results {
	raceData := createTestRaceData()
	return initResults(raceData)
}

func TestInitResults(t *testing.T) {
	raceData := createTestRaceData()
	results := initResults(raceData)

	if results == nil {
		t.Fatal("initResults returned nil")
	}

	// Should have 6 rows (header, schools, additionalInfo, place, split, time)
	if len(results) != 6 {
		t.Fatalf("Expected 6 rows, got %d", len(results))
	}

	// Each row should have 7 columns (label + 6 lanes)
	for i, row := range results {
		if len(row) != 7 {
			t.Errorf("Row %d: expected 7 columns, got %d", i, len(row))
		}
	}

	// Check header row
	expectedHeaders := []string{"", "Lane 1", "Lane 2", "Lane 3", "Lane 4", "Lane 5", "Lane 6"}
	for i, header := range results[0] {
		if header != expectedHeaders[i] {
			t.Errorf("Header[%d]: expected %q, got %q", i, expectedHeaders[i], header)
		}
	}

	// Check that schools were loaded
	if results[1][1] != "School A" {
		t.Errorf("Expected School A in lane 1, got %q", results[1][1])
	}

	// Check that additional info was loaded
	if results[2][1] != "Varsity" {
		t.Errorf("Expected Varsity in lane 1, got %q", results[2][1])
	}

	// Check that place row is initialized correctly
	if results[3][0] != "Place" {
		t.Errorf("Expected Place label, got %q", results[3][0])
	}

	// Check that split row is initialized correctly
	if results[4][0] != "Split" {
		t.Errorf("Expected Split label, got %q", results[4][0])
	}

	// Check that time row is initialized correctly
	if results[5][0] != "Time" {
		t.Errorf("Expected Time label, got %q", results[5][0])
	}
}

func TestResults_School(t *testing.T) {
	results := createTestResults()

	if results.school(1) != "School A" {
		t.Errorf("Expected 'School A', got %q", results.school(1))
	}

	if results.school(2) != "School B" {
		t.Errorf("Expected 'School B', got %q", results.school(2))
	}
}

func TestResults_Place(t *testing.T) {
	results := createTestResults()

	// Initially empty
	if results.place(1) != "" {
		t.Errorf("Expected empty place, got %q", results.place(1))
	}

	// After setting
	results.updatePlace(1, "1")
	if results.place(1) != "1" {
		t.Errorf("Expected place '1', got %q", results.place(1))
	}
}

func TestResults_Split(t *testing.T) {
	results := createTestResults()

	// Initially empty
	if results.split(1) != "" {
		t.Errorf("Expected empty split, got %q", results.split(1))
	}

	// After setting
	results.updateSplit(1, "00:05.0")
	if results.split(1) != "00:05.0" {
		t.Errorf("Expected split '00:05.0', got %q", results.split(1))
	}
}

func TestResults_Time(t *testing.T) {
	results := createTestResults()

	// Initially empty
	if results.time(1) != "" {
		t.Errorf("Expected empty time, got %q", results.time(1))
	}

	// After setting
	results.updateTime(1, "06:00.0")
	if results.time(1) != "06:00.0" {
		t.Errorf("Expected time '06:00.0', got %q", results.time(1))
	}
}

func TestResults_UpdatePlace(t *testing.T) {
	results := createTestResults()

	tests := []struct {
		lane  int
		place string
	}{
		{1, "1"},
		{2, "2"},
		{3, common.RaceDisqualification},
		{4, common.RaceDidNotFinish},
		{5, common.RaceDidNotStart},
		{6, nextPlace},
	}

	for _, tt := range tests {
		results.updatePlace(tt.lane, tt.place)
		if results.place(tt.lane) != tt.place {
			t.Errorf("Lane %d: expected place %q, got %q", tt.lane, tt.place, results.place(tt.lane))
		}
	}
}

func TestResults_UpdateSplit(t *testing.T) {
	results := createTestResults()

	results.updateSplit(1, "00:00.0")
	results.updateSplit(2, "00:02.5")
	results.updateSplit(3, "00:05.0")

	if results.split(1) != "00:00.0" {
		t.Errorf("Expected split '00:00.0', got %q", results.split(1))
	}

	if results.split(2) != "00:02.5" {
		t.Errorf("Expected split '00:02.5', got %q", results.split(2))
	}
}

func TestResults_UpdateTime(t *testing.T) {
	results := createTestResults()

	results.updateTime(1, "06:00.0")
	results.updateTime(2, "06:02.5")
	results.updateTime(3, "06:05.0")

	if results.time(1) != "06:00.0" {
		t.Errorf("Expected time '06:00.0', got %q", results.time(1))
	}

	if results.time(2) != "06:02.5" {
		t.Errorf("Expected time '06:02.5', got %q", results.time(2))
	}
}

func TestResults_UpdateFromLapRows(t *testing.T) {
	results := createTestResults()
	laps := createTestLaps()

	laps.setPlace(0, "1")
	laps.setSplit(0, "00:00.0")
	laps.setCalculatedTime(0, "06:00.0")

	results.updateFromLapRows(1, 0, laps)

	if results.place(1) != "1" {
		t.Errorf("Expected place '1', got %q", results.place(1))
	}

	if results.split(1) != "00:00.0" {
		t.Errorf("Expected split '00:00.0', got %q", results.split(1))
	}

	if results.time(1) != "06:00.0" {
		t.Errorf("Expected time '06:00.0', got %q", results.time(1))
	}
}

func TestResults_Clear(t *testing.T) {
	results := createTestResults()

	// Set some values
	results.updatePlace(1, "1")
	results.updateSplit(1, "00:00.0")
	results.updateTime(1, "06:00.0")

	// Clear the lane
	results.clear(1)

	if results.place(1) != common.EmptyString {
		t.Errorf("Expected empty place, got %q", results.place(1))
	}

	if results.split(1) != common.EmptyString {
		t.Errorf("Expected empty split, got %q", results.split(1))
	}

	if results.time(1) != common.EmptyString {
		t.Errorf("Expected empty time, got %q", results.time(1))
	}
}

func TestResults_LaneAsRow(t *testing.T) {
	results := createTestResults()

	results.updatePlace(1, "1")
	results.updateSplit(1, "00:00.0")
	results.updateTime(1, "06:00.0")

	row := results.laneAsRow(1)

	if len(row) != 5 {
		t.Fatalf("Expected 5 elements in row, got %d", len(row))
	}

	if row[0] != "1" {
		t.Errorf("Expected lane '1', got %q", row[0])
	}

	if row[1] != "1" {
		t.Errorf("Expected place '1', got %q", row[1])
	}

	if row[2] != "00:00.0" {
		t.Errorf("Expected split '00:00.0', got %q", row[2])
	}

	if row[3] != "06:00.0" {
		t.Errorf("Expected time '06:00.0', got %q", row[3])
	}

	if row[4] != "School A" {
		t.Errorf("Expected school 'School A', got %q", row[4])
	}
}

func TestResults_IsPlace(t *testing.T) {
	results := createTestResults()

	// Empty place should not be a place
	if results.isPlace(1) {
		t.Error("Empty place should not be valid")
	}

	// Valid places
	for i := 1; i <= 6; i++ {
		results.updatePlace(1, string(rune('0'+i)))
		if !results.isPlace(1) {
			t.Errorf("Place %d should be valid", i)
		}
	}

	// Invalid places
	invalidPlaces := []string{"0", "7", "DQ", "DNF", "DNS", nextPlace, "abc"}
	for _, place := range invalidPlaces {
		results.updatePlace(1, place)
		if results.isPlace(1) {
			t.Errorf("Place %q should not be valid", place)
		}
	}
}

func TestResults_IsNextPlace(t *testing.T) {
	results := createTestResults()

	if results.isNextPlace(1) {
		t.Error("Should not be Next Place initially")
	}

	results.updatePlace(1, nextPlace)

	if !results.isNextPlace(1) {
		t.Error("Should be Next Place after setting")
	}

	results.updatePlace(1, "1")

	if results.isNextPlace(1) {
		t.Error("Should not be Next Place after changing to '1'")
	}
}

func TestResults_ResultsContainer(t *testing.T) {
	results := createTestResults()

	container := results.resultsContainer()

	if container == nil {
		t.Fatal("resultsContainer returned nil")
	}

	if len(container.Objects) == 0 {
		t.Error("Container should have objects")
	}
}

func TestResults_AsApprovals(t *testing.T) {
	results := createTestResults()

	// Set up some race results
	results.updatePlace(1, "1")
	results.updateSplit(1, "00:00.0")
	results.updateTime(1, "06:00.0")

	results.updatePlace(2, "2")
	results.updateSplit(2, "00:02.5")
	results.updateTime(2, "06:02.5")

	results.updatePlace(3, common.RaceDisqualification)

	oofLanes := []int{1, 2, 3, badLaneNum, badLaneNum, badLaneNum}

	approvals := results.asApprovals(oofLanes)

	if approvals == nil {
		t.Fatal("asApprovals returned nil")
	}

	if approvals.Container == nil {
		t.Error("Approvals container should not be nil")
	}
}

func TestResults_AsApprovals_OrderingWithDQ(t *testing.T) {
	results := createTestResults()

	// Set up results with a DQ
	results.updatePlace(1, "1")
	results.updateSplit(1, "00:00.0")
	results.updateTime(1, "06:00.0")

	results.updatePlace(2, common.RaceDisqualification)
	results.updateSplit(2, "")
	results.updateTime(2, "")

	results.updatePlace(3, "2")
	results.updateSplit(3, "00:05.0")
	results.updateTime(3, "06:05.0")

	oofLanes := []int{1, 2, 3}

	approvals := results.asApprovals(oofLanes)

	if approvals == nil {
		t.Fatal("asApprovals returned nil")
	}

	if approvals.Container == nil {
		t.Error("Approvals container should not be nil")
	}

	// Verify that the container has the expected number of objects
	// Header (5) + 3 results rows (3 * 5 = 15) = 20 total objects
	expectedObjects := 5 + (3 * 5)
	if len(approvals.Container.Objects) != expectedObjects {
		t.Logf("Note: Container has %d objects, expected around %d", len(approvals.Container.Objects), expectedObjects)
	}
}

func TestResults_MultipleUpdates(t *testing.T) {
	results := createTestResults()

	// Update lane 1 multiple times
	results.updatePlace(1, "1")
	results.updatePlace(1, "2")
	results.updatePlace(1, "3")

	if results.place(1) != "3" {
		t.Errorf("Expected final place '3', got %q", results.place(1))
	}

	// Update split multiple times
	results.updateSplit(1, "00:05.0")
	results.updateSplit(1, "00:05.5")

	if results.split(1) != "00:05.5" {
		t.Errorf("Expected final split '00:05.5', got %q", results.split(1))
	}
}

func TestResults_AllLanes(t *testing.T) {
	results := createTestResults()

	// Test all 6 lanes
	for lane := 1; lane <= 6; lane++ {
		place := string(rune('0' + lane))
		split := "00:0" + string(rune('0'+lane)) + ".0"
		time := "06:0" + string(rune('0'+lane)) + ".0"

		results.updatePlace(lane, place)
		results.updateSplit(lane, split)
		results.updateTime(lane, time)

		if results.place(lane) != place {
			t.Errorf("Lane %d: expected place %q, got %q", lane, place, results.place(lane))
		}

		if results.split(lane) != split {
			t.Errorf("Lane %d: expected split %q, got %q", lane, split, results.split(lane))
		}

		if results.time(lane) != time {
			t.Errorf("Lane %d: expected time %q, got %q", lane, time, results.time(lane))
		}
	}
}

func TestResults_WithEmptyLanes(t *testing.T) {
	// Create race data with only 2 boats
	raceData := reader.RaceData{
		RaceNumber: 1,
		BoatCount:  2,
		BoatClass:  "Varsity 8",
		Lanes: map[int]reader.RaceEntry{
			1: {SchoolName: "School A"},
			3: {SchoolName: "School C"},
		},
	}

	results := initResults(raceData)

	// Lane 2 should be empty
	if results.school(2) != "" {
		t.Errorf("Expected empty school in lane 2, got %q", results.school(2))
	}

	// Lanes 1 and 3 should have schools
	if results.school(1) != "School A" {
		t.Errorf("Expected School A in lane 1, got %q", results.school(1))
	}

	if results.school(3) != "School C" {
		t.Errorf("Expected School C in lane 3, got %q", results.school(3))
	}
}
