package regatta

import (
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/filesystem"
)

func (r *Regatta) configContainer() fyne.Window {
	configWindow := r.App.NewWindow(common.ConfigTitle)
	configWindow.SetContent(r.configContent())
	configWindow.Resize(fyne.NewSize(regattaWidth, 100))
	configWindow.SetCloseIntercept(func() { configWindow.Hide() })
	return configWindow
}

func (r *Regatta) configContent() *fyne.Container {
	regattaDir := widget.NewLabelWithData(binding.BindPreferenceString(common.PrefRegattaDir, r.App.Preferences()))
	
	return container.NewVBox(
		container.NewVBox(
			widget.NewLabel("Regatta Directory:"),
			regattaDir,
			widget.NewButton("Change", r.changeButtonFunc()),
		),
		widget.NewCheckWithData("Deubg", binding.BindPreferenceBool(common.PrefDebug, r.App.Preferences())),
		widget.NewCheckWithData("Logging", binding.BindPreferenceBool(common.PrefLogging, r.App.Preferences())),
		container.NewHBox(
			layout.NewSpacer(),
			widget.NewButton(common.CloseButtonText, func() { r.config.Hide() }),
			layout.NewSpacer(),
		),
	)
}

func (r *Regatta) changeButtonFunc() func() {
	return func() {
		dialog.ShowFolderOpen(r.changeCallBack(), r.config)
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
		}

		blah := filepath.Join(r.App.Preferences().String(common.PrefRegattaDir), "regattaData")
		if err = filesystem.CreateDirs(blah); err != nil {
			dialog.ShowError(err, r.window)
			return
		}

	}

}
