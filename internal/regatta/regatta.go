package regatta

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/assets"
	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/filesystem"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
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
	button := widget.NewButton(common.LoadExcelButtonText, func() { r.loader(false) })
	button.Disable()

	r.loadState = &loadState{
		loadButton: button,
	}

}

// NewRegatta - loads Regata object
func NewRegatta(app fyne.App) *Regatta {
	regattaApp := &Regatta{
		window:      app.NewWindow(common.AppTitle),
		App:         app,
		title:       text.Header2(common.EmptyString),
		subtitle:    text.Header3(common.EmptyString),
		date:        text.Header3(common.EmptyString),
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

	// On a restored session this runs before any view has been set.
	if content := r.window.Content(); content != nil {
		content.Refresh()
	}
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
		r.showWelcome()
		return
	}

	r.startLogging()

	if err := r.loadRegattaData(); err != nil {
		// A missing file is the normal first run for a configured directory: the
		// user has chosen where their data lives but has not imported a regatta.
		if !errors.Is(err, fs.ErrNotExist) {
			r.warnOnStarted(err)
		}
		r.loadState.loadButton.Enable()
		r.showWelcome()
		return
	}

	applog.Info("regatta history restored", "component", "startup", "races", r.RegattaData.ScheduledRaces())
	r.refreshContent()
	r.showRaceTree()
}

// startLogging points applog at a file in the regatta directory once one is
// known. The path is provisional: the persona phases replace it with
// logs/<team>/<role>-<hostname>.log. A failure here is not fatal - timing and
// export continue without a log.
func (r *Regatta) startLogging() {
	dir := r.App.Preferences().String(common.PrefRegattaDir)
	if dir == common.EmptyString {
		return
	}

	host, _ := os.Hostname()
	applog.SetIdentity(common.EmptyString, common.EmptyString, common.EmptyString, host)

	name := "regattaClock-" + filesystem.SanitizeForFilename(host) + ".log"
	logPath := filepath.Join(dir, common.RegattaDataDir, common.LogsDir, name)
	if err := applog.SetOutput(logPath); err != nil {
		applog.Warn("log file unavailable", "component", "startup", "err", err)
	}
}

// showWelcome - view presented until a regatta has been imported. The two steps
// are numbered because both buttons open a similar looking file browser, so the
// labels alone do not make the order obvious.
func (r *Regatta) showWelcome() {
	r.window.SetContent(container.NewVBox(
		container.New(
			layout.NewCustomPaddedLayout(viewMargin, 0, 0, 0),
			container.NewCenter(banner(welcomeBannerWidth, welcomeBannerHeight)),
		),
		welcomeStep(common.WelcomeSetDirText, r.regattaDirButton()),
		welcomeStep(common.WelcomeLoadFileText, r.loadState.loadButton),
	))
}

// banner - branding image at the caller's size. The size is explicit rather than
// taken from the source file so swapping in the full resolution artwork cannot
// change any layout.
func banner(width, height float32) *canvas.Image {
	logo := canvas.NewImageFromResource(
		fyne.NewStaticResource(common.BannerResourceName, assets.RegattaClockBannerSmall),
	)
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(width, height))

	return logo
}

// welcomeStep - instruction above the button that carries it out, both inset
// from the window edge so they share one left margin.
func welcomeStep(instruction string, action *widget.Button) *fyne.Container {
	return container.NewVBox(
		container.New(
			layout.NewCustomPaddedLayout(viewMargin, 0, viewMargin, viewMargin),
			text.BoldLeading(instruction),
		),
		container.New(
			layout.NewCustomPaddedLayout(0, 0, viewMargin, 0),
			// HBox keeps the button at its natural width instead of stretching it
			// across the row.
			container.NewHBox(action),
		),
	)
}

// warnOnStarted - report err once the window is on screen. Dialogs raised while
// NewRegatta is still running have no visible canvas to draw onto.
func (r *Regatta) warnOnStarted(err error) {
	applog.Error("startup warning", "component", "startup", "err", err)
	r.App.Lifecycle().SetOnStarted(func() {
		dialog.ShowError(err, r.window)
	})
}

// directorSession - the persona.Session the single-operator app acts as for
// schedule ownership: it establishes regattaData and owns
// director/regattaSchedule.json, which is exactly the Regatta Director's role.
// The persona picker (phase 5) and the separate director binary (phase 8)
// replace this with a real chosen session. Returns false when no regatta
// directory has been configured yet.
func (r *Regatta) directorSession() (persona.Session, bool) {
	regattaDir := r.App.Preferences().String(common.PrefRegattaDir)
	if regattaDir == common.EmptyString {
		return persona.Session{}, false
	}
	return persona.Session{
		Definition: persona.DirectorDefinition,
		Root:       filepath.Join(regattaDir, common.RegattaDataDir),
	}, true
}

func (r *Regatta) saveRegattaData() {
	session, ok := r.directorSession()
	if !ok {
		return
	}

	if err := store.SaveSchedule(session, scheduleFromRegattaData(r.RegattaData)); err != nil {
		r.warnSaveSkipped(err)
	}
}

func (r *Regatta) loadRegattaData() error {
	session, ok := r.directorSession()
	if !ok {
		return fs.ErrNotExist
	}

	r.migrateLegacyData(session)

	schedule, err := store.LoadSchedule(session)
	if err != nil {
		return err
	}

	r.RegattaData = regattaDataFromSchedule(schedule)
	return nil
}

// migrateLegacyData performs the one-time move of the pre-persona
// regattaData/data.json (or an interim director/data.json) into
// director/regattaSchedule.json, stripping the result fields on the way. It is
// a no-op once the schedule file exists, which is the normal case.
func (r *Regatta) migrateLegacyData(session persona.Session) {
	schedulePath := session.SchedulePath()
	if filesystem.FileExists(schedulePath) {
		return
	}

	candidates := []string{
		filepath.Join(session.Root, common.LegacyDataFile),               // regattaData/data.json
		filepath.Join(filepath.Dir(schedulePath), common.LegacyDataFile), // regattaData/director/data.json
	}
	legacy := ""
	for _, c := range candidates {
		if filesystem.FileExists(c) {
			legacy = c
			break
		}
	}
	if legacy == common.EmptyString {
		return
	}

	var legacyData reader.RegattaData
	if err := filesystem.ReadJSONFile(&legacyData, legacy); err != nil {
		applog.Error("legacy schedule migration failed", "component", "migrate", "file", legacy, "err", err)
		return
	}

	if err := store.SaveSchedule(session, scheduleFromRegattaData(&legacyData)); err != nil {
		applog.Error("legacy schedule migration failed", "component", "migrate", "file", legacy, "err", err)
		return
	}

	if err := os.Rename(legacy, legacy+".migrated"); err != nil {
		applog.Warn("legacy data file could not be renamed aside", "component", "migrate", "file", legacy, "err", err)
	}
	applog.Info("migrated legacy data.json to regattaSchedule.json", "component", "migrate",
		"from", legacy, "to", schedulePath, "races", len(legacyData.Races))
}

// warnSaveSkipped - report that the regatta directory could not be written to.
// Timing and export still work, the session just will not be restored next start.
func (r *Regatta) warnSaveSkipped(err error) {
	applog.Error("regatta data save skipped", "component", "persist", "err", err)
	dialog.ShowInformation(
		common.SaveSkippedTitle,
		fmt.Sprintf(common.SaveSkippedMessage, err),
		r.window,
	)
}
