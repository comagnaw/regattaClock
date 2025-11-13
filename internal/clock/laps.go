package clock

import (
	"strconv"
	"time"

	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/comagnaw/regattaClock/internal/common"
)

type lapRow struct {
	previousOOFLaneNum string
	oofLaneNum         *widget.Entry
	place              *widget.Button
	split              *widget.Entry
	calculatedTime     *widget.Label
}

type lapRows []lapRow

func (l lapRows) firstLap() {
	l.setPlace(0, "1")
	l.setSplit(0, common.ZeroTime)
	l.setCalculatedTime(0, common.ZeroTime)
	l.setOOFLaneNum(0, common.EmptyString)
}

func (l lapRows) updateLap(row int, place, split, timelabel, oof string) {
	l.setPlace(row, place)
	l.setSplit(row, split)
	l.setCalculatedTime(row, timelabel)
	l.setOOFLaneNum(row, oof)
}

func (l lapRows) hasOOF(row int) bool {
	if oof := l[row].oofLaneNum.Text; oof != common.EmptyString {
		return true
	}
	return false
}

func (l lapRows) getLaneNum(row int) int {
	if l.hasOOF(row) {
		if laneNum, err := strconv.Atoi(l.oofLaneNum(row)); err == nil && laneNum >= 1 && laneNum <= 6 {
			return laneNum
		}
	}
	return badLaneNum
}

// alreadyAssigned - check if submitted text is already assigned and is not the row
func (l lapRows) alreadyAssigned(row int, text string) bool {
	for j := 0; j < len(l); j++ {
		if j != row && l[j].oofLaneNum.Text == text {
			return true
		}
	}
	return false
}

func (l lapRows) oofLaneNum(row int) string {
	return l[row].oofLaneNum.Text
}

func (l lapRows) place(row int) string {
	return l[row].place.Text
}

func (l lapRows) split(row int) string {
	return l[row].split.Text
}

func (l lapRows) emptySplit(row int) bool {
	return l[row].split.Text == common.EmptyString
}

func (l lapRows) calculatedTime(row int) string {
	return l[row].calculatedTime.Text
}

func (l lapRows) setPreviousOOFLaneNum(row int, previousOOF string) {
	l[row].previousOOFLaneNum = previousOOF
}

func (l lapRows) setOOFLaneNum(row int, oofLaneNum string) {
	l[row].oofLaneNum.SetText(oofLaneNum)
}

func (l lapRows) setPlace(row int, place string) {
	l[row].place.SetText(place)
}

func (l lapRows) setSplit(row int, split string) {
	l[row].split.SetText(split)
}

func (l lapRows) setCalculatedTime(row int, calculatedTime string) {
	l[row].calculatedTime.SetText(calculatedTime)
}

func getGoodLaneNum(inputLane string) int {
	if laneNum, err := strconv.Atoi(inputLane); err == nil && laneNum >= 1 && laneNum <= 6 {
		return laneNum
	}
	return badLaneNum
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

func (c *Clock) oofEntryOnChangedFunc(row int) func(text string) {
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

func (c *Clock) splitEntryOnChangedFunc(row int, timeAdjustment time.Duration) func(newSplit string) {
	return func(newSplit string) {
		if c.isNotRunning() {
			if laneNum := c.lapRows.getLaneNum(row); laneNum != badLaneNum && c.resultsTable.isPlace(laneNum) {
				c.adjustTime(row, timeAdjustment)
				c.resultsTable.updateSplit(laneNum, newSplit)
				c.window.Content().Refresh()
			}
		}
	}
}
