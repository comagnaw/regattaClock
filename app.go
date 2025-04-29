package regattaClock

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// App represents the main application
type App struct {
	window         fyne.Window
	app            fyne.App
	clock          *canvas.Text
	regattaTitle   *canvas.Text
	regattaDate    *canvas.Text
	scheduledRaces *canvas.Text
	tableRows      []LapTableRow
	lapTimes       []lapTime
	isRunning      bool
	startTime      time.Time
	raceNumber     *widget.Entry
	winningTime    *widget.Entry
	regattaData    *RegattaData
}

func NewApp(app fyne.App) *App {
	regattaApp := &App{
		window:    app.NewWindow("Regatta Clock"),
		app:       app,
		lapTimes:  make([]lapTime, 0),
		isRunning: false,
	}

	regattaApp.setClock()
	regattaApp.setTitle()
	regattaApp.setScheduledRaces()
	regattaApp.setRaceDate()
	regattaApp.setupRaceNumber()
	regattaApp.setupWinningTime()

	regattaApp.window.SetMaster()
	regattaApp.window.SetMainMenu(regattaApp.makeMenu())

	regattaApp.window.SetContent(regattaApp.setupContent())
	regattaApp.window.Resize(fyne.NewSize(800, 1000))
	regattaApp.window.Canvas().SetOnTypedKey(regattaApp.setupKeyboardHandler())
	

	regattaApp.showStartupDialog()

	return regattaApp
}

func (a *App) Run() {
	go func() {
		for range time.Tick(time.Millisecond) {
			if a.isRunning {
				elapsed := time.Since(a.startTime)
				hours := int(elapsed.Hours())
				minutes := int(elapsed.Minutes()) % 60
				seconds := int(elapsed.Seconds()) % 60
				milliseconds := int(elapsed.Milliseconds()) % 1000
				formatted := time.Date(0, 0, 0, hours, minutes, seconds, milliseconds*1000000, time.UTC).Format("15:04:05.000")

				// Use fyne.Do to update UI on the main thread
				fyne.Do(func() {
					a.clock.Text = formatted
					a.clock.Refresh()
				})
			}
		}
	}()
	a.window.ShowAndRun()
}

func (a *App) setClock() {
	a.clock = canvas.NewText("00:00:00.000", color.White)
	a.clock.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	a.clock.Alignment = fyne.TextAlignCenter
	a.clock.TextSize = 48
}

func (a *App) setTitle() {
	a.regattaTitle = canvas.NewText("", color.White)
	a.regattaTitle.TextStyle = fyne.TextStyle{Bold: true}
	a.regattaTitle.Alignment = fyne.TextAlignCenter
	a.regattaTitle.TextSize = 24
}

func (a *App) setScheduledRaces() {
	a.scheduledRaces = canvas.NewText("", color.White)
	a.scheduledRaces.TextStyle = fyne.TextStyle{Bold: true}
	a.scheduledRaces.Alignment = fyne.TextAlignCenter
	a.scheduledRaces.TextSize = 20
}

func (a *App) setRaceDate() {
	a.regattaDate = canvas.NewText("", color.White)
	a.regattaDate.TextStyle = fyne.TextStyle{Bold: true}
	a.regattaDate.Alignment = fyne.TextAlignCenter
	a.regattaDate.TextSize = 20
}

func (a *App) setupContent() *fyne.Container {

	return container.NewVBox(
		container.NewCenter(a.regattaTitle),
		container.NewCenter(a.scheduledRaces),
		container.NewCenter(a.regattaDate),
		container.NewCenter(a.clock),
		a.buttonPanel(),
		layout.NewSpacer(),
		a.lapTable(),
		a.inputPanel(),
		layout.NewSpacer(),
	)
}

