package regattaClock

import (
	"fmt"
	"image/color"
	"sort"
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
	resultsTable   [][]string
	lapTimes       []lapTime
	raceNumber     *widget.Entry
	winningTime    *widget.Entry
	regattaData    *RegattaData
	clockState     *clockState
}

type clockState struct {
	isRunning bool
	isCleared bool
	startTime time.Time
	stopChan  chan struct{}
}

func NewApp(app fyne.App) *App {
	regattaApp := &App{
		window:   app.NewWindow("Regatta Clock"),
		app:      app,
		lapTimes: make([]lapTime, 0),
		clockState: &clockState{
			isRunning: false,
			isCleared: true,
			stopChan:  make(chan struct{}),
		},
	}

	regattaApp.initAppData()

	regattaApp.window.SetMaster()
	regattaApp.window.SetMainMenu(regattaApp.makeMenu())
	regattaApp.window.Resize(fyne.NewSize(800, 600))

	// Set up keyboard handler for the main window
	regattaApp.window.Canvas().SetOnTypedKey(regattaApp.setupKeyboardHandler())

	regattaApp.setupStartupDialog()

	return regattaApp
}

func (a *App) Run() {
	// Start the clock update goroutine for the main window
	go a.startClockUpdate()
	a.window.ShowAndRun()
}

func (a *App) startClockUpdate() {
	ticker := time.NewTicker(100 * time.Millisecond) // Update every 0.1 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if a.clockState.isRunning {
				elapsed := time.Since(a.clockState.startTime)
				minutes := int(elapsed.Minutes()) % 60
				seconds := int(elapsed.Seconds()) % 60
				tenths := int(elapsed.Milliseconds()/100) % 10
				formatted := fmt.Sprintf("%02d:%02d.%d", minutes, seconds, tenths)

				// Use fyne.Do to update UI on the main thread
				fyne.Do(func() {
					if a.clock != nil { // Add nil check for safety
						a.clock.Text = formatted
						a.clock.Refresh()
					}
				})
			}
		case <-a.clockState.stopChan:
			return
		}
	}
}

func (a *App) initAppData() {
	a.setClock()
	a.setTitle()
	a.setScheduledRaces()
	a.setRaceDate()
	a.setupWinningTime()
}

func (a *App) setClock() {
	a.clock = canvas.NewText(zeroTime, color.White)
	a.clock.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	a.clock.Alignment = fyne.TextAlignCenter
	a.clock.TextSize = 48
}

func (a *App) setTitle() {
	a.regattaTitle = canvas.NewText(emptyString, color.White)
	a.regattaTitle.TextStyle = fyne.TextStyle{Bold: true}
	a.regattaTitle.Alignment = fyne.TextAlignCenter
	a.regattaTitle.TextSize = 24
}

func (a *App) setScheduledRaces() {
	a.scheduledRaces = canvas.NewText(emptyString, color.White)
	a.scheduledRaces.TextStyle = fyne.TextStyle{Bold: true}
	a.scheduledRaces.Alignment = fyne.TextAlignCenter
	a.scheduledRaces.TextSize = 20
}

func (a *App) setRaceDate() {
	a.regattaDate = canvas.NewText(emptyString, color.White)
	a.regattaDate.TextStyle = fyne.TextStyle{Bold: true}
	a.regattaDate.Alignment = fyne.TextAlignCenter
	a.regattaDate.TextSize = 20
}

func (a *App) setupContent() *fyne.Container {
	middleContent := container.NewVBox(
		container.NewCenter(a.clock),
		a.buttonPanel(),
		a.lapTable(),
		widget.NewForm(a.winningTimeInput()),
	)

	bottomContent := container.NewGridWrap(
		fyne.Size{Width: 1240, Height: 240},
		a.raceResults(),
	)

	return container.NewVBox(middleContent, bottomContent)
}

