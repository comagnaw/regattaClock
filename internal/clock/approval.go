package clock

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/text"
)

// approvals - represents the fyne Container that is used to present
// race results to referee for approval
type approvals struct {
	*fyne.Container
}

// initApprovalContainer - generates an approvals with a header
func initApprovalContainer() *approvals {
	a := &approvals{
		container.NewGridWithColumns(5),
	}
	a.setHeader()
	return a
}

// setHeader - populates the first 5 entries of grid with table header information
func (a *approvals) setHeader() {
	headers := []string{common.RaceOrderOfFinish, common.RacePlace, common.RaceSplit, common.RaceTime, common.RaceSchool}

	for i, header := range headers {

		rect := canvas.NewRectangle(color.White)
		if i <= 2 {
			rect.Resize(fyne.NewSize(100, 100))
		} else {
			rect.Resize(fyne.NewSize(200, 100))
		}

		cell := container.NewStack(
			rect,
			container.NewPadded(text.Cell(header)),
		)
		a.Add(cell)
	}
}

// setRow - with provided row and cells, populate 5 entries of grid with results
func (a *approvals) setRow(row int, cells []string) {
	for i, cell := range cells {

		var bgColor color.Color
		if row%2 == 0 {
			bgColor = color.White
		} else {
			bgColor = color.RGBA{R: 217, G: 217, B: 217, A: 255} // Light gray
		}
		rect := canvas.NewRectangle(bgColor)
		if i <= 2 {
			rect.Resize(fyne.NewSize(100, 100))
		} else {
			rect.Resize(fyne.NewSize(200, 100))
		}

		text := text.Cell(cell)
		if i == 4 { // School column
			text.Alignment = fyne.TextAlignLeading
		}

		newCell := container.NewStack(
			rect,
			container.NewPadded(text),
		)
		a.Add(newCell)
	}
}
