package clock

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/text"
	"github.com/comagnaw/regattaClock/internal/timesync"
	"github.com/comagnaw/regattaClock/internal/uitheme"
)

// Compare Secondary is the primary finish timer's read-only view of the
// SECONDARY team's committed result for the same race, in an independent window
// the operator places beside the clock. It is a visual aid only: the PFT
// reconciles by editing its OWN data, this window never writes the secondary
// file, and - unlike the Referee Approval window - it does not block the clock.
// See docs/features/personas/reconciliation.md.

func (c *Clock) compareIsOpen() bool { return c.compareWindow != nil }

// comparableSecondaryResult returns the secondary team's RaceResult for race n
// when there is one worth showing (a committed winning time), and whether there
// is. The button-click path reads the already-mirrored log - never disk.
func (c *Clock) comparableSecondaryResult(n int) (store.RaceResult, bool) {
	if c.secondaryFinish == nil {
		return store.RaceResult{}, false
	}
	res, ok := c.secondaryFinish.Races[n]
	if !ok || res.WinningTime == common.EmptyString {
		return store.RaceResult{}, false
	}
	return res, true
}

func (c *Clock) hasComparableSecondary(n int) bool {
	_, ok := c.comparableSecondaryResult(n)
	return ok
}

// refreshCompareButton enables the toggle when the secondary team has a result
// for this race, and keeps its label in step with whether the pane is open.
// Safe to call before the button exists.
func (c *Clock) refreshCompareButton() {
	if c.buttons.compare == nil {
		return
	}
	if c.compareIsOpen() {
		c.buttons.compare.SetText(common.CompareSecondaryHideText)
		c.buttons.compare.Enable()
		return
	}
	c.buttons.compare.SetText(common.CompareSecondaryButtonText)
	if c.hasComparableSecondary(c.raceData.RaceNumber) {
		c.buttons.compare.Enable()
	} else {
		c.buttons.compare.Disable()
	}
}

// toggleCompareSecondary opens the compare window or, if it is already up,
// closes it.
func (c *Clock) toggleCompareSecondary() {
	if c.compareIsOpen() {
		c.closeCompareSecondary()
		return
	}
	c.openCompareSecondary()
}

func (c *Clock) openCompareSecondary() {
	if c.compareWindow != nil {
		c.compareWindow.RequestFocus()
		return
	}
	res, ok := c.comparableSecondaryResult(c.raceData.RaceNumber)
	if !ok {
		return
	}

	w := c.App.NewWindow(fmt.Sprintf(common.CompareWindowTitle, c.raceData.RaceNumber))
	w.SetContent(c.compareBody(res))
	w.Resize(fyne.NewSize(comparePaneWidth, comparePaneHeight))
	// Fyne 2.8 has no API to place a window at an offset from another, so the
	// best we can do is centre it - predictable and easy to find, then the
	// operator drags it beside the clock (or onto a second screen).
	w.CenterOnScreen()
	w.SetOnClosed(func() {
		c.compareWindow = nil
		if c.clockClosed {
			return // the clock is tearing down; nothing to restore or raise
		}
		c.refreshCompareButton()
		// Raise the clock on the next event-loop tick. Doing it inline, mid
		// close, is too early: the window manager then picks the next focus
		// window itself and the operator lands on whatever was behind (the race
		// tree). fyne.Do defers to after the close completes. Mirrors the
		// Referee Approval window's SetOnClosed.
		fyne.Do(func() {
			c.window.Show()
			c.window.RequestFocus()
		})
	})

	c.compareWindow = w
	w.Show()
	c.refreshCompareButton()
}

func (c *Clock) closeCompareSecondary() {
	if c.compareWindow != nil {
		c.compareWindow.Close() // its SetOnClosed clears the field and the button
	}
}

