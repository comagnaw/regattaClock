package regatta

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona/store"
)

func twoRaceSchedule() *store.Schedule {
	return &store.Schedule{
		Name: "Lock Test",
		Date: "2026-10-02",
		Races: []store.ScheduleRace{
			{RaceNumber: 1, BoatClass: "V8", BoatCount: 2, Lanes: map[int]store.ScheduleEntry{
				1: {SchoolName: "A"}, 2: {SchoolName: "B"},
			}},
			{RaceNumber: 2, BoatClass: "V4", BoatCount: 2, Lanes: map[int]store.ScheduleEntry{
				1: {SchoolName: "C"}, 2: {SchoolName: "D"},
			}},
		},
	}
}

// startTimerWithFinish seeds a schedule and a finish.json, then starts a Primary
// Start Timer session so hydratePeerFinish picks the finish record up.
func startTimerWithFinish(t *testing.T, finish *store.FinishLog) (*Regatta, *store.Schedule) {
	t.Helper()
	app := test.NewTempApp(t)
	sch := twoRaceSchedule()
	root := seedRegatta(t, sch)

	if finish != nil {
		pft := timerSession(t, "pft", root)
		finish.RegattaKey = store.RegattaKey(sch.Name, sch.Date)
		if err := store.SaveFinish(pft, finish); err != nil {
			t.Fatal(err)
		}
	}

	pst := timerSession(t, "pst", root)
	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(pst, sch)
	return r, sch
}

func ptr(tm time.Time) *time.Time { return &tm }

func TestStartRowLocksWhenFinishHasInProgressResult(t *testing.T) {
	r, _ := startTimerWithFinish(t, &store.FinishLog{Races: map[int]store.RaceResult{
		1: {RaceNumber: 1, FirstFinishAt: ptr(time.Now().UTC())},
	}})

	locked := r.rows[1]
	if !locked.startBtn.Disabled() || !locked.clearBtn.Disabled() || !locked.restoreBtn.Hidden {
		t.Errorf("race 1 buttons should be locked: start=%v clear=%v restoreHidden=%v",
			locked.startBtn.Disabled(), locked.clearBtn.Disabled(), locked.restoreBtn.Hidden)
	}
	if locked.progress.Text != common.RaceInProgressText {
		t.Errorf("race 1 note = %q, want %q", locked.progress.Text, common.RaceInProgressText)
	}

	free := r.rows[2]
	if free.startBtn.Disabled() {
		t.Error("race 2 (no finish record) should not be locked")
	}
	if free.progress.Text != "" {
		t.Errorf("race 2 note = %q, want empty", free.progress.Text)
	}
}

func TestStartRowStatusMatchesFinishProgress(t *testing.T) {
	// Saved but not approved: the ST row shows the shared "saved" status, still locked.
	r, _ := startTimerWithFinish(t, &store.FinishLog{Races: map[int]store.RaceResult{
		1: {RaceNumber: 1, FirstFinishAt: ptr(time.Now().UTC()), WinningTime: "06:00.0"},
	}})
	if got := r.rows[1].progress.Text; got != common.RaceSavedText {
		t.Errorf("saved race status = %q, want %q", got, common.RaceSavedText)
	}
	if !r.rows[1].startBtn.Disabled() {
		t.Error("a saved race must stay locked")
	}

	// Approved: the shared "approved" status, still locked.
	r2, _ := startTimerWithFinish(t, &store.FinishLog{Races: map[int]store.RaceResult{
		1: {RaceNumber: 1, FirstFinishAt: ptr(time.Now().UTC()), WinningTime: "06:00.0", Approved: true},
	}})
	if got := r2.rows[1].progress.Text; got != common.RaceApprovedText {
		t.Errorf("approved race status = %q, want %q", got, common.RaceApprovedText)
	}
	if !r2.rows[1].startBtn.Disabled() {
		t.Error("an approved race must stay locked")
	}
}

func TestOnPeerFinishChangedLocksRowLive(t *testing.T) {
	r, _ := startTimerWithFinish(t, nil) // no finish.json yet
	if r.rows[1].startBtn.Disabled() {
		t.Fatal("race 1 should start unlocked")
	}

	r.onPeerFinishChanged(&store.FinishLog{Races: map[int]store.RaceResult{
		1: {RaceNumber: 1, FirstFinishAt: ptr(time.Now().UTC())},
	}})

	if !r.rows[1].startBtn.Disabled() || r.rows[1].progress.Text != common.RaceInProgressText {
		t.Error("race 1 should lock when the watcher delivers a finish record")
	}
}

func TestLockedRaceRejectsMutators(t *testing.T) {
	r, _ := startTimerWithFinish(t, nil)

	// Give race 1 a start time, then lock it.
	r.startLog.Races[1] = store.StartRecord{RaceNumber: 1, StartedAt: ptr(time.Now().UTC()), Display: "09:00:00.0"}
	r.onPeerFinishChanged(&store.FinishLog{Races: map[int]store.RaceResult{
		1: {RaceNumber: 1, FirstFinishAt: ptr(time.Now().UTC())},
	}})

	r.recordStart(1)
	r.clearStartConfirmed(1)
	if r.startLog.Races[1].StartedAt == nil {
		t.Error("clearStartConfirmed must no-op on a race locked by the finish timer")
	}

	r.startLog.Races[1] = store.StartRecord{RaceNumber: 1, Cleared: []store.ClearedStart{{Display: "x"}}}
	r.restoreStartConfirmed(1)
	if r.startLog.Races[1].StartedAt != nil {
		t.Error("restoreStartConfirmed must no-op on a locked race")
	}
}

func TestPeerFinishWrongRegattaIgnored(t *testing.T) {
	app := test.NewTempApp(t)
	sch := twoRaceSchedule()
	root := seedRegatta(t, sch)

	pft := timerSession(t, "pft", root)
	fl := &store.FinishLog{Races: map[int]store.RaceResult{1: {RaceNumber: 1, FirstFinishAt: ptr(time.Now().UTC())}}}
	fl.RegattaKey = "some-other-regatta"
	if err := store.SaveFinish(pft, fl); err != nil {
		t.Fatal(err)
	}

	pst := timerSession(t, "pst", root)
	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(pst, sch)

	if r.rows[1].startBtn.Disabled() {
		t.Error("a finish.json for a different regatta must not lock rows")
	}
}
