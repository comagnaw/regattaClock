package regatta

import (
	"errors"
	"fmt"
	"path/filepath"
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
	"github.com/comagnaw/regattaClock/internal/publish/spreadsheet"
)

// Results publishing (results-publisher.md): the Primary Finish Timer's
// per-race Publish button regenerates the RegattaClock-owned results
// workbook in this regatta's results folder. Nothing new is written to
// regattaData beyond that folder's path: the workbook is rendered from
// finish.json + the schedule via internal/publish, and its own very-hidden
// ledger sheet is the record of what has been published.
//
// The folder is regatta-scoped, not just a machine setting: it is recorded
// in finish.json (store.FinishLog.ResultsDir) once the PFT confirms it. A new
// regatta starts with a fresh finish.json, so it has no folder until the PFT
// confirms one - never silently the previous regatta's. common.PrefResultsDir
// only remembers the last folder used on this machine, to offer as the
// default in that confirmation.

// isPublisher reports whether this session owns the Publish button - only
// the primary finish timer, whose Approved result is the official one.
func (r *Regatta) isPublisher() bool {
	return r.session.Role == persona.RoleFinish && r.session.Team == persona.TeamPrimary
}

// savedResultsDir is the folder confirmed for this regatta (finish.json), or
// "" when none has been confirmed yet.
func (r *Regatta) savedResultsDir() string {
	if r.finishLog == nil {
		return common.EmptyString
	}
	return r.finishLog.ResultsDir
}

// resultsDir is this regatta's confirmed results folder, or "" when none is
// confirmed or it is no longer reachable (an unplugged drive, a renamed share).
func (r *Regatta) resultsDir() string {
	dir := r.savedResultsDir()
	if dir == common.EmptyString || !filesystem.DirExists(dir) {
		return common.EmptyString
	}
	return dir
}

// resultsPath is this regatta's results workbook, or "" with no usable folder.
func (r *Regatta) resultsPath() string {
	dir := r.resultsDir()
	if dir == common.EmptyString || r.schedule == nil {
		return common.EmptyString
	}
	return filepath.Join(dir, spreadsheet.FileName(r.schedule.Name, r.schedule.Date))
}

// startPublishing runs once a publisher's session is on screen: load what the
// existing workbook says is published, or - with no usable folder for this
// regatta - ask for one. Declining is fine; timing never depends on it, and
// Publish asks again.
func (r *Regatta) startPublishing() {
	if r.isPublisher() {
		r.ensureResultsDir(r.loadPublishedLedger)
	}
}

// ensureResultsDir runs onReady once this regatta has a reachable results
// folder, first asking for one when it does not:
//   - a folder saved for this regatta but unreachable: say so, and offer to
//     choose one (the drive may just need reconnecting - Later leaves it);
//   - none saved (a new regatta) and this machine's last-used folder is
//     reachable: ask to confirm it for this regatta, or choose another;
//   - otherwise: offer to choose one.
func (r *Regatta) ensureResultsDir(onReady func()) {
	if r.resultsDir() != common.EmptyString {
		onReady()
		return
	}
	if saved := r.savedResultsDir(); saved != common.EmptyString {
		r.promptChooseResultsFolder(fmt.Sprintf(common.ResultsFolderUnreachableFormat, saved), onReady)
		return
	}
	if last := r.App.Preferences().String(common.PrefResultsDir); last != common.EmptyString && filesystem.DirExists(last) {
		r.confirmResultsFolder(last, onReady)
		return
	}
	r.promptChooseResultsFolder(common.ResultsFolderPromptMessage, onReady)
}

