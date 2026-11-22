package regatta

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/filesystem"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/publish"
	"github.com/comagnaw/regattaClock/internal/publish/destinations"
)

// Results publishing (results-publisher.md, publish-destinations.md): the
// Primary Finish Timer's per-race Publish button regenerates this regatta's
// public results in every enabled destination - the results workbook, the
// web-results JSON file, later RegattaCentral. Nothing new is written to
// regattaData beyond the PFT's publish settings: each output is rendered from
// finish.json + the schedule via internal/publish, and records what has been
// published in its own ledger (the workbook's very-hidden sheet, the JSON's
// per-race revision).
//
// The destinations and their folders are regatta-scoped, not just machine
// settings: they are recorded in finish.json (store.FinishLog.Publish) once
// the PFT confirms them. A new regatta starts with a fresh finish.json - the
// results workbook only, and no folder until the PFT confirms one, never
// silently the previous regatta's. The per-machine preferences
// (resultsDirPref) only remember the last folder used for each destination,
// to offer as the default in that confirmation.

// isPublisher reports whether this session owns the Publish button - only
// the primary finish timer, whose Approved result is the official one.
func (r *Regatta) isPublisher() bool {
	return r.session.Role == persona.RoleFinish && r.session.Team == persona.TeamPrimary
}

// resultsDirPref is the preference remembering this machine's last-used
// folder for kind. The workbook keeps the original key, so an existing
// machine's default carries over.
func resultsDirPref(kind string) string {
	if kind == store.DestinationSpreadsheet {
		return common.PrefResultsDir
	}
	return common.PrefResultsDir + "." + kind
}

// enabledKinds is this regatta's enabled destinations, in order. A kind this
// build does not know (written by a newer one) is skipped.
func (r *Regatta) enabledKinds() []destinations.Kind {
	var out []destinations.Kind
	for _, id := range r.finishLog.PublishDestinations() {
		if k, ok := destinations.ByID(id); ok {
			out = append(out, k)
		} else {
			applog.Warn("unknown results destination skipped", "component", "publish", "kind", id)
		}
	}
	return out
}

// folderKinds is the enabled, built destinations that publish into a folder.
func (r *Regatta) folderKinds() []string {
	var out []string
	for _, k := range r.enabledKinds() {
		if k.Available && k.NeedsFolder {
			out = append(out, k.ID)
		}
	}
	return out
}

func kindLabel(kind string) string {
	if k, ok := destinations.ByID(kind); ok {
		return k.Label
	}
	return kind
}

// publishMeta is the regatta-level text every destination prints.
func (r *Regatta) publishMeta() publish.Meta {
	return publish.Meta{Name: r.schedule.Name, Date: r.schedule.Date, RegattaKey: r.regattaKey}
}

// savedResultsDir is the folder confirmed for kind in this regatta
// (finish.json), or "" when none has been confirmed yet.
func (r *Regatta) savedResultsDir(kind string) string {
	return r.finishLog.PublishDir(kind)
}

// resultsDir is kind's confirmed folder, or "" when none is confirmed or it
// is no longer reachable (an unplugged drive, a renamed share).
func (r *Regatta) resultsDir(kind string) string {
	dir := r.savedResultsDir(kind)
	if dir == common.EmptyString || !filesystem.DirExists(dir) {
		return common.EmptyString
	}
	return dir
}

// resultsPath is kind's output file for this regatta, or "" with no usable
// folder.
func (r *Regatta) resultsPath(kind string) string {
	k, ok := destinations.ByID(kind)
	dir := r.resultsDir(kind)
	if !ok || k.New == nil || dir == common.EmptyString || r.schedule == nil {
		return common.EmptyString
	}
	return filepath.Join(dir, k.New().FileName(r.publishMeta()))
}

// startPublishing runs once a publisher's session is on screen: for each
// enabled folder destination in turn, load what its existing output says is
// published, or - with no usable folder for this regatta - ask for one.
// Declining is fine; timing never depends on it, and Publish asks again.
func (r *Regatta) startPublishing() {
	r.refreshPublishSummary()
	if r.isPublisher() {
		r.ensureResultsDirs(r.folderKinds(), nil, r.loadPublishedLedgers)
	}
}

