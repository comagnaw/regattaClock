package regatta

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/timesync"
)

var displayShape = regexp.MustCompile(`^\d{2}:\d{2}:\d{2}\.\d$`)

// startedTimer spins up a NewTimer bound to a fresh regatta as the given
// persona, with the watcher stopped on cleanup.
func startedTimer(t *testing.T, id string) (*Regatta, *store.Schedule, persona.Session) {
	t.Helper()
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	sess := timerSession(t, id, root)

	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(sess, sch)
	return r, sch, sess
}

func TestTimerStartTreeRowsAndButtons(t *testing.T) {
	r, _, _ := startedTimer(t, "pst")

	row := r.rows[1]
	if row == nil {
		t.Fatal("no row for race 1")
	}
	if _, timed := r.rows[2]; timed {
		t.Error("race 2 has no lanes and should not get a row")
	}
	if row.startTime.Text != "—" {
		t.Errorf("start time label = %q, want the no-time dash", row.startTime.Text)
	}
	if row.scheduledTime.Text != common.NoStartTimeText {
		t.Errorf("scheduled time label = %q, want the no-time dash", row.scheduledTime.Text)
	}
	if row.startBtn.Disabled() {
		t.Error("Start Time should be enabled")
	}
	if !row.clearBtn.Disabled() {
		t.Error("Clear should be disabled with no start time")
	}
	if !row.restoreBtn.Hidden {
		t.Error("Restore should be hidden with no cleared history")
	}
}

func TestRecordStartWritesAndRefreshes(t *testing.T) {
	r, _, sess := startedTimer(t, "pst")

	r.recordStart(1)

	rec := r.startLog.Races[1]
	if rec.StartedAt == nil || !displayShape.MatchString(rec.Display) {
		t.Fatalf("record not captured: %+v", rec)
	}
	if r.rows[1].startTime.Text != rec.Display {
		t.Errorf("row label = %q, want %q", r.rows[1].startTime.Text, rec.Display)
	}
	if r.rows[1].clearBtn.Disabled() {
		t.Error("Clear should enable once a time exists")
	}
	if !r.rows[1].startBtn.Disabled() {
		t.Error("Start Time should disable once a time is recorded")
	}

	reloaded, err := store.LoadStart(sess)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Races[1].Display != rec.Display {
		t.Errorf("disk display = %q, want %q", reloaded.Races[1].Display, rec.Display)
	}
}

// TestRecordStartShowsOnTheWaterBeforeFinishTimerBegins - the ST's own row
// must reflect the canonical team state (race-state-machine.md), not stay
// blank until the FT locks the row: a recorded start alone already reaches
// StateStartRecorded, which displays as "On the Water".
func TestRecordStartShowsOnTheWaterBeforeFinishTimerBegins(t *testing.T) {
	r, _, _ := startedTimer(t, "pst")

	before := r.rows[1].progress.Text
	wantNotStarted := store.StateNotStarted.DisplayText(persona.TeamPrimary)
	if before != wantNotStarted {
		t.Fatalf("before Start, status = %q, want %q", before, wantNotStarted)
	}

	r.recordStart(1)

	wantOnTheWater := store.StateStartRecorded.DisplayText(persona.TeamPrimary)
	if got := r.rows[1].progress.Text; got != wantOnTheWater {
		t.Errorf("status after Start = %q, want %q", got, wantOnTheWater)
	}
}

// TestStartRow_RestartsAndWinningTimeAreVisible - the "pane of glass" column
// unification: the ST's own row now shows the same Restarts and Winning Time
// data the RD tree already showed, not just Start Time and Status.
func TestStartRow_RestartsAndWinningTimeAreVisible(t *testing.T) {
	r, _, _ := startedTimer(t, "pst")

	r.recordStart(1)
	r.clearStartConfirmed(1)
	r.recordStart(1)

	if got := r.rows[1].restarts.Text; got != "1" {
		t.Errorf("restarts = %q, want 1 after one clear", got)
	}

	r.onPeerFinishChanged(&store.FinishLog{Races: map[int]store.RaceResult{
		1: {RaceNumber: 1, WinningTime: "06:00.0", Approved: true},
	}})
	if got := r.rows[1].winTime.Text; got != "06:00.0" {
		t.Errorf("winning time = %q, want the peer FT's committed value", got)
	}
}

func TestRecordStartIsOneShot(t *testing.T) {
	r, _, _ := startedTimer(t, "pst")

	r.recordStart(1)
	first := r.startLog.Races[1].Display

	r.recordStart(1) // a second click must not re-capture
	if got := r.startLog.Races[1].Display; got != first {
		t.Errorf("second recordStart changed the time: %q -> %q", first, got)
	}

	// Clearing re-opens recording.
	r.clearStartConfirmed(1)
	if r.rows[1].startBtn.Disabled() {
		t.Error("Start Time should re-enable after Clear")
	}
}

