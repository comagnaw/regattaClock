package regatta

import (
	"fmt"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/personacfg"
	"github.com/comagnaw/regattaClock/internal/watcher"
)

func (r *Regatta) configContent() *fyne.Container {
	ntpEntry := widget.NewEntryWithData(binding.BindPreferenceString(common.PrefNTPServers, r.App.Preferences()))
	ntpEntry.SetPlaceHolder("host1, host2  (blank = public defaults)")

	return container.NewVBox(
		r.regattaDir(),
		r.personaConfigRow(),
		widget.NewForm(
			widget.NewFormItem("Debug:", widget.NewCheckWithData("", binding.BindPreferenceBool(common.PrefDebug, r.App.Preferences()))),
			widget.NewFormItem("Logging:", widget.NewCheckWithData("", binding.BindPreferenceBool(common.PrefLogging, r.App.Preferences()))),
			widget.NewFormItem("Storage:", r.storageModeRadio()),
			widget.NewFormItem("NTP servers:", ntpEntry),
			widget.NewFormItem("Theme:", r.themeButtons()),
		),
		container.NewCenter(
			widget.NewButton(common.CloseButtonText, func() {
				// The Logging / Debug checkboxes write straight to preferences;
				// re-apply them so a mid-session toggle takes effect without a
				// restart, and open the log file if Logging was just enabled.
				prefs := r.App.Preferences()
				applog.SetLevel(prefs.Bool(common.PrefLogging), prefs.Bool(common.PrefDebug))
				r.startLogging()

				r.config.Hide()
				r.window.SetContent(r.lastView)
			}),
		),
	)
}

// storageModeRadio - cloud vs SMB selector for how the app watches the shared
// regattaData tree and orders NTP servers. The value takes effect when the
// watcher and time sync are constructed at startup, so a mid-session change
// applies on the next launch.
func (r *Regatta) storageModeRadio() *widget.RadioGroup {
	prefs := r.App.Preferences()
	rg := widget.NewRadioGroup(
		[]string{common.StorageModeCloud, common.StorageModeSMB},
		func(selected string) {
			if selected != common.EmptyString {
				prefs.SetString(common.PrefStorageMode, selected)
			}
		},
	)
	rg.Horizontal = true
	rg.SetSelected(string(watcher.ParseMode(prefs.String(common.PrefStorageMode))))
	return rg
}

func (r *Regatta) regattaDir() *fyne.Container {
	return container.NewBorder(
		nil,
		nil,
		nil,
		widget.NewButton("Change", r.changeButtonFunc()),
		widget.NewForm(widget.NewFormItem("Regatta Dir:", widget.NewEntryWithData(binding.BindPreferenceString(common.PrefRegattaDir, r.App.Preferences())))),
	)
}

func (r *Regatta) changeButtonFunc() func() {
	return func() {
		dialog.ShowFolderOpen(r.changeCallBack(), r.window)
	}
}

// personaConfigRow - Configuration entry for the optional deployment persona
// config (PrefPersonaConfigFile). Mirrors regattaDir(): a bound editable path
// field with Change / Clear buttons. r.config is built once in newRegatta, so
// the buttons mutate preferences and r.personaCfg directly and never rebuild it.
func (r *Regatta) personaConfigRow() *fyne.Container {
	entry := widget.NewEntryWithData(binding.BindPreferenceString(common.PrefPersonaConfigFile, r.App.Preferences()))
	buttons := container.NewHBox(
		widget.NewButton(common.PersonaConfigChangeButtonText, r.personaConfigChangeFunc()),
		widget.NewButton(common.ClearButtonText, r.clearPersonaConfig),
	)
	return container.NewBorder(nil, nil, nil, buttons,
		widget.NewForm(widget.NewFormItem(common.PersonaConfigRowLabel, entry)),
	)
}

// clearPersonaConfig forgets the deployment persona config. Challenge overrides
// stop applying immediately; a host assignment was only consulted at launch.
func (r *Regatta) clearPersonaConfig() {
	r.App.Preferences().SetString(common.PrefPersonaConfigFile, common.EmptyString)
	r.personaCfg = nil
	applog.Info("persona config cleared", "component", "config")
}