// ensureResultsDirs runs ensureResultsDir for each kind in turn - one prompt
// at a time - calling each(kind) for every kind that ends up with a folder,
// then final (if any) once all have been offered.
func (r *Regatta) ensureResultsDirs(kinds []string, each func(kind string), final func()) {
	if len(kinds) == 0 {
		call(final)
		return
	}
	kind := kinds[0]
	r.ensureResultsDir(kind,
		func() {
			if each != nil {
				each(kind)
			}
		},
		func() { r.ensureResultsDirs(kinds[1:], each, final) })
}

// ensureResultsDir runs onReady once kind has a reachable folder for this
// regatta, first asking for one when it does not:
//   - a folder saved for this regatta but unreachable: say so, and offer to
//     choose one (the drive may just need reconnecting - Later leaves it);
//   - none saved (a new regatta) and this machine's last-used folder is
//     reachable: ask to confirm it for this regatta, or choose another;
//   - otherwise: offer to choose one.
//
// done (if any) runs when the flow ends either way - after onReady, or once
// the operator declines or cancels - so callers can offer the next kind.
func (r *Regatta) ensureResultsDir(kind string, onReady, done func()) {
	if r.resultsDir(kind) != common.EmptyString {
		call(onReady)
		call(done)
		return
	}
	label := kindLabel(kind)
	if saved := r.savedResultsDir(kind); saved != common.EmptyString {
		r.promptChooseResultsFolder(kind, fmt.Sprintf(common.ResultsFolderUnreachableFormat, label, saved), onReady, done)
		return
	}
	if last := r.App.Preferences().String(resultsDirPref(kind)); last != common.EmptyString && filesystem.DirExists(last) {
		r.confirmResultsFolder(kind, last, onReady, done)
		return
	}
	r.promptChooseResultsFolder(kind, fmt.Sprintf(common.ResultsFolderPromptFormat, label), onReady, done)
}

func call(f func()) {
	if f != nil {
		f()
	}
}

// confirmResultsFolder asks whether this regatta publishes kind to last -
// the folder this machine used most recently, quite possibly for a previous
// regatta. Only an explicit "Use This Folder" records it for this regatta.
func (r *Regatta) confirmResultsFolder(kind, last string, onReady, done func()) {
	label := kindLabel(kind)
	msg := fmt.Sprintf(common.ConfirmResultsFolderFormat, label, r.schedule.Name, r.schedule.Date, last)
	useBtn := widget.NewButton(common.UseThisFolderText, nil)
	useBtn.Importance = widget.HighImportance
	chooseBtn := widget.NewButton(common.ChooseAnotherFolderButtonText, nil)
	laterBtn := widget.NewButton(common.LaterButtonText, nil)

	d := dialog.NewCustomWithoutButtons(fmt.Sprintf(common.ResultsFolderPromptTitleFormat, label), widget.NewLabel(msg), r.window)
	d.SetButtons([]fyne.CanvasObject{laterBtn, chooseBtn, useBtn})
	useBtn.OnTapped = func() {
		d.Hide()
		if r.setResultsDir(kind, last) {
			call(onReady)
		}
		call(done)
	}
	chooseBtn.OnTapped = func() {
		d.Hide()
		r.pickResultsFolder(kind, onReady, done)
	}
	laterBtn.OnTapped = func() {
		d.Hide()
		call(done)
	}
	d.Show()
}

// promptChooseResultsFolder explains msg and offers the folder browser.
func (r *Regatta) promptChooseResultsFolder(kind, msg string, onReady, done func()) {
	d := dialog.NewConfirm(fmt.Sprintf(common.ResultsFolderPromptTitleFormat, kindLabel(kind)), msg, func(ok bool) {
		if ok {
			r.pickResultsFolder(kind, onReady, done)
			return
		}
		call(done)
	}, r.window)
	d.SetConfirmText(common.ChooseFolderButtonText)
	d.SetDismissText(common.LaterButtonText)
	d.Show()
}

// pickResultsFolder opens the folder browser (at kind's folder for this
// regatta, else this machine's last-used one), records the choice for this
// regatta, then runs onReady and done (either may be nil).
func (r *Regatta) pickResultsFolder(kind string, onReady, done func()) {
	r.browseResultsFolder(kind, r.resultsDir(kind), func(dir string) {
		if r.setResultsDir(kind, dir) {
			call(onReady)
		}
	}, done)
}

