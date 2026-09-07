package regatta

import (
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/comagnaw/regattaClock/internal/common"
)

func TestApplyLoggingDefault(t *testing.T) {
	t.Run("unset is seeded on", func(t *testing.T) {
		prefs := test.NewTempApp(t).Preferences()
		applyLoggingDefault(prefs)
		if !prefs.Bool(common.PrefLogging) {
			t.Error("logging should default on when the preference has never been set")
		}
	})

	t.Run("explicit opt-out is preserved", func(t *testing.T) {
		prefs := test.NewTempApp(t).Preferences()
		prefs.SetBool(common.PrefLogging, false)
		applyLoggingDefault(prefs)
		if prefs.Bool(common.PrefLogging) {
			t.Error("an explicit false must not be flipped back on")
		}
	})

	t.Run("explicit on stays on", func(t *testing.T) {
		prefs := test.NewTempApp(t).Preferences()
		prefs.SetBool(common.PrefLogging, true)
		applyLoggingDefault(prefs)
		if !prefs.Bool(common.PrefLogging) {
			t.Error("an explicit true must stay on")
		}
	})
}
