package regatta

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/text"
	"github.com/comagnaw/regattaClock/internal/timesync"
	"github.com/comagnaw/regattaClock/internal/uitheme"
	"github.com/comagnaw/regattaClock/internal/watcher"
)

// timingTeams are the teams that can own a start.json + finish.json. Only the
// guard that runs before a schedule is replaced in place needs to consider both
// (replacing the schedule would orphan either team's data). The Regatta
// Director's progress tree reads the primary team only.
var timingTeams = []persona.Team{persona.TeamPrimary, persona.TeamSecondary}

// teamTiming is one team's mirrored start.json + finish.json, held read-only by
// the Regatta Director.
type teamTiming struct {
	start  *store.StartLog
	finish *store.FinishLog
}

// directorTeamSession builds a persona.Session for path construction only - the
// path helpers key off Team, not the (unset) Role.
func directorTeamSession(root string, team persona.Team) persona.Session {
	return persona.Session{Definition: persona.Definition{Team: team}, Root: root}
}

// hydrateDirectorLogs loads the primary team's start.json and finish.json under
// the peer rules (missing is normal, unreadable is a warning, a different
// regatta is ignored). Called once before the RD tree is first shown. The RD
// tree shows primary-team values only; the secondary pair reconciles into the
// primary finish.json via the PFT (reconciliation.md).
func (r *Regatta) hydrateDirectorLogs(root, key string) {
	s := directorTeamSession(root, persona.TeamPrimary)
	r.teamLogs = map[persona.Team]*teamTiming{
		persona.TeamPrimary: {
			start:  r.hydratePeerStart(s, key),
			finish: r.hydratePeerFinish(s, key),
		},
	}
}

// refreshDirectorRow fills the read-only progress columns for one race from the
// primary team's timing files. A race the primary pair has not started shows
// placeholders until the PFT reconciles the secondary numbers in
// (reconciliation.md).
func (r *Regatta) refreshDirectorRow(row *raceRow) {
	n := row.raceNumber

	restarts, start := r.directorStartCells(n)
	row.restarts.SetText(restarts)
	row.startTime.SetText(start)

	win, status := r.directorFinishCells(n)
	row.winTime.SetText(win)
	row.approved.SetText(status)
}

// directorStartCells returns the restart count and start-time text for race n
// from the primary team's start.json.
func (r *Regatta) directorStartCells(n int) (restarts, start string) {
	tt := r.teamLogs[persona.TeamPrimary]
	if tt == nil || tt.start == nil {
		return common.NoStartTimeText, common.NoStartTimeText
	}
	rec, ok := tt.start.Races[n]
	if !ok || (rec.StartedAt == nil && len(rec.Cleared) == 0) {
		return common.NoStartTimeText, common.NoStartTimeText
	}
	start = common.NoStartTimeText
	if rec.StartedAt != nil {
		start = rec.Display
	}
	return strconv.Itoa(len(rec.Cleared)), start
}

// directorFinishCells returns the winning-time and status text for race n from
// the primary team's finish.json.
func (r *Regatta) directorFinishCells(n int) (win, status string) {
	tt := r.teamLogs[persona.TeamPrimary]
	if tt == nil || tt.finish == nil {
		return common.NoStartTimeText, common.EmptyString
	}
	res, ok := tt.finish.Races[n]
	if !ok || (res.WinningTime == common.EmptyString && !res.Approved && res.FirstFinishAt == nil) {
		return common.NoStartTimeText, common.EmptyString
	}
	win = common.NoStartTimeText
	if res.WinningTime != common.EmptyString {
		win = res.WinningTime
	}
	return win, raceProgressStatus(res)
}

// --- watcher plumbing -----------------------------------------------------

// directorWatchPaths are the primary team's start.json + finish.json, which the
// RD mirrors in addition to the schedule.
func directorWatchPaths(root string) []string {
	ts := directorTeamSession(root, persona.TeamPrimary)
	return []string{ts.StartPath(), ts.FinishPath()}
}

