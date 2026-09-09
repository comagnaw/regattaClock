package regatta

import (
	"path/filepath"
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
)

func findButtonByLabel(o fyne.CanvasObject, label string) *widget.Button {
	switch v := o.(type) {
	case *widget.Button:
		if v.Text == label {
			return v
		}
	case *container.AppTabs:
		for _, item := range v.Items {
			if b := findButtonByLabel(item.Content, label); b != nil {
				return b
			}
		}
	case *fyne.Container:
		for _, c := range v.Objects {
			if b := findButtonByLabel(c, label); b != nil {
				return b
			}
		}
	}
	return nil
}

func TestNew_ShowsUnifiedPicker(t *testing.T) {
	app := test.NewTempApp(t)
	r := New(app)

	tabs := findAppTabs(r.window.Content())
	if tabs == nil {
		t.Fatal("startup has no persona tabs")
	}
	labels := buttonLabels(r.window.Content())
	if !slices.Contains(labels, persona.DirectorDefinition.Label) {
		t.Errorf("picker %v is missing the Regatta Director", labels)
	}
	if r.mode != modeUnset || r.session.Root != "" {
		t.Error("no mode or session should be bound before a persona is chosen")
	}
	if slices.Contains(menuLabels(r.window.MainMenu()), common.LoadDataTitle) {
		t.Error("the loader menu item must not appear before the director persona is chosen")
	}
}

func TestOnPersonaChosen_DirectorEntersDirectorFlow(t *testing.T) {
	app := test.NewTempApp(t)
	r := New(app)

	r.onPersonaChosen(persona.DirectorDefinition, "rc-rd")

	if r.mode != modeDirector {
		t.Fatalf("mode = %v, want modeDirector", r.mode)
	}
	if !onWelcome(r) {
		t.Error("a director with no configured directory should land on the welcome view")
	}
	if !slices.Contains(menuLabels(r.window.MainMenu()), common.LoadDataTitle) {
		t.Error("the director menu should carry the loader once chosen")
	}
}

func TestOnPersonaChosen_DirectorWrongChallengeStays(t *testing.T) {
	app := test.NewTempApp(t)
	r := New(app)

	r.onPersonaChosen(persona.DirectorDefinition, "nope")

	if r.mode != modeUnset {
		t.Error("a rejected challenge must not enter any flow")
	}
	if findAppTabs(r.window.Content()) == nil {
		t.Error("the picker should still be on screen after a rejected challenge")
	}
}

func TestOnPersonaChosen_DirectorAlwaysStartsAtSetup(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	app.Preferences().SetString(common.PrefRegattaDir, filepath.Dir(root))

	r := New(app)
	r.onPersonaChosen(persona.DirectorDefinition, "rc-rd")

	// Deliberately choosing "Regatta Director" must open the Set Directory /
	// Load Excel view even when a regatta is already configured - only the
	// picker's separate Resume shortcut reopens the previous regatta.
	if !onWelcome(r) {
		t.Error("director pick should land on the setup view, not auto-restore")
	}
	if r.mode != modeDirector {
		t.Errorf("mode = %v, want modeDirector", r.mode)
	}
	if !slices.Contains(menuLabels(r.window.MainMenu()), common.LoadDataTitle) {
		t.Error("the director menu should carry the loader once chosen")
	}
	if r.RegattaData.Name == sch.Name {
		t.Error("the previous regatta must not be loaded on the setup path")
	}
	if got := app.Preferences().String(common.PrefLastPersonaID); got == persona.DirectorDefinition.ID {
		t.Error("PrefLastPersonaID should not be set until the director actually enters a regatta")
	}
}

func TestLoadRegattaDataMenuReturnsToSetup(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	app.Preferences().SetString(common.PrefRegattaDir, filepath.Dir(root))
	app.Preferences().SetString(common.PrefLastPersonaID, persona.DirectorDefinition.ID)

	r := NewDirector(app) // restored into the seeded regatta's tree
	stopWatch(t, r)
	if onWelcome(r) {
		t.Fatal("precondition: NewDirector should have restored the seeded regatta")
	}

	r.importItem().Action()

	if !onWelcome(r) {
		t.Error("Load Regatta Data should return the director to the Set Directory / Load Excel view")
	}
}

