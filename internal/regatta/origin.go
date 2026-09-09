package regatta

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"

	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/filesystem"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/reader"
)

// originPoller re-hashes the source workbook every originPollInterval and, when
// the schedule *content* has changed (not just the file), raises the RD's
// origin banner. RD only; started from startWatcher, stops with the watcher
// context (persona-plan.md 3b).
func (r *Regatta) originPoller(uri, acceptedHash string, stop <-chan struct{}) {
	if uri == common.EmptyString {
		return // a migrated / originless schedule: nothing to poll
	}
	last := acceptedHash
	t := time.NewTicker(originPollInterval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			last = r.pollOrigin(uri, last)
		}
	}
}

// pollOrigin hashes uri; on a change it re-reads the workbook off the UI thread
// and hands the candidate to inspectOriginCandidate. Returns the file hash to
// carry forward (unchanged when the file could not be read - a partial save
// retries next tick).
func (r *Regatta) pollOrigin(uri, lastHash string) string {
	h, err := filesystem.FileHash(uri)
	if err != nil {
		applog.Debug("origin poll: hash failed", "component", "origin", "err", err)
		return lastHash
	}
	if h == lastHash {
		return lastHash
	}

	candidate, err := reader.ReadExcelFile(uri)
	if err != nil {
		// Locked or mid-save (Excel writes a temp then renames). Do not advance
		// the hash so the next tick tries again.
		applog.Debug("origin poll: re-read failed", "component", "origin", "err", err)
		return lastHash
	}

	fyne.Do(func() { r.inspectOriginCandidate(candidate) })
	return h
}

// inspectOriginCandidate compares a freshly parsed workbook to the live schedule
// by content hash. Unchanged content just clears the banner; a real change the
// RD has not already dismissed raises it. Runs on the UI thread.
func (r *Regatta) inspectOriginCandidate(candidate *reader.RegattaData) {
	if r.originBanner == nil {
		return
	}
	current := scheduleFromRegattaData(r.RegattaData)
	next := scheduleFromRegattaData(candidate)
	nextHash := next.ContentHash()

	if nextHash == current.ContentHash() {
		applog.Debug("origin touched, schedule unchanged", "component", "origin")
		r.pendingOrigin = nil
		r.originBanner.hide()
		return
	}
	if nextHash == r.dismissedContentHash {
		return // the RD already declined exactly this change
	}

	r.pendingOrigin = candidate
	summary := originChangeSummary(current, next)
	applog.Info("origin has schedule changes", "component", "origin", "summary", summary)
	r.originBanner.show(fmt.Sprintf(common.OriginChangedBannerFormat, summary))
}

// applyPendingOrigin publishes the pending workbook: it becomes the candidate
// for the RegattaKey guard, which writes regattaSchedule.json when clear to and
// re-enters the director flow so timers pick the change up via their watcher.
func (r *Regatta) applyPendingOrigin() {
	if r.pendingOrigin == nil {
		return
	}
	r.RegattaData = r.pendingOrigin
	r.pendingOrigin = nil
	r.dismissedContentHash = common.EmptyString
	r.originBanner.hide()

	r.guardScheduleWrite(func() {
		applog.Info("schedule origin applied", "component", "origin",
			"name", r.RegattaData.Name, "races", r.RegattaData.ScheduledRaces())
		r.startDirectorFlow()
	})
}

// dismissOrigin hides the banner and records the content hash so the poll does
// not re-raise it for the same change.
func (r *Regatta) dismissOrigin() {
	if r.pendingOrigin != nil {
		r.dismissedContentHash = scheduleFromRegattaData(r.pendingOrigin).ContentHash()
	}
	r.pendingOrigin = nil
	r.originBanner.hide()
}

// originChangeSummary is the one-line "what changed" for the banner. It reuses
// diffSchedule for touched races and adds race additions / removals.
func originChangeSummary(current, next *store.Schedule) string {
	cur := regattaDataFromSchedule(current)
	nxt := regattaDataFromSchedule(next)

	changed := diffSchedule(cur, nxt)
	scratches, moves := 0, 0
	for _, ch := range changed {
		if ch.scratch {
			scratches++
		}
		if ch.moved || ch.meta {
			moves++
		}
	}
	added, removed := raceSetDelta(cur, nxt)

	parts := make([]string, 0, 4)
	if n := len(changed); n > 0 {
		detail := make([]string, 0, 2)
		if scratches > 0 {
			detail = append(detail, fmt.Sprintf("%d scratch/restore", scratches))
		}
		if moves > 0 {
			detail = append(detail, fmt.Sprintf("%d lane/class", moves))
		}
		seg := fmt.Sprintf("%d race(s) changed", n)
		if len(detail) > 0 {
			seg += " (" + strings.Join(detail, ", ") + ")"
		}
		parts = append(parts, seg)
	}
	if added > 0 {
		parts = append(parts, fmt.Sprintf("%d added", added))
	}
	if removed > 0 {
		parts = append(parts, fmt.Sprintf("%d removed", removed))
	}
	if len(parts) == 0 {
		return "metadata only"
	}
	return strings.Join(parts, ", ")
}

// raceSetDelta counts races present in next but not current, and vice versa.
func raceSetDelta(current, next *reader.RegattaData) (added, removed int) {
	has := func(rd *reader.RegattaData) map[int]bool {
		m := map[int]bool{}
		for _, race := range rd.Races {
			m[race.RaceNumber] = true
		}
		return m
	}
	cur, nxt := has(current), has(next)
	for n := range nxt {
		if !cur[n] {
			added++
		}
	}
	for n := range cur {
		if !nxt[n] {
			removed++
		}
	}
	return added, removed
}
