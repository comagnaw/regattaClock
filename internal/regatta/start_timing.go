package regatta

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2/dialog"

	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/clock"
	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/timesync"
)

// recordStart captures the current corrected time as race n's start time and
// writes the whole start.json. It is a one-shot: once a race has a start time
// the button disables, and recording a different one means Clear then Start
// Time again.
func (r *Regatta) recordStart(n int) {
	if r.writesBlocked {
		r.warnWritesBlocked()
		return
	}
	if r.startLog.Races[n].StartedAt != nil {
		return // already recorded; Clear first to record a different time
	}

	now, ref := timesync.Now()
	captured := now

	rec := r.startLog.Races[n]
	rec.RaceNumber = n
	rec.StartedAt = &captured
	rec.Display = now.Format(common.StartTimeDisplayLayout)
	rec.Clock = ref
	r.startLog.Races[n] = rec

	if !r.persistStart() {
		return
	}
	applog.Info("start time recorded", "component", "race_tree", "action", "start_time",
		"race", n, "display", rec.Display, "offset_ms", ref.Offset.Milliseconds())
	r.refreshRow(n)
}

// clearStart prompts, then non-destructively clears race n's start time: the
// value moves into that race's Cleared history and StartedAt goes nil.
func (r *Regatta) clearStart(n int) {
	if r.startLog.Races[n].StartedAt == nil {
		return
	}
	dialog.ShowConfirm(common.ClearStartTitle, fmt.Sprintf(common.ClearStartMessage, n),
		func(yes bool) {
			if yes {
				r.clearStartConfirmed(n)
			}
		}, r.window)
}

func (r *Regatta) clearStartConfirmed(n int) {
	if r.writesBlocked {
		r.warnWritesBlocked()
		return
	}
	rec := r.startLog.Races[n]
	if rec.StartedAt == nil {
		return
	}

	rec.Cleared = append(rec.Cleared, store.ClearedStart{
		StartedAt: *rec.StartedAt,
		Display:   rec.Display,
		Clock:     rec.Clock,
		ClearedAt: time.Now().UTC(),
	})
	rec.TrimCleared()
	rec.StartedAt = nil
	rec.Display = common.EmptyString
	r.startLog.Races[n] = rec

	if !r.persistStart() {
		return
	}
	applog.Info("start time cleared", "component", "race_tree", "action", "clear", "race", n)
	r.refreshRow(n)
}

// restoreStart pops the most recently cleared start time back onto race n,
// together with the clock offset that was in force when it was captured. It
// always confirms first, naming the value being restored (and the current one
// it would replace, if a newer time was recorded since the clear).
func (r *Regatta) restoreStart(n int) {
	rec := r.startLog.Races[n]
	if len(rec.Cleared) == 0 {
		return
	}
	previous := rec.Cleared[len(rec.Cleared)-1].Display

	msg := fmt.Sprintf(common.RestoreStartPlainMessage, previous, n)
	if rec.StartedAt != nil {
		msg = fmt.Sprintf(common.RestoreStartMessage, rec.Display, previous, n)
	}

	dialog.ShowConfirm(common.RestoreStartTitle, msg, func(yes bool) {
		if yes {
			r.restoreStartConfirmed(n)
		}
	}, r.window)
}

func (r *Regatta) restoreStartConfirmed(n int) {
	if r.writesBlocked {
		r.warnWritesBlocked()
		return
	}
	rec := r.startLog.Races[n]
	if len(rec.Cleared) == 0 {
		return
	}

	last := rec.Cleared[len(rec.Cleared)-1]
	rec.Cleared = rec.Cleared[:len(rec.Cleared)-1]
	restored := last.StartedAt
	rec.StartedAt = &restored
	rec.Display = last.Display
	rec.Clock = last.Clock // the offset in force when this value was captured
	r.startLog.Races[n] = rec

	if !r.persistStart() {
		return
	}
	applog.Info("start time restored", "component", "race_tree", "action", "restore",
		"race", n, "display", rec.Display)
	r.refreshRow(n)
}

// persistStart stamps the envelope clock and atomically writes the whole
// StartLog. Returns false (and surfaces the error) on failure so the caller
// does not log success.
func (r *Regatta) persistStart() bool {
	r.startLog.Clock = timesync.Ref()
	if err := store.SaveStart(r.session, r.startLog); err != nil {
		applog.Error("start log write failed", "component", "race_tree", "err", err)
		dialog.ShowError(err, r.window)
		return false
	}
	return true
}

func (r *Regatta) warnWritesBlocked() {
	applog.Error("recording blocked; timing file needs recovery", "component", "race_tree")
	dialog.ShowInformation(common.CorruptTimingFileTitle, common.WritesBlockedMessage, r.window)
}

// openClock opens the race-timing window for a finish timer. Phase 7 threads the
// persona session into the clock; for now it takes the same arguments as before.
func (r *Regatta) openClock(n int) {
	race, ok := r.raceByNumber(n)
	if !ok {
		return
	}
	applog.Info("time race opened", "component", "race_tree", "action", "time_race", "race", n)
	clock.NewClock(r.App, r.RegattaData, race).OpenRaceClock()
}
