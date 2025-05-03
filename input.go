package regattaClock

import (
	"fyne.io/fyne/v2/widget"
)

func (a *App) winningTimeInput() *widget.FormItem {
	item := widget.NewFormItem(
		"Winning Time:",
		a.winningTime,
	)
	item.HintText = zeroTime
	return item
}
