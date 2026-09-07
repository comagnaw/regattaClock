package clock

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/reader"
)

// results table row indices (see initResults): the school and additional-info
// rows are the only ones a schedule change touches.
const (
	schoolRow     = 1
	additionalRow = 2
)

// scheduleBannerWidget builds the (initially hidden) schedule-conflict notice
// for the clock. UpdateSchedule fills it; Dismiss hides it until the next
// schedule change (persona-plan.md 3c).
func (c *Clock) scheduleBannerWidget() fyne.CanvasObject {
	c.scheduleLabel = widget.NewLabel(common.EmptyString)
	c.scheduleLabel.Wrapping = fyne.TextWrapWord
	c.scheduleLabel.Importance = widget.WarningImportance
	dismiss := widget.NewButton(common.CloseButtonText, func() { c.scheduleBanner.Hide() })
	c.scheduleBanner = container.NewBorder(nil, nil, nil, dismiss, c.scheduleLabel)
	c.scheduleBanner.Hide()
	return c.scheduleBanner
}

// UpdateSchedule refreshes the lane labels (school / additional info) and the
// race title from a new schedule without disturbing timing state - lap rows,
// OOF, splits, and the winning time are left exactly as they are
// (persona-plan.md 3c item 3). changedLanes are drawn in a warning style so the
// finish timer can see which assignments moved.
func (c *Clock) UpdateSchedule(race reader.RaceData, changedLanes []int) {
	c.raceData = race

	c.changedLanes = make(map[int]bool, len(changedLanes))
	for _, lane := range changedLanes {
		c.changedLanes[lane] = true
	}

	c.results.setSchoolLabels(race)

	if c.raceTitle != nil {
		c.raceTitle.Text = race.RaceTitle()
		c.raceTitle.Refresh()
	}
	if c.resultsTable != nil {
		c.resultsTable.Refresh()
	}
	if c.scheduleBanner != nil {
		c.scheduleLabel.SetText(fmt.Sprintf(common.ClockScheduleNoticeFormat, race.RaceNumber))
		c.scheduleBanner.Show()
	}

	applog.Info("open clock schedule refreshed", "component", "clock",
		"race", race.RaceNumber, "changed_lanes", len(changedLanes))
}

// setSchoolLabels overwrites just the school and additional-info rows from a new
// schedule, leaving the lane header and every place / split / time cell intact.
func (r results) setSchoolLabels(rd reader.RaceData) {
	schools := append([]string{""}, rd.SchoolNames()...)
	additional := append([]string{""}, rd.AdditionalInfos()...)
	copy(r[schoolRow], schools)
	copy(r[additionalRow], additional)
}
