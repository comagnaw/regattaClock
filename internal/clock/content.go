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
	"github.com/comagnaw/regattaClock/internal/persona/store"
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
// window heading, read as one centred phrase: the regattaClock wordmark, then
// "timing for <race title>", both in the band's forced-light contrast.
// c.raceTitle is kept so UpdateSchedule can retitle it in place.
func (c *Clock) timingBand() fyne.CanvasObject {
	c.raceTitle = text.BannerHeading(fmt.Sprintf(common.ClockTimingForFormat, c.raceData.RaceTitle()))
	c.raceTitle.Color = uitheme.White

	phrase := container.NewHBox(
		container.NewCenter(bandLogo(bandLogoHeight)),
		hgap(bandLogoGap),
		container.NewCenter(c.raceTitle),
	)
	return uitheme.AccentBand(container.NewCenter(phrase), zoneBandVPad)
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

// content - the race clock laid out as a fixed top over a scrollable bottom,
// so a high-DPI-scaled display (e.g. Windows at 150%) that can't fit the
// whole window on screen loses the least-time-critical content first rather
// than clipping unreachable content with no way to get to it. The top -
// the "Timing" band (which also carries the race title and the wordmark, so
// it doubles as the window heading), the stopwatch, run controls beside the
// approval buttons with the commit-status line under them, lap grid, and
// winning-time field - is never scrolled: per AGENTS.md, timing clicks are
// the highest-priority path, and the approval buttons (Referee/Save/Close)
// are exactly what an operator still needs to *act on*, so both must always
// be reachable at full size. Below that, only the "Results" band over a
// reverse-contrast card (the per-lane readout, already known once captured)
// sits inside container.NewVScroll, which - per Fyne's own behavior, not any
// custom screen-size detection - only shows a scrollbar when its content
// doesn't fit; on a normal-DPI display this renders identically to before.
func (c *Clock) content() *fyne.Container {
	variant := clockThemeVariant()

	timing := container.NewVBox(
		container.NewCenter(c.clock),
		c.controlsAndApprovalPanel(),
		c.lapsContainer(),
		c.winningTimeInput(),
	)

	top := container.NewVBox(
		c.skewBannerWidget(),
		c.scheduleBannerWidget(),

		c.timingBand(),
		timing,
	)

	results := container.NewVBox(
		container.NewPadded(c.resultsPanel()),
	)

	bottom := container.NewVBox(
		uitheme.AccentBand(text.BoldLabelCenter(common.ClockResultsZoneLabel), zoneBandVPad),
		uitheme.FullBleed(uitheme.ReverseCard(variant, results)),
	)

	root := container.NewBorder(top, nil, nil, nil, container.NewVScroll(bottom))
	c.refreshCompareButton()
	return root
}

// resultsPanel is the live clock's lanes table. The *widget.Table handle is
// kept on the clock so a schedule refresh can repaint lane labels in place;
// changed lanes are drawn in a warning style (persona-plan.md 3c).
func (c *Clock) resultsPanel() *fyne.Container {
	panel, tbl := newResultsTable(c.results, c.changedLanes)
	c.resultsTable = tbl
	return panel
}

// newResultsTable builds the six-row lanes table over res, sized so its viewport
// shows every cell with no scrollbars. Shared by the live clock and the
// read-only Compare Secondary pane so the two read identically. changed may be
// nil (the compare pane never highlights schedule conflicts).
func newResultsTable(res results, changed map[int]bool) (*fyne.Container, *widget.Table) {
	tbl := widget.NewTable(
		func() (int, int) { return len(res), len(res[0]) },
		func() fyne.CanvasObject {
			return text.TruncatingCenter("wide wide wide content")
		},
		func(i widget.TableCellID, o fyne.CanvasObject) {
			label := o.(*widget.Label)
			label.SetText(res[i.Row][i.Col])
			if changed != nil && (i.Row == schoolRow || i.Row == additionalRow) && i.Col >= 1 && changed[i.Col] {
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
	cols := len(res[0])
	rows := len(res)
	pad := theme.Padding()
	lanes := float32(cols - 1)

	laneW := (resultsWidth - resultsLabelColWidth - lanes*pad) / lanes
	tbl.SetColumnWidth(0, resultsLabelColWidth)
	for col := 1; col < cols; col++ {
		tbl.SetColumnWidth(col, laneW)
	}
	for row := range rows {
		tbl.SetRowHeight(row, resultsRowHeight)
	}

	size := fyne.NewSize(
		resultsLabelColWidth+lanes*laneW+lanes*pad+2,
		(resultsRowHeight+pad)*float32(rows)+2,
	)

	return container.NewGridWrap(size, container.NewStack(tbl)), tbl
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

// controlsAndApprovalPanel - the run controls (Start/Lap/Stop/Clear) beside
// whichever approval buttons currently apply, on one row, with the
// commit-status line directly under it. The primary FT gets Compare
// Secondary (only once there is a secondary mirror to compare) and Referee
// Approval + Close (Close disabled until approved); the Secondary Finish
// Timer has no Referee Approval step (reconciliation.md) - just Save.
// Keeping this whole block out of the scrollable region means the buttons
// an operator must act on are always reachable regardless of window height
// (the PC-scale overflow fix).
func (c *Clock) controlsAndApprovalPanel() *fyne.Container {
	controls := container.NewHBox(
		c.buttons.start,
		hgap(controlGap),
		c.buttons.lap,
		hgap(controlGap),
		c.buttons.stop,
		hgap(controlGap),
		c.buttons.clear,
	)

	var approval []fyne.CanvasObject
	if c.isPrimaryFinish() && c.secondaryFinish != nil {
		approval = append(approval, c.buttons.compare, hgap(controlGap))
	}
	if c.isSecondaryFinish() {
		approval = append(approval, c.buttons.save)
	} else {
		approval = append(approval, c.buttons.referee, hgap(controlGap), c.buttons.close)
	}

	row := container.New(
		layout.NewCustomPaddedLayout(0, 0, controlRowSideMargin, controlRowSideMargin),
		container.NewBorder(nil, nil, controls, container.NewHBox(approval...)),
	)

	return container.NewVBox(
		row,
		uitheme.AccentBand(c.commitStatus, zoneBandVPad),
	)
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
// the first boat across the line (the total from race start to finish),
// with a note to its right saying where a pre-filled value came from, or why
// there is none (persona-plan.md 2.1). The note has a fixed width so its text
// changing doesn't shift the entry field.
func (c *Clock) winningTimeInput() *fyne.Container {
	label := text.BoldLabel(common.WinningTimeInputText)
	entry := container.NewGridWrap(
		fyne.NewSize(winningEntryWidth, c.winningTime.MinSize().Height),
		c.winningTime,
	)

	c.winningNote = text.Wrapping(common.EmptyString)
	c.winningNote.Importance = widget.MediumImportance
	noteArea := container.NewGridWrap(fyne.NewSize(winningNoteWidth, c.winningTime.MinSize().Height), c.winningNote)

	row := container.NewCenter(container.NewHBox(label, entry, hgap(controlGap), noteArea))

	return container.NewVBox(
		vgap(winningTopGap),
		row,
	)
}

// initCommitStatus - build the status line under the approval panel. It starts
// at Pending and refreshCommitStatus advances it as the race is persisted.
func (c *Clock) initCommitStatus() {
	c.commitStatus = widget.NewLabel(store.StateNotStarted.DisplayText(c.session.Team))
	c.commitStatus.Alignment = fyne.TextAlignCenter
	c.commitStatus.Importance = widget.MediumImportance
}

