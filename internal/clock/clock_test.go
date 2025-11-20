package clock

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/reader"
)

func createTestRegattaData() *reader.RegattaData {
	return &reader.RegattaData{
		Name: "Test Regatta",
		Date: "2024-01-15",
		Races: []reader.RaceData{
			{
				RaceNumber: 1,
				BoatCount:  4,
				BoatClass:  "Varsity 8",
				Lanes: map[int]reader.RaceEntry{
					1: {SchoolName: "School A"},
					2: {SchoolName: "School B"},
					3: {SchoolName: "School C"},
					4: {SchoolName: "School D"},
				},
			},
		},
	}
}

func createTestRaceData() reader.RaceData {
	return reader.RaceData{
		RaceNumber: 1,
		BoatCount:  4,
		BoatClass:  "Varsity 8",
		Lanes: map[int]reader.RaceEntry{
			1: {SchoolName: "School A", AdditionalInfo: "Varsity"},
			2: {SchoolName: "School B", AdditionalInfo: "JV"},
			3: {SchoolName: "School C", AdditionalInfo: "Varsity"},
			4: {SchoolName: "School D", AdditionalInfo: "Novice"},
		},
	}
}

func TestNewClock(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regattaData := createTestRegattaData()
	raceData := createTestRaceData()

	clock := NewClock(app, regattaData, raceData)

	if clock == nil {
		t.Fatal("NewClock returned nil")
	}

	if clock.clock == nil {
		t.Error("Clock display should be initialized")
	}

	if clock.clock.Text != common.ZeroTime {
		t.Errorf("Expected initial clock text %q, got %q", common.ZeroTime, clock.clock.Text)
	}

	if clock.lapCount != 1 {
		t.Errorf("Expected initial lapCount to be 1, got %d", clock.lapCount)
	}

	if len(clock.laps) != 6 {
		t.Errorf("Expected 6 lap rows, got %d", len(clock.laps))
	}

	if clock.clockState == nil {
		t.Fatal("clockState should be initialized")
	}

	if clock.clockState.isRunning {
		t.Error("Clock should not be running initially")
	}

	if !clock.clockState.isCleared {
		t.Error("Clock should be cleared initially")
	}

	if clock.window == nil {
		t.Error("Window should be initialized")
	}

	if clock.buttons.start == nil {
		t.Error("Start button should be initialized")
	}

	if clock.buttons.lap == nil {
		t.Error("Lap button should be initialized")
	}

	if clock.buttons.stop == nil {
		t.Error("Stop button should be initialized")
	}

	if clock.winningTime == nil {
		t.Error("Winning time entry should be initialized")
	}
}

func TestClock_IsNotRunning(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regattaData := createTestRegattaData()
	raceData := createTestRaceData()
	clock := NewClock(app, regattaData, raceData)

	if !clock.isNotRunning() {
		t.Error("Clock should not be running initially")
	}

	clock.clockState.isRunning = true

	if clock.isNotRunning() {
		t.Error("Clock should be running after setting isRunning to true")
	}
}

func TestClock_HasWinningTime(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regattaData := createTestRegattaData()
	raceData := createTestRaceData()
	clock := NewClock(app, regattaData, raceData)

	if clock.hasWinningTime() {
		t.Error("Should not have winning time initially")
	}

	// Set up window content to prevent nil pointer issues
	clock.OpenRaceClock()
	defer close(clock.clockState.stopChan)

	// Allow UI to initialize
	time.Sleep(50 * time.Millisecond)

	clock.winningTime.SetText("06:00.0")

	if !clock.hasWinningTime() {
		t.Error("Should have winning time after setting text")
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		name        string
		timeStr     string
		want        time.Duration
		expectError bool
	}{
		{
			name:        "zero time",
			timeStr:     "00:00.0",
			want:        0,
			expectError: false,
		},
		{
			name:        "one minute",
			timeStr:     "01:00.0",
			want:        1 * time.Minute,
			expectError: false,
		},
		{
			name:        "one second",
			timeStr:     "00:01.0",
			want:        1 * time.Second,
			expectError: false,
		},
		{
			name:        "one tenth",
			timeStr:     "00:00.1",
			want:        100 * time.Millisecond,
			expectError: false,
		},
		{
			name:        "complex time",
			timeStr:     "06:30.5",
			want:        6*time.Minute + 30*time.Second + 500*time.Millisecond,
			expectError: false,
		},
		{
			name:        "empty string",
			timeStr:     "",
			want:        0,
			expectError: false,
		},
		{
			name:        "invalid format - no colon",
			timeStr:     "0000.0",
			want:        0,
			expectError: true,
		},
		{
			name:        "invalid format - no decimal",
			timeStr:     "00:00",
			want:        0,
			expectError: true,
		},
		{
			name:        "invalid minutes",
			timeStr:     "XX:00.0",
			want:        0,
			expectError: true,
		},
		{
			name:        "invalid seconds",
			timeStr:     "00:XX.0",
			want:        0,
			expectError: true,
		},
		{
			name:        "invalid tenths",
			timeStr:     "00:00.X",
			want:        0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTime(tt.timeStr)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError && got != tt.want {
				t.Errorf("Expected duration %v, got %v", tt.want, got)
			}
		})
	}
}

