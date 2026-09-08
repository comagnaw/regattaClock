package regatta

import (
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
)

// buttonLabels - collect the button text of the view currently on screen,
// descending into AppTabs (every tab's content, not just the selected one).
func buttonLabels(o fyne.CanvasObject) []string {
	labels := []string{}
	switch t := o.(type) {
	case *widget.Button:
		labels = append(labels, t.Text)
	case *container.AppTabs:
		for _, item := range t.Items {
			labels = append(labels, buttonLabels(item.Content)...)
		}
	case *fyne.Container:
		for _, c := range t.Objects {
			labels = append(labels, buttonLabels(c)...)
		}
	}
	return labels
}

func onWelcome(r *Regatta) bool {
	for _, label := range buttonLabels(r.window.Content()) {
		if label == common.LoadExcelButtonText {
			return true
		}
	}
	return false
}

func listerFor(t *testing.T, dir string) fyne.ListableURI {
	t.Helper()

	lister, err := storage.ListerForURI(storage.NewFileURI(dir))
	if err != nil {
		t.Fatalf("could not list %s: %v", dir, err)
	}
	return lister
}

// TestStartup_NoPreferences - a first run offers the setup view. Loading the
// workbook is step 1 and available immediately; Start Regatta is what stays
// disabled until both steps are done.
func TestStartup_NoPreferences(t *testing.T) {
	app := test.NewTempApp(t)

	r := NewDirector(app)

	if !onWelcome(r) {
		t.Fatalf("expected the setup view, got buttons %v", buttonLabels(r.window.Content()))
	}

	if r.loadState.loadButton.Disabled() {
		t.Error("loading the workbook is step 1 and must be available on a first run")
	}

	start := findButtonByLabel(r.window.Content(), common.StartRegattaButtonText)
	if start == nil {
		t.Fatal("the setup view should carry a Start Regatta button")
	}
	if !start.Disabled() {
		t.Error("Start Regatta must stay disabled until the workbook and save folder are both set")
	}
}

// TestStartup_DirectorySetWithoutHistory - a configured directory holding no
// saved regatta is a normal first run, not an error, so the setup view stays up.
// Step 2 is NOT pre-checked from the leftover preference - the director still
// picks where this regatta's data goes - and Start Regatta waits on both steps.
func TestStartup_DirectorySetWithoutHistory(t *testing.T) {
	app := test.NewTempApp(t)
	app.Preferences().SetString(common.PrefRegattaDir, t.TempDir())

	r := NewDirector(app)

	if !onWelcome(r) {
		t.Fatalf("expected the setup view, got buttons %v", buttonLabels(r.window.Content()))
	}

	if r.loadState.loadButton.Disabled() {
		t.Error("import should be enabled once a regatta directory is set")
	}

	if r.loadState.dirChosen {
		t.Error("the save-folder step must not be pre-satisfied by a leftover preference")
	}
	if findButtonByLabel(r.window.Content(), common.SetRegattaDirButtonText) == nil {
		t.Errorf("expected the unset 'Set Regatta Directory' button; buttons %v", buttonLabels(r.window.Content()))
	}
	if findButtonByLabel(r.window.Content(), common.SetupChangeDirButtonText) != nil {
		t.Error("the 'Change…' button (step 2 done) must not show before a folder is picked")
	}

	start := findButtonByLabel(r.window.Content(), common.StartRegattaButtonText)
	if start == nil || !start.Disabled() {
		t.Error("Start Regatta must stay disabled until both steps are done")
	}
}

// TestStartup_DirectorSetupDoesNotPrecheckSaveDir - deliberately choosing
// "Regatta Director" while PrefRegattaDir still points at a previous regatta
// opens the setup view with the save folder unchosen; picking one this session
// is what completes step 2.
func TestStartup_DirectorSetupDoesNotPrecheckSaveDir(t *testing.T) {
	app := test.NewTempApp(t)
	app.Preferences().SetString(common.PrefRegattaDir, t.TempDir())

	r := New(app)
	r.onPersonaChosen(persona.DirectorDefinition, "rc-rd")

	if !onWelcome(r) {
		t.Fatalf("expected the setup view, got buttons %v", buttonLabels(r.window.Content()))
	}
	if r.loadState.dirChosen {
		t.Fatal("step 2 must start unchosen despite the configured directory")
	}
	if findButtonByLabel(r.window.Content(), common.SetupChangeDirButtonText) != nil {
		t.Error("the 'Change…' button must not show before a folder is picked this session")
	}
	if start := findButtonByLabel(r.window.Content(), common.StartRegattaButtonText); start == nil || !start.Disabled() {
		t.Error("Start Regatta must be disabled while step 2 is unchosen")
	}

	r.welcomeFolderCallback()(listerFor(t, t.TempDir()), nil)

	if !r.loadState.dirChosen {
		t.Error("picking a folder should complete step 2")
	}
	if findButtonByLabel(r.window.Content(), common.SetupChangeDirButtonText) == nil {
		t.Error("step 2 should now show the 'Change…' button")
	}
	if start := findButtonByLabel(r.window.Content(), common.StartRegattaButtonText); start == nil || !start.Disabled() {
		t.Error("Start Regatta stays disabled until the workbook step is also done")
	}
}

