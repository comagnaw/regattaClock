package regatta

import (
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/filesystem"
)

func (r *Regatta) configContent() *fyne.Container {
	return container.NewVBox(
		r.regattaDir(),
		widget.NewForm(
			widget.NewFormItem("Debug:", widget.NewCheckWithData("", binding.BindPreferenceBool(common.PrefDebug, r.App.Preferences()))),
			widget.NewFormItem("Logging:", widget.NewCheckWithData("", binding.BindPreferenceBool(common.PrefLogging, r.App.Preferences()))),
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

func (r *Regatta) regattaDir() *fyne.Container {
	return container.NewBorder(
		nil,
		nil,
		nil,
		widget.NewButton("Change", r.changeButtonFunc()),
		widget.NewForm(widget.NewFormItem("Regatta Dir:", widget.NewEntryWithData(binding.BindPreferenceString(common.PrefRegattaDir, r.App.Preferences())))),
	)
}

// regattaDirButton - directory chooser for views that want the action alone,
// without the editable path the config form exposes.
func (r *Regatta) regattaDirButton() *widget.Button {
	return widget.NewButton(common.SetRegattaDirButtonText, r.changeButtonFunc())
}

func (r *Regatta) changeButtonFunc() func() {
	return func() {
		dialog.ShowFolderOpen(r.changeCallBack(), r.window)
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

		// Someone reading from a shared regatta directory may not be allowed to
		// create the results tree. They can still load and time races, so warn
		// rather than fail.
		resultsDir := filepath.Join(regattaDir, common.RegattaDataDir, common.ResultsDir)
		if err = filesystem.CreateDirs(resultsDir); err != nil {
			r.warnSaveSkipped(err)
		}
	}

}
