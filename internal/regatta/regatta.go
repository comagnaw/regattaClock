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
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/assets"
	"github.com/comagnaw/regattaClock/internal/clock"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/filesystem"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/personacfg"
	"github.com/comagnaw/regattaClock/internal/reader"
	"github.com/comagnaw/regattaClock/internal/text"
)

// mode records which kind of persona the operator picked, so the menu and race
// tree can differ without threading the role everywhere. It is modeUnset until
// the persona picker completes. The Regatta Director establishes regattaData and
// owns the schedule (Excel import); a timer picks one of the four timing
// personas and its menu carries no loader.
type mode int

const (
	modeUnset mode = iota
	modeDirector
	modeTimer
)

// Regatta represents the main application
type Regatta struct {

	// window - main app window
	window fyne.Window

	// App - the fyne.App passed by main into New
	App fyne.App

	// mode - unset until the persona picker resolves, then director or timer
	mode mode

	loadState *loadState

	lastView fyne.CanvasObject

	// treeContent - the race-tree Border set by showRaceTree, kept so the config
	// screen can recognise it was the last view and rebuild it (a theme change
	// does not repaint the header's reverse-contrast raw canvas objects).
	treeContent fyne.CanvasObject

	config *fyne.Container

	// versionWindow - the independent build-info window (menu > Version), or nil
	// when it is closed. Kept so a second menu click focuses it rather than
	// opening a duplicate.
	versionWindow fyne.Window

	// personaCfg - parsed deployment persona config (PrefPersonaConfigFile), or
	// nil when unset or unreadable. Pins this host to a persona and/or overrides
	// challenge codes; nil means the normal persona picker.
	personaCfg *personacfg.Config

	// persona - race-tree header line naming the operator's role, e.g.
	// "Role: Primary Start Timer". Empty until a session is bound.
	persona *canvas.Text

	// title - text field that represents imported title from RegattaData
	title *canvas.Text

	// date - text field that represents imported date from RegattaData
	date *canvas.Text

	// subtitle - text field that represents imported number of races from RegattaData
	subtitle *canvas.Text

	// themeVariant - the app's chosen theme (VariantLight / VariantDark), set by
	// setTheme. The race-tree details card is painted in reverse contrast off
	// this, rather than off Fyne's builtin palette, which follows the OS
	// appearance and can disagree with the in-app choice.
	themeVariant fyne.ThemeVariant

	// RegattaData - reference to loaded RegattaData
	RegattaData *reader.RegattaData

	// session - the chosen persona and regattaData root. Zero value until the
	// picker completes (or NewDirector binds it from PrefRegattaDir).
	session persona.Session

	// startLog / finishLog - in-memory mirrors held for the whole app run.
	// startLog is this team's start.json (own for a start timer, peer for a
	// finish timer); finishLog is the finish timer's own finish.json.
	startLog  *store.StartLog
	finishLog *store.FinishLog

	// teamLogs - the Regatta Director's read-only mirror of the primary team's
	// start.json + finish.json, keyed by team. The progress tree reads primary
	// values only; the secondary pair reconciles into the primary finish.json
	// via the PFT (reconciliation.md).
	teamLogs map[persona.Team]*teamTiming

	// directorSkew / directorStale - dismissible RD-tree banners: measured clock
	// offsets across the primary team's timing files disagree (persona-plan.md
	// 2.1), and no timing file has been written for a while (persona-plan.md 9).
	directorSkew  *dismissibleBanner
	directorStale *dismissibleBanner

	// origin-refresh (persona-plan.md 3b): a background poll notices the source
	// workbook changed; originBanner offers Apply/Dismiss for the parsed
	// candidate held in pendingOrigin; dismissedContentHash suppresses re-nagging
	// for a change the RD already declined.
	originBanner         *actionBanner
	pendingOrigin        *reader.RegattaData
	dismissedContentHash string

	// regattaKey - the schedule's RegattaKey captured when the session started,
	// used to reject watched peer data that belongs to another regatta.
	regattaKey string

	// rows - per-race widget handles for the timer race tree, so watcher
	// updates and button actions refresh a row in place instead of rebuilding.
	rows map[int]*raceRow

	// openClocks - finish-timer race clocks currently on screen, keyed by race
	// number, so a late-arriving peer start time can be pushed into the open
	// window for a reactive winning-time recompute (persona-plan.md 2.2).
	openClocks map[int]*clock.Clock

	// scheduleConflicts - races whose schedule row changed after this persona
	// already had timing for them (or an open clock). Drives the race-tree
	// banner and per-row marks until the operator dismisses them
	// (persona-plan.md 3c). Cleared, never a silent rewrite of timing files.
	scheduleConflicts   map[int]scheduleChange
	scheduleBanner      *fyne.Container
	scheduleBannerLabel *widget.Label

	// staleLaneLegend - caution strip explaining common.StaleLaneMapMark, shown
	// only when a visible row carries it (persona-plan.md 3c item 4). Same
	// dismissible banner as directorSkew et al., sitting above the column header.
	staleLaneLegend *dismissibleBanner

	// watchedHashes - last-applied content hash per watched file, seeded at
	// startup so the watcher's initial "current content" event for a file that
	// has not changed since hydrate is skipped rather than rebuilding the tree.
	watchedHashes map[string]string

	// writesBlocked - set when a persona's own timing file existed but failed to
	// parse. Phases 6-7 must refuse to write while this is true rather than
	// replacing a corrupt history with an empty one.
	writesBlocked bool

	// stopWatcher - tears the shared-file watcher (and the RD's stale / origin
	// tickers) down and blocks until they have exited, so it is safe to start a
	// new one. Called at window close and at the top of startWatcher.
	stopWatcher func()
}

