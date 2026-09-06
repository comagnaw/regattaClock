package regatta

import (
	"fyne.io/fyne/v2"
	"github.com/comagnaw/regattaClock/internal/common"
)

// makeMenu - generate app menu. Excel import and lane-image export belong to the
// Regatta Director; a timer's menu carries neither, so the loader is
// unreachable from a timing window.
func (r *Regatta) makeMenu() *fyne.MainMenu {
	var items []*fyne.MenuItem

	if r.mode == modeDirector {
		items = append(items, r.importItem(), r.createLaneImages())
	}
	items = append(items,
		r.showWindowItem(),
		fyne.NewMenuItemSeparator(),
		r.configItem(),
		r.exitItem(),
	)

	return fyne.NewMainMenu(fyne.NewMenu(common.AppTitle, items...))
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
