package clock

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/assets"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/text"
	"github.com/comagnaw/regattaClock/internal/uitheme"
)

// hgap / vgap - fixed-size transparent spacers, for putting deliberate air
// between widgets in an HBox / VBox (layout.NewSpacer expands, which collapses
// to nothing inside a container.Center).
func hgap(w float32) *canvas.Rectangle {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(w, 1))
	return r
}

func vgap(h float32) *canvas.Rectangle {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(1, h))
	return r
}

// clockThemeVariant is the app's chosen light/dark variant, read from the same
// preference the main window writes (fyne's Settings().ThemeVariant() reports the
// OS appearance, which the in-app Light/Dark choice overrides). It drives the
// direction of the reverse-contrast Results card. Defaults to dark, matching the
// main window's own fallback.
func clockThemeVariant() fyne.ThemeVariant {
	if fyne.CurrentApp().Preferences().String(common.PrefTheme) == common.PrefLight {
		return theme.VariantLight
	}
	return theme.VariantDark
}

// timingBand - the accent band that opens the Timing zone. It doubles as the
// window heading: "Timing for <race title>" on the left, the regattaClock
// wordmark on the right, both in the band's forced-light contrast. c.raceTitle
// is kept so UpdateSchedule can retitle it in place.
func (c *Clock) timingBand() fyne.CanvasObject {
	c.raceTitle = text.BannerHeading(fmt.Sprintf(common.ClockTimingForFormat, c.raceData.RaceTitle()))
	c.raceTitle.Color = uitheme.White

	logo := container.New(
		layout.NewCustomPaddedLayout(0, 0, 0, bandLogoRightPad),
		bandLogo(bandLogoHeight),
	)
	return uitheme.AccentBand(
		container.NewBorder(nil, nil, nil, logo, c.raceTitle),
		zoneBandVPad,
	)
}

// bandLogo - the regattaClock wordmark at a fixed height, its native white fill
// (viewBox 2793x430) left as authored so it reads on the blue band. Not run
// through theme.NewThemedResource, which would recolour it to the app theme.
func bandLogo(height float32) *canvas.Image {
	res := fyne.NewStaticResource(common.BannerResourceName, assets.RegattaClockBanner)
	img := canvas.NewImageFromResource(res)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(height*bandLogoAspect, height))
	return img
}

// content - the race clock laid out in two banded zones: a "Timing" band (which
// also carries the race title and the wordmark, so it doubles as the window
// heading) over the stopwatch / run controls / lap grid / winning-time field,
// then a "Results" band over a reverse-contrast card (the per-lane readout),
// matching the race tree's accent-band / reverse-card visual language. The
// referee / save panel sits below on the plain surface.
func (c *Clock) content() *fyne.Container {
	variant := clockThemeVariant()

	timing := container.NewVBox(
		container.NewCenter(c.clock),
		c.controlPanel(),
		c.lapsContainer(),
		c.winningTimeInput(),
	)

	results := container.NewVBox(
		container.NewPadded(c.resultsPanel()),
	)

	return container.NewVBox(
		c.skewBannerWidget(),
		c.scheduleBannerWidget(),

		c.timingBand(),
		timing,

		uitheme.AccentBand(text.BoldLabelCenter(common.ClockResultsZoneLabel), zoneBandVPad),
		uitheme.FullBleed(uitheme.ReverseCard(variant, results)),

		c.approvalPanel(),
	)
}