type loadState struct {
	loadButton *widget.Button

	// excelLoaded - the setup view's Excel step is satisfied: a workbook has been
	// parsed into r.RegattaData and confirmed by the Regatta Director. It gates
	// the Start Regatta button alongside dirChosen and is reset whenever the
	// director deliberately returns to setup.
	excelLoaded bool

	// dirChosen - the setup view's save-folder step is satisfied: the director
	// has picked a regatta directory this session (welcomeFolderCallback). Reset
	// whenever the director returns to setup, so a PrefRegattaDir left over from
	// a previous regatta never pre-ticks Step 2 - the RD chooses where each
	// regatta's data goes rather than inheriting the last folder.
	dirChosen bool
}

func (r *Regatta) newLoadState() {
	button := widget.NewButton(common.LoadExcelButtonText, func() { r.loader(false) })
	button.Disable()

	r.loadState = &loadState{
		loadButton: button,
	}

}

// New - the single application entry point. It shows one persona picker listing
// every persona (Regatta Director + the four timers); the challenge code is what
// keeps a timing operator out of the loader. A deployment persona config
// (PrefPersonaConfigFile) can pin this host to a persona and skip the picker.
func New(app fyne.App) *Regatta {
	r := newRegatta(app)
	r.loadPersonaConfig()
	if def, ok := r.assignedPersona(); ok {
		r.startAssignedPersona(def)
		return r
	}
	r.showPersonaPicker()
	return r
}

// NewTimer - construct and go straight to the picker, as a timing operator
// would. Retained for tests that then drive startSession directly.
func NewTimer(app fyne.App) *Regatta {
	r := newRegatta(app)
	r.mode = modeTimer
	r.showPersonaPicker()
	return r
}

// NewDirector - construct as the Regatta Director and restore the regatta
// configured in PrefRegattaDir (else the welcome view) - the same path as the
// picker's "Resume as Regatta Director" shortcut. Retained as a constructor for
// the director test suite. The picker's deliberate "Regatta Director" choice
// goes through startDirectorSetup instead, which never auto-restores.
func NewDirector(app fyne.App) *Regatta {
	r := newRegatta(app)
	r.mode = modeDirector
	r.startDirectorFlow()
	return r
}

func newRegatta(app fyne.App) *Regatta {
	regattaApp := &Regatta{
		window:      app.NewWindow(common.AppTitle),
		App:         app,
		persona:     text.Header3(common.EmptyString),
		title:       text.Header3(common.EmptyString),
		subtitle:    text.Header3(common.EmptyString),
		date:        text.Header3(common.EmptyString),
		RegattaData: reader.NewRegattaData(),
	}
	// The four race-tree details fields sit in a left-aligned 2x2 grid.
	for _, t := range []*canvas.Text{regattaApp.persona, regattaApp.title, regattaApp.subtitle, regattaApp.date} {
		t.Alignment = fyne.TextAlignLeading
	}
	regattaApp.setTheme(regattaApp.App.Preferences().String(common.PrefTheme))
	regattaApp.window.SetMaster()
	regattaApp.window.SetMainMenu(regattaApp.makeMenu())
	regattaApp.window.Resize(fyne.NewSize(regattaWidth, regattaHeight))
	regattaApp.newLoadState()
	regattaApp.config = regattaApp.configContent()

	return regattaApp
}

// Run - start the main app
func (r *Regatta) Run() {
	r.window.ShowAndRun()
}

