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
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// CustomTheme overrides the default theme to provide larger text for the clock
type CustomTheme struct{}

func (CustomTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(name, variant)
}

func (CustomTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (CustomTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (CustomTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameText {
		return 48 // Larger text size for the clock
	}
	return theme.DefaultTheme().Size(name)
}

type lapTime struct {
	number         int
	time           string
	calculatedTime string
	oof            string
	dq             bool
}

type keyboardHandler struct {
	startTime *time.Time
	isRunning *bool
	startFunc func()
	stopFunc  func()
	lapFunc   func()
}

func (h *keyboardHandler) TypedKey(event *fyne.KeyEvent) {
	switch event.Name {
	case fyne.KeyF2:
		if !*h.isRunning {
			*h.startTime = time.Now()
			*h.isRunning = true
			h.startFunc()
		}
	case fyne.KeyF4:
		if *h.isRunning {
			h.lapFunc()
		}
	}
}

func (h *keyboardHandler) TypedRune(rune) {}

type LapTableRow struct {
	oofEntry   *widget.Entry
	placeLabel *widget.Label
	splitLabel *widget.Label
	timeLabel  *widget.Label
	dqCheck    *widget.Check
}

// LapTableData represents a row in the lap table
type LapTableData struct {
	OOF   string `table:"OOF"`
	DQ    bool   `table:"DQ"`
	Place string `table:"Place"`
	Split string `table:"Split"`
	Time  string `table:"Time"`
}


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
	systray        fyne.Window
	raceNumber     *widget.Entry
	winningTime    *widget.Entry
	regattaData    *RegattaData
}

func (a *App) setupTables() {
	// Create a container for both tables with proper spacing
	tablesContainer := container.NewVBox()

	// Create lap table header
	lapHeader := container.NewHBox(
		widget.NewLabelWithStyle("OOF", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("DQ", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Place", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Split", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Time", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
	)
	lapHeader.Resize(fyne.NewSize(800, 30))

	tablesContainer.Add(lapHeader)

	// Initialize tableRows
	a.tableRows = make([]LapTableRow, 6)
	for i := 0; i < 6; i++ {
		row := container.NewHBox()

		// Create widgets for each column
		oofEntry := widget.NewEntry()
		oofEntry.Resize(fyne.NewSize(80, 30))

		dqCheck := widget.NewCheck("", nil)
		dqCheck.Resize(fyne.NewSize(30, 30))

		placeLabel := widget.NewLabel("")
		placeLabel.Resize(fyne.NewSize(80, 30))

		splitLabel := widget.NewLabel("")
		splitLabel.Resize(fyne.NewSize(280, 30))

		timeLabel := widget.NewLabel("")
		timeLabel.Resize(fyne.NewSize(280, 30))

		// Add widgets to row
		row.Add(oofEntry)
		row.Add(dqCheck)
		row.Add(placeLabel)
		row.Add(splitLabel)
		row.Add(timeLabel)

		// Store the widgets
		a.tableRows[i] = LapTableRow{
			oofEntry:   oofEntry,
			placeLabel: placeLabel,
			splitLabel: splitLabel,
			timeLabel:  timeLabel,
			dqCheck:    dqCheck,
		}

		// Add row to container
		tablesContainer.Add(row)
	}

	// Add event handler for winning time changes
	a.winningTime.OnChanged = func(text string) {
		a.refreshTable()
	}

	// Create the main content container
	mainContent := container.NewVBox(
		container.NewCenter(a.clock),
		container.NewCenter(a.regattaTitle),
		container.NewCenter(a.scheduledRaces),
		container.NewCenter(a.regattaDate),
		container.NewHBox(
			layout.NewSpacer(),
			widget.NewButton("Start (F2)", func() {
				if !a.isRunning {
					a.startTime = time.Now()
					a.isRunning = true
					a.lapTimes = append(a.lapTimes, lapTime{
						number:         1,
						time:           "00:00.0",
						calculatedTime: "00:00.0",
						oof:            "",
						dq:             false,
					})
					a.refreshTable()
					a.raceNumber.Disable()
					a.winningTime.Disable()
				}
			}),
			layout.NewSpacer(),
			widget.NewButton("Lap (F4)", func() {
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
					a.refreshTable()
				}
			}),
			layout.NewSpacer(),
			widget.NewButton("Stop", func() {
				a.isRunning = false
				a.refreshTable()
				a.raceNumber.Enable()
				a.winningTime.Enable()
			}),
			layout.NewSpacer(),
			widget.NewButton("Clear", func() {
				a.isRunning = false
				a.clock.Text = "00:00:00.000"
				a.clock.Refresh()
				a.lapTimes = make([]lapTime, 0)
				a.refreshTable()
				a.raceNumber.Enable()
				a.winningTime.Enable()
			}),
			layout.NewSpacer(),
		),
		container.NewHBox(
			layout.NewSpacer(),
			container.NewHBox(
				widget.NewLabel("Race Number:"),
				a.raceNumber,
				widget.NewButton("Load Race", func() {
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
				}),
			),
			layout.NewSpacer(),
			container.NewHBox(
				widget.NewLabel("Winning Time (00:00.0):"),
				a.winningTime,
			),
			layout.NewSpacer(),
		),
		tablesContainer,
	)

	// Create a scroll container for the main content
	scrollContent := container.NewScroll(mainContent)

	// Set the window content
	a.window.SetContent(scrollContent)
	a.window.Resize(fyne.NewSize(800, 1000))
}

func (a *App) setupTable() {
	// Setup the tables
	a.setupTables()

	// Create title labels
	a.regattaTitle = canvas.NewText("", color.White)
	a.regattaTitle.TextStyle = fyne.TextStyle{Bold: true}
	a.regattaTitle.Alignment = fyne.TextAlignCenter
	a.regattaTitle.TextSize = 24

	a.scheduledRaces = canvas.NewText("", color.White)
	a.scheduledRaces.TextStyle = fyne.TextStyle{Bold: true}
	a.scheduledRaces.Alignment = fyne.TextAlignCenter
	a.scheduledRaces.TextSize = 20

	a.regattaDate = canvas.NewText("", color.White)
	a.regattaDate.TextStyle = fyne.TextStyle{Bold: true}
	a.regattaDate.Alignment = fyne.TextAlignCenter
	a.regattaDate.TextSize = 20

	// Create input fields
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
	a.raceNumber.Resize(fyne.NewSize(400, 40))

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
	a.winningTime.Resize(fyne.NewSize(600, 40))
	a.winningTime.Enable()
	a.winningTime.TextStyle = fyne.TextStyle{Monospace: true}

	// Create labels for the input fields
	raceNumberLabel := widget.NewLabel("Race Number:")
	raceNumberLabel.TextStyle = fyne.TextStyle{Bold: true}
	winningTimeLabel := widget.NewLabel("Winning Time (00:00.0):")
	winningTimeLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Create a horizontal box for the input fields with labels and spacers
	inputBox := container.NewHBox(
		layout.NewSpacer(),
		container.NewHBox(
			raceNumberLabel,
			a.raceNumber,
			widget.NewButton("Load Race", func() {
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
			}),
		),
		layout.NewSpacer(),
		container.NewHBox(
			winningTimeLabel,
			a.winningTime,
		),
		layout.NewSpacer(),
	)

	// Create the main content container
	mainContent := container.NewVBox(
		container.NewCenter(a.clock),
		container.NewCenter(a.regattaTitle),
		container.NewCenter(a.scheduledRaces),
		container.NewCenter(a.regattaDate),
		container.NewHBox(
			layout.NewSpacer(),
			widget.NewButton("Start (F2)", func() {
				if !a.isRunning {
					a.startTime = time.Now()
					a.isRunning = true
					a.lapTimes = append(a.lapTimes, lapTime{
						number:         1,
						time:           "00:00.0",
						calculatedTime: "00:00.0",
						oof:            "",
						dq:             false,
					})
					a.refreshTable()
					a.raceNumber.Disable()
					a.winningTime.Disable()
				}
			}),
			layout.NewSpacer(),
			widget.NewButton("Lap (F4)", func() {
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
					a.refreshTable()
				}
			}),
			layout.NewSpacer(),
			widget.NewButton("Stop", func() {
				a.isRunning = false
				a.refreshTable()
				a.raceNumber.Enable()
				a.winningTime.Enable()
			}),
			layout.NewSpacer(),
			widget.NewButton("Clear", func() {
				a.isRunning = false
				a.clock.Text = "00:00:00.000"
				a.clock.Refresh()
				a.lapTimes = make([]lapTime, 0)
				a.refreshTable()
				a.raceNumber.Enable()
				a.winningTime.Enable()
			}),
			layout.NewSpacer(),
		),
		inputBox,
	)

	// Create a scroll container for the main content
	scrollContent := container.NewScroll(mainContent)

	// Set the window content
	a.window.SetContent(scrollContent)
	a.window.Resize(fyne.NewSize(800, 1000))
}

func (a *App) refreshTable() {
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
						a.refreshTable() // Refresh the entire table to update all place numbers
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

func (a *App) setupKeyboardHandler() {
	handler := &keyboardHandler{
		startTime: &a.startTime,
		isRunning: &a.isRunning,
		startFunc: func() {
			a.lapTimes = append(a.lapTimes, lapTime{
				number:         1,
				time:           "00:00.0",
				calculatedTime: "00:00.0",
				oof:            "",
				dq:             false,
			})
			a.refreshTable()
			a.startTime = time.Now()
			a.isRunning = true
		},
		stopFunc: func() {
			a.isRunning = false
			a.clock.Text = "00:00:00.000"
			a.clock.Refresh()
		},
		lapFunc: func() {
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
				a.refreshTable()
			}
		},
	}

	a.window.Canvas().SetOnTypedKey(handler.TypedKey)
}

func (a *App) setupSystray() {
	// Create a new window for the system tray
	a.systray = a.app.NewWindow("Regatta Clock")
	a.systray.SetIcon(theme.MailComposeIcon()) // You can replace this with a custom icon

	// Create menu items
	importItem := fyne.NewMenuItem("Import Regatta Table", func() {
		// Create a file dialog with .xlsx filter
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, a.window)
				return
			}
			if reader == nil {
				return
			}
			defer reader.Close()

			// Verify file extension
			uri := reader.URI()
			if uri.Extension() != ".xlsx" {
				dialog.ShowError(fmt.Errorf("only .xlsx files are supported"), a.window)
				return
			}

			// Get the file path from the URI
			filePath := uri.Path()
			fmt.Printf("Debug: Importing Excel file: %s\n", filePath)

			// Read the Excel file
			regattaData, err := ReadExcelFile(filePath)
			if err != nil {
				dialog.ShowError(err, a.window)
				return
			}

			// Store the regatta data
			a.regattaData = regattaData

			// Calculate scheduled races (races with at least one lane)
			scheduledRaces := 0
			for _, race := range regattaData.Races {
				if len(race.Lanes) > 0 {
					scheduledRaces++
				}
			}

			fmt.Printf("Debug: Successfully loaded regatta data - %d total races, %d scheduled races\n",
				len(regattaData.Races), scheduledRaces)
			fmt.Printf("Debug: Regatta Name: %s\n", regattaData.RegattaName)
			fmt.Printf("Debug: Regatta Date: %s\n", regattaData.Date)

			// Update the title, scheduled races count, and date
			a.regattaTitle.Text = regattaData.RegattaName
			a.scheduledRaces.Text = fmt.Sprintf("Scheduled Races: %d", scheduledRaces)
			a.regattaDate.Text = regattaData.Date
			a.regattaTitle.Refresh()
			a.scheduledRaces.Refresh()
			a.regattaDate.Refresh()

			// Show success message
			dialog.ShowInformation("Import", "Successfully read Excel file", a.window)
		}, a.window)
	})

	showWindowItem := fyne.NewMenuItem("Show Window", func() {
		a.window.Show()
	})

	exitItem := fyne.NewMenuItem("Exit", func() {
		a.app.Quit()
	})

	// Create the menu
	menu := fyne.NewMainMenu(fyne.NewMenu("Regatta Clock",
		importItem,
		showWindowItem,
		fyne.NewMenuItemSeparator(),
		exitItem,
	))

	// Set the menu
	a.systray.SetMainMenu(menu)
	a.systray.Show() // Show the system tray window
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

