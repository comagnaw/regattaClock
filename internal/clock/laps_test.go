package clock

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
)

func createTestLaps() laps {
	testLaps := make([]lapRow, 6)
	for i := range testLaps {
		testLaps[i] = lapRow{
			previousOOFLaneNum: "",
			oofLaneNum:         widget.NewEntry(),
			place:              widget.NewButton("", nil),
			split:              widget.NewEntry(),
			calculatedTime:     widget.NewLabel(""),
		}
	}
	return testLaps
}

func TestLapRow_AsGridRow(t *testing.T) {
	row := lapRow{
		oofLaneNum:     widget.NewEntry(),
		place:          widget.NewButton("1", nil),
		split:          widget.NewEntry(),
		calculatedTime: widget.NewLabel("00:00.0"),
	}

	container := row.asGridRow()

	if container == nil {
		t.Fatal("asGridRow returned nil")
	}

	if len(container.Objects) != 4 {
		t.Errorf("Expected 4 objects in grid row, got %d", len(container.Objects))
	}
}

func TestLaps_FirstLap(t *testing.T) {
	laps := createTestLaps()

	laps.firstLap()

	if laps.place(0) != "1" {
		t.Errorf("Expected first lap place to be '1', got %q", laps.place(0))
	}

	if laps.split(0) != common.ZeroTime {
		t.Errorf("Expected first lap split to be %q, got %q", common.ZeroTime, laps.split(0))
	}

	if laps.calculatedTime(0) != common.ZeroTime {
		t.Errorf("Expected first lap calculated time to be %q, got %q", common.ZeroTime, laps.calculatedTime(0))
	}

	if laps.oofLaneNum(0) != common.EmptyString {
		t.Errorf("Expected first lap OOF to be empty, got %q", laps.oofLaneNum(0))
	}
}

func TestLaps_UpdateLap(t *testing.T) {
	laps := createTestLaps()

	laps.updateLap(0, "1", "06:00.0", "06:00.0", "1")

	if laps.place(0) != "1" {
		t.Errorf("Expected place '1', got %q", laps.place(0))
	}

	if laps.split(0) != "06:00.0" {
		t.Errorf("Expected split '06:00.0', got %q", laps.split(0))
	}

	if laps.calculatedTime(0) != "06:00.0" {
		t.Errorf("Expected calculated time '06:00.0', got %q", laps.calculatedTime(0))
	}

	if laps.oofLaneNum(0) != "1" {
		t.Errorf("Expected OOF '1', got %q", laps.oofLaneNum(0))
	}
}

func TestLaps_HasOOF(t *testing.T) {
	laps := createTestLaps()

	if laps.hasOOF(0) {
		t.Error("Should not have OOF initially")
	}

	laps.setOOFLaneNum(0, "1")

	if !laps.hasOOF(0) {
		t.Error("Should have OOF after setting")
	}
}

func TestLaps_GetLaneNum(t *testing.T) {
	laps := createTestLaps()

	// No OOF set
	if laps.getLaneNum(0) != badLaneNum {
		t.Errorf("Expected badLaneNum, got %d", laps.getLaneNum(0))
	}

	// Valid lane number
	laps.setOOFLaneNum(0, "3")
	if laps.getLaneNum(0) != 3 {
		t.Errorf("Expected lane 3, got %d", laps.getLaneNum(0))
	}

	// Invalid lane number
	laps.setOOFLaneNum(1, "invalid")
	if laps.getLaneNum(1) != badLaneNum {
		t.Errorf("Expected badLaneNum for invalid input, got %d", laps.getLaneNum(1))
	}
}

