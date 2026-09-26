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
// workbook in the operator's results folder (common.PrefResultsDir - a
// per-machine path, since the published drive is not regattaData). Nothing
// new is written to regattaData: the workbook is rendered from finish.json +
// the schedule via internal/publish, and its own very-hidden ledger sheet is
// the record of what has been published.

// isPublisher reports whether this session owns the Publish button - only
// the primary finish timer, whose Approved result is the official one.
func (r *Regatta) isPublisher() bool {
	return r.session.Role == persona.RoleFinish && r.session.Team == persona.TeamPrimary
}

// resultsDir is the configured results folder, or "" when unset or no longer
// reachable (an unplugged drive, a renamed share).
func (r *Regatta) resultsDir() string {
	dir := r.App.Preferences().String(common.PrefResultsDir)
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
	return filepath.Join(dir, spreadsheet.FileName(r.schedule.Name))
}

// startPublishing runs once a publisher's session is on screen: load what the
// existing workbook says is published, or - with no usable results folder -
// offer to choose one. Declining is fine; timing never depends on it, and
// Publish asks again.
func (r *Regatta) startPublishing() {
	if !r.isPublisher() {
		return
	}
	if r.resultsDir() != common.EmptyString {
		r.loadPublishedLedger()
		return
	}
	d := dialog.NewConfirm(common.ResultsFolderPromptTitle, common.ResultsFolderPromptMessage, func(ok bool) {
		if ok {
			r.pickResultsFolder(nil)
		}
	}, r.window)
	d.SetConfirmText(common.ChooseFolderButtonText)
	d.SetDismissText(common.LaterButtonText)
	d.Show()
}

// pickResultsFolder opens the folder browser (at the current results folder
// when there is one), remembers the choice, reloads the ledger from the
// workbook there, then runs then (if any).
func (r *Regatta) pickResultsFolder(then func()) {
	fd := dialog.NewFolderOpen(func(dir fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, r.window)
			return
		}
		if dir == nil {
			return // cancelled
		}
		r.setResultsDir(filepath.FromSlash(dir.Path()))
		if then != nil {
			then()
		}
	}, r.window)
	fd.SetTitleText(common.ResultsFolderTitle)
	fd.SetConfirmText(common.UseThisFolderText)
	if cur := r.resultsDir(); cur != common.EmptyString {
		if location, err := storage.ListerForURI(storage.NewFileURI(cur)); err == nil {
			fd.SetLocation(location)
		}
	}
	fd.Show()
}

// setResultsDir persists dir and re-reads the published state from the
// workbook there - a different folder is a different publish history.
func (r *Regatta) setResultsDir(dir string) {
	r.App.Preferences().SetString(common.PrefResultsDir, dir)
	applog.Info("results folder set", "component", "publish", "dir", dir)
	r.applyLedger(nil)
	r.loadPublishedLedger()
}

// loadPublishedLedger reads the results workbook off the UI goroutine (the
// published drive may be slow or remote) and applies its ledger.
func (r *Regatta) loadPublishedLedger() {
	path := r.resultsPath()
	if path == common.EmptyString {
		return
	}
	go func() {
		pub, err := spreadsheet.Read(path)
		if err != nil {
			applog.Warn(common.PublishLedgerUnreadableNote, "component", "publish", "path", path, "error", err)
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
		r.pickResultsFolder(func() { r.publishRace(n) })
		return
	}

	meta := spreadsheet.Meta{Name: r.schedule.Name, Date: r.schedule.Date}
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
// the ledger now in the file.
func publishResults(path string, n int, meta spreadsheet.Meta, skeleton, approved []publish.PublishableRace, now time.Time) (spreadsheet.Ledger, error) {
	prev, err := spreadsheet.Read(path)
	if err != nil {
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
