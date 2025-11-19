package clock

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/reader"
)

// results - two dimensional slice
// first dimension is each row, which represents different data on the y axis
// second dimension is each lane, with index 0 being labels for the rows of data.
type results [][]string

// initResults - initialize results table
func initResults(rd reader.RaceData) results {
	// schools needs to be padded as first column for labels
	schools := []string{""}
	schools = append(schools, rd.SchoolNames()...)
	// additionalInfo needs to be padded as first column for labels
	additionalInfo := []string{""}
	additionalInfo = append(additionalInfo, rd.AdditionalInfos()...)

	return [][]string{
		{"", "Lane 1", "Lane 2", "Lane 3", "Lane 4", "Lane 5", "Lane 6"},
		schools,
		additionalInfo,
		{"Place", "", "", "", "", "", ""},
		{"Split", "", "", "", "", "", ""},
		{"Time", "", "", "", "", "", ""},
	}
}

// updateFromLapRows - using provide lane, row, and laps update the lane colum
func (r results) updateFromLapRows(lane, row int, l laps) {
	r.updatePlace(lane, l.place(row))
	r.updateSplit(lane, l.split(row))
	r.updateTime(lane, l.calculatedTime(row))
}

// clear - with provided lane, update lane column with empty strings
func (r results) clear(lane int) {
	r.updatePlace(lane, common.EmptyString)
	r.updateSplit(lane, common.EmptyString)
	r.updateTime(lane, common.EmptyString)
}

// laneAsRow - with proved lane, return the lane data as a slice, which could represent a row
func (r results) laneAsRow(lane int) []string {
	return []string{
		strconv.Itoa(lane),
		r.place(lane),
		r.split(lane),
		r.time(lane),
		r.school(lane),
	}
}

// school - with proved lane, return school name
func (r results) school(lane int) string {
	return r[1][lane]
}

// place - with proved lane, return place
func (r results) place(lane int) string {
	return r[3][lane]
}

// split - with proved lane, return split
func (r results) split(lane int) string {
	return r[4][lane]
}

// time - with proved lane, return time
func (r results) time(lane int) string {
	return r[5][lane]
}

// updatePlace - with proved lane and place, update place for lane
func (r results) updatePlace(lane int, place string) {
	r[3][lane] = place
}

// updateSplit - with proved lane and split, update split for lane
func (r results) updateSplit(lane int, split string) {
	r[4][lane] = split
}

// updateTime - with proved lane and time, update time for lane
func (r results) updateTime(lane int, time string) {
	r[5][lane] = time
}

// isPlace - with provided lane, check if place string is valid integer char
func (r results) isPlace(lane int) bool {
	place, err := strconv.Atoi(r.place(lane))
	if err != nil {
		return false
	}
	return place >= 1 && place <= 6
}

// isNextPlace - with provided lane, see if place string value matches "Next Place"
func (r results) isNextPlace(lane int) bool {
	return r.place(lane) == nextPlace
}

// resultsContainer - container with table that presents race results
func (r results) resultsContainer() *fyne.Container {

	list := widget.NewTable(
		func() (int, int) {
			return len(r), len(r[0])
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("wide wide wide content")
			label.Alignment = fyne.TextAlignCenter
			return label
		},
		func(i widget.TableCellID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(r[i.Row][i.Col])
		})

	return container.NewGridWrap(
		fyne.Size{Width: clockWidth, Height: resultsHeight},
		container.NewStack(list),
	)
}

// asApprovals - with provided oofLanes, return results as approvals
func (r results) asApprovals(oofLanes []int) *approvals {

	approvals := initApprovalContainer()
	dqData := make([][]string, 0)
	finishedLanes := 0

	for _, lane := range oofLanes {

		if lane != badLaneNum {

			row := r.laneAsRow(lane)

			if r.isPlace(lane) { // Lane has valid place, add to approvals
				finishedLanes++
				approvals.setRow(finishedLanes, row)
			} else { // Lane does not have valid place, hold and add to approvals later
				dqData = append(dqData, row)
			}
		}
	}

	// Append not valid lanes to end of approvals
	for _, row := range dqData {
		finishedLanes++
		approvals.setRow(finishedLanes, row)
	}

	return approvals
}
