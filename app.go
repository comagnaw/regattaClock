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
	window             fyne.Window
	app                fyne.App
	clock              *canvas.Text
	regattaTitle       *canvas.Text
	regattaDate        *canvas.Text
	scheduledRaces     *canvas.Text
	tableRows          []LapTableRow
	resultsTable       [][]string
	lapTimes           []lapTime
	raceNumber         *widget.Entry
	winningTime        *widget.Entry
	regattaData        *RegattaData
	clockState         *clockState
	originalPlaces     map[int]string // Add map to store original place values
	resultsTableWidget *widget.Table
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
		originalPlaces: make(map[int]string), // Initialize the map
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

	// Initialize the originalPlaces map if it's nil
	if a.originalPlaces == nil {
		a.originalPlaces = make(map[int]string)
	}

	if a.resultsTable == nil {
		a.resultsTable = [][]string{
			{"", "Lane 1", "Lane 2", "Lane 3", "Lane 4", "Lane 5", "Lane 6"},
			{"", "", "", "", "", "", ""},
			{"Place", "", "", "", "", "", ""},
			{"Split", "", "", "", "", "", ""},
			{"Time", "", "", "", "", "", ""},
			{"", "", "", "", "", "", ""}, // Add fifth data row
			{"", "", "", "", "", "", ""}, // Add sixth row for storing original place values
		}
	}

	// Ensure we have enough rows for the data
	if len(a.resultsTable) < 7 {
		// Add any missing rows
		for i := len(a.resultsTable); i < 7; i++ {
			a.resultsTable = append(a.resultsTable, make([]string, 7))
		}
	}
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
	for i := 0; i < len(a.lapTimes); i++ {
		adjustedPlaces[i] = a.lapTimes[i].number
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
				// Store the calculated time in the lapTime struct
				a.lapTimes[0].calculatedTime = formatTime(winningTime)
			}
		}
	}

	// First pass: update all time labels and calculated times
	for i := 0; i < len(a.lapTimes); i++ {
		if timeAdjustment != 0 {
			lapTime, err := parseTime(a.lapTimes[i].time)
			if err == nil {
				adjustedTime := lapTime + timeAdjustment
				adjustedTimeStr := formatTime(adjustedTime)
				a.lapTimes[i].calculatedTime = adjustedTimeStr
				// Update results table if OOF is set
				if oof := a.lapTimes[i].oof; oof != emptyString {
					if laneNum, err := strconv.Atoi(oof); err == nil && laneNum >= 1 && laneNum <= 6 {
						a.resultsTable[5][laneNum] = adjustedTimeStr
					}
				}
			}
		} else {
			a.lapTimes[i].calculatedTime = a.lapTimes[i].time
			// Update results table if OOF is set
			if oof := a.lapTimes[i].oof; oof != emptyString {
				if laneNum, err := strconv.Atoi(oof); err == nil && laneNum >= 1 && laneNum <= 6 {
					a.resultsTable[5][laneNum] = a.lapTimes[i].time
				}
			}
		}
	}

	// Second pass: update all rows and resultsTable
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
								a.resultsTable[3][laneNum] = a.tableRows[row].placeButton.Text // Update Place
								a.resultsTable[4][laneNum] = a.tableRows[row].splitEntry.Text  // Update Split
								a.resultsTable[5][laneNum] = a.lapTimes[row].calculatedTime    // Update Time with calculated time
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
			} else {
				a.tableRows[i].oofEntry.Disable()
			}

			// Set Place button
			placeText := fmt.Sprintf("%d", adjustedPlaces[i])
			a.tableRows[i].placeButton.SetText(placeText)
			a.tableRows[i].splitEntry.SetText(a.lapTimes[i].time)
			a.tableRows[i].timeLabel.SetText(a.lapTimes[i].calculatedTime)

			// Set up the place button click handler
			row := i // Capture the row index
			a.tableRows[i].placeButton.OnTapped = func() {
				if !a.clockState.isRunning {
					// Get the lane number from OOF
					oof := a.lapTimes[row].oof
					if oof == emptyString {
						return // Don't allow editing if no lane is assigned
					}

					laneNum, err := strconv.Atoi(oof)
					if err != nil || laneNum < 1 || laneNum > 6 {
						return // Invalid lane number
					}

					// Create a dialog to edit the place value
					currentPlace := a.resultsTable[3][laneNum]
					options := []string{"DNS", "DNF", "DQ", "Next Place"}

					selectWidget := widget.NewSelect(options, func(value string) {
						// Store the old place value
						oldPlace := a.resultsTable[3][laneNum]

						// Handle DQ/DNF/DNS status
						if value == "DQ" || value == "DNF" || value == "DNS" {
							// Update the place value in the results table
							a.resultsTable[3][laneNum] = value

							// Clear Split and Time values in results table
							a.resultsTable[4][laneNum] = emptyString
							a.resultsTable[5][laneNum] = emptyString

							// If the new place is DQ, adjust other place values
							if value == "DQ" {
								// Convert old place to number if possible
								if oldPlaceNum, err := strconv.Atoi(oldPlace); err == nil {
									// Decrease place values greater than the DQ'd place
									for l := 1; l <= 6; l++ {
										if l != laneNum {
											if placeStr := a.resultsTable[3][l]; placeStr != emptyString {
												if placeNum, err := strconv.Atoi(placeStr); err == nil && placeNum > oldPlaceNum {
													a.resultsTable[3][l] = fmt.Sprintf("%d", placeNum-1)
													// Update the corresponding place button
													for i := 0; i < len(a.lapTimes); i++ {
														if a.lapTimes[i].oof == fmt.Sprintf("%d", l) {
															a.tableRows[i].placeButton.SetText(a.resultsTable[3][l])
															break
														}
													}
												}
											}
										}
									}
								}
							} else if value == "Next Place" {
								// First, update the current lane to Next Place
								a.resultsTable[3][laneNum] = "Next Place"

								// Restore Split and Time values from the lap table
								for i := 0; i < len(a.lapTimes); i++ {
									if a.lapTimes[i].oof == fmt.Sprintf("%d", laneNum) {
										a.resultsTable[4][laneNum] = a.tableRows[i].splitEntry.Text
										a.resultsTable[5][laneNum] = a.lapTimes[i].calculatedTime
										break
									}
								}

								// Now rescan and reassign all place numbers based on lap times sequence
								nextPlace := 1
								for i := 0; i < len(a.lapTimes); i++ {
									if oof := a.lapTimes[i].oof; oof != emptyString {
										if laneNum, err := strconv.Atoi(oof); err == nil && laneNum >= 1 && laneNum <= 6 {
											placeStr := a.resultsTable[3][laneNum]
											if placeStr != "DQ" && placeStr != "DNS" && placeStr != "DNF" && placeStr != emptyString {
												a.resultsTable[3][laneNum] = fmt.Sprintf("%d", nextPlace)
												// Update the corresponding place button
												a.tableRows[i].placeButton.SetText(a.resultsTable[3][laneNum])
												nextPlace++
											}
										}
									}
								}
							}
						} else if value == "Next Place" {
							// First, update the current lane to Next Place
							a.resultsTable[3][laneNum] = "Next Place"

							// Restore Split and Time values from the lap table
							for i := 0; i < len(a.lapTimes); i++ {
								if a.lapTimes[i].oof == fmt.Sprintf("%d", laneNum) {
									a.resultsTable[4][laneNum] = a.tableRows[i].splitEntry.Text
									a.resultsTable[5][laneNum] = a.lapTimes[i].calculatedTime
									break
								}
							}

							// Now rescan and reassign all place numbers based on lap times sequence
							nextPlace := 1
							for i := 0; i < len(a.lapTimes); i++ {
								if oof := a.lapTimes[i].oof; oof != emptyString {
									if laneNum, err := strconv.Atoi(oof); err == nil && laneNum >= 1 && laneNum <= 6 {
										placeStr := a.resultsTable[3][laneNum]
										if placeStr != "DQ" && placeStr != "DNS" && placeStr != "DNF" && placeStr != emptyString {
											a.resultsTable[3][laneNum] = fmt.Sprintf("%d", nextPlace)
											// Update the corresponding place button
											a.tableRows[i].placeButton.SetText(a.resultsTable[3][laneNum])
											nextPlace++
										}
									}
								}
							}
						}

						// Update the place button text
						a.tableRows[row].placeButton.SetText(a.resultsTable[3][laneNum])

						// Refresh the window content to show the updated place values
						a.window.Content().Refresh()
					})

					// Set the current value if it exists in options
					for _, option := range options {
						if option == currentPlace {
							selectWidget.SetSelected(option)
							break
						}
					}

					dialog.ShowCustom(
						"Edit Place",
						"Close",
						selectWidget,
						a.window,
					)
				}
			}

			// Update resultsTable if OOF is set
			if oof := a.lapTimes[i].oof; oof != emptyString {
				if laneNum, err := strconv.Atoi(oof); err == nil && laneNum >= 1 && laneNum <= 6 {
					a.resultsTable[3][laneNum] = placeText                    // Update Place
					a.resultsTable[4][laneNum] = a.lapTimes[i].time           // Update Split
					a.resultsTable[5][laneNum] = a.lapTimes[i].calculatedTime // Update Time
				}
			}

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
								adjustedTimeStr := formatTime(adjustedTime)
								a.tableRows[row].timeLabel.SetText(adjustedTimeStr)
								a.lapTimes[row].calculatedTime = adjustedTimeStr
							} else {
								a.tableRows[row].timeLabel.SetText(formatTime(lapTime))
								a.lapTimes[row].calculatedTime = formatTime(lapTime)
							}
						}

						// Update resultsTable if OOF matches a lane number
						if oof := a.lapTimes[row].oof; oof != emptyString {
							if laneNum, err := strconv.Atoi(oof); err == nil && laneNum >= 1 && laneNum <= 6 {
								// Update Place, Split, and Time rows in resultsTable
								a.resultsTable[3][laneNum] = a.tableRows[row].placeButton.Text // Update Place
								a.resultsTable[4][laneNum] = text                              // Update Split
								a.resultsTable[5][laneNum] = a.lapTimes[row].calculatedTime    // Update Time
								a.window.Content().Refresh()
							}
						}
					}
				}
			}
		} else {
			// Clear row
			a.tableRows[i].oofEntry.SetText(emptyString)
			a.tableRows[i].oofEntry.Disable()
			a.tableRows[i].placeButton.SetText(emptyString)
			a.tableRows[i].splitEntry.SetText(emptyString)
			a.tableRows[i].timeLabel.SetText(emptyString)
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

	title := canvas.NewText(titleText, color.White)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter
	title.TextSize = 48

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

	// Create the main content
	content := raceApp.setupContent()

	// Create the action buttons
	refereeButton := widget.NewButton("Referee Approval", func() {
		raceApp.showRefereeApproval(race)
	})
	refereeButton.Disable() // Initially disabled until winning time is set

	saveButton := widget.NewButton("Save", func() {
		// Save logic will be implemented later
	})
	saveButton.Disable() // Initially disabled until approved

	// Create a container for the buttons
	buttonContainer := container.NewHBox(
		layout.NewSpacer(),
		refereeButton,
		layout.NewSpacer(),
		saveButton,
		layout.NewSpacer(),
	)

	// Create the final content with all elements
	finalContent := container.NewVBox(
		container.NewCenter(title),
		content,
		buttonContainer,
	)

	raceWindow.SetContent(finalContent)
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

// showRefereeApproval creates and shows the referee approval window
func (a *App) showRefereeApproval(race RaceData) {
	// Create a new window for referee approval
	approvalWindow := a.app.NewWindow(fmt.Sprintf("Referee Approval - Race %d", race.RaceNumber))

	// Create the title
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

	title := canvas.NewText(titleText, color.White)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter
	title.TextSize = 48

	// Create the table data
	tableData := make([][]string, 0)
	headers := []string{"OOF", "Place", "Split", "Time", "School"}
	tableData = append(tableData, headers)

	// First add numerical places in order
	for i := 1; i <= 6; i++ {
		for lane := 1; lane <= 6; lane++ {
			if a.resultsTable[3][lane] == fmt.Sprintf("%d", i) {
				row := []string{
					fmt.Sprintf("%d", lane),
					a.resultsTable[3][lane],
					a.resultsTable[4][lane],
					a.resultsTable[5][lane],
					a.resultsTable[1][lane],
				}
				tableData = append(tableData, row)
			}
		}
	}

	// Then add DQ/DNS/DNF entries
	for lane := 1; lane <= 6; lane++ {
		place := a.resultsTable[3][lane]
		if place == "DQ" || place == "DNS" || place == "DNF" {
			row := []string{
				fmt.Sprintf("Lane %d", lane),
				place,
				a.resultsTable[4][lane],
				a.resultsTable[5][lane],
				a.resultsTable[1][lane],
			}
			tableData = append(tableData, row)
		}
	}

	// Create the table using a grid layout
	table := container.NewGridWithColumns(5)

	// Add all cells to the grid
	for i, row := range tableData {
		for col, cell := range row {
			text := canvas.NewText(cell, color.Black)
			if i == 0 { // Header row
				text.TextStyle = fyne.TextStyle{Bold: true}
			} else {
				text.TextStyle = fyne.TextStyle{Monospace: true}
			}
			// Left align the school column (index 4), center all others
			if col == 4 { // School column
				text.Alignment = fyne.TextAlignLeading
			} else {
				text.Alignment = fyne.TextAlignCenter
			}
			text.TextSize = 48

			// Create a container with alternating background colors
			var bgColor color.Color
			if col%2 == 0 {
				bgColor = color.White
			} else {
				bgColor = color.RGBA{R: 217, G: 217, B: 217, A: 255} // Light gray
			}

			// Create a rectangle for the background
			rect := canvas.NewRectangle(bgColor)
			rect.Resize(fyne.NewSize(200, 100)) // Set a specific size for the rectangle

			// Create a container with the background and text
			cellContainer := container.NewStack(
				rect,
				container.NewPadded(text),
			)
			table.Add(cellContainer)
		}
	}

	// Create the action buttons
	approveButton := widget.NewButton("Approve", func() {
		// Find the race in regattaData and set its Approved flag
		for i := range a.regattaData.Races {
			if a.regattaData.Races[i].RaceNumber == race.RaceNumber {
				a.regattaData.Races[i].Approved = true
				// Find and enable the Save button in the main window
				for _, content := range a.window.Content().(*fyne.Container).Objects {
					if buttonContainer, ok := content.(*fyne.Container); ok {
						for _, button := range buttonContainer.Objects {
							if saveButton, ok := button.(*widget.Button); ok && saveButton.Text == "Save" {
								saveButton.Enable()
								break
							}
						}
					}
				}
				break
			}
		}
		approvalWindow.Close()
	})

	cancelButton := widget.NewButton("Cancel", func() {
		approvalWindow.Close()
	})

	// Create a container for the buttons
	buttonContainer := container.NewHBox(
		layout.NewSpacer(),
		approveButton,
		layout.NewSpacer(),
		cancelButton,
		layout.NewSpacer(),
	)

	// Create the final content
	content := container.NewVBox(
		container.NewCenter(title),
		table,
		buttonContainer,
	)

	approvalWindow.SetContent(content)
	approvalWindow.Resize(fyne.NewSize(1000, 800))
	approvalWindow.Show()
}

func (a *App) setupWinningTime() {
	a.winningTime = widget.NewEntry()
	a.winningTime.SetPlaceHolder("00:00.0")
	a.winningTime.OnChanged = func(text string) {
		// If winning time is empty, just disable referee button
		if text == "" {
			// Find and disable the referee button
			for _, content := range a.window.Content().(*fyne.Container).Objects {
				if buttonContainer, ok := content.(*fyne.Container); ok {
					for _, button := range buttonContainer.Objects {
						if refereeButton, ok := button.(*widget.Button); ok && refereeButton.Text == "Referee Approval" {
							refereeButton.Disable()
							break
						}
					}
				}
			}
			// Clear all results table times
			for i := 1; i <= 6; i++ {
				a.resultsTable[5][i] = emptyString
			}
			a.window.Content().Refresh() // Refresh the window
			return
		}

		// Try to parse the winning time
		winningTime, err := parseTime(text)
		if err != nil {
			// Invalid time format, disable referee button
			for _, content := range a.window.Content().(*fyne.Container).Objects {
				if buttonContainer, ok := content.(*fyne.Container); ok {
					for _, button := range buttonContainer.Objects {
						if refereeButton, ok := button.(*widget.Button); ok && refereeButton.Text == "Referee Approval" {
							refereeButton.Disable()
							break
						}
					}
				}
			}
			return
		}

		// If we have a valid winning time and at least one lap time, enable the referee button
		if len(a.lapTimes) > 0 {
			firstLapTime, err := parseTime(a.lapTimes[0].time)
			if err == nil {
				// Calculate the time adjustment
				timeAdjustment := winningTime - firstLapTime

				// Update all lap times and results table
				for i := 0; i < len(a.lapTimes); i++ {
					lapTime, err := parseTime(a.lapTimes[i].time)
					if err == nil {
						adjustedTime := lapTime + timeAdjustment
						adjustedTimeStr := formatTime(adjustedTime)
						a.lapTimes[i].calculatedTime = adjustedTimeStr

						// Update results table if OOF is set
						if oof := a.lapTimes[i].oof; oof != emptyString {
							if laneNum, err := strconv.Atoi(oof); err == nil && laneNum >= 1 && laneNum <= 6 {
								a.resultsTable[5][laneNum] = adjustedTimeStr
							}
						}
					}
				}

				// Find and enable the referee button
				for _, content := range a.window.Content().(*fyne.Container).Objects {
					if buttonContainer, ok := content.(*fyne.Container); ok {
						for _, button := range buttonContainer.Objects {
							if refereeButton, ok := button.(*widget.Button); ok && refereeButton.Text == "Referee Approval" {
								refereeButton.Enable()
								break
							}
						}
					}
				}
				a.refreshContent()           // Refresh content when winning time is set
				a.window.Content().Refresh() // Refresh the window
			}
		}
	}
}

func (a *App) setupResultsTable() *widget.Table {
	// Create the results table widget
	resultsTable := widget.NewTable(
		func() (int, int) {
			return len(a.resultsTable), len(a.resultsTable[0])
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			label := cell.(*widget.Label)
			label.SetText(a.resultsTable[id.Row][id.Col])
		},
	)

	// Set column widths
	resultsTable.SetColumnWidth(0, 100) // Lane
	for i := 1; i <= 6; i++ {
		resultsTable.SetColumnWidth(i, 150) // Lane times
	}

	// Store the widget reference
	a.resultsTableWidget = resultsTable

	return resultsTable
}