// applyDirectorTimingEvent routes a changed primary start.json / finish.json
// into the mirror. Runs on the watcher goroutine.
func (r *Regatta) applyDirectorTimingEvent(ev watcher.Event) {
	ts := directorTeamSession(r.session.Root, persona.TeamPrimary)
	switch ev.Path {
	case ts.StartPath():
		var log store.StartLog
		if err := json.Unmarshal(ev.Data, &log); err != nil {
			applog.Warn("watched start.json did not parse", "component", "race_tree", "err", err)
			return
		}
		if !r.matchesRegatta(log.RegattaKey) {
			applog.Warn("watched start.json is a different regatta; ignored", "component", "race_tree")
			return
		}
		if log.Races == nil {
			log.Races = map[int]store.StartRecord{}
		}
		applog.Info("director start times updated", "component", "race_tree", "races", len(log.Races))
		fyne.Do(func() { r.onDirectorTeamChanged(persona.TeamPrimary, &log, nil) })
	case ts.FinishPath():
		var log store.FinishLog
		if err := json.Unmarshal(ev.Data, &log); err != nil {
			applog.Warn("watched finish.json did not parse", "component", "race_tree", "err", err)
			return
		}
		if !r.matchesRegatta(log.RegattaKey) {
			applog.Warn("watched finish.json is a different regatta; ignored", "component", "race_tree")
			return
		}
		if log.Races == nil {
			log.Races = map[int]store.RaceResult{}
		}
		applog.Info("director finish progress updated", "component", "race_tree", "races", len(log.Races))
		fyne.Do(func() { r.onDirectorTeamChanged(persona.TeamPrimary, nil, &log) })
	}
}

func (r *Regatta) matchesRegatta(key string) bool {
	return key == common.EmptyString || key == r.regattaKey
}

// onDirectorTeamChanged swaps in one team's fresh log and repaints the rows and
// banners. Runs on the UI thread.
func (r *Regatta) onDirectorTeamChanged(team persona.Team, start *store.StartLog, finish *store.FinishLog) {
	tt := r.teamLogs[team]
	if tt == nil {
		tt = &teamTiming{}
		r.teamLogs[team] = tt
	}
	if start != nil {
		tt.start = start
	}
	if finish != nil {
		tt.finish = finish
	}
	r.refreshAllRows()
	r.checkDirectorSkew()
	r.checkDirectorStale()
}

// --- banners ------------------------------------------------------------

// directorHeaderExtras is the RD-only block under the column headers: an
// origin-change action banner, a dismissible clock-skew banner and a dismissible
// staleness banner. All three are hidden until they apply, so the header stays
// compact.
func (r *Regatta) directorHeaderExtras() fyne.CanvasObject {
	r.directorSkew = newDismissibleBanner()
	r.directorStale = newDismissibleBanner()
	r.originBanner = newActionBanner(common.ApplyButtonText, r.applyPendingOrigin, r.dismissOrigin)

	r.checkDirectorSkew()
	r.checkDirectorStale()

	return container.NewVBox(
		r.originBanner.root,
		r.directorSkew.root,
		r.directorStale.root,
	)
}

// checkDirectorSkew shows the skew banner when the widest gap between any two
// measured machine offsets across the primary team's timing files exceeds
// timesync.SkewWarnThreshold (persona-plan.md 2.1).
func (r *Regatta) checkDirectorSkew() {
	if r.directorSkew == nil {
		return
	}

	type measured struct {
		name string
		off  time.Duration
	}
	var seen []measured
	add := func(e store.Envelope) {
		if e.Machine == common.EmptyString || !strings.HasPrefix(e.Clock.Source, "ntp:") {
			return
		}
		seen = append(seen, measured{e.Machine, e.Clock.Offset})
	}
	if tt := r.teamLogs[persona.TeamPrimary]; tt != nil {
		if tt.start != nil {
			add(tt.start.Envelope)
		}
		if tt.finish != nil {
			add(tt.finish.Envelope)
		}
	}
	if len(seen) < 2 {
		r.directorSkew.hide()
		return
	}

	lo, hi := seen[0], seen[0]
	for _, m := range seen {
		if m.off < lo.off {
			lo = m
		}
		if m.off > hi.off {
			hi = m
		}
	}
	delta := hi.off - lo.off
	if delta <= timesync.SkewWarnThreshold || hi.name == lo.name {
		r.directorSkew.hide()
		return
	}
	r.directorSkew.show(fmt.Sprintf(common.DirectorSkewBannerFormat,
		hi.name, lo.name, fmt.Sprintf("%.1fs", delta.Seconds())))
}

