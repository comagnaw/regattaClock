package regattaClock

import (

	"fyne.io/fyne/v2"

)

func (a *App) makeMenu() *fyne.MainMenu {

	return fyne.NewMainMenu(fyne.NewMenu("Regatta Clock",
		a.importItem(),
		a.showWindowItem(),
		fyne.NewMenuItemSeparator(),
		a.exitItem(),
	))

}

func (a *App) importItem() *fyne.MenuItem {
	return fyne.NewMenuItem("Import Regatta Table", func() {
		a.loadExcel(false)
	})
}

func (a *App) showWindowItem() *fyne.MenuItem {
	return fyne.NewMenuItem("Show Window", func() {
		a.window.Show()
	})
}

func (a *App) exitItem() *fyne.MenuItem {
	return fyne.NewMenuItem("Exit", func() {
		a.app.Quit()
	})
}