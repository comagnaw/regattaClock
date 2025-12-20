package regatta

import (
	"fyne.io/fyne/v2"
	"github.com/comagnaw/regattaClock/internal/common"
)

// makeMenu - generate app menu
func (r *Regatta) makeMenu() *fyne.MainMenu {

	return fyne.NewMainMenu(
		fyne.NewMenu(
			common.AppTitle,
			r.importItem(),
			r.createLaneImages(),
			r.showWindowItem(),
			fyne.NewMenuItemSeparator(),
			r.configItem(),
			r.exitItem(),
		),
	)

}

// configItem - menu item to modify user config
func (r *Regatta) configItem() *fyne.MenuItem {
	return fyne.NewMenuItem(common.ConfigTitle, func() {
		r.lastView = r.window.Content()
		r.window.SetContent(r.config)
		r.config.Show()
	})
}

// importItem - menu item to load RegattaData
func (r *Regatta) importItem() *fyne.MenuItem {
	return fyne.NewMenuItem(common.LoadDataTitle, func() {
		r.loader(false)
	})
}

// createLaneImages - menu item to load RegattaData
func (r *Regatta) createLaneImages() *fyne.MenuItem {
	return fyne.NewMenuItem(common.CreateLaneImagesTitle, func() {
		r.exporter()
	})
}

// showWindowItem - menu to bring main app back into focus
func (r *Regatta) showWindowItem() *fyne.MenuItem {
	return fyne.NewMenuItem(common.ShowWindowText, func() {
		r.window.Show()
	})
}

// exitItem - menu to exit the main app
func (r *Regatta) exitItem() *fyne.MenuItem {
	return fyne.NewMenuItem(common.ExitButtonText, func() {
		r.App.Quit()
	})
}
