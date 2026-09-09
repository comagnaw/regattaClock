package clock

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"

	"github.com/comagnaw/regattaClock/internal/common"
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
