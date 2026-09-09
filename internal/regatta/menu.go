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
		items = append(items, r.importItem(), r.reloadScheduleItem(), r.createLaneImages())
	}
	items = append(items,
		r.showWindowItem(),
		fyne.NewMenuItemSeparator(),
		r.switchPersonaItem(),
		r.configItem(),
		r.versionItem(),
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

// importItem - "Load Regatta Data" menu item. It returns to the Set Regatta
// Directory / Load Excel File view, the safe way for the director to switch to
// another regatta without restarting the app. Re-reading the *current*
// workbook is the separate Reload Schedule item.
func (r *Regatta) importItem() *fyne.MenuItem {
	return fyne.NewMenuItem(common.LoadDataTitle, func() {
		r.startDirectorSetup()
	})
}

// reloadScheduleItem - re-read the current regatta's workbook and run it back
// through the confirm + RegattaKey guard.
func (r *Regatta) reloadScheduleItem() *fyne.MenuItem {
	return fyne.NewMenuItem(common.ReloadScheduleTitle, func() {
		r.reloadSchedule()
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

// versionItem - opens the build-info window (see version_window.go).
func (r *Regatta) versionItem() *fyne.MenuItem {
	return fyne.NewMenuItem(common.VersionTitle, r.showVersionWindow)
}

// exitItem - menu to exit the main app
func (r *Regatta) exitItem() *fyne.MenuItem {
	return fyne.NewMenuItem(common.ExitButtonText, func() {
		r.App.Quit()
	})
}
