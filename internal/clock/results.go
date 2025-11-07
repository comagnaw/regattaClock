package clock

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/reader"
)

type resultsTable [][]string

func (r resultsTable) updatePlace(lane int, place string) {
	r[3][lane] = place
}

func (r resultsTable) updateSplit(lane int, split string) {
	r[4][lane] = split
}

func (r resultsTable) updateTime(lane int, time string) {
	r[5][lane] = time
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
		fyne.Size{Width: 1240, Height: 240},
		container.NewStack(list),
	)
}