func TestDirectorImportWritesScheduleOnConfirm(t *testing.T) {
	app := test.NewTempApp(t)
	regattaDir := t.TempDir()

	r := New(app)
	r.onPersonaChosen(persona.DirectorDefinition, "rc-rd")
	r.changeCallBack()(listerFor(t, regattaDir), nil)

	xlsx, err := filepath.Abs(filepath.Join("..", "..", "examples", "Example Regatta Input Table.xlsx"))
	if err != nil {
		t.Fatal(err)
	}
	fileReader, err := storage.Reader(storage.NewFileURI(xlsx))
	if err != nil {
		t.Fatal(err)
	}
	r.callback(false)(fileReader, nil)
	if len(r.RegattaData.Races) == 0 {
		t.Fatal("workbook did not parse")
	}

	dirSession := persona.Session{
		Definition: persona.DirectorDefinition,
		Root:       filepath.Join(regattaDir, common.RegattaDataDir),
	}

	// The parse alone writes nothing; the schedule waits on Start Regatta.
	if _, err := store.LoadSchedule(dirSession); err == nil {
		t.Fatal("schedule should not exist before the director confirms")
	}

	// Start Regatta (here applyImportedRegatta directly) writes it and enters the tree.
	r.applyImportedRegatta()
	if _, err := store.LoadSchedule(dirSession); err != nil {
		t.Fatalf("schedule should exist after confirm: %v", err)
	}
	if onWelcome(r) {
		t.Error("the director should be on the tree after a confirmed import")
	}
}

// TestSetupStartButtonRunsImport - the setup view's Start Regatta button is
// disabled until both steps are done, and tapping it writes the schedule and
// enters the tree.
func TestSetupStartButtonRunsImport(t *testing.T) {
	app := test.NewTempApp(t)
	regattaDir := t.TempDir()

	r := New(app)
	r.onPersonaChosen(persona.DirectorDefinition, "rc-rd")
	r.welcomeFolderCallback()(listerFor(t, regattaDir), nil) // step 2 done

	start := findButtonByLabel(r.window.Content(), common.StartRegattaButtonText)
	if start == nil || !start.Disabled() {
		t.Fatal("Start Regatta must be disabled with only the save folder set")
	}

	xlsx, err := filepath.Abs(filepath.Join("..", "..", "examples", "Example Regatta Input Table.xlsx"))
	if err != nil {
		t.Fatal(err)
	}
	fileReader, err := storage.Reader(storage.NewFileURI(xlsx))
	if err != nil {
		t.Fatal(err)
	}
	r.callback(false)(fileReader, nil) // parse
	r.markExcelStepDone()              // stands in for confirming the dialog

	start = findButtonByLabel(r.window.Content(), common.StartRegattaButtonText)
	if start == nil || start.Disabled() {
		t.Fatal("Start Regatta must ungate once both steps are done")
	}
	start.OnTapped()
	stopWatch(t, r)

	if onWelcome(r) {
		t.Error("Start Regatta should enter the director tree")
	}
	dirSession := persona.Session{
		Definition: persona.DirectorDefinition,
		Root:       filepath.Join(regattaDir, common.RegattaDataDir),
	}
	if _, err := store.LoadSchedule(dirSession); err != nil {
		t.Fatalf("schedule should be written after Start Regatta: %v", err)
	}
}

func TestResumeDirectorButton(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	app.Preferences().SetString(common.PrefRegattaDir, filepath.Dir(root))

	// No resume when the last persona was a timer.
	app.Preferences().SetString(common.PrefLastPersonaID, "pst")
	if New(app).resumeDirectorButton() != nil {
		t.Error("resume button should not appear when the last persona was a timer")
	}

	// Resume when the last persona was the director and the schedule is readable.
	app.Preferences().SetString(common.PrefLastPersonaID, persona.DirectorDefinition.ID)
	r := New(app)
	label := "Resume as Regatta Director — " + sch.Name
	btn := findButtonByLabel(r.window.Content(), label)
	if btn == nil {
		t.Fatalf("resume button %q not on the picker; buttons=%v", label, buttonLabels(r.window.Content()))
	}
	btn.OnTapped()
	if r.mode != modeDirector || onWelcome(r) {
		t.Error("tapping resume should enter the director tree")
	}
}

func TestResumeDirectorButtonAbsentWithoutSchedule(t *testing.T) {
	app := test.NewTempApp(t)
	app.Preferences().SetString(common.PrefRegattaDir, t.TempDir()) // no schedule seeded
	app.Preferences().SetString(common.PrefLastPersonaID, persona.DirectorDefinition.ID)

	if New(app).resumeDirectorButton() != nil {
		t.Error("resume button should not appear when no schedule can be read")
	}
}
