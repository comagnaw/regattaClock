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

	applyLoggingDefault(prefs)
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

// applyLoggingDefault turns event logging on the first time the app runs.
// Logging is on by default; the operator opts out through the config screen,
// and that choice (stored as an explicit false) is preserved. The key is only
// seeded when it has never been written, so the config checkbox reflects the
// real state - BoolWithFallback returns the fallback verbatim only when the key
// is absent.
func applyLoggingDefault(prefs fyne.Preferences) {
	unset := prefs.BoolWithFallback(common.PrefLogging, true) &&
		!prefs.BoolWithFallback(common.PrefLogging, false)
	if unset {
		prefs.SetBool(common.PrefLogging, true)
	}
}
