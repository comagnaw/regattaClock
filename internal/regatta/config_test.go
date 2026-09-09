package regatta

import (
	"slices"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/comagnaw/regattaClock/internal/common"
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