// browseResultsFolder opens the folder browser for kind at start (else this
// machine's last-used folder for kind) and passes the chosen folder to
// onPicked without recording it anywhere. done (if any) runs when the
// browser closes, whether or not a folder was chosen.
func (r *Regatta) browseResultsFolder(kind, start string, onPicked func(dir string), done func()) {
	fd := dialog.NewFolderOpen(func(dir fyne.ListableURI, err error) {
		defer call(done)
		if err != nil {
			dialog.ShowError(err, r.window)
			return
		}
		if dir == nil {
			return // cancelled
		}
		onPicked(filepath.FromSlash(dir.Path()))
	}, r.window)
	fd.SetTitleText(fmt.Sprintf(common.ResultsFolderTitleFormat, kindLabel(kind)))
	fd.SetConfirmText(common.UseThisFolderText)
	if start == common.EmptyString || !filesystem.DirExists(start) {
		start = r.App.Preferences().String(resultsDirPref(kind))
	}
	if start != common.EmptyString && filesystem.DirExists(start) {
		if location, err := storage.ListerForURI(storage.NewFileURI(start)); err == nil {
			fd.SetLocation(location)
		}
	}
	fd.Show()
}

// setResultsDir records dir as kind's folder for this regatta in finish.json
// (and as this machine's last-used folder for kind), and clears kind's
// published state - a different folder is a different publish history.
// Reports whether it was recorded; outside a publisher session (the
// Configuration screen before a PFT session starts) only the machine default
// is updated.
func (r *Regatta) setResultsDir(kind, dir string) bool {
	r.App.Preferences().SetString(resultsDirPref(kind), dir)
	if !r.isPublisher() || r.finishLog == nil {
		return false
	}
	if r.writesBlocked {
		r.showPublishError(common.ResultsFolderNotRecordedMessage)
		return false
	}
	prevDir, prevPublish := r.finishLog.ResultsDir, r.finishLog.Publish.Clone()
	r.finishLog.SetPublishDir(kind, dir)
	if err := store.SaveFinish(r.session, r.finishLog); err != nil {
		r.finishLog.ResultsDir, r.finishLog.Publish = prevDir, prevPublish
		applog.Error("results folder not recorded", "component", "publish", "kind", kind, "dir", dir, "error", err)
		r.showPublishError(common.ResultsFolderNotRecordedMessage)
		return false
	}
	applog.Info("results folder set", "component", "publish", "kind", kind, "dir", dir)
	r.applyLedger(kind, nil)
	r.refreshPublishSummary()
	return true
}

// loadPublishedLedgers reads every enabled destination's existing output in
// one background task (the published drive may be slow or remote) and
// applies the ledgers together, in one fyne.Do. Output belonging to another
// regatta contributes nothing - Publish will refuse it.
func (r *Regatta) loadPublishedLedgers() {
	type source struct {
		kind, path string
		dest       publish.Destination
	}
	var sources []source
	for _, kind := range r.folderKinds() {
		if path := r.resultsPath(kind); path != common.EmptyString {
			k, _ := destinations.ByID(kind)
			sources = append(sources, source{kind: kind, path: path, dest: k.New()})
		}
	}
	if len(sources) == 0 {
		return
	}
	key := r.regattaKey
	r.ledgerLoads.Go(func() {
		ledgers := make([]publish.Ledger, len(sources))
		for i, src := range sources {
			pub, err := src.dest.Read(src.path)
			if err == nil {
				err = pub.CheckRegatta(key)
			}
			if err != nil {
				applog.Warn(common.PublishLedgerUnreadableNote, "component", "publish", "kind", src.kind, "path", src.path, "error", err)
				continue
			}
			ledgers[i] = pub.Ledger
		}
		fyne.Do(func() {
			for i, src := range sources {
				r.setLedger(src.kind, ledgers[i])
			}
			r.refreshAllRows()
		})
	})
}

// applyLedger swaps in kind's published revisions and repaints the rows.
func (r *Regatta) applyLedger(kind string, ledger publish.Ledger) {
	r.setLedger(kind, ledger)
	r.refreshAllRows()
}

// setLedger records kind's published revisions without repainting.
func (r *Regatta) setLedger(kind string, ledger publish.Ledger) {
	revs := make(map[int]string, len(ledger))
	for n, e := range ledger {
		revs[n] = e.Revision
	}
	if r.publishedLedgers == nil {
		r.publishedLedgers = map[string]map[int]string{}
	}
	r.publishedLedgers[kind] = revs
}

