package clock

import (
	"fmt"
	"strconv"
	"time"

	"fyne.io/fyne/v2/dialog"

	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/timesync"
)

// maxPlausibleRace bounds the derived winning time. A value that is negative or
// longer than this means the ST start time is bad or stale, so the auto-fill is
// suppressed and manual entry stands (persona-plan.md 2.1).
const maxPlausibleRace = 30 * time.Minute

// minPlausibleRace is the shortest winning time that looks like a real race.
// Below this the value still pre-fills - it is the honest computation - but the
// note warns that the ST start and the first finish were only seconds apart,
// which usually means a back-to-back test rather than a race.
const minPlausibleRace = 30 * time.Second

// recordFirstFinish stamps the FT's clock-Start moment onto finish.json as an
// in-progress RaceResult. This is what engages the Start Timer lock for the
// race (persona-plan.md section 9). A write failure is logged but never blocks
// timing.
func (c *Clock) recordFirstFinish() {
	if !c.canPersist() {
		return
	}

	// Store the raw local timestamp and the offset separately - never a
	// pre-corrected time - so a bad NTP offset found later can be recomputed
	// away (persona-plan.md 2.1). .UTC() also strips the monotonic reading that
	// would otherwise be lost silently at marshal time.
	ff := time.Now().UTC()
	ref := timesync.Ref()
	n := c.raceData.RaceNumber

	res := c.finishLog.Races[n]
	res.RaceNumber = n
	if res.FirstFinishAt == nil {
		res.FirstFinishAt = &ff
		res.FirstFinishClock = ref
	}
	res.UpdatedAt = time.Now().UTC()
	c.setRace(n, res)

	// Pre-fill the winning time from the ST start time before the write, so the
	// audited ST side (StartedAt / StartedAtClock) lands in the same finish.json.
	c.deriveWinningTime()

	if err := store.SaveFinish(c.session, c.finishLog); err != nil {
		applog.Error("finish log write failed", "component", "clock", "race", n, "err", err)
		return
	}
	applog.Info("clock started", "component", "clock", "race", n)
}

// deriveWinningTime pre-fills winningTime with the finish timer's Start click
// minus the start timer's start time, each shifted by its own machine's measured
// offset (persona-plan.md 2.1). The field stays editable and a referee override
// is never touched. Returns true when it applied (or refreshed) a derived value.
//
//   - No FirstFinishAt: the FT has not started this race; nothing to derive.
//   - No ST start time yet: show the "waiting for start time" placeholder and
//     recompute when UpdateStartTime delivers one (persona-plan.md 2.2).
//   - Negative or implausibly long: the ST time is bad or stale; suppress the
//     auto-fill and leave manual entry.
func (c *Clock) deriveWinningTime() bool {
	if !c.canPersist() {
		return false
	}
	n := c.raceData.RaceNumber
	res, ok := c.finishLog.Races[n]
	if !ok || res.FirstFinishAt == nil {
		return false
	}

	rec, ok := c.startRecord(n)
	if !ok || rec.StartedAt == nil {
		c.awaitStartTime()
		return false
	}

	finish := res.FirstFinishClock.Corrected(*res.FirstFinishAt)
	start := rec.Clock.Corrected(*rec.StartedAt)
	wt := finish.Sub(start)

	applog.Info("winning time derived", "component", "clock", "race", n,
		"seconds", wt.Seconds(),
		"first_finish", finish.UTC().Format(time.RFC3339Nano),
		"start", start.UTC().Format(time.RFC3339Nano),
		"ft_offset_ms", res.FirstFinishClock.Offset.Milliseconds(),
		"st_offset_ms", rec.Clock.Offset.Milliseconds())

	c.awaitingStart = false
	c.checkSkew(rec.Clock, res.FirstFinishClock)

	switch {
	case wt <= 0:
		applog.Warn("derived winning time is negative; manual entry stands",
			"component", "clock", "race", n, "seconds", wt.Seconds())
		c.noteWinningTime(fmt.Sprintf(common.WinningTimeNegativeNote, (-wt).Round(time.Millisecond)))
		return false
	case wt > maxPlausibleRace:
		applog.Warn("derived winning time exceeds a plausible race; manual entry stands",
			"component", "clock", "race", n, "seconds", wt.Seconds())
		c.noteWinningTime(fmt.Sprintf(common.WinningTimeStaleNote, wt.Round(time.Minute)))
		return false
	}

	// Record the ST side actually used so the winning time can be audited or
	// recomputed later (store.RaceResult.StartedAt).
	started := *rec.StartedAt
	res.StartedAt = &started
	res.StartedAtClock = rec.Clock
	c.setRace(n, res)

	c.applyDerivedWinningTime(formatTime(wt))
	if wt < minPlausibleRace {
		c.noteWinningTime(fmt.Sprintf(common.WinningTimeTinyNote, wt.Round(100*time.Millisecond)))
	} else {
		c.noteWinningTime(common.WinningTimeDerivedNote)
	}
	return true
}

