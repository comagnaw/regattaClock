package regatta

import (
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
)

// buttonLabels - collect the button text of the view currently on screen
func buttonLabels(o fyne.CanvasObject) []string {
	labels := []string{}
	switch t := o.(type) {
	case *widget.Button:
		labels = append(labels, t.Text)
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

// TestStartup_NoPreferences - a first run offers the welcome view with importing
// held back until a regatta directory has been chosen.
func TestStartup_NoPreferences(t *testing.T) {
	app := test.NewTempApp(t)

	r := NewDirector(app)

	if !onWelcome(r) {
		t.Fatalf("expected the welcome view, got buttons %v", buttonLabels(r.window.Content()))
	}

	if !r.loadState.loadButton.Disabled() {
		t.Error("import should stay disabled until a regatta directory is set")
	}
}

// TestStartup_DirectorySetWithoutHistory - a configured directory holding no
// saved regatta is a normal first run, not an error, so the welcome view stays
// up with importing enabled.
func TestStartup_DirectorySetWithoutHistory(t *testing.T) {
	app := test.NewTempApp(t)
	app.Preferences().SetString(common.PrefRegattaDir, t.TempDir())

	r := NewDirector(app)

	if !onWelcome(r) {
		t.Fatalf("expected the welcome view, got buttons %v", buttonLabels(r.window.Content()))
	}

	if r.loadState.loadButton.Disabled() {
		t.Error("import should be enabled once a regatta directory is set")
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

// TestStartup_CancelledDirectoryDialog - cancelling leaves the preference alone
// instead of reporting an empty path as an error.
func TestStartup_CancelledDirectoryDialog(t *testing.T) {
	app := test.NewTempApp(t)

	r := NewDirector(app)
	r.changeCallBack()(nil, nil)

	if got := app.Preferences().String(common.PrefRegattaDir); got != common.EmptyString {
		t.Errorf("expected the regatta directory to stay unset, got %q", got)
	}

	if !r.loadState.loadButton.Disabled() {
		t.Error("import should stay disabled after a cancelled dialog")
	}
}
