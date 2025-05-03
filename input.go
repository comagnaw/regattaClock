package regattaClock

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

func (a *App) setupWinningTime() {
	a.winningTime = widget.NewEntry()
	a.winningTime.Validator = func(s string) error {
		if s == emptyString {
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

	// Add event handler for winning time changes
	a.winningTime.OnChanged = func(text string) {
		if text == emptyString {
			// If winning time is cleared, reset all times
			for i := 0; i < len(a.lapTimes); i++ {
				if a.lapTimes[i].oof != emptyString {
					if laneNum, err := strconv.Atoi(a.lapTimes[i].oof); err == nil && laneNum >= 1 && laneNum <= 6 {
						a.resultsTable[4][laneNum] = a.lapTimes[i].time // Reset Split
						a.resultsTable[5][laneNum] = a.lapTimes[i].time // Reset Time
					}
				}
			}
			a.window.Content().Refresh()
			return
		}

		winningTime, err := parseTime(text)
		if err != nil {
			return
		}

		// Get the first lap time
		var firstLapTime time.Duration
		if len(a.lapTimes) > 0 {
			firstLapTime, err = parseTime(a.lapTimes[0].time)
			if err != nil {
				return
			}
		}

		// Calculate the time adjustment
		timeAdjustment := winningTime - firstLapTime

		// Update all lap times and results table
		for i := 0; i < len(a.lapTimes); i++ {
			lapTime, err := parseTime(a.lapTimes[i].time)
			if err == nil {
				adjustedTime := lapTime + timeAdjustment
				a.lapTimes[i].calculatedTime = formatTime(adjustedTime)
				a.tableRows[i].timeLabel.SetText(formatTime(adjustedTime))

				// Update results table if OOF is set
				if a.lapTimes[i].oof != emptyString {
					if laneNum, err := strconv.Atoi(a.lapTimes[i].oof); err == nil && laneNum >= 1 && laneNum <= 6 {
						a.resultsTable[4][laneNum] = a.lapTimes[i].time       // Update Split
						a.resultsTable[5][laneNum] = formatTime(adjustedTime) // Update Time
					}
				}
			}
		}
		a.window.Content().Refresh()
	}

	a.winningTime.Enable()
	a.winningTime.TextStyle = fyne.TextStyle{Monospace: true}
}

func (a *App) winningTimeInput() *widget.FormItem {
	item := widget.NewFormItem(
		"Winning Time:",
		a.winningTime,
	)
	item.HintText = zeroTime
	return item
}
