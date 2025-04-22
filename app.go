package regattaClock

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
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
	number int
	time   string
	oof    string // Add OOF field
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

type TableRow struct {
	oofEntry   *widget.Entry
	placeLabel *widget.Label
	splitLabel *widget.Label
}

type App struct {
	window    fyne.Window
	clock     *canvas.Text
	tableRows []TableRow
	lapTimes  []lapTime
	isRunning bool
	startTime time.Time
}

func (a *App) setupTable() {
	// Create a container for the table
	tableContainer := container.NewVBox()

	// Create header row with fixed widths
	headerRow := container.NewHBox(
		widget.NewLabelWithStyle("OOF", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Place", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Split", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
	)
	// Set fixed widths for header cells
	headerRow.Objects[0].Resize(fyne.NewSize(50, 30))  // OOF
	headerRow.Objects[1].Resize(fyne.NewSize(60, 30))  // Place
	headerRow.Objects[2].Resize(fyne.NewSize(150, 30)) // Split
	tableContainer.Add(headerRow)

	// Create data rows
	a.tableRows = make([]TableRow, 10)
	for i := 0; i < 10; i++ {
		row := container.NewHBox()

		// OOF entry
		oofEntry := widget.NewEntry()
		oofEntry.Resize(fyne.NewSize(50, 30))
		oofEntry.Disable()

		// Place label with padding
		placeLabel := widget.NewLabel("")
		placeLabel.TextStyle = fyne.TextStyle{Bold: true}
		placeLabel.Alignment = fyne.TextAlignTrailing // Right-align the numbers
		placeLabel.Resize(fyne.NewSize(60, 30))

		// Split label
		splitLabel := widget.NewLabel("")
		splitLabel.TextStyle = fyne.TextStyle{Bold: true}
		splitLabel.Resize(fyne.NewSize(150, 30))

		// Store the widgets
		a.tableRows[i] = TableRow{
			oofEntry:   oofEntry,
			placeLabel: placeLabel,
			splitLabel: splitLabel,
		}

		row.Add(oofEntry)
		row.Add(placeLabel)
		row.Add(splitLabel)

		tableContainer.Add(row)
	}

	// Create a scroll container for the table
	scrollContainer := container.NewScroll(tableContainer)
	scrollContainer.Resize(fyne.NewSize(350, 330))

	// Add the table to the window content
	content := container.NewBorder(
		container.NewVBox( // Top
			container.NewCenter(a.clock),
			container.NewHBox(
				layout.NewSpacer(),
				widget.NewButton("Start (F2)", func() {
					if !a.isRunning {
						a.startTime = time.Now()
						a.isRunning = true
						a.lapTimes = append(a.lapTimes, lapTime{
							number: 1,
							time:   "00:00:00.000",
							oof:    "",
						})
						a.refreshTable()
					}
				}),
				layout.NewSpacer(),
				widget.NewButton("Lap (F4)", func() {
					if a.isRunning {
						elapsed := time.Since(a.startTime)
						hours := int(elapsed.Hours())
						minutes := int(elapsed.Minutes()) % 60
						seconds := int(elapsed.Seconds()) % 60
						milliseconds := int(elapsed.Milliseconds()) % 1000
						formatted := time.Date(0, 0, 0, hours, minutes, seconds, milliseconds*1000000, time.UTC).Format("15:04:05.000")

						a.lapTimes = append(a.lapTimes, lapTime{
							number: len(a.lapTimes) + 1,
							time:   formatted,
							oof:    "",
						})
						a.refreshTable()
					}
				}),
				layout.NewSpacer(),
				widget.NewButton("Stop", func() {
					a.isRunning = false
					a.refreshTable()
				}),
				layout.NewSpacer(),
				widget.NewButton("Clear", func() {
					a.isRunning = false
					a.clock.Text = "00:00:00.000"
					a.clock.Refresh()
					a.lapTimes = make([]lapTime, 0)
					a.refreshTable()
				}),
				layout.NewSpacer(),
			),
		),
		nil,             // Bottom
		nil,             // Left
		nil,             // Right
		scrollContainer, // Center content
	)

	// Set a minimum size for the content
	content.Resize(fyne.NewSize(800, 1000))

	a.window.SetContent(content)
}

func (a *App) refreshTable() {
	// Update all rows
	for i := 0; i < 10; i++ {
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
			a.tableRows[i].placeLabel.SetText(fmt.Sprintf("%d", a.lapTimes[i].number))

			// Set Split label
			a.tableRows[i].splitLabel.SetText(a.lapTimes[i].time)
		} else {
			// Clear row
			a.tableRows[i].oofEntry.SetText("")
			a.tableRows[i].oofEntry.Disable()
			a.tableRows[i].placeLabel.SetText("")
			a.tableRows[i].splitLabel.SetText("")
		}
	}
}

func (a *App) setupKeyboardHandler() {
	handler := &keyboardHandler{
		startTime: &a.startTime,
		isRunning: &a.isRunning,
		startFunc: func() {
			a.lapTimes = append(a.lapTimes, lapTime{
				number: 1,
				time:   "00:00:00.000",
				oof:    "",
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
				hours := int(elapsed.Hours())
				minutes := int(elapsed.Minutes()) % 60
				seconds := int(elapsed.Seconds()) % 60
				milliseconds := int(elapsed.Milliseconds()) % 1000
				formatted := time.Date(0, 0, 0, hours, minutes, seconds, milliseconds*1000000, time.UTC).Format("15:04:05.000")

				a.lapTimes = append(a.lapTimes, lapTime{
					number: len(a.lapTimes) + 1,
					time:   formatted,
					oof:    "",
				})
				a.refreshTable()
			}
		},
	}

	a.window.Canvas().SetOnTypedKey(handler.TypedKey)
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
		lapTimes:  make([]lapTime, 0),
		isRunning: false,
	}

	// Create the clock display
	regattaApp.clock = canvas.NewText("00:00:00.000", color.White)
	regattaApp.clock.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	regattaApp.clock.Alignment = fyne.TextAlignCenter
	regattaApp.clock.TextSize = 48

	// Set up the window
	regattaApp.window.Resize(fyne.NewSize(800, 1000))
	regattaApp.window.SetMaster()

	// Set up the table and keyboard handler
	regattaApp.setupTable()
	regattaApp.setupKeyboardHandler()

	return regattaApp
}