func (a *App) setupStartupDialog() {
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
	if a.winningTime.Text != emptyString {
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
			if !a.clockState.isRunning {
				a.tableRows[i].oofEntry.Enable()
				// Set up the OnChanged handler for OOF editing
				row := i // Capture the row index
				a.tableRows[i].oofEntry.OnChanged = func(text string) {
					if !a.clockState.isRunning && row < len(a.lapTimes) {
						// Update resultsTable if OOF matches a lane number
						if laneNum, err := strconv.Atoi(text); err == nil && laneNum >= 1 && laneNum <= 6 {
							// Check for duplicate OOF values in other rows
							isDuplicate := false
							for j := 0; j < len(a.lapTimes); j++ {
								if j != row && a.lapTimes[j].oof == text {
									isDuplicate = true
									break
								}
							}

							if !isDuplicate {
								// Store previous OOF value before updating
								prevOOF := a.lapTimes[row].oof
								// Update the lap time's OOF value
								a.lapTimes[row].oof = text
								// Update Place, Split, and Time rows in resultsTable
								a.resultsTable[3][laneNum] = a.tableRows[row].placeLabel.Text // Update Place
								a.resultsTable[4][laneNum] = a.tableRows[row].splitEntry.Text // Update Split
								a.resultsTable[5][laneNum] = a.tableRows[row].timeLabel.Text  // Update Time
								// Clear previous lane if it was different
								if prevOOF != emptyString && prevOOF != text {
									if prevLaneNum, err := strconv.Atoi(prevOOF); err == nil && prevLaneNum >= 1 && prevLaneNum <= 6 {
										a.resultsTable[3][prevLaneNum] = emptyString // Clear Place
										a.resultsTable[4][prevLaneNum] = emptyString // Clear Split
										a.resultsTable[5][prevLaneNum] = emptyString // Clear Time
									}
								}
								a.window.Content().Refresh()
							} else {
								// If duplicate, clear the input
								a.tableRows[row].oofEntry.SetText(emptyString)
								// Clear the previous lane if it exists
								if prevOOF := a.lapTimes[row].oof; prevOOF != emptyString {
									if prevLaneNum, err := strconv.Atoi(prevOOF); err == nil && prevLaneNum >= 1 && prevLaneNum <= 6 {
										a.resultsTable[3][prevLaneNum] = emptyString // Clear Place
										a.resultsTable[4][prevLaneNum] = emptyString // Clear Split
										a.resultsTable[5][prevLaneNum] = emptyString // Clear Time
										a.window.Content().Refresh()
									}
								}
								a.lapTimes[row].oof = emptyString
							}
						} else {
							// If OOF is cleared or invalid, clear the previous lane
							prevOOF := a.lapTimes[row].oof
							// Update the lap time's OOF value
							a.lapTimes[row].oof = text
							if prevOOF != emptyString {
								if prevLaneNum, err := strconv.Atoi(prevOOF); err == nil && prevLaneNum >= 1 && prevLaneNum <= 6 {
									a.resultsTable[3][prevLaneNum] = emptyString // Clear Place
									a.resultsTable[4][prevLaneNum] = emptyString // Clear Split
									a.resultsTable[5][prevLaneNum] = emptyString // Clear Time
									a.window.Content().Refresh()
								}
							}
						}
					}
				}

				// Set up the OnSubmitted handler for OOF editing (Tab or Enter)
				if !a.clockState.isRunning {
					row := i // Capture the row index
					a.tableRows[i].oofEntry.OnSubmitted = func(text string) {
						if !a.clockState.isRunning && row < len(a.tableRows) && row < len(a.lapTimes) {
							// Move focus to next row's OOF entry if it exists
							if row+1 < len(a.tableRows) && row+1 < len(a.lapTimes) {
								// Clear any existing text in the next entry
								a.tableRows[row+1].oofEntry.SetText(emptyString)
								// Move focus to the next entry
								a.window.Canvas().Focus(a.tableRows[row+1].oofEntry)
							}
						}
					}
				}
			} else {
				a.tableRows[i].oofEntry.Disable()
			}

			// Set Place label
			if a.lapTimes[i].dq {
				a.tableRows[i].placeLabel.SetText("DQ")
				a.tableRows[i].splitEntry.SetText(emptyString)
				a.tableRows[i].timeLabel.SetText(emptyString)
			} else {
				a.tableRows[i].placeLabel.SetText(fmt.Sprintf("%d", adjustedPlaces[i]))
				a.tableRows[i].splitEntry.SetText(a.lapTimes[i].time)

				// Set up the OnChanged handler for split time editing
				if !a.clockState.isRunning {
					row := i // Capture the row index
					a.tableRows[i].splitEntry.OnChanged = func(text string) {
						if !a.clockState.isRunning && row < len(a.lapTimes) {
							// Update the lap time
							a.lapTimes[row].time = text

							// Calculate and update the time label
							lapTime, err := parseTime(text)
							if err == nil {
								if timeAdjustment != 0 {
									adjustedTime := lapTime + timeAdjustment
									a.tableRows[row].timeLabel.SetText(formatTime(adjustedTime))
								} else {
									a.tableRows[row].timeLabel.SetText(formatTime(lapTime))
								}
							}

							// Update resultsTable if OOF matches a lane number
							if oof := a.lapTimes[row].oof; oof != emptyString {
								if laneNum, err := strconv.Atoi(oof); err == nil && laneNum >= 1 && laneNum <= 6 {
									// Update Place, Split, and Time rows in resultsTable
									a.resultsTable[3][laneNum] = a.tableRows[row].placeLabel.Text // Update Place
									a.resultsTable[4][laneNum] = text                             // Update Split
									a.resultsTable[5][laneNum] = a.tableRows[row].timeLabel.Text  // Update Time
									a.window.Content().Refresh()
								}
							}
						}
					}
				}

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
			if !a.clockState.isRunning {
				a.tableRows[i].dqCheck.Enable()
				// Add handler for DQ checkbox changes
				row := i // Capture the row index
				a.tableRows[i].dqCheck.OnChanged = func(checked bool) {
					if !a.clockState.isRunning && row < len(a.lapTimes) {
						a.lapTimes[row].dq = checked
						a.refreshContent() // Refresh the entire table to update all place numbers
					}
				}
			}
		} else {
			// Clear row
			a.tableRows[i].oofEntry.SetText(emptyString)
			a.tableRows[i].oofEntry.Disable()
			a.tableRows[i].placeLabel.SetText(emptyString)
			a.tableRows[i].splitEntry.SetText(emptyString)
			a.tableRows[i].timeLabel.SetText(emptyString)
			a.tableRows[i].dqCheck.Checked = false
			a.tableRows[i].dqCheck.Disable()
		}
	}
}

