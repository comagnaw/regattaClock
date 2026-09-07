package clock

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/text"
)

// content - primary fyne objects presented as clock and race input
func (c *Clock) content() *fyne.Container {
	c.raceTitle = text.Header1(c.raceData.RaceTitle())
	return container.NewVBox(
		container.NewCenter(c.raceTitle),
		c.skewBannerWidget(),
		c.scheduleBannerWidget(),
		container.NewVBox(
			container.NewCenter(c.clock),
			c.controlPanel(),
			c.lapsContainer(),
			c.winningTimeInput(),
			c.resultsPanel(),
			c.approvalPanel(),
		),
	)
}

// resultsPanel is resultsContainer with the table handle kept, so a schedule
// refresh can repaint lane labels in place, and with changed lanes drawn in a
// warning style (persona-plan.md 3c).
func (c *Clock) resultsPanel() *fyne.Container {
	c.resultsTable = widget.NewTable(
		func() (int, int) { return len(c.results), len(c.results[0]) },
		func() fyne.CanvasObject {
			label := widget.NewLabel("wide wide wide content")
			label.Alignment = fyne.TextAlignCenter
			return label
		},
		func(i widget.TableCellID, o fyne.CanvasObject) {
			label := o.(*widget.Label)
			label.SetText(c.results[i.Row][i.Col])
			if (i.Row == schoolRow || i.Row == additionalRow) && i.Col >= 1 && c.changedLanes[i.Col] {
				label.TextStyle = fyne.TextStyle{Bold: true}
				label.Importance = widget.WarningImportance
			} else {
				label.TextStyle = fyne.TextStyle{}
				label.Importance = widget.MediumImportance
			}
		})

	return container.NewGridWrap(
		fyne.Size{Width: clockWidth, Height: resultsHeight},
		container.NewStack(c.resultsTable),
	)
}

// skewBannerWidget builds the (initially hidden) clock-skew banner. checkSkew
// fills the label and shows it; the Dismiss button hides it for good
// (persona-plan.md 2.1).
func (c *Clock) skewBannerWidget() fyne.CanvasObject {
	c.skewLabel = widget.NewLabel(common.EmptyString)
	c.skewLabel.Wrapping = fyne.TextWrapWord
	dismiss := widget.NewButton(common.DismissButtonText, func() {
		c.skewDismissed = true
		c.skewBanner.Hide()
	})
	c.skewBanner = container.NewBorder(nil, nil, nil, dismiss, c.skewLabel)
	c.skewBanner.Hide()
	return c.skewBanner
}

// controlPanel - container with buttons that control clock and clear results
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

// lapContainer - container that updates as clock is started and lap button pushed.
// Also collectes order-of-finish (OOF) and adjustement of place and split time.
func (c *Clock) lapsContainer() *fyne.Container {
	laps := container.NewVBox()

	racePlace := fmt.Sprintf("%s / %s / %s / %s", common.RacePlace, common.RaceDisqualification, common.RaceDidNotStart, common.RaceDidNotFinish)
	headers := []string{common.RaceOrderOfFinish, racePlace, common.RaceSplit, common.RaceTime}

	header := container.NewGridWithColumns(4)
	for _, h := range headers {
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

// winningTimeInput - container to collect official winning time for first boat that
// crosses finish line.  This reflects the total time from when the race began and finished.
// The note line under it says where a pre-filled value came from, or why there
// is none (persona-plan.md 2.1).
func (c *Clock) winningTimeInput() *fyne.Container {
	form := widget.NewForm(
		widget.NewFormItem(
			common.WinningTimeInputText,
			c.winningTime,
		),
	)

	c.winningNote = widget.NewLabel(common.EmptyString)
	c.winningNote.Wrapping = fyne.TextWrapWord
	c.winningNote.Importance = widget.MediumImportance
	c.winningNote.Hide()

	return container.NewVBox(form, c.winningNote)
}

// approvalPanel - container with buttons to make the results official.
func (c *Clock) approvalPanel() *fyne.Container {
	return container.NewHBox(
		layout.NewSpacer(),
		c.buttons.referee,
		layout.NewSpacer(),
		c.buttons.save,
		layout.NewSpacer(),
	)
}
