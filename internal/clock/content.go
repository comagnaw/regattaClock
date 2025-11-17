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
	return container.NewVBox(
		container.NewCenter(c.raceTitle),
		container.NewVBox(
			container.NewCenter(c.clock),
			c.controlPanel(),
			c.lapsContainer(),
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

func (c *Clock) lapsContainer() *fyne.Container {
	laps := container.NewVBox()

	header := container.NewGridWithColumns(4)
	for _, h := range []string{"OOF", "Place / DQ / DNS / DNF", "Split", "Time"} {
		text := widget.NewLabel(h)
		text.TextStyle = fyne.TextStyle{Bold: true}
		header.Add(text)
	}
	laps.Add(header)

	for rowNum := range c.laps {
		c.laps[rowNum] = lapRow{
			oofLaneNum:     c.oofEntry(rowNum),
			place:          c.initPlace(rowNum),
			split:          widget.NewEntry(),
			calculatedTime: widget.NewLabel(common.EmptyString),
		}
		laps.Add(c.laps[rowNum].asGridRow())
	}
	return laps
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
