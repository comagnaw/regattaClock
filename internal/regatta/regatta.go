package regatta

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/reader"
)

// Regatta represents the main application
type Regatta struct {

	// window - main app window
	window fyne.Window

	// App - app passed by main into NewRegatta
	App fyne.App

	// title - text field that represents imported title from RegattaData
	title *canvas.Text

	// date - text field that represents imported date from RegattaData
	date *canvas.Text

	// subtitle - text field that represents imported number of races from RegattaData
	subtitle *canvas.Text

	// RegattaData - reference to loaded RegattaData
	RegattaData *reader.RegattaData
}

// NewRegatta - loads Regata object
func NewRegatta(app fyne.App) *Regatta {
	regattaApp := &Regatta{
		window: app.NewWindow(common.AppTitle),
		App:    app,
	}

	regattaApp.initRegattaDetails()
	regattaApp.window.SetMaster()
	regattaApp.window.SetMainMenu(regattaApp.makeMenu())
	regattaApp.window.Resize(fyne.NewSize(500, 600))
	regattaApp.setupStartupDialog()

	return regattaApp
}

// Run - start the main app
func (r *Regatta) Run() {
	r.window.ShowAndRun()
}

// initRegattaDetails - load the textual fields that summarize the Regatta
func (r *Regatta) initRegattaDetails() {
	r.initTitle()
	r.initSubtitle()
	r.initDate()
}

func (r *Regatta) updateRegattaDetails() {
	r.updateTitle()
	r.updateSubTitle()
	r.updateDate()
}

// initTitle - initialize title of regatta
func (r *Regatta) initTitle() {
	r.title = canvas.NewText(common.EmptyString, color.White)
	r.title.TextStyle = fyne.TextStyle{Bold: true}
	r.title.Alignment = fyne.TextAlignCenter
	r.title.TextSize = 24
}

// updateTitle - use RegattaData.RegattaName to update title
func (r *Regatta) updateTitle() {
	r.title.Text = r.RegattaData.Name
	r.title.Refresh()
}

// initSubtitle - initialize subtitle of regatta
func (r *Regatta) initSubtitle() {
	r.subtitle = canvas.NewText(common.EmptyString, color.White)
	r.subtitle.TextStyle = fyne.TextStyle{Bold: true}
	r.subtitle.Alignment = fyne.TextAlignCenter
	r.subtitle.TextSize = 20
}

// updateTitle - use RegattaData.ScheduledRaces to update subtitle
func (r *Regatta) updateSubTitle() {
	r.subtitle.Text = fmt.Sprintf("Scheduled Races: %d", r.RegattaData.ScheduledRaces())
	r.subtitle.Refresh()
}

// initDate - initialize date of regatta
func (r *Regatta) initDate() {
	r.date = canvas.NewText(common.EmptyString, color.White)
	r.date.TextStyle = fyne.TextStyle{Bold: true}
	r.date.Alignment = fyne.TextAlignCenter
	r.date.TextSize = 20
}

// updateTitle - use RegattaData.Date to update date
func (r *Regatta) updateDate() {
	r.date.Text = r.RegattaData.Date
	r.date.Refresh()
}

// setupStartupDialog - present dialog asking to load RegattaData
func (r *Regatta) setupStartupDialog() {
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
				r.loader(true)
			} else {
				dialog.ShowInformation(
					"Load Later",
					"You can load the Excel file later by selecting 'Import Regatta Table' from the menu.",
					r.window,
				)
			}
		},
		r.window,
	)
}