// publishedRev is race n's published revision across every enabled, built
// destination: (rev, true) when all of them hold the same revision; ("",
// true) when some have published it but they disagree or one has not - so the
// button shows Re-publish, which back-fills the laggard; ("", false) when
// none has published it.
func (r *Regatta) publishedRev(n int) (string, bool) {
	rev, seen, agree := common.EmptyString, false, true
	for _, k := range r.enabledKinds() {
		if !k.Available {
			continue
		}
		v, ok := r.publishedLedgers[k.ID][n]
		switch {
		case !ok:
			agree = false
		case !seen:
			rev, seen = v, true
		case v != rev:
			agree = false
		}
	}
	if !seen {
		return common.EmptyString, false
	}
	if !agree {
		return common.EmptyString, true
	}
	return rev, true
}

// publishRace publishes race n to every enabled destination. A folder
// destination with no usable folder is asked for one first (one prompt at a
// time); one still without a folder is skipped and reported, never blocking
// the others.
func (r *Regatta) publishRace(n int) {
	if r.publishing || r.schedule == nil {
		return
	}
	var missing []string
	for _, kind := range r.folderKinds() {
		if r.resultsPath(kind) == common.EmptyString {
			missing = append(missing, kind)
		}
	}
	if len(missing) > 0 {
		r.ensureResultsDirs(missing, nil, func() { r.publishTargets(n) })
		return
	}
	r.publishTargets(n)
}

// publishTarget is one destination a publish writes, resolved on the UI
// goroutine.
type publishTarget struct {
	kind destinations.Kind
	path string
}

// publishTargets snapshots the schedule and approved results on the UI
// goroutine and writes each ready destination off it, independently: a
// locked workbook does not stop the web results going out. Every Publish
// button is disabled while one is in flight.
func (r *Regatta) publishTargets(n int) {
	if r.publishing || r.schedule == nil {
		return
	}
	kinds := r.enabledKinds()
	labelled := len(kinds) > 1
	var targets []publishTarget
	var problems []string
	for _, k := range kinds {
		switch {
		case !k.Available:
			problems = append(problems, fmt.Sprintf(common.PublishDestinationFormat, k.Label))
		case k.NeedsFolder && r.resultsPath(k.ID) == common.EmptyString:
			problems = append(problems, fmt.Sprintf(common.PublishNoFolderFormat, k.Label))
		default:
			targets = append(targets, publishTarget{kind: k, path: r.resultsPath(k.ID)})
		}
	}
	if len(targets) == 0 {
		if len(problems) > 0 {
			r.showPublishError(strings.Join(problems, "\n\n"))
		}
		return
	}

	meta := r.publishMeta()
	skeleton := publish.ScheduleView(r.schedule)
	approved := publish.BuildView(r.schedule, r.finishLog)
	now := time.Now()

	r.setPublishing(true)
	r.publishes.Go(func() {
		ledgers := make([]publish.Ledger, len(targets))
		errs := make([]error, len(targets))
		for i, t := range targets {
			ledgers[i], errs[i] = publishResults(t.kind.New(), t.path, n, meta, skeleton, approved, now)
		}
		fyne.Do(func() {
			for i, t := range targets {
				if errs[i] != nil {
					applog.Error("results publish failed", "component", "publish", "race", n, "kind", t.kind.ID, "path", t.path, "error", errs[i])
					msg := publishErrorMessage(t.path, errs[i])
					if labelled {
						msg = fmt.Sprintf(common.PublishPartialFormat, t.kind.Label, msg)
					}
					problems = append(problems, msg)
					continue
				}
				applog.Info("results published", "component", "publish", "race", n, "kind", t.kind.ID, "path", t.path)
				r.setLedger(t.kind.ID, ledgers[i])
			}
			r.setPublishing(false)
			if len(problems) > 0 {
				r.showPublishError(strings.Join(problems, "\n\n"))
			}
		})
	})
}

// publishResults reads dest's existing output at path, merges race n into
// what it already publishes (mergePublished), and regenerates it. Returns
// the ledger now in the file. Output belonging to another regatta
// (meta.RegattaKey) is refused, never merged into or overwritten.
func publishResults(dest publish.Destination, path string, n int, meta publish.Meta, skeleton, approved []publish.PublishableRace, now time.Time) (publish.Ledger, error) {
	prev, err := dest.Read(path)
	if err != nil {
		return nil, err
	}
	if err := prev.CheckRegatta(meta.RegattaKey); err != nil {
		return nil, err
	}
	races, ledger := mergePublished(n, skeleton, approved, prev, now)
	if err := dest.Write(path, meta, races, ledger); err != nil {
		return nil, err
	}
	return ledger, nil
}

