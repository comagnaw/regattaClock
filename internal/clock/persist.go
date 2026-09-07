package clock

import (
	"strconv"
	"time"

	"fyne.io/fyne/v2/dialog"

	"github.com/comagnaw/regattaClock/internal/applog"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/timesync"
)

// recordFirstFinish stamps the FT's clock-Start moment onto finish.json as an
// in-progress RaceResult. This is what engages the Start Timer lock for the
// race (persona-plan.md section 9). A write failure is logged but never blocks
// timing.
func (c *Clock) recordFirstFinish() {
	if !c.canPersist() {
		return
	}

	ff, ref := timesync.Now()
	n := c.raceData.RaceNumber

	res := c.finishLog.Races[n]
	res.RaceNumber = n
	if res.FirstFinishAt == nil {
		res.FirstFinishAt = &ff
		res.FirstFinishClock = ref
	}
	res.UpdatedAt = time.Now().UTC()
	c.setRace(n, res)

	if err := store.SaveFinish(c.session, c.finishLog); err != nil {
		applog.Error("finish log write failed", "component", "clock", "race", n, "err", err)
		return
	}
	applog.Info("clock started", "component", "clock", "race", n)
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