func TestFormatTime(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "zero time",
			duration: 0,
			want:     "00:00.0",
		},
		{
			name:     "one minute",
			duration: 1 * time.Minute,
			want:     "01:00.0",
		},
		{
			name:     "one second",
			duration: 1 * time.Second,
			want:     "00:01.0",
		},
		{
			name:     "one tenth",
			duration: 100 * time.Millisecond,
			want:     "00:00.1",
		},
		{
			name:     "complex time",
			duration: 6*time.Minute + 30*time.Second + 500*time.Millisecond,
			want:     "06:30.5",
		},
		{
			name:     "9 tenths",
			duration: 900 * time.Millisecond,
			want:     "00:00.9",
		},
		{
			name:     "59 seconds",
			duration: 59 * time.Second,
			want:     "00:59.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTime(tt.duration)
			if got != tt.want {
				t.Errorf("Expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestParseAndFormatTime_RoundTrip(t *testing.T) {
	testTimes := []string{
		"00:00.0",
		"01:00.0",
		"00:01.0",
		"06:30.5",
		"10:45.9",
	}

	for _, timeStr := range testTimes {
		t.Run(timeStr, func(t *testing.T) {
			duration, err := parseTime(timeStr)
			if err != nil {
				t.Fatalf("parseTime failed: %v", err)
			}

			formatted := formatTime(duration)
			if formatted != timeStr {
				t.Errorf("Round trip failed: %q -> %v -> %q", timeStr, duration, formatted)
			}
		})
	}
}

func TestClock_GetElapsedTime(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regattaData := createTestRegattaData()
	raceData := createTestRaceData()
	clock := NewClock(app, regattaData, raceData)

	// Set start time to 1 minute ago
	clock.clockState.startTime = time.Now().Add(-1 * time.Minute)

	elapsed := clock.getElapsedTime()

	// Should be approximately "01:00.0" but may vary slightly
	// The exact format is "MM:SS.T"
	if len(elapsed) < 7 || len(elapsed) > 8 {
		t.Errorf("Expected elapsed time format MM:SS.T, got %q (length %d)", elapsed, len(elapsed))
	}

	// Check it starts with "01:" or "00:" (depending on timing)
	if elapsed[0:3] != "01:" && elapsed[0:3] != "00:" {
		t.Logf("Elapsed time: %q - timing may vary slightly", elapsed)
	}
}

func TestClock_AdjustTime(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regattaData := createTestRegattaData()
	raceData := createTestRaceData()
	clock := NewClock(app, regattaData, raceData)

	// Open clock to ensure all widgets are initialized
	clock.OpenRaceClock()
	defer close(clock.clockState.stopChan)
	time.Sleep(50 * time.Millisecond)

	// Set up a lap with split time
	clock.laps[0].split.SetText("00:05.0")
	clock.laps[0].oofLaneNum.SetText("1")

	// No time adjustment
	clock.adjustTime(0, 0)

	if clock.laps.calculatedTime(0) != "00:05.0" {
		t.Errorf("Expected calculated time '00:05.0', got %q", clock.laps.calculatedTime(0))
	}

	// With time adjustment of 1 second
	timeAdjustment := 1 * time.Second
	clock.adjustTime(0, timeAdjustment)

	if clock.laps.calculatedTime(0) != "00:06.0" {
		t.Errorf("Expected calculated time '00:06.0', got %q", clock.laps.calculatedTime(0))
	}
}

func TestClock_AdjustPlaceForNextPlace(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regattaData := createTestRegattaData()
	raceData := createTestRaceData()
	clock := NewClock(app, regattaData, raceData)

	// Open clock to initialize widgets
	clock.OpenRaceClock()
	defer close(clock.clockState.stopChan)
	time.Sleep(50 * time.Millisecond)

	// Set up lanes with places
	clock.laps[0].oofLaneNum.SetText("1")
	clock.laps[1].oofLaneNum.SetText("2")
	clock.laps[2].oofLaneNum.SetText("3")

	clock.results.updatePlace(1, "1")
	clock.results.updatePlace(2, nextPlace)
	clock.results.updatePlace(3, "2")

	clock.adjustPlaceForNextPlace()

	// Lane 1 should be place 1
	if clock.results.place(1) != "1" {
		t.Errorf("Expected lane 1 to have place '1', got %q", clock.results.place(1))
	}

	// Lane 2 should be place 2 (was Next Place)
	if clock.results.place(2) != "2" {
		t.Errorf("Expected lane 2 to have place '2', got %q", clock.results.place(2))
	}

	// Lane 3 should be place 3
	if clock.results.place(3) != "3" {
		t.Errorf("Expected lane 3 to have place '3', got %q", clock.results.place(3))
	}
}

func TestClock_AdjustPlaceForDQ(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regattaData := createTestRegattaData()
	raceData := createTestRaceData()
	clock := NewClock(app, regattaData, raceData)

	// Open clock to initialize widgets
	clock.OpenRaceClock()
	defer close(clock.clockState.stopChan)
	time.Sleep(50 * time.Millisecond)

	// Set up lanes with places 1, 2, 3, 4
	clock.laps[0].oofLaneNum.SetText("1")
	clock.laps[1].oofLaneNum.SetText("2")
	clock.laps[2].oofLaneNum.SetText("3")
	clock.laps[3].oofLaneNum.SetText("4")

	clock.laps[0].place.SetText("1")
	clock.laps[1].place.SetText("2")
	clock.laps[2].place.SetText("3")
	clock.laps[3].place.SetText("4")

	clock.results.updatePlace(1, "1")
	clock.results.updatePlace(2, "2")
	clock.results.updatePlace(3, "3")
	clock.results.updatePlace(4, "4")

	// DQ lane 2 (place 2)
	oldPlace := 2
	clock.adjustPlaceForDQ(oldPlace)

	// Places after the DQ should be decremented
	if clock.results.place(3) != "2" {
		t.Errorf("Expected lane 3 to have place '2' after DQ, got %q", clock.results.place(3))
	}

	if clock.results.place(4) != "3" {
		t.Errorf("Expected lane 4 to have place '3' after DQ, got %q", clock.results.place(4))
	}

	// Place 1 should remain unchanged
	if clock.results.place(1) != "1" {
		t.Errorf("Expected lane 1 to keep place '1', got %q", clock.results.place(1))
	}
}

func TestClock_Constants(t *testing.T) {
	if clockWidth != 1240 {
		t.Errorf("Expected clockWidth 1240, got %f", clockWidth)
	}

	if clockHeight != 800 {
		t.Errorf("Expected clockHeight 800, got %f", clockHeight)
	}

	if resultsHeight != 240 {
		t.Errorf("Expected resultsHeight 240, got %f", resultsHeight)
	}

	if badLaneNum != -1 {
		t.Errorf("Expected badLaneNum -1, got %d", badLaneNum)
	}

	if badPlaceNum != -1 {
		t.Errorf("Expected badPlaceNum -1, got %d", badPlaceNum)
	}

	if nextPlace != "Next Place" {
		t.Errorf("Expected nextPlace 'Next Place', got %q", nextPlace)
	}
}

func TestClockState_Initialization(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regattaData := createTestRegattaData()
	raceData := createTestRaceData()
	clock := NewClock(app, regattaData, raceData)

	if clock.clockState.isRunning {
		t.Error("Clock should not be running initially")
	}

	if !clock.clockState.isCleared {
		t.Error("Clock should be cleared initially")
	}

	if clock.clockState.stopChan == nil {
		t.Error("stopChan should be initialized")
	}
}

func TestClock_StartFunc(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regattaData := createTestRegattaData()
	raceData := createTestRaceData()
	clock := NewClock(app, regattaData, raceData)

	// Ensure clock is in initial state
	if clock.clockState.isRunning {
		t.Fatal("Clock should not be running initially")
	}

	// Simulate starting the clock
	clock.clockState.startTime = time.Now()
	clock.clockState.isRunning = true
	clock.clockState.isCleared = false

	if !clock.clockState.isRunning {
		t.Error("Clock should be running after start")
	}

	if clock.clockState.isCleared {
		t.Error("Clock should not be cleared after start")
	}
}

func TestClock_WinningTimeEnabled(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regattaData := createTestRegattaData()
	raceData := createTestRaceData()
	clock := NewClock(app, regattaData, raceData)

	// Initially disabled
	if !clock.winningTime.Disabled() {
		t.Error("Winning time should be disabled initially")
	}
}

func TestClock_RefreshContent_WithWinningTime(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	regattaData := createTestRegattaData()
	raceData := createTestRaceData()
	clock := NewClock(app, regattaData, raceData)

	// Set up window content
	clock.OpenRaceClock()
	time.Sleep(50 * time.Millisecond) // Give goroutine time to start

	// Stop the clock
	clock.clockState.isRunning = false

	// Set winning time
	clock.winningTime.SetText("06:00.0")

	// Set up a lap
	clock.laps.setSplit(0, "00:05.0")
	clock.laps.setOOFLaneNum(0, "1")

	// Refresh content
	clock.refreshContent()

	// Calculated time should be winning time + split
	expected := "06:05.0"
	if clock.laps.calculatedTime(0) != expected {
		t.Errorf("Expected calculated time %q, got %q", expected, clock.laps.calculatedTime(0))
	}

	// Clean up
	close(clock.clockState.stopChan)
}

