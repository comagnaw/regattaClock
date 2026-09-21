package clock

import (
	"image/color"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/reader"
	"github.com/comagnaw/regattaClock/internal/text"
)

// approvals - represents the fyne Container that is used to present
// race results to referee for approval
type approvals struct {
	*fyne.Container
}

// initApprovalContainer - generates an approvals with a header. The grid uses
// scalingGridLayout: weighted columns (narrow OOF/Place, wide School) and a font
// that scales with the Referee Approval window (referee_window.go).
func initApprovalContainer() *approvals {
	a := &approvals{
		container.New(scalingGridLayout{}),
	}
	a.setHeader()
	return a
}

// approvalCell - one grid cell: a background rectangle behind the text. Colours
// are passed in explicitly (not read from the current theme) because the
// window's ThemeOverride re-themes widgets but not raw canvas.Text.
func approvalCell(str string, fg, bg color.Color, leading bool) *fyne.Container {
	t := text.Cell(str)
	t.Color = fg
	if leading {
		t.Alignment = fyne.TextAlignLeading
	}
	return container.NewStack(
		canvas.NewRectangle(bg),
		container.NewPadded(t),
	)
}

// setHeader - populates the first row of the grid with the table header, in
// reverse contrast: a dark background with the lightest table white as the text.
func (a *approvals) setHeader() {
	headers := []string{common.RaceOrderOfFinish, common.RacePlace, common.RaceSplit, common.RaceTime, common.RaceSchool}
	fg := refereeColor(theme.ColorNameBackground)     // lightest white used in the table
	bg := refereeDarkColor(theme.ColorNameBackground) // default dark background
	for i, header := range headers {
		a.Add(approvalCell(header, fg, bg, i == 4))
	}
}

// setRow - append one results row, striped so rows read across.
func (a *approvals) setRow(row int, cells []string) {
	fg := refereeColor(theme.ColorNameForeground)
	bg := refereeColor(theme.ColorNameBackground)
	if row%2 != 0 {
		bg = refereeColor(theme.ColorNameInputBackground)
	}
	for i, cell := range cells {
		a.Add(approvalCell(cell, fg, bg, i == 4))
	}
}

// ApprovalWindowContent wraps grid in the same light-themed, race-title-headed
// layout the Referee Approval window uses (fixedVariantTheme, a background
// rectangle so the whole window stays light even in dark mode, a VScroll'd
// grid, and a bottom-anchored footer) - footer holds the caller's own
// buttons (Approve/Cancel for the live clock; a single Close for a read-only
// viewer). Exported so internal/regatta's Director/Awards results window
// renders the exact same look a Referee saw, not a re-derived approximation.
func ApprovalWindowContent(raceTitle string, grid *fyne.Container, footer fyne.CanvasObject) fyne.CanvasObject {
	// Header1 (large) so it reads at the same distance the Referee Approval
	// window is designed for.
	title := text.Header1(raceTitle)
	title.Color = refereeColor(theme.ColorNameForeground) // canvas.Text ignores the ThemeOverride

	body := container.NewBorder(
		container.NewCenter(title),
		footer,
		nil, nil,
		container.NewVScroll(grid),
	)

	// The window canvas paints the *global* theme's background behind everything,
	// and a ThemeOverride does not change that - so lay an explicit light
	// rectangle under the content to keep the whole window light in dark mode.
	return container.NewStack(
		canvas.NewRectangle(refereeColor(theme.ColorNameBackground)),
		container.NewThemeOverride(body, refereeTheme),
	)
}

// ApprovalGrid renders the exact grid the Referee Approval window shows
// (OOF, Place, Split, Time, School - scalingGridLayout, striped rows) from a
// store.RaceResult's own Rows, joined against race for school names. Rows is
// already in finish order (places first, then any DQ/DNF/DNS rows) - the
// same order results.asApprovals produces from a live clock's oofLanes, since
// that is the order a committed RaceResult's Rows were written in
// (approval.go's own setRow numbering during Referee Approval). Unlike
// results.asApprovals, which reads live laps/results Clock state, this works
// from a plain value, so a read-only viewer with no live clock (the
// Director/Awards results window) can render the identical grid a Referee
// saw and approved. Exported for internal/regatta's read-only race-tree
// personas.
func ApprovalGrid(race reader.RaceData, rows []store.LapRow) *fyne.Container {
	a := initApprovalContainer()
	for i, lr := range rows {
		oof := common.EmptyString
		school := common.EmptyString
		if lr.Lane >= 1 && lr.Lane <= 6 {
			oof = strconv.Itoa(lr.Lane)
			school = race.Lanes[lr.Lane].SchoolName
		}
		a.setRow(i+1, []string{oof, lr.Place, lr.Split, lr.Time, school})
	}
	return a.Container
}
