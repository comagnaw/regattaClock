package regatta

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/personacfg"
	"github.com/comagnaw/regattaClock/internal/reader"
	"github.com/comagnaw/regattaClock/internal/text"
)

// hostName reports this computer's hostname, or "" when the OS will not give one.
// A package var so tests can substitute a deterministic value.
var hostName = func() string {
	h, _ := os.Hostname()
	return h
}

// loadPersonaConfig reads the deployment persona config named by
// PrefPersonaConfigFile, when one is set. Every failure is non-fatal: personaCfg
// stays nil (so the normal picker runs) and a notice is queued for once the
// window has a canvas. A blank preference is silent - the common case.
func (r *Regatta) loadPersonaConfig() {
	path := r.App.Preferences().String(common.PrefPersonaConfigFile)
	if path == common.EmptyString {
		return
	}
	cfg, err := personacfg.Load(path)
	if err != nil {
		applog.Warn("persona config not loaded", "component", "startup", "path", path,
			"missing", errors.Is(err, fs.ErrNotExist), "err", err)
		r.warnOnStarted(fmt.Errorf(common.PersonaConfigLoadFailedFormat, err))
		return
	}
	r.personaCfg = cfg
	applog.Info("persona config loaded", "component", "startup",
		"hosts", len(cfg.Hosts), "challenges", len(cfg.Challenges))
}

// assignedPersona is the persona this host is pinned to by the deployment
// config, or false when there is no config or no matching host entry.
func (r *Regatta) assignedPersona() (persona.Definition, bool) {
	if r.personaCfg == nil {
		return persona.Definition{}, false
	}
	return r.personaCfg.AssignmentFor(hostName())
}

// startAssignedPersona routes a pinned host past the picker without ever asking
// for a challenge. The director goes to the two-step Director Setup view (which
// never auto-restores); a timer lands on showAssignedPersona - a view, not a
// dialog, because New runs before the window has a canvas.
func (r *Regatta) startAssignedPersona(def persona.Definition) {
	applog.Info("persona assigned by deployment config", "component", "startup",
		"persona_id", def.ID, "host", hostName())
	if def.Role == persona.RoleDirector {
		r.startDirectorSetup()
		return
	}
	r.showAssignedPersona(def)
}

// showAssignedPersona is the minimal landing view for a pinned timer host:
// banner, a line naming the assigned persona, a button that opens the regatta
// folder dialog (the one thing still required), and a note pointing at the
// Switch Persona hatch. A cancelled folder dialog leaves the operator here.
func (r *Regatta) showAssignedPersona(def persona.Definition) {
	selectBtn := widget.NewButton(common.AssignedPersonaSelectFolderButtonText, func() {
		r.pickPersonaDirectory(def)
	})
	note := text.Note(common.AssignedPersonaSwitchNote)

	body := container.New(
		layout.NewCustomPaddedLayout(0, 0, viewMargin, viewMargin),
		container.NewVBox(
			text.BoldLeading(fmt.Sprintf(common.AssignedPersonaBannerFormat, def.Label)),
			container.NewHBox(selectBtn),
			note,
		),
	)

	r.window.SetContent(container.NewVBox(
		container.New(
			layout.NewCustomPaddedLayout(viewMargin, 0, 0, 0),
			container.NewCenter(banner(welcomeBannerWidth, welcomeBannerHeight)),
		),
		body,
	))
}

// matchesChallenge applies a deployment challenge override when one is loaded,
// otherwise the persona's built-in code.
func (r *Regatta) matchesChallenge(def persona.Definition, input string) bool {
	if r.personaCfg != nil {
		return r.personaCfg.MatchesChallenge(def, input)
	}
	return def.MatchesChallenge(input)
}

// switchPersonaItem - the safety hatch present on every menu: end the current
// session and re-open the picker, so a pinned-host operator can step into a
// different role. Open race-clock windows keep their own bound session.
func (r *Regatta) switchPersonaItem() *fyne.MenuItem {
	return fyne.NewMenuItem(common.SwitchPersonaMenuLabel, func() {
		dialog.ShowConfirm(common.SwitchPersonaConfirmTitle, common.SwitchPersonaConfirmMessage,
			func(yes bool) {
				if yes {
					r.switchPersona()
				}
			}, r.window)
	})
}

// switchPersona tears the current session down and returns to the picker. The
// shared-file watcher (and the RD's tickers) are stopped first; in-memory timing
// state and the menu are reset so the next persona starts clean.
func (r *Regatta) switchPersona() {
	applog.Info("switch persona", "component", "startup", "from", r.session.ID)
	if r.stopWatcher != nil {
		r.stopWatcher()
		r.stopWatcher = nil
	}
	r.mode = modeUnset
	r.session = persona.Session{}
	r.RegattaData = reader.NewRegattaData()
	r.regattaKey = common.EmptyString
	r.startLog = nil
	r.finishLog = nil
	r.writesBlocked = false
	r.window.SetMainMenu(r.makeMenu())
	r.showPersonaPicker()
}