func (a *App) refreshContent() {
	// First pass: calculate adjusted place numbers
	adjustedPlaces := make([]int, len(a.lapTimes))
	placeAdjustment := 0
	for i := 0; i < len(a.lapTimes); i++ {
		if a.lapTimes[i].dq {
			adjustedPlaces[i] = -1 // Mark DQ'd entries
			placeAdjustment++
		} else {
			adjustedPlaces[i] = a.lapTimes[i].number - placeAdjustment
		}
	}

	// Calculate time adjustments if winning time is set
	var timeAdjustment time.Duration
	if a.winningTime.Text != "" {
		winningTime, err := parseTime(a.winningTime.Text)
		if err == nil && len(a.lapTimes) > 0 {
			// Calculate the adjustment based on the first lap time
			firstLapTime, err := parseTime(a.lapTimes[0].time)
			if err == nil {
				timeAdjustment = winningTime - firstLapTime
			}
		}
	}

	// Update all rows
	for i := 0; i < 6; i++ {
		if i < len(a.lapTimes) {
			// Set OOF entry
			a.tableRows[i].oofEntry.SetText(a.lapTimes[i].oof)
			if !a.isRunning {
				a.tableRows[i].oofEntry.Enable()
				// Set up the OnChanged handler for OOF editing
				row := i // Capture the row index
				a.tableRows[i].oofEntry.OnChanged = func(text string) {
					if !a.isRunning && row < len(a.lapTimes) {
						a.lapTimes[row].oof = text
					}
				}
				// Add return key handling to move to next row
				a.tableRows[i].oofEntry.OnSubmitted = func(text string) {
					if !a.isRunning && row < len(a.lapTimes) {
						// Update the current entry's text
						a.tableRows[row].oofEntry.SetText(text)
						a.lapTimes[row].oof = text

						// Move focus to next row's OOF entry if it exists
						if row+1 < len(a.tableRows) && row+1 < len(a.lapTimes) {
							// Clear any existing text in the next entry
							a.tableRows[row+1].oofEntry.SetText("")
							// Move focus to the next entry
							a.window.Canvas().Focus(a.tableRows[row+1].oofEntry)
						}
					}
				}
			} else {
				a.tableRows[i].oofEntry.Disable()
			}

			// Set Place label
			if a.lapTimes[i].dq {
				a.tableRows[i].placeLabel.SetText("DQ")
				a.tableRows[i].splitLabel.SetText("")
				a.tableRows[i].timeLabel.SetText("")
			} else {
				a.tableRows[i].placeLabel.SetText(fmt.Sprintf("%d", adjustedPlaces[i]))
				a.tableRows[i].splitLabel.SetText(a.lapTimes[i].time)

				// Calculate and set the adjusted time
				if timeAdjustment != 0 {
					lapTime, err := parseTime(a.lapTimes[i].time)
					if err == nil {
						adjustedTime := lapTime + timeAdjustment
						a.tableRows[i].timeLabel.SetText(formatTime(adjustedTime))
					} else {
						a.tableRows[i].timeLabel.SetText(a.lapTimes[i].calculatedTime)
					}
				} else {
					a.tableRows[i].timeLabel.SetText(a.lapTimes[i].calculatedTime)
				}
			}

			// Set DQ checkbox state and enable/disable based on running state
			a.tableRows[i].dqCheck.Checked = a.lapTimes[i].dq
			a.tableRows[i].dqCheck.Disable()
			if !a.isRunning {
				a.tableRows[i].dqCheck.Enable()
				// Add handler for DQ checkbox changes
				row := i // Capture the row index
				a.tableRows[i].dqCheck.OnChanged = func(checked bool) {
					if !a.isRunning && row < len(a.lapTimes) {
						a.lapTimes[row].dq = checked
						a.refreshContent() // Refresh the entire table to update all place numbers
					}
				}
			}
		} else {
			// Clear row
			a.tableRows[i].oofEntry.SetText("")
			a.tableRows[i].oofEntry.Disable()
			a.tableRows[i].placeLabel.SetText("")
			a.tableRows[i].splitLabel.SetText("")
			a.tableRows[i].timeLabel.SetText("")
			a.tableRows[i].dqCheck.Checked = false
			a.tableRows[i].dqCheck.Disable()
		}
	}
}

func (a *App) showStartupDialog() {
	// Create a custom dialog
	dialog.ShowCustomConfirm(
		"Load Regatta Data",
		"Load",
		"Cancel",
		container.NewVBox(
			widget.NewLabel("Welcome to Regatta Clock!"),
			widget.NewLabel("Please load your regatta Excel file to begin."),
			widget.NewLabel("You can also load it later from the menu."),
		),
		func(load bool) {
			if load {
				a.loadExcel(true)
			} else {
				dialog.ShowInformation(
					"Load Later",
					"You can load the Excel file later by selecting 'Import Regatta Table' from the menu.",
					a.window,
				)
			}
		},
		a.window,
	)
}

// parseTime parses a time string in format "00:00.0" or "00:00:00.000" to time.Duration
func parseTime(timeStr string) (time.Duration, error) {
	if timeStr == "" {
		return 0, nil
	}

	// Try parsing as "00:00.0" format first
	parts := strings.Split(timeStr, ":")
	if len(parts) == 2 {
		minutes, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid minutes")
		}

		secondsParts := strings.Split(parts[1], ".")
		if len(secondsParts) != 2 {
			return 0, fmt.Errorf("invalid seconds format")
		}

		seconds, err := strconv.Atoi(secondsParts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid seconds")
		}

		tenths, err := strconv.Atoi(secondsParts[1])
		if err != nil {
			return 0, fmt.Errorf("invalid tenths")
		}

		return time.Duration(minutes)*time.Minute +
			time.Duration(seconds)*time.Second +
			time.Duration(tenths)*100*time.Millisecond, nil
	}

	// Try parsing as "00:00:00.000" format
	if len(parts) == 3 {
		hours, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid hours")
		}

		minutes, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, fmt.Errorf("invalid minutes")
		}

		secondsParts := strings.Split(parts[2], ".")
		if len(secondsParts) != 2 {
			return 0, fmt.Errorf("invalid seconds format")
		}

		seconds, err := strconv.Atoi(secondsParts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid seconds")
		}

		milliseconds, err := strconv.Atoi(secondsParts[1])
		if err != nil {
			return 0, fmt.Errorf("invalid milliseconds")
		}

		return time.Duration(hours)*time.Hour +
			time.Duration(minutes)*time.Minute +
			time.Duration(seconds)*time.Second +
			time.Duration(milliseconds)*time.Millisecond, nil
	}

	return 0, fmt.Errorf("invalid time format")
}

// formatTime formats a time.Duration to "00:00.0"
func formatTime(d time.Duration) string {
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	tenths := int(d.Milliseconds()/100) % 10
	return fmt.Sprintf("%02d:%02d.%d", minutes, seconds, tenths)
}

type Race struct {
	RaceNumber int
	Lanes      map[int]LaneEntry
}

type LaneEntry struct {
	SchoolName     string
	AdditionalInfo string
	Place          string
	Split          string
	Time           string
}
