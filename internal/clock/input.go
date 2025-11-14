package clock

import (
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
)

func (c *Clock) initWinningTime() {
	c.winningTime = widget.NewEntry()
	c.winningTime.SetPlaceHolder(common.ZeroTime)
	c.winningTime.OnChanged = c.onChangedWinningTimeFunc()
	c.winningTime.Disable()
}

func (c *Clock) onChangedWinningTimeFunc() func(text string) {
	return func(text string) {

		// If winning time is empty, just disable referee button
		if text == common.EmptyString {
			c.buttons.referee.Disable()
			return
		}

		// Try to parse the winning time
		_, err := parseTime(text)
		if err != nil {
			c.buttons.referee.Disable()
			return
		}

		c.buttons.referee.Enable()
		c.refreshContent()
		c.window.Content().Refresh()

	}
}

func (c *Clock) oofEntry(rowNum int) *widget.Entry {
	oofEntry := widget.NewEntry()
	oofEntry.OnChanged = c.oofEntryOnChangedFunc(rowNum)
	oofEntry.OnSubmitted = c.oofEntryOnSubmittedFunc(rowNum)
	return oofEntry
}

func (c *Clock) oofEntryOnChangedFunc(row int) func(newOOF string) {
	return func(newOOF string) {
		if c.isNotRunning() {
			// Update resultsTable if OOF matches a lane number
			if laneNum := getGoodLaneNum(newOOF); laneNum != badLaneNum {

				if !c.lapRows.alreadyAssigned(row, newOOF) {

					// Update the lap time's OOF value
					c.lapRows.setOOFLaneNum(row, newOOF)
					c.lapRows.setPreviousOOFLaneNum(row, newOOF)

					// Update Place, Split, and Time rows in resultsTable
					c.resultsTable.updateFromLapRows(laneNum, row, c.lapRows)
					c.window.Content().Refresh()

				} else {
					// If already assigned, clear the input
					c.lapRows.setOOFLaneNum(row, common.EmptyString)
					c.window.Content().Refresh()
				}
			} else {
				// If OOF is cleared or invalid, clear the previous lane
				previousLaneNum := getGoodLaneNum(c.lapRows[row].previousOOFLaneNum)
				c.lapRows.setOOFLaneNum(row, common.EmptyString)
				c.lapRows.setPreviousOOFLaneNum(row, common.EmptyString)

				if previousLaneNum != badLaneNum {
					c.resultsTable.clear(previousLaneNum)
					c.window.Content().Refresh()
				}
			}
		}
	}
}

func (c *Clock) oofEntryOnSubmittedFunc(row int) func(text string) {
	return func(text string) {
		if c.isNotRunning() {
			nextRow := row + 1
			// Move focus to next row's OOF entry if it exists
			if nextRow < len(c.lapRows) {
				// Clear any existing text in the next entry
				c.lapRows.setOOFLaneNum(nextRow, common.EmptyString)
				// Move focus to the next entry
				c.window.Canvas().Focus(c.lapRows[nextRow].oofLaneNum)
			}
		}
	}
}