// noteWinningTime shows a helper line under the Winning Time field. An empty
// message hides it.
func (c *Clock) noteWinningTime(msg string) {
	if c.winningNote == nil {
		return
	}
	c.winningNote.SetText(msg)
	if msg == common.EmptyString {
		c.winningNote.Hide()
		return
	}
	c.winningNote.Show()
}

// UpdateStartTime replaces the peer start-time mirror and re-derives the winning
// time in place, unless the referee has typed one in. Call on the UI thread when
// the watcher reports a fresh start.json (persona-plan.md 2.2).
func (c *Clock) UpdateStartTime(log *store.StartLog) {
	c.startLog = log
	if !c.canPersist() {
		return
	}
	n := c.raceData.RaceNumber
	if res, ok := c.finishLog.Races[n]; !ok || res.FirstFinishAt == nil {
		return // this race is not being timed yet
	}

	// A manual entry is one that differs from what we last auto-filled. Leave it.
	if c.winningTime.Text != common.EmptyString && c.winningTime.Text != c.derivedWinningTime {
		return
	}

	if c.deriveWinningTime() {
		if err := store.SaveFinish(c.session, c.finishLog); err != nil {
			applog.Error("finish log write failed", "component", "clock", "race", n, "err", err)
		}
	}
}

// startRecord returns this race's ST start record from the mirrored start.json.
func (c *Clock) startRecord(n int) (store.StartRecord, bool) {
	if c.startLog == nil || c.startLog.Races == nil {
		return store.StartRecord{}, false
	}
	rec, ok := c.startLog.Races[n]
	return rec, ok
}

// applyDerivedWinningTime fills the field and remembers the value, so a later
// recompute can tell an untouched pre-fill from a referee's manual entry.
func (c *Clock) applyDerivedWinningTime(v string) {
	c.derivedWinningTime = v
	c.winningTime.SetText(v)
}

// awaitStartTime flags the race as waiting on the ST start time and shows the
// placeholder, without disturbing anything the operator may have typed.
func (c *Clock) awaitStartTime() {
	c.awaitingStart = true
	if c.winningTime.Text == common.EmptyString {
		c.winningTime.SetPlaceHolder(common.WaitingForStartTimeText)
		c.winningTime.Refresh()
	}
	c.noteWinningTime(common.WinningTimeWaitingNote)
}

