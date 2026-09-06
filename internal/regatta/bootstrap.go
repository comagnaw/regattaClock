package regatta

import (
	"context"

	"fyne.io/fyne/v2"

	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/timesync"
)

// Bootstrap wires the process-wide services both binaries need - structured
// logging (internal/applog) and the background NTP clock-offset syncer
// (internal/timesync) - from preferences, and returns a shutdown function the
// caller defers.
func Bootstrap(app fyne.App) func() {
	prefs := app.Preferences()

	applog.Init(prefs.Bool(common.PrefLogging), prefs.Bool(common.PrefDebug))

	// The offset is measured, never applied to the system clock. PrefNTPServers
	// overrides the public default list (blank = defaults); under smb mode the
	// operator puts the LAN NTP host first.
	timesync.Start(context.Background(), timesync.Config{
		Servers: timesync.ParseServers(prefs.String(common.PrefNTPServers)),
	})

	return func() {
		timesync.Stop()
		applog.Close()
	}
}
