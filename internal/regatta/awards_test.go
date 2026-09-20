package regatta

import (
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
)

// startedAwards spins up a NewTimer bound to a fresh regatta as Awards, with
// the watcher stopped on cleanup - mirrors startedTimer (timer_races_test.go)
// since Awards enters through the same folder-pick/startSession path a timer
// persona does, not the Director's own startDirectorFlow.
func startedAwards(t *testing.T, sch *store.Schedule, root string) *Regatta {
	t.Helper()
	app := test.NewTempApp(t)
	sess := timerSession(t, "awd", root)

	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(sess, sch)
	return r
}

// TestAwardsStartupHydratesPrimary - Awards reuses hydrateDirectorLogs via
// startSession's own RoleAwards case, exactly as the Director's
// startDirectorFlow does, and never populates its own startLog/finishLog -
// it has none, since File is "" and it never writes.
func TestAwardsStartupHydratesPrimary(t *testing.T) {
	sch := twoRaceSchedule()
	root := seedRegatta(t, sch)
	key := store.RegattaKey(sch.Name, sch.Date)

	pst := timerSession(t, "pst", root)
	ps := &store.StartLog{Races: map[int]store.StartRecord{1: {RaceNumber: 1, StartedAt: tm(-6), Display: "08:00:00.0"}}}
	ps.RegattaKey = key
	if err := store.SaveStart(pst, ps); err != nil {
		t.Fatal(err)
	}

	r := startedAwards(t, sch, root)

	if tt := r.teamLogs[persona.TeamPrimary]; tt == nil || tt.start.Races[1].StartedAt == nil {
		t.Fatal("primary start.json not hydrated")
	}
	if r.rows[1].startTime.Text != "08:00:00.0" {
		t.Errorf("race 1 start = %q", r.rows[1].startTime.Text)
	}
	if r.startLog != nil {
		t.Error("Awards must not populate its own startLog - it owns no timing file")
	}
	if r.finishLog != nil {
		t.Error("Awards must not populate its own finishLog - it owns no timing file")
	}
}

// TestAwardsRow_SharesDirectorViewResultsBehavior - the Awards row gets the
// same store.CanPublish-gated View Results button as the Director's, via
// their shared RoleDirector/RoleAwards case (timer_races.go).
func TestAwardsRow_SharesDirectorViewResultsBehavior(t *testing.T) {
	sch := twoRaceSchedule()
	root := seedRegatta(t, sch)
	key := store.RegattaKey(sch.Name, sch.Date)

	pft := timerSession(t, "pft", root)
	fin := &store.FinishLog{Races: map[int]store.RaceResult{
		1: {RaceNumber: 1, WinningTime: "06:00.0", Approved: true},
	}}
	fin.RegattaKey = key
	if err := store.SaveFinish(pft, fin); err != nil {
		t.Fatal(err)
	}

	r := startedAwards(t, sch, root)

	if r.rows[1].resultsBtn == nil {
		t.Fatal("expected a results button on the Awards row")
	}
	if r.rows[1].resultsBtn.Disabled() {
		t.Error("results button should be enabled for an approved race")
	}
	if r.rows[2].resultsBtn == nil || !r.rows[2].resultsBtn.Disabled() {
		t.Error("results button for an untimed race should exist, disabled")
	}
}

// TestAwardsWatchesPrimaryTimingFiles - Awards shares RoleDirector's case in
// startWatcher (directorWatchPaths), not a bespoke path list - confirmed by
// checking the watcher's seeded hashes include the primary team's start and
// finish paths.
func TestAwardsWatchesPrimaryTimingFiles(t *testing.T) {
	sch := twoRaceSchedule()
	root := seedRegatta(t, sch)
	key := store.RegattaKey(sch.Name, sch.Date)

	pst := timerSession(t, "pst", root)
	ps := &store.StartLog{Races: map[int]store.StartRecord{}}
	ps.RegattaKey = key
	if err := store.SaveStart(pst, ps); err != nil {
		t.Fatal(err)
	}
	pft := timerSession(t, "pft", root)
	fin := &store.FinishLog{Races: map[int]store.RaceResult{}}
	fin.RegattaKey = key
	if err := store.SaveFinish(pft, fin); err != nil {
		t.Fatal(err)
	}

	r := startedAwards(t, sch, root)

	// startWatcher only seeds a hash for a path it could actually read, so
	// both the primary start.json and finish.json must exist on disk first
	// (above) for this to prove the watch-path wiring rather than an absent
	// file's absent hash.
	for _, p := range directorWatchPaths(root) {
		if _, ok := r.watchedHashes[p]; !ok {
			t.Errorf("Awards watcher does not track %q, want the primary team's start/finish paths", p)
		}
	}
}