// confirmResultsFolder asks whether this regatta publishes to last - the
// folder this machine used most recently, quite possibly for a previous
// regatta. Only an explicit "Use This Folder" records it for this regatta.
func (r *Regatta) confirmResultsFolder(last string, onReady func()) {
	msg := fmt.Sprintf(common.ConfirmResultsFolderFormat, r.schedule.Name, r.schedule.Date, last)
	useBtn := widget.NewButton(common.UseThisFolderText, nil)
	useBtn.Importance = widget.HighImportance
	chooseBtn := widget.NewButton(common.ChooseAnotherFolderButtonText, nil)
	laterBtn := widget.NewButton(common.LaterButtonText, nil)

	d := dialog.NewCustomWithoutButtons(common.ResultsFolderPromptTitle, widget.NewLabel(msg), r.window)
	d.SetButtons([]fyne.CanvasObject{laterBtn, chooseBtn, useBtn})
	useBtn.OnTapped = func() {
		d.Hide()
		if r.setResultsDir(last) {
			onReady()
		}
	}
	chooseBtn.OnTapped = func() {
		d.Hide()
		r.pickResultsFolder(onReady)
	}
	laterBtn.OnTapped = d.Hide
	d.Show()
}

// promptChooseResultsFolder explains msg and offers the folder browser.
func (r *Regatta) promptChooseResultsFolder(msg string, onReady func()) {
	d := dialog.NewConfirm(common.ResultsFolderPromptTitle, msg, func(ok bool) {
		if ok {
			r.pickResultsFolder(onReady)
		}
	}, r.window)
	d.SetConfirmText(common.ChooseFolderButtonText)
	d.SetDismissText(common.LaterButtonText)
	d.Show()
}

// pickResultsFolder opens the folder browser (at this regatta's folder, else
// this machine's last-used one), records the choice for this regatta, then
// runs onReady (if any).
func (r *Regatta) pickResultsFolder(onReady func()) {
	fd := dialog.NewFolderOpen(func(dir fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, r.window)
			return
		}
		if dir == nil {
			return // cancelled
		}
		if r.setResultsDir(filepath.FromSlash(dir.Path())) && onReady != nil {
			onReady()
		}
	}, r.window)
	fd.SetTitleText(common.ResultsFolderTitle)
	fd.SetConfirmText(common.UseThisFolderText)
	start := r.resultsDir()
	if start == common.EmptyString {
		start = r.App.Preferences().String(common.PrefResultsDir)
	}
	if start != common.EmptyString && filesystem.DirExists(start) {
		if location, err := storage.ListerForURI(storage.NewFileURI(start)); err == nil {
			fd.SetLocation(location)
		}
	}
	fd.Show()
}

// setResultsDir records dir as this regatta's results folder in finish.json
// (and as this machine's last-used folder), and clears the published state -
// a different folder is a different publish history. Reports whether it was
// recorded; outside a publisher session (the Configuration screen before a
// PFT session starts) only the machine default is updated.
func (r *Regatta) setResultsDir(dir string) bool {
	r.App.Preferences().SetString(common.PrefResultsDir, dir)
	if !r.isPublisher() || r.finishLog == nil {
		return false
	}
	if r.writesBlocked {
		r.showPublishError(common.ResultsFolderNotRecordedMessage)
		return false
	}
	prev := r.finishLog.ResultsDir
	r.finishLog.ResultsDir = dir
	if err := store.SaveFinish(r.session, r.finishLog); err != nil {
		r.finishLog.ResultsDir = prev
		applog.Error("results folder not recorded", "component", "publish", "dir", dir, "error", err)
		r.showPublishError(common.ResultsFolderNotRecordedMessage)
		return false
	}
	applog.Info("results folder set", "component", "publish", "dir", dir)
	r.applyLedger(nil)
	return true
}

// loadPublishedLedger reads the results workbook off the UI goroutine (the
// published drive may be slow or remote) and applies its ledger. A workbook
// belonging to another regatta contributes nothing - Publish will refuse it.
func (r *Regatta) loadPublishedLedger() {
	path := r.resultsPath()
	if path == common.EmptyString {
		return
	}
	key := r.regattaKey
	go func() {
		pub, err := spreadsheet.Read(path)
		if err == nil {
			err = pub.CheckRegatta(key)
		}
		if err != nil {
			applog.Warn(common.PublishLedgerUnreadableNote, "component", "publish", "path", path, "error", err)
			pub.Ledger = nil
		}
		fyne.Do(func() { r.applyLedger(pub.Ledger) })
	}()
}

