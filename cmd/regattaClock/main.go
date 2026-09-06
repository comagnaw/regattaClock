package main

import (
	"fyne.io/fyne/v2/app"
	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/regatta"
)

func main() {
	fyneApp := app.NewWithID(common.AppBundleID)

	prefs := fyneApp.Preferences()
	applog.Init(prefs.Bool(common.PrefLogging), prefs.Bool(common.PrefDebug))
	defer applog.Close()

	regattaApp := regatta.NewRegatta(fyneApp)
	regattaApp.Run()
}
