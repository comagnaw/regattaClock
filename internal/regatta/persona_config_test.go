package regatta

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/personacfg"
)

// stubHostName pins hostName() to a fixed value for the test and restores it
// afterwards, so the deployment-config host match is deterministic on any OS.
func stubHostName(t *testing.T, name string) {
	t.Helper()
	prev := hostName
	hostName = func() string { return name }
	t.Cleanup(func() { hostName = prev })
}

func writePersonaConfigFile(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "personas.json")
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return p
}

func canvasTextContains(o fyne.CanvasObject, want string) bool {
	switch v := o.(type) {
	case *canvas.Text:
		return v.Text == want
	case *fyne.Container:
		for _, c := range v.Objects {
			if canvasTextContains(c, want) {
				return true
			}
		}
	}
	return false
}

func setPersonaConfigPref(t *testing.T, app fyne.App, body string) {
	t.Helper()
	app.Preferences().SetString(common.PrefPersonaConfigFile, writePersonaConfigFile(t, body))
}

func TestNew_NoPersonaConfig_ShowsPicker(t *testing.T) {
	app := test.NewTempApp(t)

	r := New(app)

	if findAppTabs(r.window.Content()) == nil {
		t.Error("with no deployment config the persona picker should be shown")
	}
	if r.personaCfg != nil {
		t.Error("personaCfg should be nil when no config file is configured")
	}
}

func TestNew_PersonaConfigAssignsTimer_ShowsLandingView(t *testing.T) {
	app := test.NewTempApp(t)
	setPersonaConfigPref(t, app, `{"hosts":{"race-pc":"pst"}}`)
	stubHostName(t, "race-pc")

	r := New(app)

	if findAppTabs(r.window.Content()) != nil {
		t.Error("a pinned host must skip the persona picker")
	}
	if findButtonByLabel(r.window.Content(), common.AssignedPersonaSelectFolderButtonText) == nil {
		t.Errorf("landing view is missing the folder button; buttons=%v", buttonLabels(r.window.Content()))
	}
	if !canvasTextContains(r.window.Content(), "You are set up as Primary Start Timer on this computer.") {
		t.Error("landing view should name the assigned persona")
	}
	if r.mode != modeUnset {
		t.Errorf("mode = %v, want modeUnset until a folder is chosen", r.mode)
	}
	if r.session.Root != common.EmptyString {
		t.Error("no session should be bound before the folder is chosen")
	}
	if r.personaCfg == nil {
		t.Error("personaCfg should be set after a successful load")
	}
}

func TestNew_PersonaConfigDottedHostname(t *testing.T) {
	app := test.NewTempApp(t)
	setPersonaConfigPref(t, app, `{"hosts":{"race-pc":"pft"}}`)
	stubHostName(t, "race-pc.regatta.example.com")

	r := New(app)

	if findAppTabs(r.window.Content()) != nil {
		t.Error("an FQDN should still match the short-name host entry")
	}
	if !canvasTextContains(r.window.Content(), "You are set up as Primary Finish Timer on this computer.") {
		t.Error("landing view should name the assigned persona")
	}
}

func TestNew_PersonaConfigAssignsDirector_ShowsSetup(t *testing.T) {
	app := test.NewTempApp(t)
	setPersonaConfigPref(t, app, `{"hosts":{"rd-pc":"rd"}}`)
	stubHostName(t, "rd-pc")

	r := New(app)

	if !onWelcome(r) {
		t.Errorf("a director-pinned host should land on Director Setup; buttons=%v", buttonLabels(r.window.Content()))
	}
	if r.mode != modeDirector {
		t.Errorf("mode = %v, want modeDirector", r.mode)
	}
}

func TestNew_PersonaConfigTimerFolderPickReachesCallback(t *testing.T) {
	app := test.NewTempApp(t)
	setPersonaConfigPref(t, app, `{"hosts":{"race-pc":"pst"}}`)
	stubHostName(t, "race-pc")
	root := seedRegatta(t, testSchedule())

	r := New(app)
	stopWatch(t, r)

	btn := findButtonByLabel(r.window.Content(), common.AssignedPersonaSelectFolderButtonText)
	if btn == nil {
		t.Fatal("landing view is missing the folder button")
	}
	btn.OnTapped()
	if len(r.window.Canvas().Overlays().List()) == 0 {
		t.Fatal("the folder button should open a dialog")
	}

	def, _ := persona.ByID("pst")
	r.personaDirCallback(def)(listerFor(t, filepath.Dir(root)), nil)
	if len(r.window.Canvas().Overlays().List()) == 0 {
		t.Error("choosing the regatta folder should raise the confirm dialog")
	}
	if r.session.Root != common.EmptyString {
		t.Error("the session should not bind until the confirm dialog is accepted")
	}
}