// TestStartup_RestoresHistory - the point of the user config: a regatta imported
// in one session is on screen again at the start of the next.
func TestStartup_RestoresHistory(t *testing.T) {
	app := test.NewTempApp(t)
	regattaDir := t.TempDir()

	first := NewDirector(app)
	first.changeCallBack()(listerFor(t, regattaDir), nil)

	if first.loadState.loadButton.Disabled() {
		t.Fatal("import should be enabled after choosing a directory")
	}

	xlsx, err := filepath.Abs(filepath.Join("..", "..", "examples", "Example Regatta Input Table.xlsx"))
	if err != nil {
		t.Fatalf("could not resolve the example workbook: %v", err)
	}

	fileReader, err := storage.Reader(storage.NewFileURI(xlsx))
	if err != nil {
		t.Fatalf("could not read the example workbook: %v", err)
	}
	first.callback(false)(fileReader, nil)

	imported := len(first.RegattaData.Races)
	if imported == 0 {
		t.Fatal("expected races to be imported from the example workbook")
	}

	// callback now parks the parsed workbook behind a confirm dialog and the
	// setup stepper; calling applyImportedRegatta directly stands in for pressing
	// Start Regatta once both steps are done.
	first.applyImportedRegatta()

	if onWelcome(first) {
		t.Error("a successful import should leave the welcome view")
	}

	saved := persona.Session{
		Definition: persona.DirectorDefinition,
		Root:       filepath.Join(regattaDir, common.RegattaDataDir),
	}.SchedulePath()
	if _, err = storage.Exists(storage.NewFileURI(saved)); err != nil {
		t.Fatalf("expected schedule at %s: %v", saved, err)
	}

	// Restarting reads the history back rather than returning to the welcome view.
	second := NewDirector(app)

	if len(second.RegattaData.Races) != imported {
		t.Errorf("expected %d races restored, got %d", imported, len(second.RegattaData.Races))
	}

	if onWelcome(second) {
		t.Error("a restored regatta should not land on the welcome view")
	}
}

// TestStartup_CancelledDirectoryDialog - cancelling the config screen's folder
// dialog leaves the preference alone instead of reporting an empty path as an
// error.
func TestStartup_CancelledDirectoryDialog(t *testing.T) {
	app := test.NewTempApp(t)

	r := NewDirector(app)
	r.changeCallBack()(nil, nil)

	if got := app.Preferences().String(common.PrefRegattaDir); got != common.EmptyString {
		t.Errorf("expected the regatta directory to stay unset, got %q", got)
	}

	start := findButtonByLabel(r.window.Content(), common.StartRegattaButtonText)
	if start == nil || !start.Disabled() {
		t.Error("Start Regatta must stay disabled after a cancelled dialog")
	}
}

// TestStartup_CancelledWelcomeFolderDialog - cancelling the setup view's own
// folder dialog is a no-op: the preference stays unset and the view stays up.
func TestStartup_CancelledWelcomeFolderDialog(t *testing.T) {
	app := test.NewTempApp(t)

	r := NewDirector(app)
	r.welcomeFolderCallback()(nil, nil)

	if got := app.Preferences().String(common.PrefRegattaDir); got != common.EmptyString {
		t.Errorf("cancel must leave the regatta directory unset, got %q", got)
	}
	if !onWelcome(r) {
		t.Error("a cancelled folder dialog should leave the setup view up")
	}
	start := findButtonByLabel(r.window.Content(), common.StartRegattaButtonText)
	if start == nil || !start.Disabled() {
		t.Error("Start Regatta must stay disabled after a cancelled folder dialog")
	}
}

// TestStartup_WelcomeFolderCallbackCompletesStepTwo - choosing a save folder on
// the setup view persists it and fills in step 2, but Start Regatta still waits
// on the workbook step.
func TestStartup_WelcomeFolderCallbackCompletesStepTwo(t *testing.T) {
	app := test.NewTempApp(t)

	r := NewDirector(app)
	r.welcomeFolderCallback()(listerFor(t, t.TempDir()), nil)

	if app.Preferences().String(common.PrefRegattaDir) == common.EmptyString {
		t.Fatal("choosing a folder should persist the regatta directory")
	}
	if !onWelcome(r) {
		t.Error("setting the save folder alone should not leave the setup view")
	}
	start := findButtonByLabel(r.window.Content(), common.StartRegattaButtonText)
	if start == nil || !start.Disabled() {
		t.Error("Start Regatta needs the workbook step too, so it must stay disabled")
	}
}
