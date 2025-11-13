package clock

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/reader"
)

// resultsTable - two dimensional slice

type resultsTable [][]string

func (r resultsTable) updateFromLapRows(laneNum, row int, l lapRows) {
	r.updatePlace(laneNum, l.place(row))
	r.updateSplit(laneNum, l.split(row))
	r.updateTime(laneNum, l.calculatedTime(row))
}

func (r resultsTable) clear(laneNum int) {
	r.updatePlace(laneNum, common.EmptyString)
	r.updateSplit(laneNum, common.EmptyString)
	r.updateTime(laneNum, common.EmptyString)
}

func (r resultsTable) place(lane int) string {
	return r[3][lane]
}

func (r resultsTable) split(lane int) string {
	return r[4][lane]
}

func (r resultsTable) time(lane int) string {
	return r[5][lane]
}

func (r resultsTable) updatePlace(lane int, place string) {
	r[3][lane] = place
}

func (r resultsTable) updateSplit(lane int, split string) {
	r[4][lane] = split
}

func (r resultsTable) updateTime(lane int, time string) {
	r[5][lane] = time
}

func (r resultsTable) isPlace(lane int) bool {
	place, err := strconv.Atoi(r.place(lane))
	if err != nil {
		return false
	}
	return place >= 1 && place <= 6
}

func initResultsTable(rd reader.RaceData) resultsTable {
	schools := []string{""}
	schools = append(schools, rd.SchoolNames()...)
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

func (r resultsTable) resultsContainer() *fyne.Container {

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