func TestNew_MissingPersonaConfigPath_FallsBackToPicker(t *testing.T) {
	app := test.NewTempApp(t)
	app.Preferences().SetString(common.PrefPersonaConfigFile, filepath.Join(t.TempDir(), "gone.json"))

	r := New(app)

	if findAppTabs(r.window.Content()) == nil {
		t.Error("a missing config file should fall back to the picker")
	}
	if r.personaCfg != nil {
		t.Error("personaCfg should stay nil after a failed load")
	}
}

func TestNew_BadJSONPersonaConfig_FallsBackToPicker(t *testing.T) {
	app := test.NewTempApp(t)
	setPersonaConfigPref(t, app, `{ not json`)

	r := New(app)

	if findAppTabs(r.window.Content()) == nil {
		t.Error("a malformed config file should fall back to the picker")
	}
	if r.personaCfg != nil {
		t.Error("personaCfg should stay nil after a failed load")
	}
}

func TestNew_UnknownIDPersonaConfig_FallsBackToPicker(t *testing.T) {
	app := test.NewTempApp(t)
	setPersonaConfigPref(t, app, `{"hosts":{"race-pc":"zzz"}}`)
	stubHostName(t, "race-pc")

	r := New(app)

	if findAppTabs(r.window.Content()) == nil {
		t.Error("an unknown persona ID should fail the load and fall back to the picker")
	}
	if r.personaCfg != nil {
		t.Error("personaCfg should stay nil after a failed load")
	}
}

func TestMatchesChallenge_OverrideWhenConfigLoaded(t *testing.T) {
	r := NewTimer(test.NewTempApp(t))
	r.personaCfg = &personacfg.Config{Challenges: map[string]string{"pft": "letmein"}}

	pft, _ := persona.ByID("pft")
	pst, _ := persona.ByID("pst")

	if !r.matchesChallenge(pft, "letmein") {
		t.Error("the override code should be accepted")
	}
	if r.matchesChallenge(pft, "rc-pft") {
		t.Error("the built-in code should be rejected once overridden")
	}
	if !r.matchesChallenge(pst, "rc-pst") {
		t.Error("a persona with no override should still take its built-in code")
	}
}

func TestMatchesChallenge_NoConfigUsesBuiltIn(t *testing.T) {
	r := NewTimer(test.NewTempApp(t))
	pst, _ := persona.ByID("pst")

	if !r.matchesChallenge(pst, "rc-pst") {
		t.Error("with no config the built-in code should be accepted")
	}
	if r.matchesChallenge(pst, "nope") {
		t.Error("with no config a wrong code should be rejected")
	}
}

func TestOnPersonaChosen_RejectsBuiltInWhenOverridden(t *testing.T) {
	r := New(test.NewTempApp(t))
	r.personaCfg = &personacfg.Config{Challenges: map[string]string{"pft": "letmein"}}
	def, _ := persona.ByID("pft")

	r.onPersonaChosen(def, "rc-pft")
	if len(r.window.Canvas().Overlays().List()) == 0 {
		t.Fatal("a rejected override should raise an error dialog")
	}
	if r.session.Root != common.EmptyString || r.mode != modeUnset {
		t.Error("a rejected challenge must not enter any flow")
	}

	r.onPersonaChosen(def, "letmein")
	if len(r.window.Canvas().Overlays().List()) == 0 {
		t.Error("the accepted override should open the regatta folder dialog")
	}
}

func TestSwitchPersonaItem_OnEveryMenu(t *testing.T) {
	t.Run("picker", func(t *testing.T) {
		r := New(test.NewTempApp(t))
		if !slices.Contains(menuLabels(r.window.MainMenu()), common.SwitchPersonaMenuLabel) {
			t.Error("Switch Persona missing from the picker menu")
		}
	})
	t.Run("timer", func(t *testing.T) {
		r := NewTimer(test.NewTempApp(t))
		stopWatch(t, r)
		if !slices.Contains(menuLabels(r.window.MainMenu()), common.SwitchPersonaMenuLabel) {
			t.Error("Switch Persona missing from the timer menu")
		}
	})
	t.Run("director", func(t *testing.T) {
		r := NewDirector(test.NewTempApp(t))
		if !slices.Contains(menuLabels(r.window.MainMenu()), common.SwitchPersonaMenuLabel) {
			t.Error("Switch Persona missing from the director menu")
		}
	})
}