func (r *Regatta) refreshContent() {
	// The director's identity is implicit; bind it lazily so the role line and
	// title work through the same path the timer uses.
	if r.mode == modeDirector && r.session.Root == common.EmptyString {
		r.session, _ = r.directorSession()
	}

	r.title.Text = fmt.Sprintf(common.TreeRegattaLabel, r.RegattaData.Name)
	r.subtitle.Text = fmt.Sprintf(common.NumScheduledRacesTitle, r.RegattaData.ScheduledRaces())
	r.date.Text = fmt.Sprintf(common.TreeDateLabel, r.RegattaData.Date)

	if r.session.Label != common.EmptyString {
		r.persona.Text = fmt.Sprintf(common.PersonaHeaderFormat, r.session.Label)
		r.window.SetTitle(fmt.Sprintf(common.WindowTitleFormat, common.AppTitle, r.session.Label))
	}

	// On a restored session this runs before any view has been set.
	if content := r.window.Content(); content != nil {
		content.Refresh()
	}
}

// startDirectorSetup puts the Regatta Director on the welcome view - Set Regatta
// Directory / Load Excel File - without restoring the previously configured
// regatta. This is the deliberate "choose a regatta" path: picking "Regatta
// Director" on the startup picker, and the "Load Regatta Data" menu item. Only
// the picker's "Resume as Regatta Director" shortcut auto-restores the last
// regatta (startDirectorFlow). It is the safe way to switch regattas without
// restarting the app.
func (r *Regatta) startDirectorSetup() {
	r.mode = modeDirector
	r.window.SetMainMenu(r.makeMenu())

	// The deliberate "choose a regatta" path never carries a prior selection into
	// the setup view: the Excel step starts empty even if a workbook was loaded
	// earlier this session, and the save folder starts unchosen even if
	// PrefRegattaDir points at a previous regatta.
	r.loadState.excelLoaded = false
	r.loadState.dirChosen = false

	r.showDirectorSetup()
}

// startDirectorFlow runs the Regatta Director's startup: mark the mode, rebuild
// the menu so the loader items appear, bind the session, then either restore the
// saved schedule or fall back to the welcome view. Reached from the picker's
// "Resume as Regatta Director" shortcut, from a confirmed import, and from
// NewDirector.
func (r *Regatta) startDirectorFlow() {
	r.mode = modeDirector
	r.window.SetMainMenu(r.makeMenu())

	if session, ok := r.directorSession(); ok {
		r.session = session
	}

	if r.App.Preferences().String(common.PrefRegattaDir) == common.EmptyString {
		r.loadState.excelLoaded = false
		r.loadState.dirChosen = false
		r.showDirectorSetup()
		return
	}

	r.startLogging()

	if err := r.loadRegattaData(); err != nil {
		// A missing file is the normal first run for a configured directory: the
		// user has chosen where their data lives but has not imported a regatta.
		if !errors.Is(err, fs.ErrNotExist) {
			r.warnOnStarted(err)
		}
		// Both setup steps start unchosen: the director loads a workbook and
		// reaffirms where this regatta's data is saved rather than inheriting the
		// folder a previous regatta used.
		r.loadState.excelLoaded = false
		r.loadState.dirChosen = false
		r.showDirectorSetup()
		return
	}

	r.App.Preferences().SetString(common.PrefLastPersonaID, persona.DirectorDefinition.ID)
	applog.Info("regatta schedule restored", "component", "startup", "races", r.RegattaData.ScheduledRaces())

	// The RD reads every team's timing files (persona-plan.md 9). The schedule
	// name/date drive the same RegattaKey the timers stamp.
	r.regattaKey = store.RegattaKey(r.RegattaData.Name, r.RegattaData.Date)
	r.hydrateDirectorLogs(r.session.Root, r.regattaKey)

	r.refreshContent()
	r.showRaceTree()
	r.startWatcher(r.session)
}

// startLogging points applog at this persona's file,
// regattaData/logs/<team>/<role>-<hostname>.log, and stamps every line with the
// persona identity. A failure here is not fatal - timing and export continue
// without a log.
func (r *Regatta) startLogging() {
	session, ok := r.loggingSession()
	if !ok {
		return
	}

	host := hostName()
	applog.SetIdentity(session.ID, string(session.Team), string(session.Role), host)

	name := string(session.Role) + "-" + filesystem.SanitizeForFilename(host) + ".log"
	logPath := filepath.Join(session.Root, common.LogsDir, string(session.Team), name)
	if err := applog.SetOutput(logPath); err != nil {
		applog.Warn("log file unavailable", "component", "startup", "err", err)
	}
}

// loggingSession - the session startLogging writes under: the bound persona
// once the picker (or NewDirector) has chosen one, falling back to the director
// session derived from PrefRegattaDir.
func (r *Regatta) loggingSession() (persona.Session, bool) {
	if r.session.Root != common.EmptyString {
		return r.session, true
	}
	return r.directorSession()
}

