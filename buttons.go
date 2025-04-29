package regattaClock

import (
	"fmt"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
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
	return widget.NewButton("Start (F2)", func() {
		if !a.isRunning {
			a.startTime = time.Now()
			a.isRunning = true
			a.lapTimes = append(a.lapTimes, lapTime{
				number:         1,
				time:           zeroTime,
				calculatedTime: zeroTime,
				oof:            "",
				dq:             false,
			})
			a.refreshContent()
			a.raceNumber.Disable()
			a.winningTime.Disable()
		}
	})
}

func (a *App) startFunc() func() {
	return func() {
		if !a.isRunning {
			a.startTime = time.Now()
			a.isRunning = true
			a.lapTimes = append(a.lapTimes, lapTime{
				number:         1,
				time:           zeroTime,
				calculatedTime: zeroTime,
				oof:            "",
				dq:             false,
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
		if a.isRunning {
			elapsed := time.Since(a.startTime)
			minutes := int(elapsed.Minutes()) % 60
			seconds := int(elapsed.Seconds()) % 60
			tenths := int(elapsed.Milliseconds()/100) % 10
			formatted := fmt.Sprintf("%02d:%02d.%d", minutes, seconds, tenths)

			a.lapTimes = append(a.lapTimes, lapTime{
				number:         len(a.lapTimes) + 1,
				time:           formatted,
				calculatedTime: formatted,
				oof:            "",
				dq:             false,
			})
			a.refreshContent()
		}
	}
}

func (a *App) stopButton() *widget.Button {
	return widget.NewButton("Stop", func() {
		a.isRunning = false
		a.refreshContent()
		a.raceNumber.Enable()
		a.winningTime.Enable()
	})
}

func (a *App) clearButton() *widget.Button {
	return widget.NewButton("Clear", func() {
		a.isRunning = false
		a.clock.Text = "00:00:00.000"
		a.clock.Refresh()
		a.lapTimes = make([]lapTime, 0)
		a.winningTime.Text = ""
		a.winningTime.Refresh()
		a.refreshContent()
		a.raceNumber.Enable()
		a.winningTime.Enable()
	})
}

func (a *App) loadRaceButton() *widget.Button {
	return widget.NewButton("Load Race", func() {
		if a.regattaData == nil {
			dialog.ShowInformation("Error", "No regatta data available - please import an Excel file first", a.window)
			return
		}

		raceNum, err := strconv.Atoi(a.raceNumber.Text)
		if err != nil {
			dialog.ShowInformation("Error", "Invalid race number format", a.window)
			return
		}

		// Find the race
		var foundRace *RaceData
		for _, race := range a.regattaData.Races {
			if race.RaceNumber == raceNum {
				foundRace = &race
				break
			}
		}

		if foundRace == nil {
			dialog.ShowInformation("Error", fmt.Sprintf("Race number %d not found", raceNum), a.window)
			return
		}

		// Check if the race has any lanes
		if len(foundRace.Lanes) == 0 {
			dialog.ShowInformation("Error", fmt.Sprintf("No boats found in Race %d", raceNum), a.window)
			return
		}

		// Print race details
		fmt.Printf("\nFound Race %d:\n", foundRace.RaceNumber)
		for lane := 1; lane <= 6; lane++ {
			if entry, exists := foundRace.Lanes[lane]; exists {
				fmt.Printf("  Lane %d:\n", lane)
				fmt.Printf("    School: %s\n", entry.SchoolName)
				fmt.Printf("    Additional Info: %s\n", entry.AdditionalInfo)
				fmt.Printf("    Place: %s\n", entry.Place)
				fmt.Printf("    Split: %s\n", entry.Split)
				fmt.Printf("    Time: %s\n", entry.Time)
			}
		}
	})
}
