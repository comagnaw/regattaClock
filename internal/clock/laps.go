package clock

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
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

func (l lapRow) asGridRow() *fyne.Container {
	return container.NewGridWithColumns(
		4,
		l.oofLaneNum,
		l.place,
		l.split,
		l.calculatedTime,
	)
}

type laps []lapRow

func (l laps) getLapRowByLaneNum(laneNum string) lapRow {
	for _, row := range l {
		if row.oofLaneNum.Text == laneNum {
			return row
		}
	}
	return lapRow{}
}
func (l laps) firstLap() {
	l.setPlace(0, "1")
	l.setSplit(0, common.ZeroTime)
	l.setCalculatedTime(0, common.ZeroTime)
	l.setOOFLaneNum(0, common.EmptyString)
}

func (l laps) updateLap(row int, place, split, timelabel, oof string) {
	l.setPlace(row, place)
	l.setSplit(row, split)
	l.setCalculatedTime(row, timelabel)
	l.setOOFLaneNum(row, oof)
}

func (l laps) hasOOF(row int) bool {
	if oof := l[row].oofLaneNum.Text; oof != common.EmptyString {
		return true
	}
	return false
}

func (l laps) getLaneNum(row int) int {
	if l.hasOOF(row) {
		return getGoodLaneNum(l.oofLaneNum(row))
	}
	return badLaneNum
}

func getGoodLaneNum(inputLane string) int {
	if laneNum, err := strconv.Atoi(inputLane); err == nil && laneNum >= 1 && laneNum <= 6 {
		return laneNum
	}
	return badLaneNum
}

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

func (l laps) oofLaneNum(row int) string {
	return l[row].oofLaneNum.Text
}

func (l laps) place(row int) string {
	return l[row].place.Text
}

func (l laps) split(row int) string {
	return l[row].split.Text
}

func (l laps) emptySplit(row int) bool {
	return l[row].split.Text == common.EmptyString
}

func (l laps) calculatedTime(row int) string {
	return l[row].calculatedTime.Text
}

func (l laps) setPreviousOOFLaneNum(row int, previousOOF string) {
	l[row].previousOOFLaneNum = previousOOF
}

func (l laps) setOOFLaneNum(row int, oofLaneNum string) {
	l[row].oofLaneNum.SetText(oofLaneNum)
}

func (l laps) setPlace(row int, place string) {
	l[row].place.SetText(place)
}

func (l laps) setSplit(row int, split string) {
	l[row].split.SetText(split)
}

func (l laps) setCalculatedTime(row int, calculatedTime string) {
	l[row].calculatedTime.SetText(calculatedTime)
}

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