func TestClearStartIsNonDestructive(t *testing.T) {
	r, _, sess := startedTimer(t, "pst")
	r.recordStart(1)
	before := r.startLog.Races[1]

	r.clearStartConfirmed(1)

	rec := r.startLog.Races[1]
	if rec.StartedAt != nil {
		t.Error("StartedAt should be nil after clear")
	}
	if len(rec.Cleared) != 1 || rec.Cleared[0].Display != before.Display {
		t.Fatalf("cleared history = %+v, want one entry carrying %q", rec.Cleared, before.Display)
	}
	if rec.Cleared[0].Clock != before.Clock {
		t.Errorf("cleared entry lost its ClockRef: %+v vs %+v", rec.Cleared[0].Clock, before.Clock)
	}
	if r.rows[1].startTime.Text != "—" || r.rows[1].restoreBtn.Hidden {
		t.Error("row should show no time and reveal Restore")
	}

	reloaded, _ := store.LoadStart(sess)
	if reloaded.Races[1].StartedAt != nil || len(reloaded.Races[1].Cleared) != 1 {
		t.Errorf("disk did not reflect the clear: %+v", reloaded.Races[1])
	}
}

func TestRestoreRecoversValueAndOriginalClockRef(t *testing.T) {
	r, _, _ := startedTimer(t, "pst")

	origClock := timesync.ClockRef{Offset: 1234 * time.Millisecond, RTT: 7 * time.Millisecond, Source: "ntp:test"}
	r.startLog.Races[2] = store.StartRecord{
		RaceNumber: 2,
		Cleared: []store.ClearedStart{{
			StartedAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
			Display:   "12:00:00.0",
			Clock:     origClock,
			ClearedAt: time.Now().UTC(),
		}},
	}

	r.restoreStartConfirmed(2)

	rec := r.startLog.Races[2]
	if rec.StartedAt == nil || rec.Display != "12:00:00.0" {
		t.Fatalf("value not restored: %+v", rec)
	}
	if rec.Clock != origClock {
		t.Errorf("restored the current offset instead of the captured one: %+v", rec.Clock)
	}
	if len(rec.Cleared) != 0 {
		t.Errorf("cleared entry should be consumed, got %+v", rec.Cleared)
	}
}

func TestRestoreAlwaysConfirmsFirst(t *testing.T) {
	r, _, _ := startedTimer(t, "pst")
	r.recordStart(1)
	r.clearStartConfirmed(1) // row now blank, one cleared entry
	clearedLen := len(r.startLog.Races[1].Cleared)

	r.restoreStart(1) // shows a confirmation; must not mutate until answered

	rec := r.startLog.Races[1]
	if rec.StartedAt != nil || len(rec.Cleared) != clearedLen {
		t.Errorf("restoreStart changed state before confirmation: %+v", rec)
	}
}

func TestClearedHistoryCapsAtMax(t *testing.T) {
	r, _, _ := startedTimer(t, "pst")

	for i := 0; i < store.MaxClearedPerRace+2; i++ {
		r.recordStart(1)
		r.clearStartConfirmed(1)
	}

	if got := len(r.startLog.Races[1].Cleared); got != store.MaxClearedPerRace {
		t.Fatalf("cleared history len = %d, want %d", got, store.MaxClearedPerRace)
	}
}

func TestWritesBlockedStopsRecording(t *testing.T) {
	r, _, sess := startedTimer(t, "pst")
	r.writesBlocked = true
	r.refreshAllRows()

	if !r.rows[1].startBtn.Disabled() {
		t.Error("Start Time should be disabled while writes are blocked")
	}

	r.recordStart(1)
	if r.startLog.Races[1].StartedAt != nil {
		t.Error("recordStart must not mutate state while writes are blocked")
	}
	if _, err := os.Stat(sess.StartPath()); !os.IsNotExist(err) {
		t.Error("no start.json should have been written")
	}
}

