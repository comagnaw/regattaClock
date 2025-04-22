package regattaClock

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
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

type App struct {
	window    fyne.Window
	clock     *canvas.Text
	table     *widget.Table
	lapTimes  []lapTime
	isRunning bool
	startTime time.Time
}

func NewApp() *App {
	a := app.New()
	w := a.NewWindow("Clock")
	w.Resize(fyne.NewSize(800, 1000)) // Increased window height significantly

	// Create the clock text with custom size
	clockText := canvas.NewText("00:00:00.000", theme.ForegroundColor())
	clockText.TextStyle = fyne.TextStyle{
		Monospace: true,
		Bold:      true,
	}
	clockText.Alignment = fyne.TextAlignCenter
	clockText.TextSize = 48

	app := &App{
		window:    w,
		clock:     clockText,
		lapTimes:  make([]lapTime, 0),
		isRunning: false,
		startTime: time.Now(),
	}

	app.setupTable()
	app.setupButtons()
	app.setupKeyboardHandler()

	return app
}

func (a *App) setupTable() {
	a.table = widget.NewTable(
		func() (int, int) {
			rows := len(a.lapTimes)
			if rows < 20 {
				rows = 20
			}
			return rows, 2
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Resize(fyne.NewSize(200, 30))
			return label
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			label := cell.(*widget.Label)
			if id.Row < len(a.lapTimes) {
				if id.Col == 0 {
					label.SetText(a.lapTimes[id.Row].time)
				} else {
					label.SetText(fmt.Sprintf("Boat %d", a.lapTimes[id.Row].number))
				}
			} else {
				label.SetText("")
			}
		},
	)
	a.table.SetColumnWidth(0, 200)
	a.table.SetColumnWidth(1, 100)

	// Set a fixed size for the table
	a.table.Resize(fyne.NewSize(300, 600))
}

func (a *App) setupButtons() {
	startButton := widget.NewButton("Start (F2)", func() {
		if !a.isRunning {
			a.startTime = time.Now()
			a.isRunning = true
			a.lapTimes = append(a.lapTimes, lapTime{
				number: 1,
				time:   "00:00:00.000",
			})
			a.table.Refresh()
		}
	})

	lapButton := widget.NewButton("Lap (F4)", func() {
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
			})
			a.table.Refresh()
		}
	})

	stopButton := widget.NewButton("Stop", func() {
		a.isRunning = false
		a.clock.Text = "00:00:00.000"
		a.clock.Refresh()
	})

	clearButton := widget.NewButton("Clear", func() {
		a.lapTimes = make([]lapTime, 0)
		a.table.Refresh()
	})

	// Create a container with centered buttons and spacing
	buttonContainer := container.NewHBox(
		layout.NewSpacer(),
		startButton,
		layout.NewSpacer(),
		lapButton,
		layout.NewSpacer(),
		stopButton,
		layout.NewSpacer(),
		clearButton,
		layout.NewSpacer(),
	)

	// Center the clock in its own container
	clockContainer := container.NewCenter(a.clock)

	// Create a fixed-size container for the table
	tableContainer := container.NewMax(a.table)
	tableContainer.Resize(fyne.NewSize(300, 600))

	content := container.NewVBox(
		clockContainer,
		buttonContainer,
		widget.NewLabel("Boat Times:"),
		tableContainer,
	)

	// Set a minimum size for the content
	content.Resize(fyne.NewSize(800, 1000))

	a.window.SetContent(content)
}

func (a *App) setupKeyboardHandler() {
	handler := &keyboardHandler{
		startTime: &a.startTime,
		isRunning: &a.isRunning,
		startFunc: func() {
			a.lapTimes = append(a.lapTimes, lapTime{
				number: 1,
				time:   "00:00:00.000",
			})
			a.table.Refresh()
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
				})
				a.table.Refresh()
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
