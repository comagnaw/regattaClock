package regatta

import (
	"context"
	"encoding/json"
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

// showPersonaPicker is the one startup screen: the personas are grouped into
// Timers / Media / Admins tabs. Pressing a persona's button asks for its
// challenge code in a small dialog (promptPersonaChallenge) - so choosing a
// persona is never itself a "wrong challenge" error. Media and Developer are
// disabled placeholders for personas that do not exist yet. The Regatta
// Director also gets a "resume" shortcut below the tabs when the last run was as
// the director and its schedule is still readable.
func (r *Regatta) showPersonaPicker() {
	personaButton := func(id string) *widget.Button {
		def, _ := persona.ByID(id)
		return widget.NewButton(def.Label, func() {
			r.promptPersonaChallenge(def)
		})
	}
	placeholderButton := func(label string) *widget.Button {
		b := widget.NewButton(label, nil)
		b.Disable()
		return b
	}

	timers := container.NewVBox(
		personaButton("pst"), personaButton("pft"),
		personaButton("sst"), personaButton("sft"),
	)
	media := container.NewVBox(
		placeholderButton(common.PersonaSocialMediaLabel),
		placeholderButton(common.PersonaStreamingLabel),
		placeholderButton(common.PersonaRegisterResultsLabel),
	)
	admins := container.NewVBox(
		widget.NewButton(persona.DirectorDefinition.Label, func() {
			r.promptPersonaChallenge(persona.DirectorDefinition)
		}),
		placeholderButton(common.PersonaDeveloperLabel),
	)

	tabs := container.NewAppTabs(
		container.NewTabItem(common.PersonaTabTimers, timers),
		container.NewTabItem(common.PersonaTabMedia, media),
		container.NewTabItem(common.PersonaTabAdmins, admins),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	note := widget.NewLabelWithStyle(common.PersonaPlaceholderNote, fyne.TextAlignLeading, fyne.TextStyle{Italic: true})

	rows := []fyne.CanvasObject{
		text.BoldLeading(common.PersonaPickerPrompt),
		tabs,
		note,
	}
	if resume := r.resumeDirectorButton(); resume != nil {
		rows = append(rows, widget.NewSeparator(), container.NewHBox(resume))
	}

	body := container.New(
		layout.NewCustomPaddedLayout(0, 0, viewMargin, viewMargin),
		container.NewVBox(rows...),
	)

	r.window.SetContent(container.NewVBox(
		container.New(
			layout.NewCustomPaddedLayout(viewMargin, 0, 0, 0),
			container.NewCenter(banner(welcomeBannerWidth, welcomeBannerHeight)),
		),
		body,
	))
}

// resumeDirectorButton returns a "Resume as Regatta Director - <name>" button
// when the previous run was the director and its schedule is still readable, or
// nil. Resume skips the challenge - same machine, same operator.
func (r *Regatta) resumeDirectorButton() *widget.Button {
	if r.App.Preferences().String(common.PrefLastPersonaID) != persona.DirectorDefinition.ID {
		return nil
	}
	session, ok := r.directorSession()
	if !ok {
		return nil
	}
	schedule, err := store.LoadSchedule(session)
	if err != nil {
		return nil
	}
	return widget.NewButton(fmt.Sprintf(common.ResumeDirectorFormat, schedule.Name), func() {
		applog.Info("resume as director", "component", "startup", "regatta", schedule.Name)
		r.startDirectorFlow()
	})
}

// promptPersonaChallenge asks for a persona's challenge code in a small dialog
// once its button is pressed, so selecting a persona is never itself an error.
// Cancel dismisses; confirm runs onPersonaChosen with what was typed.
func (r *Regatta) promptPersonaChallenge(def persona.Definition) {
	entry := widget.NewEntry()
	dialog.ShowForm(
		fmt.Sprintf(common.PersonaChallengeTitle, def.Label),
		common.ContinueButtonText,
		common.CancelButtonText,
		[]*widget.FormItem{widget.NewFormItem(common.ChallengeFieldLabel, entry)},
		func(ok bool) {
			if ok {
				r.onPersonaChosen(def, entry.Text)
			}
		},
		r.window,
	)
}

// onPersonaChosen validates the challenge for the pressed persona button, then
// routes to the director flow or the timer folder dialog. A failure keeps the
// picker on screen.
func (r *Regatta) onPersonaChosen(def persona.Definition, challengeInput string) {
	if !r.matchesChallenge(def, challengeInput) {
		applog.Info("persona challenge rejected", "component", "startup", "persona_id", def.ID)
		dialog.ShowError(errors.New(common.ChallengeMismatchMessage), r.window)
		return
	}
	applog.Info("persona challenge accepted", "component", "startup", "persona_id", def.ID)

	if def.Role == persona.RoleDirector {
		// The deliberate director choice always opens the Set Directory / Load
		// Excel view; the picker's separate "Resume" shortcut is the only path
		// that reopens the previous regatta.
		r.startDirectorSetup()
		return
	}
	r.pickPersonaDirectory(def)
}

func (r *Regatta) pickPersonaDirectory(def persona.Definition) {
	fd := dialog.NewFolderOpen(r.personaDirCallback(def), r.window)
	fd.SetTitleText(common.SelectRegattaFolderTitle)
	fd.SetConfirmText(common.UseThisFolderText)
	fd.Show()
}

// personaDirCallback validates the chosen folder, reads the schedule, and asks
// the operator to confirm the regatta before the session starts. On a failure
// the error dialog is shown first and the folder dialog re-opens only once it
// is dismissed, so the two never stack.
func (r *Regatta) personaDirCallback(def persona.Definition) func(fyne.ListableURI, error) {
	return func(dirReader fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, r.window)
			return
		}
		if dirReader == nil {
			return // cancelled - stay on the picker
		}

		root := resolvePersonaRoot(filepath.FromSlash(dirReader.Path()))
		if root == common.EmptyString {
			r.rechooseDirectory(def, errors.New(common.NotRegattaDataDirMessage))
			return
		}

		session := persona.Session{Definition: def, Root: root}

		schedule, err := store.LoadSchedule(session)
		if err != nil {
			r.rechooseDirectory(def, fmt.Errorf("%s: %w", common.ScheduleUnreadableMessage, err))
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
				r.confirmRegattaDate(def, session, schedule)
			},
			r.window,
		)
	}
}

