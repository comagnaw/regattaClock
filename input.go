package regattaClock

import (
	"fmt"
	"strings"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

func (a *App) inputPanel() *fyne.Container {
	return container.NewHBox(
		layout.NewSpacer(),
		a.raceNumberInput(),
		layout.NewSpacer(),
		a.winningTimeInput(),
		layout.NewSpacer(),
	)
}

func (a *App) setupRaceNumber() {
	a.raceNumber = widget.NewEntry()
	a.raceNumber.Validator = func(s string) error {
		// Only allow numbers
		for _, r := range s {
			if r < '0' || r > '9' {
				return fmt.Errorf("only numbers allowed")
			}
		}
		return nil
	}
}

func (a *App) raceNumberInput() *fyne.Container {
	return container.NewHBox(
		widget.NewLabel("Race Number:"),
		a.raceNumber,
		a.loadRaceButton(),
	)
}

func (a *App) setupWinningTime() {
	a.winningTime = widget.NewEntry()
	a.winningTime.Validator = func(s string) error {
		if s == "" {
			return nil
		}
		// Validate format 00:00.0
		parts := strings.Split(s, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid time format")
		}
		minutes, err := strconv.Atoi(parts[0])
		if err != nil || minutes < 0 || minutes > 59 {
			return fmt.Errorf("invalid minutes")
		}
		secondsParts := strings.Split(parts[1], ".")
		if len(secondsParts) != 2 {
			return fmt.Errorf("invalid seconds format")
		}
		seconds, err := strconv.Atoi(secondsParts[0])
		if err != nil || seconds < 0 || seconds > 59 {
			return fmt.Errorf("invalid seconds")
		}
		tenths, err := strconv.Atoi(secondsParts[1])
		if err != nil || tenths < 0 || tenths > 9 {
			return fmt.Errorf("invalid tenths")
		}
		return nil
	}
	a.winningTime.Enable()
	a.winningTime.TextStyle = fyne.TextStyle{Monospace: true}
}

func (a *App) winningTimeInput() *fyne.Container {
	return container.NewHBox(
		widget.NewLabel("Winning Time (00:00.0):"),
		a.winningTime,
	)
}