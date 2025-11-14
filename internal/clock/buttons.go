package clock

import (
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
)

type buttons struct {
	start *widget.Button
	lap   *widget.Button
	stop  *widget.Button
	clear *widget.Button

	// referee - a button that is activated on capture of winningTime and is used to gain refereee approval of OOF and finish times
	referee *widget.Button

	// save - a button that is activated on approval of referee and used to save the race results to a Excel spreadsheet
	save *widget.Button
}

func (c *Clock) initButtons() {
	c.buttons.referee = c.initReferee()
	c.buttons.save = c.initSave()
	c.buttons.start = c.initStart()
	c.buttons.lap = c.initLap()
	c.buttons.stop = c.initStop()
	c.buttons.clear = c.initClear()
}

func (c *Clock) initStart() *widget.Button {
	return widget.NewButton(
		"Start (F2)",
		c.startFunc(),
	)
}

func (c *Clock) startFunc() func() {
	return func() {
		if c.isNotRunning() && c.clockState.isCleared {
			c.clockState.startTime = time.Now()
			c.clockState.isRunning = true
			c.clockState.isCleared = false
			c.lapRows.firstLap()
			c.refreshContent()
			c.winningTime.Disable()
		}
	}
}

func (c *Clock) initLap() *widget.Button {
	return widget.NewButton(
		"Lap (F4)",
		c.lapFunc(),
	)
}

func (c *Clock) lapFunc() func() {
	return func() {
		if c.clockState.isRunning && c.laps < 6 {
			c.laps++
			formatted := c.getElapsedTime()
			place := strconv.Itoa(c.laps)
			c.lapRows.updateLap(
				c.laps-1,
				place,
				formatted,
				formatted,
				common.EmptyString,
			)
			c.refreshContent()
		}
	}
}

func (c *Clock) initStop() *widget.Button {
	return widget.NewButton("Stop", func() {
		c.clockState.isRunning = false
		c.refreshContent()
		c.winningTime.Enable()
	})
}

func (c *Clock) initClear() *widget.Button {
	return widget.NewButton("Clear", func() {
		if c.isNotRunning() {
			c.clockState.isRunning = false
			c.clockState.isCleared = true

			c.clock.Text = common.ZeroTime

			c.laps = 1

			c.resultsTable = initResultsTable(c.raceData)

			c.initWinningTime()

			c.window.SetContent(c.content())

			c.window.Content().Refresh()

		}
	})
}

func (c *Clock) initReferee() *widget.Button {
	button := widget.NewButton(
		common.RefereeButtonText,
		c.refereeFunc(),
	)
	button.Disable()
	return button
}

func (c *Clock) refereeFunc() func() {
	return func() {
		c.showRefereeeApproval(c.raceData.RaceNumber)
	}
}

func (c *Clock) initSave() *widget.Button {
	button := widget.NewButton("Save", func() {
		// Save logic will be implemented later
	})
	button.Disable() // Initially disabled until approved
	return button
}

func (c Clock) initPlace(rowNum int) *widget.Button {
	button := widget.NewButton(common.EmptyString, nil)
	button.Importance = widget.MediumImportance
	button.Resize(fyne.NewSize(100, 30)) // Set minimum size
	button.OnTapped = c.placeButtonOnTappedFunc(rowNum)
	return button
}

func (c *Clock) placeButtonOnTappedFunc(row int) func() {
	return func() {
		if c.isNotRunning() {
			if laneNum := c.lapRows.getLaneNum(row); laneNum != badLaneNum {
				dialog.ShowCustom(
					"Edit Place",
					"Close",
					c.placeSelection(row, laneNum),
					c.window,
				)
			}
		}
	}
}
