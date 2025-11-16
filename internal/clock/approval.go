package clock

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

type approvals struct {
	*fyne.Container
}

func initApprovalContainer() *approvals {
	a := &approvals{
		container.NewGridWithColumns(5),
	}
	a.setHeader()
	return a
}

func (a *approvals) setHeader() {
	for _, h := range []string{"OOF", "Place", "Split", "Time", "School"} {
		rect := canvas.NewRectangle(color.White)
		rect.Resize(fyne.NewSize(200, 100))

		header := canvas.NewText(h, color.Black)
		header.TextStyle = fyne.TextStyle{Bold: true}
		header.TextSize = 48

		cell := container.NewStack(
			rect,
			container.NewPadded(header),
		)
		a.Add(cell)
	}
}

func (a *approvals) setRow(row int, cells []string) {
	for i, cell := range cells {

		var bgColor color.Color
		if row%2 == 0 {
			bgColor = color.White
		} else {
			bgColor = color.RGBA{R: 217, G: 217, B: 217, A: 255} // Light gray
		}
		rect := canvas.NewRectangle(bgColor)
		rect.Resize(fyne.NewSize(200, 100))

		text := canvas.NewText(cell, color.Black)
		text.TextStyle = fyne.TextStyle{Monospace: true}
		text.TextSize = 48
		if i == 4 { // School column
			text.Alignment = fyne.TextAlignLeading
		} else {
			text.Alignment = fyne.TextAlignCenter
		}

		newCell := container.NewStack(
			rect,
			container.NewPadded(text),
		)
		a.Add(newCell)
	}
}
