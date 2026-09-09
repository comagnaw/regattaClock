package clock

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/reader"
)

// bannerButton returns the first button inside a banner container (the Dismiss /
// Close affordance).
func bannerButton(c *fyne.Container) *widget.Button {
	for _, o := range c.Objects {
		if b, ok := o.(*widget.Button); ok {
			return b
		}
	}
	return nil
}

// TestUpdateScheduleRefreshesLabelsKeepsTiming covers persona-plan.md 3c item 3:
// a schedule change delivered while the clock is open refreshes the lane labels
// and marks the moved lanes, without disturbing lap rows, OOF, splits, results,
// or the winning time.
func TestUpdateScheduleRefreshesLabelsKeepsTiming(t *testing.T) {
	app := test.NewTempApp(t)
	clk := NewClock(app, createTestRegattaData(), createTestRaceData())
	clk.OpenRaceClock()
	t.Cleanup(func() { clk.window.Close() })

	// Timing state the operator has entered.
	clk.laps.setOOFLaneNum(0, "2")
	clk.laps.setPlace(0, "1")
	clk.laps.setSplit(0, "00:05.0")
	clk.winningTime.SetText("01:00.0")
	clk.results.updatePlace(2, "1")
	clk.results.updateTime(2, "06:00.0")

	// New schedule: lane 2's crew changes.
	race := createTestRaceData()
	race.Lanes[2] = reader.RaceEntry{SchoolName: "Different Crew", AdditionalInfo: "B"}

	clk.UpdateSchedule(race, []int{2})

	if clk.results.school(2) != "Different Crew" {
		t.Errorf("lane 2 school label = %q, want the new name", clk.results.school(2))
	}
	if clk.laps.oofLaneNum(0) != "2" || clk.laps.place(0) != "1" || clk.laps.split(0) != "00:05.0" {
		t.Errorf("lap row disturbed: oof=%q place=%q split=%q",
			clk.laps.oofLaneNum(0), clk.laps.place(0), clk.laps.split(0))
	}
	if clk.winningTime.Text != "01:00.0" {
		t.Errorf("winning time disturbed: %q", clk.winningTime.Text)
	}
	if clk.results.place(2) != "1" || clk.results.time(2) != "06:00.0" {
		t.Errorf("result rows disturbed: place=%q time=%q", clk.results.place(2), clk.results.time(2))
	}
	if !clk.changedLanes[2] {
		t.Error("changed lane 2 not tracked for the highlight")
	}
	if clk.scheduleBanner.Hidden {
		t.Error("schedule notice banner should be visible after a change")
	}
}

func TestUpdateScheduleDismissHidesNotice(t *testing.T) {
	app := test.NewTempApp(t)
	clk := NewClock(app, createTestRegattaData(), createTestRaceData())
	clk.OpenRaceClock()
	t.Cleanup(func() { clk.window.Close() })

	clk.UpdateSchedule(createTestRaceData(), []int{1})
	if clk.scheduleBanner.Hidden {
		t.Fatal("precondition: banner visible")
	}

	dismiss := bannerButton(clk.scheduleBanner)
	if dismiss == nil {
		t.Fatal("no dismiss button on the schedule banner")
	}
	dismiss.OnTapped()
	if !clk.scheduleBanner.Hidden {
		t.Error("dismiss should hide the schedule notice")
	}
}