// showDirectorSetup - the Regatta Director's setup screen, presented until a
// regatta has been imported. Both steps live on one screen and fill in with a
// check mark, the parsed regatta details and the chosen path as they are
// completed; Start Regatta stays disabled until the workbook is confirmed and a
// save folder is set. Rebuilt wholesale on every state change, so the gating is
// just a build-time check and cannot go stale. Called again by markExcelStepDone
// and welcomeFolderCallback after each selection.
func (r *Regatta) showDirectorSetup() {
	// The Excel step is always available now - it is step 1, no longer gated on
	// the save folder.
	r.loadState.loadButton.Enable()

	// Step 2 is "done" only once the director has picked a folder this session -
	// never just because PrefRegattaDir carries over from a previous regatta. The
	// pref string is still read, but only to show the path once dirChosen is set.
	dir := r.App.Preferences().String(common.PrefRegattaDir)
	dirSet := r.loadState.dirChosen

	// Step 1 - Excel workbook.
	step1 := []fyne.CanvasObject{}
	step1Text := common.SetupExcelStepText
	if r.loadState.excelLoaded {
		step1Text = common.SetupExcelDoneText
		step1 = append(step1,
			text.BoldLeading(r.RegattaData.Name),
			text.BoldLeading(r.RegattaData.Date),
			text.BoldLeading(fmt.Sprintf(common.NumScheduledRacesTitle, r.RegattaData.ScheduledRaces())),
			text.BoldLeading(common.SetupExcelFilePathLabel),
			pathEntry(r.RegattaData.URI),
		)
	}
	// The Load button stays on the view even once the step is done - it doubles
	// as "load a different workbook" - and keeps a button labelled
	// LoadExcelButtonText reachable for the tests that probe for the setup view.
	step1 = append(step1, container.NewHBox(r.loadState.loadButton))

	// Step 2 - save folder.
	step2 := []fyne.CanvasObject{}
	step2Text := common.SetupSaveDirStepText
	if dirSet {
		step2Text = common.SetupSaveDirDoneText
		step2 = append(step2,
			text.BoldLeading(common.SetupSaveDirPathLabel),
			pathEntry(dir),
		)
	}
	step2 = append(step2, container.NewHBox(r.welcomeDirButton(dirSet)))

	startButton := widget.NewButton(common.StartRegattaButtonText, func() { r.applyImportedRegatta() })
	if !(r.loadState.excelLoaded && dirSet) {
		startButton.Disable()
	}

	r.window.SetContent(container.NewVBox(
		container.New(
			layout.NewCustomPaddedLayout(viewMargin, 0, 0, 0),
			container.NewCenter(banner(welcomeBannerWidth, welcomeBannerHeight)),
		),
		setupStep(step1Text, container.NewVBox(step1...)),
		widget.NewSeparator(),
		setupStep(step2Text, container.NewVBox(step2...)),
		container.New(
			layout.NewCustomPaddedLayout(viewMargin, 0, viewMargin, 0),
			container.NewHBox(startButton),
		),
	))
}

// pathEntry - a read-only field showing a chosen filesystem path. Disabled so it
// reads as output, not input, while staying selectable for copy.
func pathEntry(path string) *widget.Entry {
	e := widget.NewEntry()
	e.SetText(path)
	e.Disable()
	return e
}

// banner - the branding wordmark at the caller's size. The SVG is a single-fill
// path wrapped in a themed resource, so it takes the current theme's foreground
// color (readable on both the light and dark themes). The size is explicit so it
// never depends on the source viewBox.
func banner(width, height float32) *canvas.Image {
	res := fyne.NewStaticResource(common.BannerResourceName, assets.RegattaClockBanner)
	logo := canvas.NewImageFromResource(theme.NewThemedResource(res))
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(width, height))

	return logo
}

// setupStep - instruction above the widget that carries it out, both inset from
// the window edge so they share one left margin. The body is any object (a
// button, or a button under the step's completed summary), not just a button.
func setupStep(instruction string, body fyne.CanvasObject) *fyne.Container {
	return container.NewVBox(
		container.New(
			layout.NewCustomPaddedLayout(viewMargin, 0, viewMargin, viewMargin),
			text.BoldLeading(instruction),
		),
		container.New(
			layout.NewCustomPaddedLayout(0, 0, viewMargin, 0),
			body,
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

// directorSession - the Regatta Director's persona.Session, derived from the
// saved PrefRegattaDir. Used to bind the session in startDirectorFlow and to
// offer the picker's "resume as director" shortcut. Returns false when no
// regatta directory has been configured yet.
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