// checkSkew shows the dismissible skew banner when the finish and start
// machines' measured offsets disagree by more than timesync.SkewWarnThreshold -
// every winning time in the regatta is then wrong by roughly that much
// (persona-plan.md 2.1).
func (c *Clock) checkSkew(stClock, ftClock timesync.ClockRef) {
	if c.skewBanner == nil || c.skewDismissed {
		return
	}
	delta := stClock.Offset - ftClock.Offset
	if delta < 0 {
		delta = -delta
	}
	if delta <= timesync.SkewWarnThreshold {
		c.skewBanner.Hide()
		return
	}

	stName := "start timer"
	if c.startLog != nil && c.startLog.Machine != common.EmptyString {
		stName = c.startLog.Machine
	}
	ftName := "finish timer"
	if c.finishLog != nil && c.finishLog.Machine != common.EmptyString {
		ftName = c.finishLog.Machine
	}
	c.skewLabel.SetText(fmt.Sprintf(common.ClockSkewBannerFormat,
		ftName, stName, fmt.Sprintf("%.1fs", delta.Seconds()),
		formatSkew(ftClock.Offset), formatSkew(stClock.Offset)))
	c.skewBanner.Show()
}

// formatSkew renders one machine's offset for the banner as a signed number of
// seconds, so "ahead" and "behind" read at a glance.
func formatSkew(d time.Duration) string {
	return fmt.Sprintf("%+.1fs", d.Seconds())
}

// persistFinish serializes the current lap rows and winning time into the race's
// RaceResult and writes the whole finish.json. Called from Referee Approval and
// Save (persona-plan.md section 9 - both perform the identical write).
func (c *Clock) persistFinish(approved bool) {
	if !c.canPersist() {
		return
	}
	n := c.raceData.RaceNumber
	now := time.Now().UTC()

	res := c.finishLog.Races[n]
	res.RaceNumber = n
	res.WinningTime = c.winningTime.Text
	res.Rows = c.serializeLapRows()
	res.Approved = approved
	if approved && res.ApprovedAt == nil {
		res.ApprovedAt = &now
	}
	res.UpdatedAt = now
	c.setRace(n, res)

	if err := store.SaveFinish(c.session, c.finishLog); err != nil {
		applog.Error("finish log write failed", "component", "clock", "race", n, "err", err)
		dialog.ShowError(err, c.window)
		return
	}
	applog.Info("race results saved", "component", "clock", "race", n,
		"approved", approved, "winning_time", res.WinningTime)
}

func (c *Clock) setRace(n int, res store.RaceResult) {
	if c.finishLog.Races == nil {
		c.finishLog.Races = map[int]store.RaceResult{}
	}
	c.finishLog.Races[n] = res
}

// serializeLapRows turns the six lap widgets into store.LapRow values, a
// one-to-one copy (persona-plan.md section 4).
func (c *Clock) serializeLapRows() []store.LapRow {
	rows := make([]store.LapRow, 0, len(c.laps))
	for i := range c.laps {
		lane := 0
		if n := getGoodLaneNum(c.laps.oofLaneNum(i)); n != badLaneNum {
			lane = n
		}
		rows = append(rows, store.LapRow{
			Lane:  lane,
			Place: c.laps.place(i),
			Split: c.laps.split(i),
			Time:  c.laps.calculatedTime(i),
		})
	}
	return rows
}

// rehydrate restores a saved race into the freshly built widgets: lap rows,
// winning time, and the referee / save button state. An in-progress record
// (Start clicked, nothing saved) has nothing visible to restore.
func (c *Clock) rehydrate() {
	if c.finishLog == nil {
		return
	}
	res, ok := c.finishLog.Races[c.raceData.RaceNumber]
	if !ok {
		return
	}

	restoredRows := 0
	for i, lr := range res.Rows {
		if i >= len(c.laps) {
			break
		}
		oof := ""
		if lr.Lane != 0 {
			oof = strconv.Itoa(lr.Lane)
		}
		c.laps.updateLap(i, lr.Place, lr.Split, lr.Time, oof)
		if lr.Split != "" || lr.Place != "" {
			restoredRows++
		}
	}

	if res.WinningTime != "" {
		c.winningTime.SetText(res.WinningTime)
		c.buttons.referee.Enable()
	}
	if res.Approved {
		c.buttons.save.Enable()
	}

	if restoredRows > 0 || res.WinningTime != "" {
		c.clockState.isCleared = false
		if restoredRows > c.lapCount {
			c.lapCount = restoredRows
		}
		c.refreshContent()
	}
}
