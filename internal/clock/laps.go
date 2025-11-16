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

func (l lapRows) getOOFLanes() []int {
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
