package regattaClock

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type lapTime struct {
	number         int
	time           string
	calculatedTime string
	oof            string
	dq             bool
}

type LapTableRow struct {
	oofEntry   *widget.Entry
	placeLabel *widget.Label
	splitEntry *widget.Entry
	timeLabel  *widget.Label
	dqCheck    *widget.Check
}

var data = [][]string{[]string{"Class", "Lane 1", "Lane 2", "Lane 3", "Lane 4", "Lane 5", "Lane 6"},
	[]string{"Heat/Flight", "", "", "", "", "", ""},
	[]string{"Place", "", "", "", "", "", ""},
	[]string{"Split", "", "", "", "", "", ""},
	[]string{"Time", "", "", "", "", "", ""},
}

func (a *App) newTable() *fyne.Container {
	list := widget.NewTable(
		func() (int, int) {
			return len(data), len(data[0])
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("wide content")
		},
		func(i widget.TableCellID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(data[i.Row][i.Col])
		})
		
	return container.NewStack(list)
}

func (a *App) lapHeader() *fyne.Container {
	header := container.NewGridWithColumns(5)

	oofHeader := widget.NewLabel("OOF")
	oofHeader.TextStyle = fyne.TextStyle{Bold: true}

	dqHeader := widget.NewLabel("DQ")
	dqHeader.TextStyle = fyne.TextStyle{Bold: true}

	placeHeader := widget.NewLabel("Place")
	placeHeader.TextStyle = fyne.TextStyle{Bold: true}

	splitHeader := widget.NewLabel("Split")
	splitHeader.TextStyle = fyne.TextStyle{Bold: true}

	timeHeader := widget.NewLabel("Time")
	timeHeader.TextStyle = fyne.TextStyle{Bold: true}

	header.Add(oofHeader)
	header.Add(dqHeader)
	header.Add(placeHeader)
	header.Add(splitHeader)
	header.Add(timeHeader)

	return header
}

func (a *App) lapTable() *fyne.Container {

	tablesContainer := container.NewVBox()
	tablesContainer.Add(a.lapHeader())

	a.tableRows = make([]LapTableRow, 6)
	for i := 0; i < 6; i++ {

		row := container.NewGridWithColumns(5)

		// Create widgets for each column
		oofEntry := widget.NewEntry()
		dqCheck := widget.NewCheck(emptyString, nil)
		placeLabel := widget.NewLabel(emptyString)
		splitEntry := widget.NewEntry()
		timeLabel := widget.NewLabel(emptyString)

		// Add widgets to row
		row.Add(oofEntry)
		row.Add(dqCheck)
		row.Add(placeLabel)
		row.Add(splitEntry)
		row.Add(timeLabel)

		// Store the widgets
		a.tableRows[i] = LapTableRow{
			oofEntry:   oofEntry,
			placeLabel: placeLabel,
			splitEntry: splitEntry,
			timeLabel:  timeLabel,
			dqCheck:    dqCheck,
		}

		// Add row to container
		tablesContainer.Add(row)
	}
	return tablesContainer
}
