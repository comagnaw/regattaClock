package clock

import (
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
)


func (c *Clock) initWinningTime() {
	c.winningTime = widget.NewEntry()
	c.winningTime.SetPlaceHolder(common.ZeroTime)
	c.winningTime.OnChanged = c.onChangedWinningTimeFunc()
	c.winningTime.Disable()
}

func (c *Clock) onChangedWinningTimeFunc() func(text string) {
	return func(text string) {

		// If winning time is empty, just disable referee button
		if text == common.EmptyString {
			c.buttons.referee.Disable()
			return
		}

		// Try to parse the winning time
		_, err := parseTime(text)
		if err != nil {
			c.buttons.referee.Disable()
			return
		}

		c.buttons.referee.Enable()
		c.refreshContent()
		c.window.Content().Refresh()

	}
}
