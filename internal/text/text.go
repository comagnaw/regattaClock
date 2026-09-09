package text

import (
	// "image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// BoldLabel - a bold widget.Label, for interactive layouts (column headers,
// table headings) where a canvas.Text cannot participate. The canvas.Text
// helpers above are for static display copy.
func BoldLabel(t string) *widget.Label {
	l := widget.NewLabel(t)
	l.TextStyle = fyne.TextStyle{Bold: true}
	return l
}

// BoldLabelCenter - a BoldLabel centred in its cell, for a column header that
// should sit over centred values.
func BoldLabelCenter(t string) *widget.Label {
	l := BoldLabel(t)
	l.Alignment = fyne.TextAlignCenter
	return l
}

// Truncating - a widget.Label that clips with an ellipsis rather than
// overflowing its container, for fixed-width table / list cells.
func Truncating(t string) *widget.Label {
	l := widget.NewLabel(t)
	l.Truncation = fyne.TextTruncateEllipsis
	return l
}

// TruncatingTrailing - a Truncating label aligned to the trailing edge, for a
// value that should sit against the right of its column.
func TruncatingTrailing(t string) *widget.Label {
	l := Truncating(t)
	l.Alignment = fyne.TextAlignTrailing
	return l
}

// TruncatingCenter - a Truncating label centred in its column.
func TruncatingCenter(t string) *widget.Label {
	l := Truncating(t)
	l.Alignment = fyne.TextAlignCenter
	return l
}

// Header1 - returns title format of fyne Text
func Header1(t string) *canvas.Text {
	return newText(t, false, true, fyne.TextAlignCenter, 48)
}

// Header2 - returns title format of fyne Text
func Header2(t string) *canvas.Text {
	return newText(t, false, true, fyne.TextAlignCenter, 24)
}

// Header3 - returns title format of fyne Text
func Header3(t string) *canvas.Text {
	return newText(t, false, true, fyne.TextAlignCenter, 20)
}

// Cell - returns cell format of fyne Text
func Cell(t string) *canvas.Text {
	return newText(t, false, true, fyne.TextAlignCenter, 48)
}

// Bold - returns cell format of fyne Text
func Bold(t string) *canvas.Text {
	return newText(t, false, true, fyne.TextAlignCenter, 16)
}

// BoldLeading - returns Bold format of fyne Text aligned to the leading edge,
// for body copy that should read as a left aligned block rather than a heading
func BoldLeading(t string) *canvas.Text {
	return newText(t, false, true, fyne.TextAlignLeading, 16)
}

// newText - with provided input, return fyne Text
func newText(t string, mono, bold bool, align fyne.TextAlign, size float32) *canvas.Text {
	return &canvas.Text{
		Text:      t,
		TextStyle: fyne.TextStyle{Monospace: mono, Bold: bold},
		Alignment: align,
		TextSize:  size,
	}
}
