package regatta

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/filesystem"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/text"
	"github.com/comagnaw/regattaClock/internal/watcher"
)

// showPersonaPicker is the timer's first screen: pick one of the four timing
// personas and type its challenge code.
func (r *Regatta) showPersonaPicker() {
	labels := make([]string, len(persona.Registry))
	for i, d := range persona.Registry {
		labels[i] = d.Label
	}
	picker := widget.NewRadioGroup(labels, nil)

	challenge := widget.NewEntry()
	challenge.SetPlaceHolder(common.ChallengePlaceholder)

	cont := widget.NewButton(common.ContinueButtonText, func() {
		r.onPersonaChosen(picker.Selected, challenge.Text)
	})

	body := container.New(
		layout.NewCustomPaddedLayout(0, 0, viewMargin, viewMargin),
		container.NewVBox(
			text.BoldLeading(common.PersonaPickerPrompt),
			picker,
			widget.NewLabel(common.ChallengeFieldLabel),
			challenge,
			container.NewHBox(cont),
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

func personaByLabel(label string) (persona.Definition, bool) {
	for _, d := range persona.Registry {
		if d.Label == label {
			return d, true
		}
	}
	return persona.Definition{}, false
}

// onPersonaChosen validates the challenge, then moves on to the folder dialog.
// A failure keeps the picker on screen.
func (r *Regatta) onPersonaChosen(label, challengeInput string) {
	def, ok := personaByLabel(label)
	if !ok {
		dialog.ShowError(errors.New(common.NoPersonaSelectedMessage), r.window)
		return
	}
	if !def.MatchesChallenge(challengeInput) {
		applog.Info("persona challenge rejected", "component", "startup", "persona_id", def.ID)
		dialog.ShowError(errors.New(common.ChallengeMismatchMessage), r.window)
		return
	}
	applog.Info("persona challenge accepted", "component", "startup", "persona_id", def.ID)
	r.pickPersonaDirectory(def)
}

func (r *Regatta) pickPersonaDirectory(def persona.Definition) {
	dialog.ShowFolderOpen(r.personaDirCallback(def), r.window)
}

// personaDirCallback validates the chosen folder, reads the schedule, and asks
// the operator to confirm the regatta before the session starts. Any failure
// re-opens the folder dialog.
func (r *Regatta) personaDirCallback(def persona.Definition) func(fyne.ListableURI, error) {
	return func(dirReader fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, r.window)
			return
		}
		if dirReader == nil {
			return // cancelled - stay on the picker
		}

		session := persona.Session{
			Definition: def,
			Root:       filepath.FromSlash(dirReader.Path()),
		}

		if !validPersonaRoot(session) {
			dialog.ShowError(errors.New(common.NotRegattaDataDirMessage), r.window)
			r.pickPersonaDirectory(def)
			return
		}

		schedule, err := store.LoadSchedule(session)
		if err != nil {
			dialog.ShowError(fmt.Errorf("%s: %w", common.ScheduleUnreadableMessage, err), r.window)
			r.pickPersonaDirectory(def)
			return
		}

		dialog.ShowConfirm(
			common.ConfirmRegattaTitle,
			fmt.Sprintf(common.ConfirmRegattaMessage, schedule.Name, schedule.Date, scheduledRaceCount(schedule)),
			func(yes bool) {
				if !yes {
					r.pickPersonaDirectory(def)
					return
				}
				r.startSession(session, schedule)
			},
			r.window,
		)
	}
}

// validPersonaRoot accepts a folder whose basename is regattaData, or any
// folder that already contains a readable director/regattaSchedule.json -
// cloud clients rename shared-folder shortcuts, and an SMB share can be browsed
// under any leaf name. The confirmation dialog is the real safeguard against
// the wrong regatta.
func validPersonaRoot(s persona.Session) bool {
	if filepath.Base(s.Root) == common.RegattaDataDir {
		return true
	}
	return filesystem.FileExists(s.SchedulePath())
}

func scheduledRaceCount(sch *store.Schedule) int {
	n := 0
	for _, race := range sch.Races {
		if len(race.Lanes) > 0 {
			n++
		}
	}
	return n
}

// startSession binds the persona to the chosen directory, points logging at the
// persona file, hydrates this persona's in-memory state, shows the race tree,
// and starts the shared-file watcher.
func (r *Regatta) startSession(session persona.Session, schedule *store.Schedule) {
	r.session = session
	r.startLogging()
	applog.Info("persona session started", "component", "startup",
		"root", session.Root, "regatta", schedule.Name)

	r.RegattaData = regattaDataFromSchedule(schedule)

	key := store.RegattaKey(schedule.Name, schedule.Date)
	switch session.Role {
	case persona.RoleStart:
		r.startLog = r.hydrateOwnStart(session, key)
	case persona.RoleFinish:
		r.startLog = r.hydratePeerStart(session, key)
		r.finishLog = r.hydrateOwnFinish(session, key)
	}

	r.refreshContent()
	r.showRaceTree()
	r.startWatcher(session)
}

// hydrateOwnStart loads the start timer's own start.json under the four rules
// of section 8: missing is normal, a parse failure blocks writes, a different
// regatta is set aside, and the sequence counter carries over inside the
// returned struct.
func (r *Regatta) hydrateOwnStart(s persona.Session, key string) *store.StartLog {
	empty := &store.StartLog{Races: map[int]store.StartRecord{}}
	log, err := store.LoadStart(s)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return empty
	case errors.Is(err, store.ErrCorrupt):
		r.blockWritesForCorruptFile(s.StartPath(), err)
		return empty
	case err != nil:
		applog.Error("start log load failed", "component", "startup", "err", err)
		return empty
	}
	if log.RegattaKey != "" && log.RegattaKey != key {
		r.setAsideDifferentRegatta(s.StartPath(), log.RegattaKey, key)
		return empty
	}
	if log.Races == nil {
		log.Races = map[int]store.StartRecord{}
	}
	applog.Info("start times restored", "component", "startup", "races", len(log.Races))
	return log
}

