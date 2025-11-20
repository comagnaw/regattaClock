package clock

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/comagnaw/regattaClock/internal/common"
)

// lapRow - used to hold each lap collected as race progresses
type lapRow struct {
	// previousOOFLaneNum - used when OOF is being updated by user
	// represents the lane number previously held by this lapRow
	previousOOFLaneNum string

	// oofLaneNum - OOF input which represents the lane number for this
	// lapRow's place
	oofLaneNum *widget.Entry

	// place - a string value for place which can be toggled by user
	// to non-place values
	place *widget.Button

	// split - a string value for time between the previous
	// lapRow and this lapRow's boat crossing the finish line
	split *widget.Entry

	// calculatedTime - a string value for the winningTime plust the
	// split time value
	calculatedTime *widget.Label
}

// asGridRow - format the lapRow as fyne Grid Container
func (l lapRow) asGridRow() *fyne.Container {
	return container.NewGridWithColumns(
		4,
		l.oofLaneNum,
		l.place,
		l.split,
		l.calculatedTime,
	)
}

// laps - a slice of lapRow
type laps []lapRow

// getLapRowByLaneNum - useing lane string, find matching lapRow with same oofLaneNum
func (l laps) getLapRowByLaneNum(lane string) lapRow {
	for _, row := range l {
		if row.oofLaneNum.Text == lane {
			return row
		}
	}
	return lapRow{}
}

// firstLap - set the first lapRow as first place and zero values
func (l laps) firstLap() {
	l.setPlace(0, "1")
	l.setSplit(0, common.ZeroTime)
	l.setCalculatedTime(0, common.ZeroTime)
	l.setOOFLaneNum(0, common.EmptyString)
}

// updateLap - with provided row as lapRow index, set the place, split, calculatedTime, oofLaneNum
func (l laps) updateLap(row int, place, split, timelabel, oof string) {
	l.setPlace(row, place)
	l.setSplit(row, split)
	l.setCalculatedTime(row, timelabel)
	l.setOOFLaneNum(row, oof)
}

// hasOOF - with provided row as lapRow index, confirm if oofLaneNum is not empty
func (l laps) hasOOF(row int) bool {
	if oof := l[row].oofLaneNum.Text; oof != common.EmptyString {
		return true
	}
	return false
}

// getLaneNum - with provided row as lapRow index, return integer value of oofLaneNum
func (l laps) getLaneNum(row int) int {
	if l.hasOOF(row) {
		return getGoodLaneNum(l.oofLaneNum(row))
	}
	return badLaneNum
}

// getGoodLaneNum - with provided inputLane string, confirm it can be converted to integer
// and is a integer value within range of 1-6.  This assumes the regatta is a 6 lane course.
func getGoodLaneNum(inputLane string) int {
	if laneNum, err := strconv.Atoi(inputLane); err == nil && laneNum >= 1 && laneNum <= 6 {
		return laneNum
	}
	return badLaneNum
}

// getGoodPlaceNum - with provided inputPlace string, confirm it can be converted to integer
// and is a integer value within range of 1-6.  This assumes the regatta is a 6 lane course and
// therefore has 6 places.
func getGoodPlaceNum(inputPlace string) int {
	if placeNum, err := strconv.Atoi(inputPlace); err == nil && placeNum >= 1 && placeNum <= 6 {
		return placeNum
	}
	return badPlaceNum
}

// alreadyAssigned - check if submitted text is already assigned and is not the row
func (l laps) alreadyAssigned(row int, text string) bool {
	for j := 0; j < len(l); j++ {
		if j != row && l[j].oofLaneNum.Text == text {
			return true
		}
	}
	return false
}

// oofLaneNum - with provided row as lapRow index, return oofLaneNum as string
func (l laps) oofLaneNum(row int) string {
	return l[row].oofLaneNum.Text
}

// place - with provided row as lapRow index, return place as string
func (l laps) place(row int) string {
	return l[row].place.Text
}

// split - with provided row as lapRow index, return split as string
func (l laps) split(row int) string {
	return l[row].split.Text
}

// emptySplit - with provided row as lapRow index, return true if split is empty string
func (l laps) emptySplit(row int) bool {
	return l[row].split.Text == common.EmptyString
}

// calculatedTime - with provided row as lapRow index, return calculatedTime as string
func (l laps) calculatedTime(row int) string {
	return l[row].calculatedTime.Text
}

// setPreviousOOFLaneNum - with provided row as lapRow index and previousOOF, set lapRow previousOOFLaneNum
func (l laps) setPreviousOOFLaneNum(row int, previousOOF string) {
	l[row].previousOOFLaneNum = previousOOF
}

// setOOFLaneNum - with provided row as lapRow index and oofLaneNum, set lapRow oofLaneNum
func (l laps) setOOFLaneNum(row int, oofLaneNum string) {
	l[row].oofLaneNum.SetText(oofLaneNum)
}

// setPlace - with provided row as lapRow index and place, set lapRow place
func (l laps) setPlace(row int, place string) {
	l[row].place.SetText(place)
}

// setSplit - with provided row as lapRow index and split, set lapRow split
func (l laps) setSplit(row int, split string) {
	l[row].split.SetText(split)
}

// setCalculatedTime - with provided row as lapRow index and calculatedTime, set lapRow calculatedTime
func (l laps) setCalculatedTime(row int, calculatedTime string) {
	l[row].calculatedTime.SetText(calculatedTime)
}

// getOOFLanes - return slice of int for oofLaneNum from laps.
func (l laps) getOOFLanes() []int {
	oofLanes := []int{}
	for row := range l {
		lane, err := strconv.Atoi(l.oofLaneNum(row))
		if err != nil {
			oofLanes = append(oofLanes, badLaneNum)
			continue
		}
		oofLanes = append(oofLanes, lane)
	}
	return oofLanes
}