// checkDirectorStale shows the staleness banner when the freshest of the
// primary team's timing files was written more than directorStaleThreshold ago.
func (r *Regatta) checkDirectorStale() {
	if r.directorStale == nil {
		return
	}

	var newest time.Time
	if tt := r.teamLogs[persona.TeamPrimary]; tt != nil {
		if tt.start != nil && tt.start.WrittenAt.After(newest) {
			newest = tt.start.WrittenAt
		}
		if tt.finish != nil && tt.finish.WrittenAt.After(newest) {
			newest = tt.finish.WrittenAt
		}
	}
	if newest.IsZero() {
		r.directorStale.hide() // nothing written yet is a fresh regatta, not a stall
		return
	}
	age := time.Since(newest)
	if age < directorStaleThreshold {
		r.directorStale.hide()
		return
	}
	r.directorStale.show(fmt.Sprintf(common.DirectorStaleBannerFormat, age.Round(time.Minute)))
}

// staleTicker re-runs checkDirectorStale on an interval, since staleness is a
// wall-clock condition no file event will announce.
func (r *Regatta) staleTicker(stop <-chan struct{}) {
	t := time.NewTicker(directorStaleInterval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			fyne.Do(r.checkDirectorStale)
		}
	}
}

// bannerRoot - the one styling path for every notice strip in the race-tree
// header (dismissibleBanner, actionBanner, the timer schedule banner): the
// shared amber caution strip, returned hidden like the strips it wraps.
func bannerRoot(inner fyne.CanvasObject) *fyne.Container {
	root := uitheme.CautionStrip(inner)
	root.Hide()
	return root
}

// dismissibleBanner - a hidden-by-default caution strip with a Dismiss button
// that hides it for good.
type dismissibleBanner struct {
	root      *fyne.Container
	label     *widget.Label
	dismiss   *widget.Button
	dismissed bool
}

func newDismissibleBanner() *dismissibleBanner {
	b := &dismissibleBanner{label: text.Wrapping(common.EmptyString)}
	b.dismiss = widget.NewButton(common.DismissButtonText, func() {
		b.dismissed = true
		b.root.Hide()
	})
	b.root = bannerRoot(container.NewBorder(nil, nil, nil, b.dismiss, b.label))
	return b
}

func (b *dismissibleBanner) show(text string) {
	if b.dismissed {
		return
	}
	b.label.SetText(text)
	b.root.Show()
}

func (b *dismissibleBanner) hide() { b.root.Hide() }

// actionBanner - a hidden-by-default caution strip with a primary action button
// and a Dismiss button. Unlike dismissibleBanner, Dismiss only hides it (the
// caller decides whether the same content should re-show).
type actionBanner struct {
	root  *fyne.Container
	label *widget.Label
}

func newActionBanner(actionText string, action, dismiss func()) *actionBanner {
	b := &actionBanner{label: text.Wrapping(common.EmptyString)}
	buttons := container.NewHBox(
		widget.NewButton(actionText, action),
		widget.NewButton(common.DismissButtonText, dismiss),
	)
	b.root = bannerRoot(container.NewBorder(nil, nil, nil, buttons, b.label))
	return b
}

func (b *actionBanner) show(text string) {
	b.label.SetText(text)
	b.root.Show()
}

func (b *actionBanner) hide() { b.root.Hide() }
