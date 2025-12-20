package regatta

import (
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

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

		// User did not cancel directory load
		if dirReader != nil {

			r.App.Preferences().SetString(
				common.PrefRegattaDir,
				dirReader.Path(),
			)

			r.loadState.loadButton.Enable()
		}

		blah := filepath.Join(r.App.Preferences().String(common.PrefRegattaDir), "regattaData", "results")
		if err = filesystem.CreateDirs(blah); err != nil {
			dialog.ShowError(err, r.window)
			return
		}

	}

}
