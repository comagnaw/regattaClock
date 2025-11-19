package clock

import (
	"fmt"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/text"
)

type buttons struct {
	// start - a button that starts the clock, which indicates the first boat of a race has reached the finish line.
	start *widget.Button

	// lap - a button that captures the difference in time from the previous lap time.
	// This captures a split time between two boats crossing the finish line.
	lap *widget.Button

	// stop - a button that stops the clock from running.  This should only be pushed when all boats have crossed the finish line.
	stop *widget.Button

	// clear - a button that clears all of the content captured for a race.  This button is only active when the clock has been stopped.
	clear *widget.Button

	// referee - a button that is activated on capture of winningTime and is used to gain refereee approval of OOF and finish times
	referee *widget.Button

	// save - a button that is activated on approval of referee and used to save the race results.
	save *widget.Button
}

// initButtons - initialize buttons object
func (c *Clock) initButtons() {
	c.buttons.referee = c.initReferee()
	c.buttons.save = c.initSave()
	c.buttons.start = c.initStart()
	c.buttons.lap = c.initLap()
	c.buttons.stop = c.initStop()
	c.buttons.clear = c.initClear()
}

// initStart - initialize start button
func (c *Clock) initStart() *widget.Button {
	return widget.NewButton(
		common.StartButtonText,
		c.startFunc(),
	)
}

// startFunc - function used when start button is pushed
func (c *Clock) startFunc() func() {
	return func() {
		if c.isNotRunning() && c.clockState.isCleared {
			c.clockState.startTime = time.Now()
			c.clockState.isRunning = true
			c.clockState.isCleared = false
			c.laps.firstLap()
			c.refreshContent()
			c.winningTime.Disable()
		}
	}
}

// initLap - initialize lap button
func (c *Clock) initLap() *widget.Button {
	return widget.NewButton(
		common.LapButtonText,
		c.lapFunc(),
	)
}

// lapFunc - function used when lap button is pushed
func (c *Clock) lapFunc() func() {
	return func() {
		if c.clockState.isRunning && c.lapCount < 6 {
			c.lapCount++
			formatted := c.getElapsedTime()
			place := strconv.Itoa(c.lapCount)
			c.laps.updateLap(
				c.lapCount-1,
				place,
				formatted,
				formatted,
				common.EmptyString,
			)
			c.refreshContent()
		}
	}
}

// initStop - initialize stop button
func (c *Clock) initStop() *widget.Button {
	return widget.NewButton(common.StopButtonText, func() {
		c.clockState.isRunning = false
		c.refreshContent()
		c.winningTime.Enable()
	})
}

// initClear - initialize clear button
func (c *Clock) initClear() *widget.Button {
	return widget.NewButton("Clear", func() {
		if c.isNotRunning() {

			c.clockState.isRunning = false
			c.clockState.isCleared = true

			c.clock.Text = common.ZeroTime

			c.lapCount = 1

			c.results = initResults(c.raceData)

			c.initWinningTime()

			c.initButtons()

			c.window.SetContent(c.content())

			c.window.Content().Refresh()

		}
	})
}

// initStart - initialize referee button.
// Default is that the button is disabled until a winnning time has
// been input by user.
func (c *Clock) initReferee() *widget.Button {
	button := widget.NewButton(
		common.RefereeButtonText,
		c.refereeFunc(),
	)
	button.Disable()
	return button
}

// refereeFunc - function used when referee button is pushed
func (c *Clock) refereeFunc() func() {
	return func() {
		c.showRefereeeApproval(c.raceData.RaceNumber)
	}
}

// showRefereeApproval - present race results for refereee approval
func (c *Clock) showRefereeeApproval(raceNumber int) {
	dialog.ShowCustomConfirm(
		fmt.Sprintf(common.RefereeApproveTitle, raceNumber),
		common.ApproveButtonText,
		common.CancelButtonText,
		c.refereeApprovalContent(),
		c.refereeApprovalFunc(raceNumber),
		c.window,
	)
}

// refereeApprovalFunc - when referee approves race results, update RegattaData and enable save button
func (c *Clock) refereeApprovalFunc(raceNumber int) func(approve bool) {
	return func(approve bool) {
		if approve {
			c.RegattaData.ApproveRace(raceNumber)
			c.buttons.save.Enable()
		}
	}
}

// refereeApprovalContent - content used to present race results for referee approval
func (c *Clock) refereeApprovalContent() *fyne.Container {
	return container.NewVBox(
		container.NewCenter(text.Title(c.raceData.RaceTitle())),
		c.results.asApprovals(c.laps.getOOFLanes()).Container,
	)
}

// initSave - initialize the save button.
// Default is for button to be disabled until the race results are approved.
func (c *Clock) initSave() *widget.Button {
	button := widget.NewButton(common.SaveButtonText, func() {
		// Save logic will be implemented later
	})
	button.Disable()
	return button
}

// initPlace - initialize the place button
func (c Clock) initPlace(rowNum int) *widget.Button {
	button := widget.NewButton(common.EmptyString, nil)
	button.Importance = widget.MediumImportance
	button.Resize(fyne.NewSize(100, 30)) // Set minimum size
	button.OnTapped = c.placeButtonOnTappedFunc(rowNum)
	return button
}

// placeButtonOnTappedFunc - function used when the place button is pushed
func (c *Clock) placeButtonOnTappedFunc(row int) func() {
	return func() {
		if c.isNotRunning() {
			if laneNum := c.laps.getLaneNum(row); laneNum != badLaneNum {
				dialog.ShowCustom(
					fmt.Sprintf(common.EditPlaceTitle, row),
					common.CloseButtonText,
					c.placeSelection(row, laneNum),
					c.window,
				)
			}
		}
	}
}