func (r *Regatta) personaConfigChangeFunc() func() {
	return func() {
		fd := dialog.NewFileOpen(r.personaConfigCallback(), r.window)
		fd.SetFilter(storage.NewExtensionFileFilter(common.PersonaConfigExtensions))
		if regattaDir := r.App.Preferences().String(common.PrefRegattaDir); regattaDir != common.EmptyString {
			if location, err := storage.ListerForURI(storage.NewFileURI(regattaDir)); err == nil {
				fd.SetLocation(location)
			}
		}
		fd.Show()
	}
}

// personaConfigCallback validates the chosen file before it is remembered: the
// path is persisted (and r.personaCfg swapped in) only when personacfg.Load
// accepts it, so a broken file can never wedge the next launch. Mirrors
// loader.go's callback - nil reader is a cancel, the reader is closed, the URI
// path is brought back to native form.
func (r *Regatta) personaConfigCallback() func(fyne.URIReadCloser, error) {
	return func(rc fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, r.window)
			return
		}
		if rc == nil {
			return
		}
		defer rc.Close()
		path := filepath.FromSlash(rc.URI().Path())

		cfg, err := personacfg.Load(path)
		if err != nil {
			applog.Warn("persona config rejected", "component", "config", "path", path, "err", err)
			dialog.ShowError(fmt.Errorf("%s: %w", common.PersonaConfigInvalidTitle, err), r.window)
			return
		}
		r.App.Preferences().SetString(common.PrefPersonaConfigFile, path)
		r.personaCfg = cfg
		applog.Info("persona config loaded", "component", "config", "path", path,
			"hosts", len(cfg.Hosts), "challenges", len(cfg.Challenges))
		dialog.ShowInformation(common.PersonaConfigLoadedTitle,
			fmt.Sprintf(common.PersonaConfigLoadedFormat, len(cfg.Hosts), len(cfg.Challenges)),
			r.window)
	}
}

// welcomeDirButton - the save-folder chooser for the director setup view. Unlike
// regattaDirButton (which drives changeCallBack for the config screen and leaves
// the current view alone), this re-renders the setup screen after a pick so
// Step 2 fills in. The label reflects whether a folder is already set.
func (r *Regatta) welcomeDirButton(dirSet bool) *widget.Button {
	label := common.SetRegattaDirButtonText
	if dirSet {
		label = common.SetupChangeDirButtonText
	}
	return widget.NewButton(label, func() {
		dialog.ShowFolderOpen(r.welcomeFolderCallback(), r.window)
	})
}

// welcomeFolderCallback - as changeCallBack (persist PrefRegattaDir, enable the
// loader, point logging at the new tree), then re-render the director setup view
// so Step 2 shows its check mark and path and Start Regatta can ungate. Cancel
// is a no-op that leaves the step incomplete.
func (r *Regatta) welcomeFolderCallback() func(fyne.ListableURI, error) {
	return func(dirReader fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, r.window)
			return
		}
		if dirReader == nil {
			return
		}

		regattaDir := filepath.FromSlash(dirReader.Path())

		r.App.Preferences().SetString(common.PrefRegattaDir, regattaDir)
		r.loadState.loadButton.Enable()
		r.loadState.dirChosen = true

		r.startLogging()
		applog.Info("regatta directory set", "component", "setup", "path", regattaDir)

		r.showDirectorSetup()
	}
}

func (r *Regatta) changeCallBack() func(fyne.ListableURI, error) {
	return func(dirReader fyne.ListableURI, err error) {

		if err != nil {
			dialog.ShowError(err, r.window)
			return
		}

		// User cancelled the directory load
		if dirReader == nil {
			return
		}

		// Fyne reports URI paths with forward slashes, so restore the native form
		// before persisting a value the user reads and edits in the config form.
		regattaDir := filepath.FromSlash(dirReader.Path())

		r.App.Preferences().SetString(common.PrefRegattaDir, regattaDir)
		r.loadState.loadButton.Enable()

		r.startLogging()
		applog.Info("regatta directory set", "component", "config", "path", regattaDir)

		// director/ and timing/<team>/ are created on first write by the store,
		// so nothing needs pre-creating here now that results/ is gone.
	}

}
