package regatta

import (
	"fmt"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/filesystem"
	"github.com/comagnaw/regattaClock/internal/reader"
	"github.com/comagnaw/regattaClock/internal/text"
)

// Regatta represents the main application
type Regatta struct {

	// window - main app window
	window fyne.Window

	// App - app passed by main into NewRegatta
	App fyne.App

	loadState *loadState

	lastView fyne.CanvasObject

	config *fyne.Container

	// title - text field that represents imported title from RegattaData
	title *canvas.Text

	// date - text field that represents imported date from RegattaData
	date *canvas.Text

	// subtitle - text field that represents imported number of races from RegattaData
	subtitle *canvas.Text

	// RegattaData - reference to loaded RegattaData
	RegattaData *reader.RegattaData
}

type loadState struct {
	loadButton *widget.Button
}

func (r *Regatta) newLoadState() {
	button := widget.NewButton(common.NextButtonText, func() { r.loader(false) })
	button.Disable()

	r.loadState = &loadState{
		loadButton: button,
	}

}

// NewRegatta - loads Regata object
func NewRegatta(app fyne.App) *Regatta {
	regattaApp := &Regatta{
		window:   app.NewWindow(common.AppTitle),
		App:      app,
		title:    text.Header2(common.EmptyString),
		subtitle: text.Header3(common.EmptyString),
		date:     text.Header3(common.EmptyString),
		RegattaData: reader.NewRegattaData(),
	}
	regattaApp.setTheme(regattaApp.App.Preferences().String(common.PrefTheme))
	regattaApp.window.SetMaster()
	regattaApp.window.SetMainMenu(regattaApp.makeMenu())
	regattaApp.window.Resize(fyne.NewSize(regattaWidth, regattaHeight))
	regattaApp.newLoadState()
	regattaApp.config = regattaApp.configContent()

	regattaApp.initRegatta()
	

	return regattaApp
}

// Run - start the main app
func (r *Regatta) Run() {
	r.window.ShowAndRun()
}

func (r *Regatta) refreshContent() {
	r.title.Text = r.RegattaData.Name
	r.subtitle.Text = fmt.Sprintf(common.NumScheduledRacesTitle, r.RegattaData.ScheduledRaces())
	r.date.Text = r.RegattaData.Date
	r.window.Content().Refresh()
}

// setupStartupDialog - present dialog asking to load RegattaData
func (r *Regatta) setupStartupDialog() {
	// Create a custom dialog
	dialog.ShowCustomConfirm(
		common.LoadDataTitle,
		common.LoadButtonText,
		common.CancelButtonText,
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

func (r *Regatta) initRegatta() {

	if r.App.Preferences().String(common.PrefRegattaDir) == common.EmptyString {

		initFirstRegatta := container.NewVBox(
			container.NewCenter(text.Bold("Welcome to Regatta Clock!")),
			container.NewCenter(text.Bold("Please set the directory for loading and saving regatta data.")),
			r.regattaDir(),
			container.NewCenter(
				r.loadState.loadButton,
			),
		)

		r.window.SetContent(initFirstRegatta)
	} else {
		r.loadRegattaData()
		r.refreshContent()
		r.showRaceTree()
	}

}


func (r *Regatta) regattaFile() string {
	return filepath.Join(r.App.Preferences().String(common.PrefRegattaDir), "regattaData", "data.json")
}


func (r *Regatta) saveRegattaData() {
	err := filesystem.SaveJSONFile(r.RegattaData, r.regattaFile())
	if err != nil {
		dialog.ShowError(err, r.window)
	}
}

func (r *Regatta) loadRegattaData() {
	err := filesystem.ReadJSONFile(r.RegattaData, r.regattaFile())
	if err != nil {
		dialog.ShowError(err, r.window)
	}
}