func TestGetGoodLaneNum(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"lane 1", "1", 1},
		{"lane 2", "2", 2},
		{"lane 3", "3", 3},
		{"lane 4", "4", 4},
		{"lane 5", "5", 5},
		{"lane 6", "6", 6},
		{"lane 0", "0", badLaneNum},
		{"lane 7", "7", badLaneNum},
		{"negative", "-1", badLaneNum},
		{"empty", "", badLaneNum},
		{"text", "abc", badLaneNum},
		{"float", "1.5", badLaneNum},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getGoodLaneNum(tt.input)
			if got != tt.want {
				t.Errorf("getGoodLaneNum(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetGoodPlaceNum(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"place 1", "1", 1},
		{"place 2", "2", 2},
		{"place 3", "3", 3},
		{"place 4", "4", 4},
		{"place 5", "5", 5},
		{"place 6", "6", 6},
		{"place 0", "0", badPlaceNum},
		{"place 7", "7", badPlaceNum},
		{"negative", "-1", badPlaceNum},
		{"empty", "", badPlaceNum},
		{"text", "DNS", badPlaceNum},
		{"float", "1.5", badPlaceNum},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getGoodPlaceNum(tt.input)
			if got != tt.want {
				t.Errorf("getGoodPlaceNum(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestLaps_AlreadyAssigned(t *testing.T) {
	laps := createTestLaps()

	laps.setOOFLaneNum(0, "1")
	laps.setOOFLaneNum(1, "2")
	laps.setOOFLaneNum(2, "3")

	// Test same row - should not be considered already assigned
	if laps.alreadyAssigned(0, "1") {
		t.Error("Same row should not be considered already assigned")
	}

	// Test different row with same value
	if !laps.alreadyAssigned(3, "1") {
		t.Error("Different row with same lane should be already assigned")
	}

	// Test unassigned lane
	if laps.alreadyAssigned(3, "4") {
		t.Error("Unassigned lane should not be already assigned")
	}
}

func TestLaps_GetLapRowByLaneNum(t *testing.T) {
	laps := createTestLaps()

	laps.setOOFLaneNum(0, "1")
	laps.setOOFLaneNum(1, "2")
	laps.setPlace(1, "2")

	row := laps.getLapRowByLaneNum("2")

	if row.oofLaneNum == nil {
		t.Fatal("Should find a valid lap row")
	}

	if row.oofLaneNum.Text != "2" {
		t.Errorf("Expected OOF '2', got %q", row.oofLaneNum.Text)
	}

	if row.place.Text != "2" {
		t.Errorf("Expected place '2', got %q", row.place.Text)
	}

	// Test non-existent lane
	emptyRow := laps.getLapRowByLaneNum("5")
	if emptyRow.oofLaneNum != nil {
		t.Error("Should return empty row for non-existent lane")
	}
}

func TestLaps_EmptySplit(t *testing.T) {
	laps := createTestLaps()

	if !laps.emptySplit(0) {
		t.Error("Split should be empty initially")
	}

	laps.setSplit(0, "00:05.0")

	if laps.emptySplit(0) {
		t.Error("Split should not be empty after setting")
	}
}

func TestLaps_SetPreviousOOFLaneNum(t *testing.T) {
	laps := createTestLaps()

	laps.setPreviousOOFLaneNum(0, "3")

	if laps[0].previousOOFLaneNum != "3" {
		t.Errorf("Expected previous OOF '3', got %q", laps[0].previousOOFLaneNum)
	}
}

func TestLaps_GetOOFLanes(t *testing.T) {
	laps := createTestLaps()

	laps.setOOFLaneNum(0, "1")
	laps.setOOFLaneNum(1, "3")
	laps.setOOFLaneNum(2, "invalid")
	// Rows 3, 4, 5 remain empty

	oofLanes := laps.getOOFLanes()

	if len(oofLanes) != 6 {
		t.Errorf("Expected 6 OOF lanes, got %d", len(oofLanes))
	}

	if oofLanes[0] != 1 {
		t.Errorf("Expected OOF lane 1 at index 0, got %d", oofLanes[0])
	}

	if oofLanes[1] != 3 {
		t.Errorf("Expected OOF lane 3 at index 1, got %d", oofLanes[1])
	}

	if oofLanes[2] != badLaneNum {
		t.Errorf("Expected badLaneNum at index 2, got %d", oofLanes[2])
	}

	if oofLanes[3] != badLaneNum {
		t.Errorf("Expected badLaneNum at index 3, got %d", oofLanes[3])
	}
}

func TestLaps_Setters(t *testing.T) {
	laps := createTestLaps()

	// Test setPlace
	laps.setPlace(0, "1")
	if laps.place(0) != "1" {
		t.Errorf("setPlace failed: expected '1', got %q", laps.place(0))
	}

	// Test setSplit
	laps.setSplit(0, "00:05.0")
	if laps.split(0) != "00:05.0" {
		t.Errorf("setSplit failed: expected '00:05.0', got %q", laps.split(0))
	}

	// Test setCalculatedTime
	laps.setCalculatedTime(0, "06:05.0")
	if laps.calculatedTime(0) != "06:05.0" {
		t.Errorf("setCalculatedTime failed: expected '06:05.0', got %q", laps.calculatedTime(0))
	}

	// Test setOOFLaneNum
	laps.setOOFLaneNum(0, "2")
	if laps.oofLaneNum(0) != "2" {
		t.Errorf("setOOFLaneNum failed: expected '2', got %q", laps.oofLaneNum(0))
	}
}

func TestLaps_MultipleRows(t *testing.T) {
	laps := createTestLaps()

	// Set up multiple laps
	for i := 0; i < 4; i++ {
		laps.setOOFLaneNum(i, string(rune('1'+i)))
		laps.setPlace(i, string(rune('1'+i)))
	}

	// Verify all were set correctly
	for i := 0; i < 4; i++ {
		expected := string(rune('1' + i))
		if laps.oofLaneNum(i) != expected {
			t.Errorf("Row %d: expected OOF %q, got %q", i, expected, laps.oofLaneNum(i))
		}
		if laps.place(i) != expected {
			t.Errorf("Row %d: expected place %q, got %q", i, expected, laps.place(i))
		}
	}
}

func TestLaps_Integration(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regattaData := createTestRegattaData()
	raceData := createTestRaceData()
	clock := NewClock(app, regattaData, raceData)

	// Open clock to initialize all widgets
	clock.OpenRaceClock()
	defer close(clock.clockState.stopChan)
	time.Sleep(50 * time.Millisecond)

	// Simulate a race progression
	clock.laps.firstLap()
	clock.laps[0].oofLaneNum.SetText("1")

	clock.laps.updateLap(1, "2", "00:02.5", "00:02.5", "2")
	clock.laps.updateLap(2, "3", "00:05.0", "00:05.0", "3")

	// Verify the laps were set up correctly
	if clock.laps.place(0) != "1" {
		t.Error("First lap should have place 1")
	}

	if clock.laps.place(1) != "2" {
		t.Error("Second lap should have place 2")
	}

	if clock.laps.split(2) != "00:05.0" {
		t.Error("Third lap should have split 00:05.0")
	}
}