// rechooseDirectory shows why the folder was rejected and re-opens the folder
// dialog only after the operator dismisses the message.
func (r *Regatta) rechooseDirectory(def persona.Definition, cause error) {
	d := dialog.NewError(cause, r.window)
	d.SetOnClosed(func() { r.pickPersonaDirectory(def) })
	d.Show()
}

// resolvePersonaRoot maps the folder the operator chose to the regattaData
// root. It accepts the regattaData directory itself, a folder renamed from it
// (a readable schedule sits directly inside), or - the case operators reach for
// - the parent directory that contains regattaData, since the subdirectory
// name means nothing to them. Returns "" if none of those hold.
func resolvePersonaRoot(chosen string) string {
	if isRegattaRoot(chosen) {
		return chosen
	}
	if child := filepath.Join(chosen, common.RegattaDataDir); isRegattaRoot(child) {
		return child
	}
	return common.EmptyString
}

// isRegattaRoot reports whether dir is an existing directory that looks like a
// regattaData root: named regattaData, or holding a readable
// director/regattaSchedule.json. The confirmation dialog is the real safeguard
// against the wrong regatta.
func isRegattaRoot(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	if filepath.Base(dir) == common.RegattaDataDir {
		return true
	}
	return filesystem.FileExists(persona.SchedulePathIn(dir))
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
	r.mode = modeTimer
	r.window.SetMainMenu(r.makeMenu()) // no loader items for a timer
	r.App.Preferences().SetString(common.PrefLastPersonaID, session.ID)
	r.startLogging()
	applog.Info("persona session started", "component", "startup",
		"root", session.Root, "regatta", schedule.Name)

	r.RegattaData = regattaDataFromSchedule(schedule)

	key := store.RegattaKey(schedule.Name, schedule.Date)
	r.regattaKey = key
	host := hostName()

	switch session.Role {
	case persona.RoleStart:
		r.startLog = r.hydrateOwnStart(session, key)
		r.startLog.RegattaKey = key
		r.startLog.Machine = host
		r.finishLog = r.hydratePeerFinish(session, key) // read-only, for the ST lock
	case persona.RoleFinish:
		r.startLog = r.hydratePeerStart(session, key)
		r.finishLog = r.hydrateOwnFinish(session, key)
		r.finishLog.RegattaKey = key
		r.finishLog.Machine = host
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
		applog.Warn("peer start.json unusable; ignored", "component", "startup", "err", err)
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

// hydratePeerFinish loads the finish timer's finish.json for a start timer,
// which does not own it: a mismatch or unreadable file is a warning only, and
// the start timer only reads it (to lock rows the FT has begun timing).
func (r *Regatta) hydratePeerFinish(s persona.Session, key string) *store.FinishLog {
	empty := &store.FinishLog{Races: map[int]store.RaceResult{}}
	log, err := store.LoadFinish(s)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return empty
	case err != nil:
		applog.Warn("peer finish.json unusable; ignored", "component", "startup", "err", err)
		return empty
	}
	if log.RegattaKey != "" && log.RegattaKey != key {
		applog.Warn("peer finish.json belongs to a different regatta; ignored",
			"component", "startup", "had", log.RegattaKey, "want", key)
		return empty
	}
	if log.Races == nil {
		log.Races = map[int]store.RaceResult{}
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
// start.json) for the life of the window, refreshing race rows in place as
// files change.
func (r *Regatta) startWatcher(s persona.Session) {
	// startDirectorFlow re-runs on every Apply / Reload, so tear down the
	// previous watcher (and its stale / origin tickers) before starting a new
	// one, or they accumulate.
	if r.stopWatcher != nil {
		r.stopWatcher()
	}

	mode := watcher.ParseMode(r.App.Preferences().String(common.PrefStorageMode))
	w := watcher.New(mode, 0)

	paths := []string{s.SchedulePath()}
	switch s.Role {
	case persona.RoleFinish:
		paths = append(paths, s.StartPath()) // peer start times
	case persona.RoleStart:
		paths = append(paths, s.FinishPath()) // FT progress, for the row lock
	case persona.RoleDirector:
		paths = append(paths, directorWatchPaths(s.Root)...) // both teams' start + finish
	}
	// Seed the last-applied hash from what hydrate already read, so the
	// watcher's unconditional first event for an unchanged file is a no-op.
	r.watchedHashes = make(map[string]string, len(paths))
	for _, p := range paths {
		w.Add(p)
		if data, err := os.ReadFile(p); err == nil {
			r.watchedHashes[p] = filesystem.HashBytes(data)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	events := w.Start(ctx)

	consumed := make(chan struct{})
	go func() { r.consumeWatcher(events); close(consumed) }()

	if s.Role == persona.RoleDirector {
		go r.staleTicker(ctx.Done())
		go r.originPoller(r.RegattaData.URI, r.RegattaData.Hash, ctx.Done())
	}

	// stopWatcher blocks until the watcher goroutine and its event consumer have
	// fully exited, so a subsequent startWatcher can safely reset watchedHashes.
	stop := func() {
		cancel()
		w.Stop()
		<-consumed
	}
	r.stopWatcher = stop
	r.window.SetOnClosed(stop)
	applog.Info("watcher started", "component", "startup", "mode", string(mode))
}

func (r *Regatta) consumeWatcher(events <-chan watcher.Event) {
	for ev := range events {
		r.applyWatchEvent(ev)
	}
}

// applyWatchEvent routes a changed file to the right in-memory mirror and
// refreshes the affected race rows on the UI thread.
func (r *Regatta) applyWatchEvent(ev watcher.Event) {
	if !r.watchedContentChanged(ev) {
		return
	}
	switch ev.Path {
	case r.session.SchedulePath():
		var sch store.Schedule
		if err := json.Unmarshal(ev.Data, &sch); err != nil {
			applog.Warn("watched schedule did not parse", "component", "race_tree", "err", err)
			return
		}
		applog.Info("schedule updated", "component", "race_tree", "races", len(sch.Races))
		fyne.Do(func() { r.onScheduleChanged(&sch) })

	case r.session.StartPath():
		if r.session.Role != persona.RoleFinish {
			return
		}
		var log store.StartLog
		if err := json.Unmarshal(ev.Data, &log); err != nil {
			applog.Warn("watched start.json did not parse", "component", "race_tree", "err", err)
			return
		}
		if log.RegattaKey != common.EmptyString && log.RegattaKey != r.regattaKey {
			applog.Warn("watched peer start.json is a different regatta; ignored", "component", "race_tree")
			return
		}
		if log.Races == nil {
			log.Races = map[int]store.StartRecord{}
		}
		applog.Info("peer start times updated", "component", "race_tree", "races", len(log.Races))
		fyne.Do(func() { r.onPeerStartChanged(&log) })

	case r.session.FinishPath():
		if r.session.Role != persona.RoleStart {
			return
		}
		var log store.FinishLog
		if err := json.Unmarshal(ev.Data, &log); err != nil {
			applog.Warn("watched finish.json did not parse", "component", "race_tree", "err", err)
			return
		}
		if log.RegattaKey != common.EmptyString && log.RegattaKey != r.regattaKey {
			applog.Warn("watched finish.json is a different regatta; ignored", "component", "race_tree")
			return
		}
		if log.Races == nil {
			log.Races = map[int]store.RaceResult{}
		}
		applog.Info("finish progress updated", "component", "race_tree", "races", len(log.Races))
		fyne.Do(func() { r.onPeerFinishChanged(&log) })

	default:
		if r.session.Role == persona.RoleDirector {
			r.applyDirectorTimingEvent(ev)
		}
	}
}

// watchedContentChanged reports whether ev carries content this window has not
// applied yet, updating the per-path hash. Runs only on the watcher goroutine.
func (r *Regatta) watchedContentChanged(ev watcher.Event) bool {
	h := filesystem.HashBytes(ev.Data)
	if r.watchedHashes[ev.Path] == h {
		return false
	}
	r.watchedHashes[ev.Path] = h
	return true
}

// onScheduleChanged re-renders labels from a new schedule in place, rebuilding
// the tree only when races were added or removed. It never rewrites start.json
// or finish.json; a change to a race that already has timing (or an open clock)
// raises a dismissible notice instead (persona-plan.md 3c).
func (r *Regatta) onScheduleChanged(sch *store.Schedule) {
	old := r.RegattaData
	r.RegattaData = regattaDataFromSchedule(sch)
	changed := diffSchedule(old, r.RegattaData)

	if r.raceSetChanged() {
		r.showRaceTree()
	} else {
		r.refreshAllRows()
	}
	r.applyScheduleConflicts(changed)
	r.pushScheduleToOpenClocks(changed)
}

func (r *Regatta) onPeerStartChanged(log *store.StartLog) {
	r.startLog = log
	// Push the fresh start times into any open race clock so a winning time
	// that was waiting on the ST recomputes in place (persona-plan.md 2.2).
	for _, clk := range r.openClocks {
		clk.UpdateStartTime(log)
	}
	r.refreshAllRows()
}

// onPeerFinishChanged - a start timer seeing the finish timer's progress. It
// only reads this file; a RaceResult appearing for a race locks that ST row.
func (r *Regatta) onPeerFinishChanged(log *store.FinishLog) {
	r.finishLog = log
	r.refreshAllRows()
}

func (r *Regatta) raceSetChanged() bool {
	live := map[int]bool{}
	for _, race := range r.RegattaData.Races {
		if race.HasBoats() {
			live[race.RaceNumber] = true
		}
	}
	if len(live) != len(r.rows) {
		return true
	}
	for n := range live {
		if _, ok := r.rows[n]; !ok {
			return true
		}
	}
	return false
}
