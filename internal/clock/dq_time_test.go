package clock

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/comagnaw/regattaClock/internal/common"
)

// timedRace opens a clock with lanes 1-3 finished in that order, each with a
// place, split and calculated time in both the lap rows and the results
// table - the state a timer is in when they start finalising results.
func timedRace(t *testing.T) *Clock {
	t.Helper()
	app := test.NewApp()
	t.Cleanup(app.Quit)

	c := NewClock(app, createTestRegattaData(), createTestRaceData())
	c.OpenRaceClock()
	time.Sleep(50 * time.Millisecond)
	stopClockTicker(t, c) // stops (and closes) the ticker - no further close needed

	for i, tm := range []string{"06:01.0", "06:04.5", "06:09.2"} {
		lane := i + 1
		c.laps[i].oofLaneNum.SetText(laneStr(lane))
		c.laps[i].place.SetText(laneStr(lane))
		c.laps[i].split.SetText(tm)
		c.laps[i].calculatedTime.SetText(tm)
		c.results.updateFromLapRows(lane, i, c.laps)
	}
	return c
}

func laneStr(n int) string { return string(rune('0' + n)) }

// TestPlaceSelection_DQKeepsTime is the user-facing change: a DQ'd boat still
// shows the time it finished in, while losing its place.
func TestPlaceSelection_DQKeepsTime(t *testing.T) {
	c := timedRace(t)

	c.placeSelectionFunc(1, 2)(common.RaceDisqualification)

	if got := c.results.place(2); got != common.RaceDisqualification {
		t.Errorf("lane 2 place = %q, want DQ", got)
	}
	if got := c.results.split(2); got != "06:04.5" {
		t.Errorf("lane 2 split = %q, want the captured split kept", got)
	}
	if got := c.results.time(2); got != "06:04.5" {
		t.Errorf("lane 2 time = %q, want the captured time kept", got)
	}
	if got := c.results.place(3); got != "2" {
		t.Errorf("lane 3 place = %q, want it moved up to 2", got)
	}
}

func TestPlaceSelection_DNFAndDNSClearTime(t *testing.T) {
	for _, status := range []string{common.RaceDidNotFinish, common.RaceDidNotStart} {
		t.Run(status, func(t *testing.T) {
			c := timedRace(t)

			c.placeSelectionFunc(1, 2)(status)

			if c.results.split(2) != "" || c.results.time(2) != "" {
				t.Errorf("lane 2 split/time = %q/%q, want cleared for %s",
					c.results.split(2), c.results.time(2), status)
			}
			if got := c.results.place(3); got != "2" {
				t.Errorf("lane 3 place = %q, want it moved up to 2", got)
			}
		})
	}
}

// TestPlaceSelection_DQBackToNextPlace: undoing a DQ restores the boat's place
// and its time.
func TestPlaceSelection_DQBackToNextPlace(t *testing.T) {
	c := timedRace(t)

	c.placeSelectionFunc(1, 2)(common.RaceDisqualification)
	c.placeSelectionFunc(1, 2)(nextPlace)

	if got := c.results.place(2); got != "2" {
		t.Errorf("lane 2 place = %q, want 2 restored", got)
	}
	if got := c.results.time(2); got != "06:04.5" {
		t.Errorf("lane 2 time = %q, want the captured time", got)
	}
}

// TestAdjustTime_UpdatesDQLane: a time adjustment applied after a DQ still
// reaches the DQ'd lane's displayed time.
func TestAdjustTime_UpdatesDQLane(t *testing.T) {
	c := timedRace(t)
	c.placeSelectionFunc(1, 2)(common.RaceDisqualification)

	c.adjustTime(1, time.Second)

	if got := c.results.time(2); got != "06:05.5" {
		t.Errorf("lane 2 time = %q, want the adjusted time", got)
	}
	if got := c.results.place(2); got != common.RaceDisqualification {
		t.Errorf("lane 2 place = %q, want DQ kept", got)
	}
}

// TestRefereeApproval_ShowsDQTime: what the referee approves lists the DQ'd
// boat after the placed boats, with its time.
func TestRefereeApproval_ShowsDQTime(t *testing.T) {
	c := timedRace(t)
	c.placeSelectionFunc(1, 2)(common.RaceDisqualification)

	grid := c.results.asApprovals(c.laps.getOOFLanes())

	// Cells are laid out row-major, refereeCols per row (OOF, Place, Split,
	// Time, School), after a header row. Row 3 is the last results row.
	var cells []string
	for _, o := range grid.Objects {
		if txt := cellText(o); txt != nil {
			cells = append(cells, txt.Text)
		}
	}
	start := 3 * refereeCols
	if len(cells) < start+refereeCols {
		t.Fatalf("approval grid has %d cells, want at least %d", len(cells), start+refereeCols)
	}
	last := cells[start : start+4] // OOF, Place, Split, Time
	want := []string{"2", common.RaceDisqualification, "06:04.5", "06:04.5"}
	for i := range want {
		if last[i] != want[i] {
			t.Fatalf("last approval row = %v, want %v (DQ last, with its time)", last, want)
		}
	}
}

// TestSerializeLapRows_StatusTimes: finish.json - and so the published
// workbook - carries a DQ'd boat's time and blanks DNF/DNS, matching what the
// referee approved.
func TestSerializeLapRows_StatusTimes(t *testing.T) {
	c := timedRace(t)
	c.placeSelectionFunc(1, 2)(common.RaceDisqualification)
	c.placeSelectionFunc(2, 3)(common.RaceDidNotFinish)

	rows := c.serializeLapRows()

	if rows[1].Place != common.RaceDisqualification || rows[1].Time != "06:04.5" || rows[1].Split != "06:04.5" {
		t.Errorf("DQ row = %+v, want DQ with its split and time", rows[1])
	}
	if rows[2].Place != common.RaceDidNotFinish || rows[2].Time != "" || rows[2].Split != "" {
		t.Errorf("DNF row = %+v, want DNF with split and time blank", rows[2])
	}
}