func NewApp(app fyne.App) *App {
	regattaApp := &App{
		window:    app.NewWindow("Regatta Clock"),
		app:       app,
		lapTimes:  make([]lapTime, 0),
		isRunning: false,
	}

	// Create the clock display
	regattaApp.clock = canvas.NewText("00:00:00.000", color.White)
	regattaApp.clock.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	regattaApp.clock.Alignment = fyne.TextAlignCenter
	regattaApp.clock.TextSize = 48

	// Create title labels
	regattaApp.regattaTitle = canvas.NewText("", color.White)
	regattaApp.regattaTitle.TextStyle = fyne.TextStyle{Bold: true}
	regattaApp.regattaTitle.Alignment = fyne.TextAlignCenter
	regattaApp.regattaTitle.TextSize = 24

	regattaApp.scheduledRaces = canvas.NewText("", color.White)
	regattaApp.scheduledRaces.TextStyle = fyne.TextStyle{Bold: true}
	regattaApp.scheduledRaces.Alignment = fyne.TextAlignCenter
	regattaApp.scheduledRaces.TextSize = 20

	regattaApp.regattaDate = canvas.NewText("", color.White)
	regattaApp.regattaDate.TextStyle = fyne.TextStyle{Bold: true}
	regattaApp.regattaDate.Alignment = fyne.TextAlignCenter
	regattaApp.regattaDate.TextSize = 20

	// Create input fields
	regattaApp.raceNumber = widget.NewEntry()
	regattaApp.raceNumber.Validator = func(s string) error {
		// Only allow numbers
		for _, r := range s {
			if r < '0' || r > '9' {
				return fmt.Errorf("only numbers allowed")
			}
		}
		return nil
	}
	regattaApp.raceNumber.Resize(fyne.NewSize(400, 40))

	regattaApp.winningTime = widget.NewEntry()
	regattaApp.winningTime.Validator = func(s string) error {
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
	regattaApp.winningTime.Resize(fyne.NewSize(600, 40))
	regattaApp.winningTime.Enable()
	regattaApp.winningTime.TextStyle = fyne.TextStyle{Monospace: true}

	// Set up the window
	regattaApp.window.Resize(fyne.NewSize(800, 1000))
	regattaApp.window.SetMaster()

	// Set up the system tray
	regattaApp.setupSystray()

	// Set up the table and keyboard handler
	regattaApp.setupTables()
	regattaApp.setupKeyboardHandler()

	// Show startup dialog to load Excel file
	regattaApp.showStartupDialog()

	return regattaApp
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
				// Show file dialog to select Excel file
				dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
					if err != nil {
						dialog.ShowError(err, a.window)
						return
					}
					if reader == nil {
						// User cancelled, show reminder
						dialog.ShowInformation(
							"Load Later",
							"You can load the Excel file later by selecting 'Import Regatta Table' from the menu.",
							a.window,
						)
						return
					}
					defer reader.Close()

					// Verify file extension
					uri := reader.URI()
					if uri.Extension() != ".xlsx" {
						dialog.ShowError(fmt.Errorf("only .xlsx files are supported"), a.window)
						return
					}

					// Get the file path from the URI
					filePath := uri.Path()
					fmt.Printf("Debug: Importing Excel file: %s\n", filePath)

					// Read the Excel file
					regattaData, err := ReadExcelFile(filePath)
					if err != nil {
						dialog.ShowError(err, a.window)
						return
					}

					// Store the regatta data
					a.regattaData = regattaData

					// Calculate scheduled races (races with at least one lane)
					scheduledRaces := 0
					for _, race := range regattaData.Races {
						if len(race.Lanes) > 0 {
							scheduledRaces++
						}
					}

					fmt.Printf("Debug: Successfully loaded regatta data - %d total races, %d scheduled races\n",
						len(regattaData.Races), scheduledRaces)
					fmt.Printf("Debug: Regatta Name: %s\n", regattaData.RegattaName)
					fmt.Printf("Debug: Regatta Date: %s\n", regattaData.Date)

					// Update the title, scheduled races count, and date
					a.regattaTitle.Text = regattaData.RegattaName
					a.scheduledRaces.Text = fmt.Sprintf("Scheduled Races: %d", scheduledRaces)
					a.regattaDate.Text = regattaData.Date
					a.regattaTitle.Refresh()
					a.scheduledRaces.Refresh()
					a.regattaDate.Refresh()

					// Show success message
					dialog.ShowInformation("Import", "Successfully read Excel file", a.window)
				}, a.window)
			} else {
				// User chose to cancel, show reminder
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

// handleRaceNumberChange is now just a placeholder since we moved the functionality to the button
func (a *App) handleRaceNumberChange(s string) {
	// This function is kept for compatibility but doesn't do anything
}

// updateTableSize is kept for compatibility but doesn't change the table size
func (a *App) updateTableSize(numLanes int) {
	// Refresh the table to update the data
	a.refreshTable()
}

// startClock starts the clock
func (a *App) startClock() {
	a.isRunning = true
	a.startTime = time.Now()
	a.lapTimes = append(a.lapTimes, lapTime{
		number:         1,
		time:           "00:00.0",
		calculatedTime: "00:00.0",
		oof:            "",
		dq:             false,
	})
	a.refreshTable()
	a.raceNumber.Disable()
	a.winningTime.Disable()
	a.clock.Text = "00:00:00.000"
	a.clock.Refresh()
}

// stopClock stops the clock
func (a *App) stopClock() {
	a.isRunning = false
	a.clock.Text = "00:00:00.000"
	a.clock.Refresh()
	a.raceNumber.Enable()
	a.winningTime.Enable()
}

// resetClock resets the clock
func (a *App) resetClock() {
	a.isRunning = false
	a.clock.Text = "00:00:00.000"
	a.clock.Refresh()
	a.lapTimes = make([]lapTime, 0)
	a.refreshTable()
	a.raceNumber.Enable()
	a.winningTime.Enable()
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