func TestFinishTreeShowsPeerStartAndProgress(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)
	key := store.RegattaKey(sch.Name, sch.Date)

	pst := timerSession(t, "pst", root)
	at := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	startLog := &store.StartLog{Races: map[int]store.StartRecord{
		1: {RaceNumber: 1, StartedAt: &at, Display: "09:00:00.0"},
	}}
	startLog.RegattaKey = key
	if err := store.SaveStart(pst, startLog); err != nil {
		t.Fatal(err)
	}

	pft := timerSession(t, "pft", root)
	finishLog := &store.FinishLog{Races: map[int]store.RaceResult{
		1: {RaceNumber: 1, WinningTime: "06:00.0", Approved: true},
	}}
	finishLog.RegattaKey = key
	if err := store.SaveFinish(pft, finishLog); err != nil {
		t.Fatal(err)
	}

	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(pft, sch)

	if r.rows[1].startTime.Text != "09:00:00.0" {
		t.Errorf("row 1 start label = %q, want the peer time", r.rows[1].startTime.Text)
	}
	wantApproved := store.StateApproved.DisplayText(persona.TeamPrimary)
	if r.rows[1].progress.Text != wantApproved {
		t.Errorf("row 1 progress = %q, want %q", r.rows[1].progress.Text, wantApproved)
	}
}

func TestOnPeerStartChangedRefreshesFinishRow(t *testing.T) {
	r, _, _ := startedTimer(t, "pft")

	if r.rows[1].startTime.Text != common.WaitingForStartText {
		t.Fatalf("expected the waiting placeholder, got %q", r.rows[1].startTime.Text)
	}

	at := time.Now().UTC()
	r.onPeerStartChanged(&store.StartLog{Races: map[int]store.StartRecord{
		1: {RaceNumber: 1, StartedAt: &at, Display: "10:11:12.3"},
	}})

	if r.rows[1].startTime.Text != "10:11:12.3" {
		t.Errorf("row 1 start label = %q, want the watched peer time", r.rows[1].startTime.Text)
	}
}

func TestFinishRowNoStartTimeOnceCommitted(t *testing.T) {
	cases := []struct {
		name string
		res  store.RaceResult
	}{
		{"saved", store.RaceResult{RaceNumber: 1, WinningTime: "06:00.0"}},
		{"approved", store.RaceResult{RaceNumber: 1, WinningTime: "06:00.0", Approved: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := test.NewTempApp(t)
			sch := testSchedule()
			root := seedRegatta(t, sch)

			pft := timerSession(t, "pft", root)
			finishLog := &store.FinishLog{Races: map[int]store.RaceResult{1: tc.res}}
			finishLog.RegattaKey = store.RegattaKey(sch.Name, sch.Date)
			if err := store.SaveFinish(pft, finishLog); err != nil {
				t.Fatal(err)
			}

			r := NewTimer(app)
			stopWatch(t, r)
			r.startSession(pft, sch)

			if got := r.rows[1].startTime.Text; got != common.StartNotCollectedText {
				t.Errorf("%s race with no start recorded: start cell = %q, want %q",
					tc.name, got, common.StartNotCollectedText)
			}
		})
	}
}

// TestFinishRowShowsOnTheWaterBeforeOwnClockOpens - the PFT's own row must
// reflect the canonical team state (race-state-machine.md), not stay blank
// until this FT's own finish.json has an entry: a peer ST's recorded start
// alone already reaches StateStartRecorded, which displays as "On the Water".
func TestFinishRowShowsOnTheWaterBeforeOwnClockOpens(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)

	pst := timerSession(t, "pst", root)
	at := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	startLog := &store.StartLog{Races: map[int]store.StartRecord{
		1: {RaceNumber: 1, StartedAt: &at, Display: "09:00:00.0"},
	}}
	startLog.RegattaKey = store.RegattaKey(sch.Name, sch.Date)
	if err := store.SaveStart(pst, startLog); err != nil {
		t.Fatal(err)
	}

	pft := timerSession(t, "pft", root)
	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(pft, sch)

	wantOnTheWater := store.StateStartRecorded.DisplayText(persona.TeamPrimary)
	if got := r.rows[1].progress.Text; got != wantOnTheWater {
		t.Errorf("status before this FT's own clock opens = %q, want %q", got, wantOnTheWater)
	}
}

// TestFinishRow_RestartsAndWinningTimeAreVisible - the "pane of glass" column
// unification: the FT's own row now shows the peer ST's Restarts count and
// this FT's own Winning Time, not just Start Time and Status.
func TestFinishRow_RestartsAndWinningTimeAreVisible(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)

	pst := timerSession(t, "pst", root)
	at := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	startLog := &store.StartLog{Races: map[int]store.StartRecord{
		1: {RaceNumber: 1, StartedAt: &at, Cleared: []store.ClearedStart{{}}},
	}}
	startLog.RegattaKey = store.RegattaKey(sch.Name, sch.Date)
	if err := store.SaveStart(pst, startLog); err != nil {
		t.Fatal(err)
	}

	pft := timerSession(t, "pft", root)
	finishLog := &store.FinishLog{Races: map[int]store.RaceResult{
		1: {RaceNumber: 1, WinningTime: "06:00.0", Approved: true},
	}}
	finishLog.RegattaKey = store.RegattaKey(sch.Name, sch.Date)
	if err := store.SaveFinish(pft, finishLog); err != nil {
		t.Fatal(err)
	}

	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(pft, sch)

	if got := r.rows[1].restarts.Text; got != "1" {
		t.Errorf("restarts = %q, want the peer ST's cleared-start count", got)
	}
	if got := r.rows[1].winTime.Text; got != "06:00.0" {
		t.Errorf("winning time = %q, want this FT's own committed value", got)
	}
}