// parseTime parses a time string in format "00:00.0" or "00:00:00.000" to time.Duration
func parseTime(timeStr string) (time.Duration, error) {
	if timeStr == emptyString {
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

func (a *App) showRaceTree() {
	if a.regattaData == nil {
		return
	}

	// Create a container for the race tree
	mainContainer := container.NewVBox()

	// Add regatta information at the top
	regattaInfo := container.NewVBox(
		container.NewCenter(a.regattaTitle),
		container.NewCenter(a.scheduledRaces),
		container.NewCenter(a.regattaDate),
	)
	mainContainer.Add(regattaInfo)

	// Add a separator
	separator := widget.NewSeparator()
	mainContainer.Add(separator)

	// Add a title for the race list
	title := widget.NewLabel("Scheduled Races")
	title.TextStyle = fyne.TextStyle{Bold: true}
	mainContainer.Add(title)

	// Create a list to hold the race nodes
	raceList := container.NewVBox()

	// Sort races by race number
	races := make([]RaceData, len(a.regattaData.Races))
	copy(races, a.regattaData.Races)
	sort.Slice(races, func(i, j int) bool {
		return races[i].RaceNumber < races[j].RaceNumber
	})

	// Add each race to the tree
	for _, race := range races {
		// Count non-empty school names
		boatCount := 0
		for _, lane := range race.Lanes {
			if lane.SchoolName != "" {
				boatCount++
			}
		}

		// Get boat class and flight/heat/final information from RawData
		boatClass := ""
		flightInfo := ""
		if len(race.RawData) > 0 {
			if len(race.RawData[0]) > 0 {
				boatClass = race.RawData[0][0]
			}
			if len(race.RawData) > 1 && len(race.RawData[1]) > 0 {
				flightInfo = race.RawData[1][0]
			}
		}

		// Create the race description
		raceDesc := fmt.Sprintf("Race %d (%d boats)", race.RaceNumber, boatCount)
		if boatClass != "" {
			raceDesc = fmt.Sprintf("%s - %s", raceDesc, boatClass)
		}
		if flightInfo != "" {
			raceDesc = fmt.Sprintf("%s - %s", raceDesc, flightInfo)
		}

		// Create a container for this race
		raceContainer := container.NewHBox(
			widget.NewLabel(raceDesc),
			layout.NewSpacer(),
		)

		// Create a button to time this race
		timeButton := widget.NewButton("Time Race", func(raceData RaceData) func() {
			return func() {
				a.openRaceClock(raceData)
			}
		}(race))
		raceContainer.Add(timeButton)

		raceList.Add(raceContainer)
	}

	// Create a scroll container for the race list
	scroll := container.NewScroll(raceList)
	scroll.SetMinSize(fyne.NewSize(400, 400))
	mainContainer.Add(scroll)

	// Set the window content
	a.window.SetContent(mainContainer)
	a.window.Resize(fyne.NewSize(500, 600))
}

func (a *App) openRaceClock(race RaceData) {
	// Create a new window for this race
	raceWindow := a.app.NewWindow(fmt.Sprintf("Race %d Clock", race.RaceNumber))

	// Create a new App instance for this race
	raceApp := &App{
		window:   raceWindow,
		app:      a.app,
		lapTimes: make([]lapTime, 0),
		clockState: &clockState{
			isRunning: false,
			isCleared: true,
			stopChan:  make(chan struct{}),
		},
		regattaData: a.regattaData,
	}

	// Initialize the app data (this sets up all necessary widgets)
	raceApp.initAppData()

	// Initialize the clock specifically for this window
	raceApp.clock = canvas.NewText(zeroTime, color.White)
	raceApp.clock.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	raceApp.clock.Alignment = fyne.TextAlignCenter
	raceApp.clock.TextSize = 48

	// Count the number of boats in the race
	boatCount := 0
	for _, entry := range race.Lanes {
		if entry.SchoolName != "" {
			boatCount++
		}
	}

	// Get boat class and flight/heat/final information from RawData
	boatClass := ""
	flightInfo := ""
	if len(race.RawData) > 0 {
		if len(race.RawData[0]) > 0 {
			boatClass = race.RawData[0][0]
		}
		if len(race.RawData) > 1 && len(race.RawData[1]) > 0 {
			flightInfo = race.RawData[1][0]
		}
	}

	// Create the race title text
	titleText := fmt.Sprintf("Race %d (%d Boats)", race.RaceNumber, boatCount)
	if boatClass != "" {
		titleText = fmt.Sprintf("%s - %s", titleText, boatClass)
	}
	if flightInfo != "" {
		titleText = fmt.Sprintf("%s - %s", titleText, flightInfo)
	}

	raceTitle := canvas.NewText(titleText, color.White)
	raceTitle.TextStyle = fyne.TextStyle{Bold: true}
	raceTitle.Alignment = fyne.TextAlignCenter
	raceTitle.TextSize = 24

	// Initialize the results table with the race data
	raceApp.resultsTable = make([][]string, 6)
	for i := range raceApp.resultsTable {
		raceApp.resultsTable[i] = make([]string, 7)
	}

	// Set up the results table headers
	raceApp.resultsTable[0][0] = ""
	raceApp.resultsTable[1][0] = ""
	raceApp.resultsTable[2][0] = ""
	raceApp.resultsTable[3][0] = "Place"
	raceApp.resultsTable[4][0] = "Split"
	raceApp.resultsTable[5][0] = "Time"

	// Always show all lane headers (1-6)
	for lane := 1; lane <= 6; lane++ {
		raceApp.resultsTable[0][lane] = fmt.Sprintf("Lane %d", lane)
		// Initialize empty strings for all other columns
		raceApp.resultsTable[1][lane] = ""
		raceApp.resultsTable[2][lane] = ""
		raceApp.resultsTable[3][lane] = ""
		raceApp.resultsTable[4][lane] = ""
		raceApp.resultsTable[5][lane] = ""
	}

	// Populate school data for scheduled lanes
	for lane, entry := range race.Lanes {
		if lane >= 1 && lane <= 6 {
			// Set school name
			raceApp.resultsTable[1][lane] = entry.SchoolName
			// Set additional info
			raceApp.resultsTable[2][lane] = entry.AdditionalInfo
		}
	}

	// Initialize race number and winning time fields
	raceApp.raceNumber = widget.NewEntry()
	raceApp.raceNumber.SetText(fmt.Sprintf("%d", race.RaceNumber))
	raceApp.raceNumber.Disable()
	raceApp.setupWinningTime()

	// Set up the window content with the race title above the clock
	content := raceApp.setupContent()
	raceWindow.SetContent(container.NewVBox(
		container.NewCenter(raceTitle),
		content,
	))
	raceWindow.Resize(fyne.NewSize(1240, 800))

	// Set up keyboard handler for this window
	raceWindow.Canvas().SetOnTypedKey(raceApp.setupKeyboardHandler())

	// Start the clock update goroutine for this window
	go raceApp.startClockUpdate()

	// Set up window close handler to clean up the goroutine
	raceWindow.SetOnClosed(func() {
		close(raceApp.clockState.stopChan)
	})

	raceWindow.Show()
}