// applyLedger swaps in the published revisions and repaints the rows.
func (r *Regatta) applyLedger(ledger spreadsheet.Ledger) {
	r.publishedRevs = make(map[int]string, len(ledger))
	for n, e := range ledger {
		r.publishedRevs[n] = e.Revision
	}
	r.refreshAllRows()
}

// publishRace publishes race n: the schedule and approved results are
// snapshotted here on the UI goroutine, and the workbook read + write runs
// off it. Every Publish button is disabled while one is in flight.
func (r *Regatta) publishRace(n int) {
	if r.publishing || r.schedule == nil {
		return
	}
	if dest := r.schedule.Publish.Destination(); dest != store.DestinationSpreadsheet {
		r.showPublishError(fmt.Sprintf(common.PublishDestinationFormat, dest))
		return
	}
	path := r.resultsPath()
	if path == common.EmptyString {
		r.ensureResultsDir(func() { r.publishRace(n) })
		return
	}

	meta := spreadsheet.Meta{Name: r.schedule.Name, Date: r.schedule.Date, RegattaKey: r.regattaKey}
	skeleton := publish.ScheduleView(r.schedule)
	approved := publish.BuildView(r.schedule, r.finishLog)

	r.setPublishing(true)
	go func() {
		ledger, err := publishResults(path, n, meta, skeleton, approved, time.Now())
		fyne.Do(func() {
			r.setPublishing(false)
			if err != nil {
				applog.Error("results publish failed", "component", "publish", "race", n, "path", path, "error", err)
				r.showPublishError(publishErrorMessage(path, err))
				return
			}
			applog.Info("results published", "component", "publish", "race", n, "path", path)
			r.applyLedger(ledger)
		})
	}()
}

// publishResults reads the existing workbook at path, merges race n into
// what it already publishes (mergePublished), and regenerates it. Returns
// the ledger now in the file. A workbook belonging to another regatta
// (meta.RegattaKey) is refused, never merged into or overwritten.
func publishResults(path string, n int, meta spreadsheet.Meta, skeleton, approved []publish.PublishableRace, now time.Time) (spreadsheet.Ledger, error) {
	prev, err := spreadsheet.Read(path)
	if err != nil {
		return nil, err
	}
	if err := prev.CheckRegatta(meta.RegattaKey); err != nil {
		return nil, err
	}
	races, ledger := mergePublished(n, skeleton, approved, prev, now)
	if err := spreadsheet.Write(path, meta, races, ledger); err != nil {
		return nil, err
	}
	return ledger, nil
}

// mergePublished decides the regenerated workbook's contents. Every scheduled
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
func mergePublished(n int, skeleton, approved []publish.PublishableRace, prev spreadsheet.Published, now time.Time) ([]publish.PublishableRace, spreadsheet.Ledger) {
	approvedBy := make(map[int]publish.PublishableRace, len(approved))
	for _, pr := range approved {
		approvedBy[pr.RaceNumber] = pr
	}

	races := append([]publish.PublishableRace(nil), skeleton...)
	ledger := spreadsheet.Ledger{}
	for i, race := range races {
		num := race.RaceNumber
		old, wasPublished := prev.Ledger[num]
		if num != n && !wasPublished {
			continue
		}
		if cur, ok := approvedBy[num]; ok {
			races[i] = cur
			if !wasPublished || old.Revision != cur.Revision {
				old = spreadsheet.LedgerEntry{Revision: cur.Revision, PublishedAt: now}
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
	case errors.Is(err, spreadsheet.ErrLocked):
		return fmt.Sprintf(common.PublishLockedFormat, path)
	case errors.Is(err, spreadsheet.ErrForeignWorkbook):
		return fmt.Sprintf(common.PublishForeignFormat, path, common.AppTitle)
	case errors.Is(err, spreadsheet.ErrOtherRegatta):
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
	publishedRev, published := r.publishedRevs[row.raceNumber]
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
