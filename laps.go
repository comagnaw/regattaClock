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
}

type LapTableRow struct {
	oofEntry    *widget.Entry
	placeButton *widget.Button
	splitEntry  *widget.Entry
	timeLabel   *widget.Label
}

type RaceTreeNode struct {
	RaceNumber int
	BoatCount  int
	TimeButton *widget.Button
	RaceData   *RaceData
}

func (a *App) raceResults() *fyne.Container {
	// Initialize table data if not already done
	if a.resultsTable == nil {
		a.resultsTable = [][]string{
			{"", "Lane 1", "Lane 2", "Lane 3", "Lane 4", "Lane 5", "Lane 6"},
			{"", "", "", "", "", "", ""},
			{"Place", "", "", "", "", "", ""},
			{"Split", "", "", "", "", "", ""},
			{"Time", "", "", "", "", "", ""},
			{"", "", "", "", "", "", ""}, // Add fifth data row
		}
	}

	// Ensure we have enough rows for the data
	if len(a.resultsTable) < 6 {
		// Add any missing rows
		for i := len(a.resultsTable); i < 6; i++ {
			a.resultsTable = append(a.resultsTable, make([]string, 7))
		}
	}

	list := widget.NewTable(
		func() (int, int) {
			return len(a.resultsTable), len(a.resultsTable[0])
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("wide wide wide content")
			label.Alignment = fyne.TextAlignCenter
			return label
		},
		func(i widget.TableCellID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(a.resultsTable[i.Row][i.Col])
		})

	return container.NewStack(list)
}

func (a *App) lapHeader() *fyne.Container {
	header := container.NewGridWithColumns(4)

	oofHeader := widget.NewLabel("OOF")
	oofHeader.TextStyle = fyne.TextStyle{Bold: true}

	placeHeader := widget.NewLabel("Place")
	placeHeader.TextStyle = fyne.TextStyle{Bold: true}

	splitHeader := widget.NewLabel("Split")
	splitHeader.TextStyle = fyne.TextStyle{Bold: true}

	timeHeader := widget.NewLabel("Time")
	timeHeader.TextStyle = fyne.TextStyle{Bold: true}

	header.Add(oofHeader)
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
		row := container.NewGridWithColumns(4)

		// Create widgets for each column
		oofEntry := widget.NewEntry()
		placeButton := widget.NewButton(emptyString, nil)
		placeButton.Importance = widget.MediumImportance
		placeButton.Resize(fyne.NewSize(100, 30)) // Set minimum size
		splitEntry := widget.NewEntry()
		timeLabel := widget.NewLabel(emptyString)

		// Add widgets to row
		row.Add(oofEntry)
		row.Add(placeButton)
		row.Add(splitEntry)
		row.Add(timeLabel)

		// Store the widgets
		a.tableRows[i] = LapTableRow{
			oofEntry:    oofEntry,
			placeButton: placeButton,
			splitEntry:  splitEntry,
			timeLabel:   timeLabel,
		}

		// Add row to container
		tablesContainer.Add(row)
	}
	return tablesContainer
}
