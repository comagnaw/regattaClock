package main

import (
	"context"

	"fyne.io/fyne/v2/app"
	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/regatta"
	"github.com/comagnaw/regattaClock/internal/timesync"
)

func main() {
	fyneApp := app.NewWithID(common.AppBundleID)

	prefs := fyneApp.Preferences()
	applog.Init(prefs.Bool(common.PrefLogging), prefs.Bool(common.PrefDebug))
	defer applog.Close()

	// Measure this machine's clock offset in the background so the finish
	// timer's cross-machine winning-time math can be corrected. PrefNTPServers
	// overrides the public default list (blank = defaults); under smb mode the
	// operator puts the LAN NTP host first. Read once at startup.
	timesync.Start(context.Background(), timesync.Config{
		Servers: timesync.ParseServers(prefs.String(common.PrefNTPServers)),
	})
	defer timesync.Stop()

	regattaApp := regatta.NewRegatta(fyneApp)
	regattaApp.Run()
}
