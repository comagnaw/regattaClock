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
	splitLabel *widget.Label
	timeLabel  *widget.Label
	dqCheck    *widget.Check
}

var data = [][]string{[]string{"top left", "top right"},
	[]string{"bottom left", "bottom right"}}

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

func (a *App) lapTable() *fyne.Container {
	// Create a container for both tables with proper spacing
	tablesContainer := container.NewVBox()

	oof := widget.NewLabelWithStyle("OOF", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	oof.Resize(fyne.NewSize(80,30))

	dq := widget.NewLabelWithStyle("DQ", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	dq.Resize(fyne.NewSize(30,30))

	place := widget.NewLabelWithStyle("Place", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	place.Resize(fyne.NewSize(80,30))
	// Create lap table header
	lapHeader := container.NewHBox(
		oof,
		dq,
		place,
		// widget.NewLabelWithStyle("OOF", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		// widget.NewLabelWithStyle("DQ", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		// widget.NewLabelWithStyle("Place", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Split", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Time", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
	)
	lapHeader.Resize(fyne.NewSize(800, 30))

	tablesContainer.Add(lapHeader)

	// Initialize tableRows
	a.tableRows = make([]LapTableRow, 6)
	for i := 0; i < 6; i++ {
		row := container.NewHBox()

		// Create widgets for each column
		oofEntry := widget.NewEntry()
		oofEntry.Resize(fyne.NewSize(80, 30))

		dqCheck := widget.NewCheck("", nil)
		dqCheck.Resize(fyne.NewSize(30, 30))

		placeLabel := widget.NewLabel("")
		placeLabel.Resize(fyne.NewSize(80, 30))

		splitLabel := widget.NewLabel("")
		splitLabel.Resize(fyne.NewSize(280, 30))

		timeLabel := widget.NewLabel("")
		timeLabel.Resize(fyne.NewSize(280, 30))

		// Add widgets to row
		row.Add(oofEntry)
		row.Add(dqCheck)
		row.Add(placeLabel)
		row.Add(splitLabel)
		row.Add(timeLabel)

		// Store the widgets
		a.tableRows[i] = LapTableRow{
			oofEntry:   oofEntry,
			placeLabel: placeLabel,
			splitLabel: splitLabel,
			timeLabel:  timeLabel,
			dqCheck:    dqCheck,
		}

		// Add row to container
		tablesContainer.Add(row)
	}
	return tablesContainer
}