// TestFinishRowShowsOnTheWater_SecondaryTeam - the fix above is role-generic,
// not primary-specific: it must hold identically for the secondary team's
// own tree (SST/SFT), which reads its own start.json/finish.json, not the
// primary's.
func TestFinishRowShowsOnTheWater_SecondaryTeam(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)

	sst := timerSession(t, "sst", root)
	at := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	startLog := &store.StartLog{Races: map[int]store.StartRecord{
		1: {RaceNumber: 1, StartedAt: &at, Display: "09:00:00.0"},
	}}
	startLog.RegattaKey = store.RegattaKey(sch.Name, sch.Date)
	if err := store.SaveStart(sst, startLog); err != nil {
		t.Fatal(err)
	}

	// The secondary ST's own row should show On the Water too.
	rst := NewTimer(app)
	stopWatch(t, rst)
	rst.startSession(sst, sch)
	wantOnTheWater := store.StateStartRecorded.DisplayText(persona.TeamSecondary)
	if got := rst.rows[1].progress.Text; got != wantOnTheWater {
		t.Errorf("SST status = %q, want %q", got, wantOnTheWater)
	}

	// The secondary FT's own row, before its own clock has opened, should too.
	sft := timerSession(t, "sft", root)
	rft := NewTimer(app)
	stopWatch(t, rft)
	rft.startSession(sft, sch)
	if got := rft.rows[1].progress.Text; got != wantOnTheWater {
		t.Errorf("SFT status before its own clock opens = %q, want %q", got, wantOnTheWater)
	}
}

func TestFinishRowShowsInProgressStatus(t *testing.T) {
	app := test.NewTempApp(t)
	sch := testSchedule()
	root := seedRegatta(t, sch)

	pft := timerSession(t, "pft", root)
	finishLog := &store.FinishLog{Races: map[int]store.RaceResult{
		1: {RaceNumber: 1, FirstFinishAt: ptr(time.Now().UTC())}, // started, nothing saved
	}}
	finishLog.RegattaKey = store.RegattaKey(sch.Name, sch.Date)
	if err := store.SaveFinish(pft, finishLog); err != nil {
		t.Fatal(err)
	}

	r := NewTimer(app)
	stopWatch(t, r)
	r.startSession(pft, sch)

	wantInProgress := store.StateTimingInProgress.DisplayText(persona.TeamPrimary)
	if got := r.rows[1].progress.Text; got != wantInProgress {
		t.Errorf("FT in-progress status = %q, want %q", got, wantInProgress)
	}
}

func TestOnScheduleChangedRefreshesTitleInPlace(t *testing.T) {
	r, sch, _ := startedTimer(t, "pst")

	next := *sch
	next.Races = append([]store.ScheduleRace(nil), sch.Races...)
	next.Races[0].BoatClass = "JV8"
	next.Races[0].ScheduledTime = "09:30 AM"

	r.onScheduleChanged(&next)

	if !strings.Contains(r.rows[1].title.Text, "JV8") {
		t.Errorf("row 1 title = %q, want it to reflect the new class", r.rows[1].title.Text)
	}
	if r.rows[1].scheduledTime.Text != "09:30 AM" {
		t.Errorf("row 1 scheduled time = %q, want it to reflect the reload", r.rows[1].scheduledTime.Text)
	}
}

func TestOnScheduleChangedRebuildsWhenRaceSetChanges(t *testing.T) {
	r, sch, _ := startedTimer(t, "pst")
	if len(r.rows) != 1 {
		t.Fatalf("expected one row to start, got %d", len(r.rows))
	}

	next := *sch
	next.Races = append(append([]store.ScheduleRace(nil), sch.Races...), store.ScheduleRace{
		RaceNumber: 3, BoatClass: "Novice 4", BoatCount: 2,
		Lanes: map[int]store.ScheduleEntry{1: {SchoolName: "Gamma"}},
	})

	r.onScheduleChanged(&next)

	if len(r.rows) != 2 || r.rows[3] == nil {
		t.Fatalf("expected the tree rebuilt with rows {1,3}, got %v", r.rows)
	}
}