// resultsPanel is the lanes table with the handle kept, so a schedule refresh
// can repaint lane labels in place, and with changed lanes drawn in a warning
// style (persona-plan.md 3c). School names ellipsize inside their fixed column
// rather than widening the panel.
func (c *Clock) resultsPanel() *fyne.Container {
	c.resultsTable = widget.NewTable(
		func() (int, int) { return len(c.results), len(c.results[0]) },
		func() fyne.CanvasObject {
			return text.TruncatingCenter("wide wide wide content")
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

	// Pin every column and row so the table's content size is known exactly, then
	// size the viewport to match. widget.Table lays a theme padding between each
	// cell, so that is folded into both the lane-column width (six lanes fill the
	// card, no wasted strip) and the viewport (+2px absorbs float rounding; a
	// hairline of card beats a scrollbar clipping the last row).
	cols := len(c.results[0])
	rows := len(c.results)
	pad := theme.Padding()
	lanes := float32(cols - 1)

	laneW := (resultsWidth - resultsLabelColWidth - lanes*pad) / lanes
	c.resultsTable.SetColumnWidth(0, resultsLabelColWidth)
	for col := 1; col < cols; col++ {
		c.resultsTable.SetColumnWidth(col, laneW)
	}
	for row := range rows {
		c.resultsTable.SetRowHeight(row, resultsRowHeight)
	}

	size := fyne.NewSize(
		resultsLabelColWidth+lanes*laneW+lanes*pad+2,
		(resultsRowHeight+pad)*float32(rows)+2,
	)

	return container.NewGridWrap(size, container.NewStack(c.resultsTable))
}

// skewBannerWidget builds the (initially hidden) clock-skew banner. checkSkew
// fills the label and shows it; the Dismiss button hides it for good
// (persona-plan.md 2.1).
func (c *Clock) skewBannerWidget() fyne.CanvasObject {
	c.skewLabel = text.Wrapping(common.EmptyString)
	dismiss := widget.NewButton(common.DismissButtonText, func() {
		c.skewDismissed = true
		c.skewBanner.Hide()
	})
	c.skewBanner = container.NewBorder(nil, nil, nil, dismiss, c.skewLabel)
	c.skewBanner.Hide()
	return c.skewBanner
}

// controlPanel - the run controls, at their natural size and centred rather than
// stretched across the whole frame.
func (c *Clock) controlPanel() *fyne.Container {
	return container.NewCenter(container.NewHBox(
		c.buttons.start,
		hgap(controlGap),
		c.buttons.lap,
		hgap(controlGap),
		c.buttons.stop,
		hgap(controlGap),
		c.buttons.clear,
	))
}

// lapsContainer - the lap grid: a bold header row over six fixed-width data
// rows, updated as the clock runs and the Lap button is pushed. Also collects
// order-of-finish (OOF) and adjustment of place and split time.
func (c *Clock) lapsContainer() *fyne.Container {
	grid := container.NewVBox(lapHeaderRow(), vgap(lapRowGap))

	for rowNum := range c.laps {
		c.laps[rowNum] = lapRow{
			oofLaneNum:     c.oofEntry(rowNum),
			place:          c.initPlace(rowNum),
			split:          widget.NewEntry(),
			calculatedTime: widget.NewLabel(common.EmptyString),
		}
		if rowNum > 0 {
			grid.Add(vgap(lapRowGap))
		}
		grid.Add(c.laps[rowNum].asGridRow())
	}
	return container.NewCenter(grid)
}

// winningTimeInput - a compact labelled field for the official winning time of
// the first boat across the line (the total from race start to finish). The
// note line under it says where a pre-filled value came from, or why there is
// none (persona-plan.md 2.1); its space is reserved up front so the window
// geometry never changes after Start is pressed.
func (c *Clock) winningTimeInput() *fyne.Container {
	entry := container.NewGridWrap(
		fyne.NewSize(winningEntryWidth, c.winningTime.MinSize().Height),
		c.winningTime,
	)
	row := container.NewCenter(container.NewHBox(
		text.BoldLabel(common.WinningTimeInputText),
		entry,
	))

	c.winningNote = text.Wrapping(common.EmptyString)
	c.winningNote.Importance = widget.MediumImportance
	noteArea := container.NewGridWrap(fyne.NewSize(resultsWidth, winningNoteHeight), c.winningNote)

	return container.NewVBox(row, container.NewCenter(noteArea))
}

// initCommitStatus - build the status line under the approval panel. It starts
// at Pending and refreshCommitStatus advances it as the race is persisted.
func (c *Clock) initCommitStatus() {
	c.commitStatus = widget.NewLabel(common.CommitStatusPending)
	c.commitStatus.Alignment = fyne.TextAlignCenter
	c.commitStatus.Importance = widget.MediumImportance
}

// approvalPanel - the panel that makes the results official, with a status line
// under it. The primary FT gets Referee Approval + Close (Close disabled until
// approved); the Secondary Finish Timer has no Referee Approval step
// (reconciliation.md) - its panel is a single Save and Close button.
func (c *Clock) approvalPanel() *fyne.Container {
	var row *fyne.Container
	if c.isSecondaryFinish() {
		row = container.NewHBox(layout.NewSpacer(), c.buttons.save, layout.NewSpacer())
	} else {
		row = container.NewHBox(
			layout.NewSpacer(),
			c.buttons.referee,
			layout.NewSpacer(),
			c.buttons.close,
			layout.NewSpacer(),
		)
	}
	return container.NewVBox(row, c.commitStatus)
}
