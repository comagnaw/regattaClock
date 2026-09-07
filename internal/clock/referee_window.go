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

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/text"
)

// fixedVariantTheme pins a theme to one light/dark variant, ignoring the app
// setting. The Referee Approval window is always light: referees read it at a
// distance and the striped results table was designed for a light background.
type fixedVariantTheme struct {
	fyne.Theme
	variant fyne.ThemeVariant
}

func (t fixedVariantTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return t.Theme.Color(name, t.variant)
}

// refereeTheme is the always-light theme for the Referee Approval window. It
// wraps the window (container.NewThemeOverride) so its widgets render light, and
// approval.go reads the cell colours from it directly since a ThemeOverride does
// not recolour raw canvas.Text.
var refereeTheme fyne.Theme = fixedVariantTheme{
	Theme:   theme.DefaultTheme(),
	variant: theme.VariantLight,
}

func refereeColor(name fyne.ThemeColorName) color.Color {
	return refereeTheme.Color(name, theme.VariantLight)
}

// refereeColWeights is the fraction of the grid width each column gets: OOF and
// Place are narrow (but wide enough for their headers), School is the widest.
var refereeColWeights = [refereeCols]float32{0.11, 0.15, 0.17, 0.17, 0.40}

// scalingGridLayout lays the 5-column approvals grid into weighted columns and
// scales the cell font with the window: it grows toward refereeFontDesign as the
// window is widened and shrinks to refereeFontMin so the five columns always fit
// without a horizontal scrollbar. The font is driven by the School column, the
// binding constraint; the short columns hold "1" / "DQ" / "01:23.4" easily at
// that size.
type scalingGridLayout struct{}

func (scalingGridLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) < refereeCols {
		return
	}

	var colW [refereeCols]float32
	for i := range colW {
		colW[i] = size.Width * refereeColWeights[i]
	}

	fs := colW[refereeCols-1] / refereeCharWidthDivisor
	switch {
	case fs < refereeFontMin:
		fs = refereeFontMin
	case fs > refereeFontDesign:
		fs = refereeFontDesign
	}

	cellH := fs*refereeRowHeightFactor + theme.Padding()*2

	for idx, o := range objs {
		if t := cellText(o); t != nil && t.TextSize != fs {
			t.TextSize = fs
			t.Refresh()
		}
		col := idx % refereeCols
		row := idx / refereeCols
		var x float32
		for i := range col {
			x += colW[i]
		}
		// Inset each cell by a gutter so a wide value (e.g. "OOF" at 48pt) keeps
		// clear air from the next column instead of bleeding into it.
		o.Move(fyne.NewPos(x+refereeColGutter/2, float32(row)*cellH))
		o.Resize(fyne.NewSize(colW[col]-refereeColGutter, cellH))
	}
}

func (scalingGridLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	if len(objs) < refereeCols {
		return fyne.Size{}
	}
	rows := len(objs) / refereeCols
	cellH := refereeFontDesign*refereeRowHeightFactor + theme.Padding()*2
	return fyne.NewSize(refereeMinGridWidth, float32(rows)*cellH)
}

// cellText finds the canvas.Text inside a grid cell (Stack{ rect, Padded{ Text } }).
func cellText(o fyne.CanvasObject) *canvas.Text {
	switch v := o.(type) {
	case *canvas.Text:
		return v
	case *fyne.Container:
		for _, child := range v.Objects {
			if t := cellText(child); t != nil {
				return t
			}
		}
	}
	return nil
}

// showRefereeeApproval opens the Referee Approval results in its own movable
// window (so it can be dragged onto a referee-facing screen), pinned to the
// light theme, with the grid font scaling to fit. Fyne has no cross-window
// modal, so "block the clock" is done by disabling the clock's controls while
// the window is open.
func (c *Clock) showRefereeeApproval(raceNumber int) {
	if c.refereeWindow != nil {
		c.refereeWindow.RequestFocus()
		return
	}

	c.blockClockForReferee()

	grid := c.results.asApprovals(c.laps.getOOFLanes())

	approve := widget.NewButton(common.ApproveButtonText, func() {
		c.refereeApprovalFunc(raceNumber)(true)
		c.closeRefereeWindow()
	})
	approve.Importance = widget.HighImportance
	cancel := widget.NewButton(common.CancelButtonText, func() {
		c.closeRefereeWindow()
	})

	// Header1 (large) so a referee glancing at the window sees which race it is.
	title := text.Header1(c.raceData.RaceTitle())
	title.Color = refereeColor(theme.ColorNameForeground) // canvas.Text ignores the ThemeOverride

	body := container.NewBorder(
		container.NewCenter(title),
		container.NewHBox(layout.NewSpacer(), approve, layout.NewSpacer(), cancel, layout.NewSpacer()),
		nil, nil,
		container.NewVScroll(grid.Container),
	)

	// The window canvas paints the *global* theme's background behind everything,
	// and a ThemeOverride does not change that - so lay an explicit light
	// rectangle under the content to keep the whole window light in dark mode.
	content := container.NewStack(
		canvas.NewRectangle(refereeColor(theme.ColorNameBackground)),
		container.NewThemeOverride(body, refereeTheme),
	)

	w := c.App.NewWindow(fmt.Sprintf(common.RefereeApproveTitle, raceNumber))
	w.SetContent(content)
	w.Resize(fyne.NewSize(refereeWinWidth, refereeWinHeight))
	w.CenterOnScreen()
	w.SetOnClosed(func() {
		c.refereeWindow = nil
		c.unblockClockAfterReferee()
		c.window.RequestFocus() // return the operator to the clock, not whatever is behind it
	})
	c.refereeWindow = w
	w.Show()
}

// closeRefereeWindow closes the approval window if it is open. Its SetOnClosed
// clears c.refereeWindow and restores the clock, so this is safe to call more
// than once (e.g. a double-tapped Approve).
func (c *Clock) closeRefereeWindow() {
	if c.refereeWindow != nil {
		c.refereeWindow.Close()
	}
}

// blockClockForReferee disables every clock control while the approval window is
// open. unblockClockAfterReferee restores them from the current race state.
func (c *Clock) blockClockForReferee() {
	for _, b := range c.blockableButtons() {
		if b != nil {
			b.Disable()
		}
	}
	c.winningTime.Disable()
	for i := range c.laps {
		c.laps[i].oofLaneNum.Disable()
		c.laps[i].place.Disable()
	}
}

func (c *Clock) unblockClockAfterReferee() {
	c.buttons.start.Enable()
	c.buttons.lap.Enable()
	c.buttons.stop.Enable()
	c.buttons.clear.Enable()
	for i := range c.laps {
		c.laps[i].place.Enable()
	}

	if c.isNotRunning() {
		c.winningTime.Enable()
	}
	if c.winningTime.Text != common.EmptyString {
		if _, err := parseTime(c.winningTime.Text); err == nil {
			c.commitButton().Enable()
		}
	}
	c.refreshCommitStatus() // Close button + status line
	c.refreshContent()      // OOF entry enablement from the running state
}

func (c *Clock) blockableButtons() []*widget.Button {
	return []*widget.Button{
		c.buttons.start, c.buttons.lap, c.buttons.stop, c.buttons.clear,
		c.buttons.referee, c.buttons.save, c.buttons.close,
	}
}
