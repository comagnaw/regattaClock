package text

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

// Title - returns title format of fyne Text
func Title(t string) *canvas.Text {
	return newText(t, color.White, false, true, fyne.TextAlignCenter, 48)
}

// Cell - returns cell format of fyne Text
func Cell(t string) *canvas.Text {
	return newText(t, color.Black, false, true, fyne.TextAlignCenter, 48)
}

// newText - with provided input, return fyne Text
func newText(t string, c color.Color, mono, bold bool, align fyne.TextAlign, size float32) *canvas.Text {
	return &canvas.Text{
		Text:      t,
		Color:     c,
		TextStyle: fyne.TextStyle{Monospace: mono, Bold: bold},
		Alignment: align,
		TextSize:  size,
	}
}
