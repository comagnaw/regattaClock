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

	for rowNum := range c.lapRows {
		// Store the widgets
		c.lapRows[rowNum] = lapRow{
			oofLaneNum:     c.oofEntry(rowNum),
			place:          c.initPlace(rowNum),
			split:          widget.NewEntry(),
			calculatedTime: widget.NewLabel(common.EmptyString),
		}

		// Add row to container
		tablesContainer.Add(c.lapRows[rowNum].asGridRow())
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