// mergePublished decides a regenerated output's contents. Every scheduled
// race gets its block; results are filled for race n plus every race already
// in the ledger:
//   - still approved: rendered from the current result, so a correction made
//     since it was published goes out too (its ledger entry is re-stamped only
//     when its Revision actually moved);
//   - no longer approved: its previously published rows are carried forward
//     from the old workbook, so an un-approve never silently erases a result
//     the public has already seen.
//
// A ledger race that is no longer on the schedule has no block and drops out.
func mergePublished(n int, skeleton, approved []publish.PublishableRace, prev publish.Published, now time.Time) ([]publish.PublishableRace, publish.Ledger) {
	approvedBy := make(map[int]publish.PublishableRace, len(approved))
	for _, pr := range approved {
		approvedBy[pr.RaceNumber] = pr
	}

	races := append([]publish.PublishableRace(nil), skeleton...)
	ledger := publish.Ledger{}
	for i, race := range races {
		num := race.RaceNumber
		old, wasPublished := prev.Ledger[num]
		if num != n && !wasPublished {
			continue
		}
		if cur, ok := approvedBy[num]; ok {
			races[i] = cur
			if !wasPublished || old.Revision != cur.Revision {
				old = publish.LedgerEntry{Revision: cur.Revision, PublishedAt: now}
			}
			ledger[num] = old
			continue
		}
		if wasPublished {
			races[i].Rows = prev.Rows[num]
			ledger[num] = old
		}
	}
	return races, ledger
}

// publishErrorMessage turns a publish failure into operator-facing copy.
func publishErrorMessage(path string, err error) string {
	switch {
	case errors.Is(err, publish.ErrLocked):
		return fmt.Sprintf(common.PublishLockedFormat, path)
	case errors.Is(err, publish.ErrForeign):
		return fmt.Sprintf(common.PublishForeignFormat, path, common.AppTitle)
	case errors.Is(err, publish.ErrOtherRegatta):
		return fmt.Sprintf(common.PublishOtherRegattaFormat, path)
	default:
		return err.Error()
	}
}

func (r *Regatta) showPublishError(msg string) {
	dialog.NewInformation(common.PublishFailedTitle, msg, r.window).Show()
}

func (r *Regatta) setPublishing(on bool) {
	r.publishing = on
	r.refreshAllRows()
}

// publishState is what a publisher's row button shows for one race.
type publishState struct {
	label   string
	enabled bool
	stale   bool // published, but the approved result has changed since
}

// derivePublishState maps a race's approval and published revision to its
// button: nothing to publish until approved (store.CanPublish); "Published"
// while the workbook matches the current result (still pressable, to force a
// rewrite); "Re-publish", highlighted, once the approved result has moved on.
func derivePublishState(canPublish bool, revision, publishedRev string, published bool) publishState {
	switch {
	case !canPublish && published:
		return publishState{label: common.PublishedButtonText}
	case !canPublish:
		return publishState{label: common.PublishButtonText}
	case !published:
		return publishState{label: common.PublishButtonText, enabled: true}
	case publishedRev == revision:
		return publishState{label: common.PublishedButtonText, enabled: true}
	default:
		return publishState{label: common.RepublishButtonText, enabled: true, stale: true}
	}
}

// refreshPublishButton repaints row's Publish button from memory only - no
// disk I/O on a row refresh.
func (r *Regatta) refreshPublishButton(row *raceRow, res store.RaceResult) {
	if row.publishBtn == nil {
		return
	}
	canPublish := store.CanPublish(res)
	revision := common.EmptyString
	if canPublish {
		revision = r.raceRevision(row.raceNumber, res)
	}
	publishedRev, published := r.publishedRev(row.raceNumber)
	st := derivePublishState(canPublish, revision, publishedRev, published)

	row.publishBtn.SetText(st.label)
	importance := widget.MediumImportance
	if st.stale {
		importance = widget.HighImportance
	}
	if row.publishBtn.Importance != importance {
		row.publishBtn.Importance = importance
		row.publishBtn.Refresh()
	}
	setEnabled(row.publishBtn, st.enabled && !r.publishing)
}

// raceRevision is race n's publish.Revision as it would be published now.
func (r *Regatta) raceRevision(n int, res store.RaceResult) string {
	views := publish.BuildView(r.schedule, &store.FinishLog{Races: map[int]store.RaceResult{n: res}})
	if len(views) != 1 {
		return common.EmptyString
	}
	return views[0].Revision
}
