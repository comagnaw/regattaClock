package clock

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
)

func (c *Clock) content() *fyne.Container {
	c.initButtons()
	// Create the final content with all elements
	return container.NewVBox(
		container.NewCenter(c.raceTitle),
		container.NewVBox(
			container.NewCenter(c.clock),
			c.controlPanel(),
			c.lapTable(),
			c.winningTimeInput(),
			c.resultsContainer(),
			c.approvalPanel(),
		),
	)

}

func (c *Clock) controlPanel() *fyne.Container {
	return container.NewHBox(
		layout.NewSpacer(),
		c.buttons.start,
		layout.NewSpacer(),
		c.buttons.lap,
		layout.NewSpacer(),
		c.buttons.stop,
		layout.NewSpacer(),
		c.buttons.clear,
		layout.NewSpacer(),
	)
}

func (c *Clock) lapTable() *fyne.Container {
	tablesContainer := container.NewVBox()
	tablesContainer.Add(c.lapHeader())

	c.lapRows = make([]lapRow, 6)

	for lap := range 6 {
		row := container.NewGridWithColumns(4)

		// Create widgets for each column
		oofEntry := widget.NewEntry()
		oofEntry.OnChanged = c.oofEntryOnChangedFunc(lap)
		oofEntry.OnSubmitted = c.oofEntryOnSubmittedFunc(lap)

		placeButton := widget.NewButton(common.EmptyString, nil)
		placeButton.Importance = widget.MediumImportance
		placeButton.Resize(fyne.NewSize(100, 30)) // Set minimum size
		placeButton.OnTapped = c.placeButtonOnTappedFunc(lap)

		splitEntry := widget.NewEntry()
		timeLabel := widget.NewLabel(common.EmptyString)

		// Add widgets to row
		row.Add(oofEntry)
		row.Add(placeButton)
		row.Add(splitEntry)
		row.Add(timeLabel)

		// Store the widgets
		c.lapRows[lap] = lapRow{
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

func (c *Clock) lapHeader() *fyne.Container {
	header := container.NewGridWithColumns(4)

	oofHeader := widget.NewLabel("OOF")
	oofHeader.TextStyle = fyne.TextStyle{Bold: true}

	placeHeader := widget.NewLabel("Place / DQ / DNS / DNF")
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

func (c *Clock) winningTimeInput() *widget.Form {
	return widget.NewForm(
		widget.NewFormItem(
			"Winning Time:",
			c.winningTime,
		),
	)
}

func (c *Clock) approvalPanel() *fyne.Container {
	return container.NewHBox(
		layout.NewSpacer(),
		c.buttons.referee,
		layout.NewSpacer(),
		c.buttons.save,
		layout.NewSpacer(),
	)
}
