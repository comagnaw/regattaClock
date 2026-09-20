package regatta

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
)

func TestStorageModeRadioDefaultsToCloud(t *testing.T) {
	app := test.NewTempApp(t)
	r := NewDirector(app)

	rg := r.storageModeRadio()
	if !slices.Equal(rg.Options, []string{common.StorageModeCloud, common.StorageModeSMB}) {
		t.Fatalf("options = %v", rg.Options)
	}
	if rg.Selected != common.StorageModeCloud {
		t.Errorf("selected = %q, want %q with no preference set", rg.Selected, common.StorageModeCloud)
	}
}

func TestStorageModeRadioReflectsPreference(t *testing.T) {
	app := test.NewTempApp(t)
	app.Preferences().SetString(common.PrefStorageMode, common.StorageModeSMB)
	r := NewDirector(app)

	if got := r.storageModeRadio().Selected; got != common.StorageModeSMB {
		t.Errorf("selected = %q, want %q", got, common.StorageModeSMB)
	}
}

func TestStorageModeRadioWritesPreference(t *testing.T) {
	app := test.NewTempApp(t)
	r := NewDirector(app)

	r.storageModeRadio().OnChanged(common.StorageModeSMB)

	if got := app.Preferences().String(common.PrefStorageMode); got != common.StorageModeSMB {
		t.Errorf("preference = %q, want %q after selecting smb", got, common.StorageModeSMB)
	}
}

func TestStorageModeRadioNormalisesUnknownPreference(t *testing.T) {
	app := test.NewTempApp(t)
	app.Preferences().SetString(common.PrefStorageMode, "onedrive")
	r := NewDirector(app)

	if got := r.storageModeRadio().Selected; got != common.StorageModeCloud {
		t.Errorf("selected = %q, want cloud for an unrecognised stored value", got)
	}
}

func TestNormalizeRegattaDir(t *testing.T) {
	parent := t.TempDir()
	dataDir := filepath.Join(parent, common.RegattaDataDir)

	if got := normalizeRegattaDir(dataDir); got != parent {
		t.Errorf("picking regattaData itself: got %q, want its parent %q", got, parent)
	}
	if got := normalizeRegattaDir(parent); got != parent {
		t.Errorf("picking the parent unchanged: got %q, want %q", got, parent)
	}
}

// TestWelcomeFolderCallback_RejectsNestedRegattaData verifies the fix for
// picking .../regattaData itself in the Director Setup folder browser:
// PrefRegattaDir is always joined with RegattaDataDir later, so persisting it
// unchanged would nest a second regattaData folder inside the first the next
// time a schedule is saved.
func TestWelcomeFolderCallback_RejectsNestedRegattaData(t *testing.T) {
	app := test.NewTempApp(t)
	parent := t.TempDir()
	dataDir := filepath.Join(parent, common.RegattaDataDir)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}

	r := New(app)
	r.onPersonaChosen(persona.DirectorDefinition, "rc-rd")
	r.welcomeFolderCallback()(listerFor(t, dataDir), nil)

	if got := app.Preferences().String(common.PrefRegattaDir); got != parent {
		t.Errorf("PrefRegattaDir = %q, want the parent %q (not the picked regattaData folder itself)", got, parent)
	}
}

// TestChangeCallBack_RejectsNestedRegattaData - as above, for the
// Configuration screen's "Change" button.
func TestChangeCallBack_RejectsNestedRegattaData(t *testing.T) {
	app := test.NewTempApp(t)
	parent := t.TempDir()
	dataDir := filepath.Join(parent, common.RegattaDataDir)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}

	r := New(app)
	r.onPersonaChosen(persona.DirectorDefinition, "rc-rd")
	r.changeCallBack()(listerFor(t, dataDir), nil)

	if got := app.Preferences().String(common.PrefRegattaDir); got != parent {
		t.Errorf("PrefRegattaDir = %q, want the parent %q (not the picked regattaData folder itself)", got, parent)
	}
}