// hydratePeerStart loads the start timer's start.json for a finish timer, which
// does not own it: a mismatch or unreadable file is a warning only, never a
// rename or a write block.
func (r *Regatta) hydratePeerStart(s persona.Session, key string) *store.StartLog {
	empty := &store.StartLog{Races: map[int]store.StartRecord{}}
	log, err := store.LoadStart(s)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return empty
	case err != nil:
		applog.Warn("peer start.json unusable; no start times shown", "component", "startup", "err", err)
		return empty
	}
	if log.RegattaKey != "" && log.RegattaKey != key {
		applog.Warn("peer start.json belongs to a different regatta; ignored",
			"component", "startup", "had", log.RegattaKey, "want", key)
		return empty
	}
	if log.Races == nil {
		log.Races = map[int]store.StartRecord{}
	}
	return log
}

func (r *Regatta) hydrateOwnFinish(s persona.Session, key string) *store.FinishLog {
	empty := &store.FinishLog{Races: map[int]store.RaceResult{}}
	log, err := store.LoadFinish(s)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return empty
	case errors.Is(err, store.ErrCorrupt):
		r.blockWritesForCorruptFile(s.FinishPath(), err)
		return empty
	case err != nil:
		applog.Error("finish log load failed", "component", "startup", "err", err)
		return empty
	}
	if log.RegattaKey != "" && log.RegattaKey != key {
		r.setAsideDifferentRegatta(s.FinishPath(), log.RegattaKey, key)
		return empty
	}
	if log.Races == nil {
		log.Races = map[int]store.RaceResult{}
	}
	applog.Info("finish results restored", "component", "startup", "races", len(log.Races))
	return log
}

func (r *Regatta) setAsideDifferentRegatta(path, had, want string) {
	dst := fmt.Sprintf("%s.other-regatta-%s", path, time.Now().Format("20060102-150405"))
	if err := os.Rename(path, dst); err != nil {
		applog.Warn("could not set aside a different regatta's file", "component", "startup", "file", path, "err", err)
		return
	}
	applog.Warn("timing file belongs to a different regatta; set aside", "component", "startup",
		"file", path, "moved_to", dst, "had", had, "want", want)
	dialog.ShowInformation(common.AppTitle,
		fmt.Sprintf("%s held data for a different regatta and was moved to %s. Starting clean.",
			filepath.Base(path), filepath.Base(dst)),
		r.window)
}

// blockWritesForCorruptFile copies the unreadable file aside (leaving the
// original in place too), sets writesBlocked so phases 6-7 refuse to overwrite
// it, and tells the operator.
func (r *Regatta) blockWritesForCorruptFile(path string, cause error) {
	r.writesBlocked = true
	aside := fmt.Sprintf("%s.corrupt-%s", path, time.Now().Format("20060102-150405"))
	if data, err := os.ReadFile(path); err == nil {
		if err := os.WriteFile(aside, data, 0644); err != nil {
			applog.Warn("could not copy corrupt file aside", "component", "startup", "file", path, "err", err)
		}
	}
	applog.Error("timing file failed to parse; recording blocked", "component", "startup",
		"file", path, "aside", aside, "err", cause)
	dialog.ShowInformation(common.CorruptTimingFileTitle,
		fmt.Sprintf(common.CorruptTimingFileMessage, filepath.Base(path), filepath.Base(aside), cause),
		r.window)
}

// startWatcher watches the schedule (and, for a finish timer, the peer
// start.json) for the life of the window. Phase 6 turns the events into
// in-place race-row updates; for now they are logged.
func (r *Regatta) startWatcher(s persona.Session) {
	mode := watcher.ParseMode(r.App.Preferences().String(common.PrefStorageMode))
	w := watcher.New(mode, 0)
	w.Add(s.SchedulePath())
	if s.Role == persona.RoleFinish {
		w.Add(s.StartPath())
	}

	ctx, cancel := context.WithCancel(context.Background())
	r.stopWatcher = cancel
	events := w.Start(ctx)
	go consumeWatcher(events)

	r.window.SetOnClosed(func() {
		cancel()
		w.Stop()
	})
	applog.Info("watcher started", "component", "startup", "mode", string(mode))
}

func consumeWatcher(events <-chan watcher.Event) {
	for ev := range events {
		applog.Debug("watched file changed", "component", "startup",
			"path", ev.Path, "bytes", len(ev.Data))
	}
}
