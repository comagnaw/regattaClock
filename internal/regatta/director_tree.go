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
	"github.com/comagnaw/regattaClock/internal/timesync"
	"github.com/comagnaw/regattaClock/internal/watcher"
)

// directorTeams is the read order for the per-value primary-then-secondary
// fallback (persona-plan.md 9).
var directorTeams = []persona.Team{persona.TeamPrimary, persona.TeamSecondary}

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

// hydrateDirectorLogs loads both teams' start.json and finish.json under the
// peer rules (missing is normal, unreadable is a warning, a different regatta is
// ignored). Called once before the RD tree is first shown.
func (r *Regatta) hydrateDirectorLogs(root, key string) {
	r.teamLogs = make(map[persona.Team]*teamTiming, len(directorTeams))
	for _, team := range directorTeams {
		s := directorTeamSession(root, team)
		r.teamLogs[team] = &teamTiming{
			start:  r.hydratePeerStart(s, key),
			finish: r.hydratePeerFinish(s, key),
		}
	}
}

// refreshDirectorRow fills the read-only progress columns for one race, taking
// each value from the primary team and falling back to the secondary per file
// (persona-plan.md 9). A value sourced from the secondary team is suffixed with
// common.SecondaryValueMark.
func (r *Regatta) refreshDirectorRow(row *raceRow) {
	n := row.raceNumber

	restarts, start, startSecondary := r.directorStartCells(n)
	row.restarts.SetText(secMark(restarts, startSecondary))
	row.startTime.SetText(secMark(start, startSecondary))

	win, status, finishSecondary := r.directorFinishCells(n)
	row.winTime.SetText(secMark(win, finishSecondary))
	row.approved.SetText(secMark(status, finishSecondary))
}

// directorStartCells returns the restart count and start-time text for race n
// from the first team whose start.json actually has that race, and whether that
// team was the secondary.
func (r *Regatta) directorStartCells(n int) (restarts, start string, secondary bool) {
	for _, team := range directorTeams {
		tt := r.teamLogs[team]
		if tt == nil || tt.start == nil {
			continue
		}
		rec, ok := tt.start.Races[n]
		if !ok || (rec.StartedAt == nil && len(rec.Cleared) == 0) {
			continue
		}
		start = common.NoStartTimeText
		if rec.StartedAt != nil {
			start = rec.Display
		}
		return strconv.Itoa(len(rec.Cleared)), start, team == persona.TeamSecondary
	}
	return common.NoStartTimeText, common.NoStartTimeText, false
}

// directorFinishCells returns the winning-time and status text for race n from
// the first team whose finish.json has begun that race, and whether that team
// was the secondary.
func (r *Regatta) directorFinishCells(n int) (win, status string, secondary bool) {
	for _, team := range directorTeams {
		tt := r.teamLogs[team]
		if tt == nil || tt.finish == nil {
			continue
		}
		res, ok := tt.finish.Races[n]
		if !ok || (res.WinningTime == common.EmptyString && !res.Approved && res.FirstFinishAt == nil) {
			continue
		}
		win = common.NoStartTimeText
		if res.WinningTime != common.EmptyString {
			win = res.WinningTime
		}
		switch {
		case res.Approved:
			status = common.RaceApprovedText
		case res.WinningTime != common.EmptyString:
			status = common.RaceSavedText
		default:
			status = common.RaceLockedTimingText // in progress: FirstFinishAt set
		}
		return win, status, team == persona.TeamSecondary
	}
	return common.NoStartTimeText, common.EmptyString, false
}

// secMark appends the secondary-team marker to a real value.
func secMark(v string, secondary bool) string {
	if secondary && v != common.EmptyString && v != common.NoStartTimeText {
		return v + common.SecondaryValueMark
	}
	return v
}

// --- watcher plumbing -----------------------------------------------------

// directorWatchPaths are the four team timing files the RD mirrors, in addition
// to the schedule.
func directorWatchPaths(root string) []string {
	paths := make([]string, 0, 2*len(directorTeams))
	for _, team := range directorTeams {
		ts := directorTeamSession(root, team)
		paths = append(paths, ts.StartPath(), ts.FinishPath())
	}
	return paths
}