// compareBody is the read-only mirror of the secondary team's result, laid out
// to parallel the live clock (same bands, same lap-grid columns, same lanes
// table) but with plain labels instead of inputs and no controls or approval.
func (c *Clock) compareBody(res store.RaceResult) fyne.CanvasObject {
	variant := clockThemeVariant()

	title := text.BannerHeading(fmt.Sprintf(common.CompareSecondaryBandFormat, c.raceData.RaceTitle()))
	title.Color = uitheme.BrandNavy // dark text on the amber band
	// The bands here are amber, not the live clock's blue, so this read-only
	// window can never be mistaken for the primary one at a glance. Pin the
	// header's inner height to the clock masthead's wordmark height so the two
	// windows still line up when placed side by side.
	header := uitheme.CautionBand(
		container.NewGridWrap(fyne.NewSize(title.MinSize().Width, bandLogoHeight), container.NewCenter(title)),
		zoneBandVPad,
	)

	secResults := initResults(c.raceData)
	for _, lr := range res.Rows {
		if lr.Lane >= 1 && lr.Lane <= 6 {
			secResults.updatePlace(lr.Lane, lr.Place)
			secResults.updateSplit(lr.Lane, lr.Split)
			secResults.updateTime(lr.Lane, lr.Time)
		}
	}
	resultsPanel, _ := newResultsTable(secResults, nil)

	// Reserve the vertical space the live clock spends on the run controls and
	// on the winning-time helper line, so the lap grid and the Results band line
	// up across the two panes for a straight visual scan.
	controlSpacer := vgap(widget.NewButton(common.StartButtonText, nil).MinSize().Height)
	noteSpacer := vgap(winningNoteHeight)

	return container.NewVBox(
		header,
		c.compareSkewBanner(),
		container.NewCenter(text.Header1(compareHeadlineTime(res))),
		controlSpacer,
		compareLapGrid(res.Rows),
		compareWinningLine(res.WinningTime),
		noteSpacer,
		uitheme.CautionBand(text.BoldLabelCenter(common.ClockResultsZoneLabel), zoneBandVPad),
		uitheme.FullBleed(uitheme.ReverseCard(variant, container.NewVBox(container.NewPadded(resultsPanel)))),
	)
}

// compareHeadlineTime is what the read-only pane shows where the live clock has
// its stopwatch: the last split - the elapsed the secondary timer had on the
// board when the last boat crossed, which is what the primary's stopwatch shows
// when Stop is pressed. "00:00.0" when the secondary recorded no splits.
func compareHeadlineTime(res store.RaceResult) string {
	last := common.ZeroTime
	for _, r := range res.Rows {
		if r.Split != common.EmptyString {
			last = r.Split
		}
	}
	return last
}

// compareLapGrid mirrors lapsContainer with read-only labels built from the
// secondary result's rows.
func compareLapGrid(rows []store.LapRow) *fyne.Container {
	grid := container.NewVBox(lapHeaderRow(), vgap(lapRowGap))
	for i := range 6 {
		var lr store.LapRow
		if i < len(rows) {
			lr = rows[i]
		}
		oof := common.EmptyString
		if lr.Lane >= 1 && lr.Lane <= 6 {
			oof = strconv.Itoa(lr.Lane)
		}
		row := container.NewHBox(
			lapCell(lapOOFColWidth, widget.NewLabel(oof)),
			lapCell(lapPlaceColWidth, widget.NewLabel(lr.Place)),
			lapCell(lapSplitColWidth, widget.NewLabel(lr.Split)),
			lapCell(lapTimeColWidth, widget.NewLabel(lr.Time)),
		)
		if i > 0 {
			grid.Add(vgap(lapRowGap))
		}
		grid.Add(row)
	}
	return container.NewCenter(grid)
}

// compareWinningLine mirrors winningTimeInput's row, left-aligned to the lap
// grid, as a read-only label.
func compareWinningLine(winningTime string) *fyne.Container {
	inner := container.NewHBox(
		text.BoldLabel(common.WinningTimeInputText),
		widget.NewLabel(orZero(winningTime)),
	)
	gridW := lapGridWidth()
	return container.NewVBox(
		vgap(winningTopGap),
		container.NewCenter(container.NewGridWrap(fyne.NewSize(gridW, inner.MinSize().Height), inner)),
	)
}

// compareSkewBanner is an amber caution strip shown only when the two timers'
// machine clocks disagree by more than the skew threshold, so the operator
// knows the raw winning times are not directly comparable
// (reconciliation.md - clock skew between the two FT machines).
func (c *Clock) compareSkewBanner() fyne.CanvasObject {
	label := text.Wrapping(common.EmptyString)
	strip := uitheme.CautionStrip(container.NewPadded(label))
	strip.Hide()

	if c.finishLog == nil || c.secondaryFinish == nil {
		return strip
	}
	a, b := c.finishLog.Envelope, c.secondaryFinish.Envelope
	if !strings.HasPrefix(a.Clock.Source, "ntp:") || !strings.HasPrefix(b.Clock.Source, "ntp:") {
		return strip
	}
	delta := a.Clock.Offset - b.Clock.Offset
	if delta < 0 {
		delta = -delta
	}
	if delta <= timesync.SkewWarnThreshold {
		return strip
	}
	label.SetText(fmt.Sprintf(common.CompareSkewNoteFormat,
		a.Machine, b.Machine, fmt.Sprintf("%.1fs", delta.Seconds())))
	strip.Show()
	return strip
}

// orZero renders an empty string as the zero-time placeholder, matching what the
// live clock shows before a value is entered.
func orZero(s string) string {
	if s == common.EmptyString {
		return common.ZeroTime
	}
	return s
}