func TestSwitchPersona_FromLiveTimerSession(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	pst := timerSession(t, "pst", root)

	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(pst, sch)

	if r.stopWatcher == nil || r.session.Root == common.EmptyString || r.mode != modeTimer {
		t.Fatal("precondition: a live timer session")
	}

	r.switchPersona()

	if r.stopWatcher != nil {
		t.Error("the watcher should be torn down")
	}
	if r.mode != modeUnset {
		t.Errorf("mode = %v, want modeUnset", r.mode)
	}
	if r.session != (persona.Session{}) {
		t.Error("the session should be cleared")
	}
	if len(r.RegattaData.Races) != 0 {
		t.Error("regatta data should be reset")
	}
	if r.regattaKey != common.EmptyString || r.writesBlocked {
		t.Error("regattaKey / writesBlocked should be reset")
	}
	if findAppTabs(r.window.Content()) == nil {
		t.Error("the persona picker should be back on screen")
	}
}

func TestPersonaConfigCallback_LoadsValidFile(t *testing.T) {
	app := test.NewTempApp(t)
	r := NewDirector(app)
	path := writePersonaConfigFile(t, `{"hosts":{"pc-1":"pst"},"challenges":{"rd":"x"}}`)

	rc, err := storage.Reader(storage.NewFileURI(path))
	if err != nil {
		t.Fatal(err)
	}
	r.personaConfigCallback()(rc, nil)

	if got := app.Preferences().String(common.PrefPersonaConfigFile); got != path {
		t.Errorf("pref = %q, want %q", got, path)
	}
	if r.personaCfg == nil || len(r.personaCfg.Hosts) != 1 || len(r.personaCfg.Challenges) != 1 {
		t.Errorf("personaCfg not loaded as expected: %+v", r.personaCfg)
	}
}

func TestPersonaConfigCallback_RejectsInvalidFile(t *testing.T) {
	app := test.NewTempApp(t)
	r := NewDirector(app)
	path := writePersonaConfigFile(t, `{ not json`)

	rc, err := storage.Reader(storage.NewFileURI(path))
	if err != nil {
		t.Fatal(err)
	}
	r.personaConfigCallback()(rc, nil)

	if got := app.Preferences().String(common.PrefPersonaConfigFile); got != common.EmptyString {
		t.Errorf("a rejected file must not be persisted, got %q", got)
	}
	if r.personaCfg != nil {
		t.Error("a rejected file must not replace personaCfg")
	}
	if len(r.window.Canvas().Overlays().List()) == 0 {
		t.Error("a rejected file should raise an error dialog")
	}
}

func TestPersonaConfigCallback_CancelIsNoOp(t *testing.T) {
	app := test.NewTempApp(t)
	r := NewDirector(app)

	r.personaConfigCallback()(nil, nil)

	if got := app.Preferences().String(common.PrefPersonaConfigFile); got != common.EmptyString {
		t.Errorf("cancel must not persist anything, got %q", got)
	}
	if r.personaCfg != nil {
		t.Error("cancel must not set personaCfg")
	}
}

func TestClearPersonaConfig(t *testing.T) {
	app := test.NewTempApp(t)
	app.Preferences().SetString(common.PrefPersonaConfigFile, "/some/path.json")
	r := NewDirector(app)
	r.personaCfg = &personacfg.Config{}

	r.clearPersonaConfig()

	if got := app.Preferences().String(common.PrefPersonaConfigFile); got != common.EmptyString {
		t.Errorf("pref should be cleared, got %q", got)
	}
	if r.personaCfg != nil {
		t.Error("personaCfg should be nil after clear")
	}
}

func TestConfigContentHasPersonaConfigRow(t *testing.T) {
	r := NewDirector(test.NewTempApp(t))
	if !slices.Contains(buttonLabels(r.config), common.PersonaConfigChangeButtonText) {
		t.Errorf("config screen is missing the persona config row; buttons=%v", buttonLabels(r.config))
	}
}