// applyDirectorTimingEvent routes a changed team start.json / finish.json into
// the right mirror. Runs on the watcher goroutine.
func (r *Regatta) applyDirectorTimingEvent(ev watcher.Event) {
	for _, team := range directorTeams {
		ts := directorTeamSession(r.session.Root, team)
		switch ev.Path {
		case ts.StartPath():
			var log store.StartLog
			if err := json.Unmarshal(ev.Data, &log); err != nil {
				applog.Warn("watched start.json did not parse", "component", "race_tree",
					"team", string(team), "err", err)
				return
			}
			if !r.matchesRegatta(log.RegattaKey) {
				applog.Warn("watched start.json is a different regatta; ignored",
					"component", "race_tree", "team", string(team))
				return
			}
			if log.Races == nil {
				log.Races = map[int]store.StartRecord{}
			}
			applog.Info("director start times updated", "component", "race_tree",
				"team", string(team), "races", len(log.Races))
			fyne.Do(func() { r.onDirectorTeamChanged(team, &log, nil) })
			return
		case ts.FinishPath():
			var log store.FinishLog
			if err := json.Unmarshal(ev.Data, &log); err != nil {
				applog.Warn("watched finish.json did not parse", "component", "race_tree",
					"team", string(team), "err", err)
				return
			}
			if !r.matchesRegatta(log.RegattaKey) {
				applog.Warn("watched finish.json is a different regatta; ignored",
					"component", "race_tree", "team", string(team))
				return
			}
			if log.Races == nil {
				log.Races = map[int]store.RaceResult{}
			}
			applog.Info("director finish progress updated", "component", "race_tree",
				"team", string(team), "races", len(log.Races))
			fyne.Do(func() { r.onDirectorTeamChanged(team, nil, &log) })
			return
		}
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

// directorHeaderExtras is the RD-only header block under the column headers: a
// dismissible clock-skew banner, a dismissible staleness banner, and the
// secondary-value legend.
func (r *Regatta) directorHeaderExtras() fyne.CanvasObject {
	r.directorSkew = newDismissibleBanner()
	r.directorStale = newDismissibleBanner()
	legend := widget.NewLabel(common.SecondaryValueLegend)
	legend.TextStyle = fyne.TextStyle{Italic: true}

	r.checkDirectorSkew()
	r.checkDirectorStale()

	return container.NewVBox(r.directorSkew.root, r.directorStale.root, legend)
}

// checkDirectorSkew shows the skew banner when the widest gap between any two
// measured machine offsets across the four timing files exceeds
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
	for _, team := range directorTeams {
		if tt := r.teamLogs[team]; tt != nil {
			if tt.start != nil {
				add(tt.start.Envelope)
			}
			if tt.finish != nil {
				add(tt.finish.Envelope)
			}
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

// checkDirectorStale shows the staleness banner when the freshest of the four
// timing files was written more than directorStaleThreshold ago.
func (r *Regatta) checkDirectorStale() {
	if r.directorStale == nil {
		return
	}

	var newest time.Time
	for _, team := range directorTeams {
		if tt := r.teamLogs[team]; tt != nil {
			if tt.start != nil && tt.start.WrittenAt.After(newest) {
				newest = tt.start.WrittenAt
			}
			if tt.finish != nil && tt.finish.WrittenAt.After(newest) {
				newest = tt.finish.WrittenAt
			}
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

// dismissibleBanner - a hidden-by-default warning strip with a Dismiss button
// that hides it for good.
type dismissibleBanner struct {
	root      *fyne.Container
	label     *widget.Label
	dismissed bool
}

func newDismissibleBanner() *dismissibleBanner {
	b := &dismissibleBanner{label: widget.NewLabel(common.EmptyString)}
	b.label.Wrapping = fyne.TextWrapWord
	dismiss := widget.NewButton(common.DismissButtonText, func() {
		b.dismissed = true
		b.root.Hide()
	})
	b.root = container.NewBorder(nil, nil, nil, dismiss, b.label)
	b.root.Hide()
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
