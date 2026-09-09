package regatta

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/comagnaw/regattaClock/internal/persona/store"
)

// TestOpenClockFanOutRecomputesWinningTime covers the 7b wiring: openClock
// registers the race window in r.openClocks, and a peer start time delivered by
// the watcher is pushed into that open clock, which recomputes and persists the
// derived winning time (persona-plan.md 2.2).
func TestOpenClockFanOutRecomputesWinningTime(t *testing.T) {
	app := test.NewTempApp(t)
	sch := twoRaceSchedule()
	root := seedRegatta(t, sch)
	key := store.RegattaKey(sch.Name, sch.Date)

	// The ST file exists but has no start time for race 1 yet.
	pst := timerSession(t, "pst", root)
	sl := &store.StartLog{Races: map[int]store.StartRecord{}}
	sl.RegattaKey = key
	if err := store.SaveStart(pst, sl); err != nil {
		t.Fatal(err)
	}

	pft := timerSession(t, "pft", root)
	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(pft, sch)

	// The FT has begun race 1 on the clock: an in-progress RaceResult exists.
	ff := time.Now().UTC()
	r.finishLog.Races[1] = store.RaceResult{RaceNumber: 1, FirstFinishAt: &ff}

	r.openClock(1)
	if r.openClocks[1] == nil {
		t.Fatal("openClock did not register the race clock")
	}
	if _, open := r.openClocks[2]; open {
		t.Error("only the opened race should be registered")
	}

	// The ST start time finally lands via the watcher.
	started := time.Now().UTC().Add(-5 * time.Minute)
	r.onPeerStartChanged(&store.StartLog{Races: map[int]store.StartRecord{
		1: {RaceNumber: 1, StartedAt: &started},
	}})

	onDisk, err := store.LoadFinish(pft)
	if err != nil {
		t.Fatalf("LoadFinish: %v", err)
	}
	if onDisk.Races[1].StartedAt == nil {
		t.Error("a late ST start time did not recompute the open clock's winning time")
	}
	if r.startLog.Races[1].StartedAt == nil {
		t.Error("onPeerStartChanged should also refresh the in-memory start mirror")
	}
}
