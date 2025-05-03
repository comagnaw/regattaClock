package regattaClock

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

func (a *App) buttonPanel() *fyne.Container {
	return container.NewHBox(
		layout.NewSpacer(),
		a.startButton(),
		layout.NewSpacer(),
		a.lapButton(),
		layout.NewSpacer(),
		a.stopButton(),
		layout.NewSpacer(),
		a.clearButton(),
		layout.NewSpacer(),
	)
}

func (a *App) startButton() *widget.Button {
	return widget.NewButton(
		"Start (F2)",
		a.startFunc(),
	)
}

func (a *App) startFunc() func() {
	return func() {
		if !a.clockState.isRunning && a.clockState.isCleared {
			a.clockState.startTime = time.Now()
			a.clockState.isRunning = true
			a.clockState.isCleared = false
			a.lapTimes = append(a.lapTimes, lapTime{
				number:         1,
				time:           zeroTime,
				calculatedTime: zeroTime,
				oof:            emptyString,
			})
			a.refreshContent()
			a.raceNumber.Disable()
			a.winningTime.Disable()
		}
	}
}

func (a *App) lapButton() *widget.Button {
	return widget.NewButton(
		"Lap (F4)",
		a.lapFunc(),
	)
}

func (a *App) lapFunc() func() {
	return func() {
		if a.clockState.isRunning {
			elapsed := time.Since(a.clockState.startTime)
			minutes := int(elapsed.Minutes()) % 60
			seconds := int(elapsed.Seconds()) % 60
			tenths := int(elapsed.Milliseconds()/100) % 10
			formatted := fmt.Sprintf("%02d:%02d.%d", minutes, seconds, tenths)

			a.lapTimes = append(a.lapTimes, lapTime{
				number:         len(a.lapTimes) + 1,
				time:           formatted,
				calculatedTime: formatted,
				oof:            emptyString,
			})
			a.refreshContent()
		}
	}
}

func (a *App) stopButton() *widget.Button {
	return widget.NewButton("Stop", func() {
		a.clockState.isRunning = false
		a.refreshContent()
		a.raceNumber.Enable()
		a.winningTime.Enable()
	})
}

func (a *App) clearButton() *widget.Button {
	return widget.NewButton("Clear", func() {
		if !a.clockState.isRunning {
			a.clockState.isRunning = false
			a.clock.Text = zeroTime
			a.clock.Refresh()
			a.lapTimes = make([]lapTime, 0)
			a.winningTime.Text = emptyString
			a.winningTime.Refresh()
			a.refreshContent()
			a.raceNumber.Enable()
			a.winningTime.Enable()
			a.clockState.isCleared = true
		}
	})
}
