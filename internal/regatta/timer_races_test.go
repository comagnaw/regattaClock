package regatta

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

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
	if r.rows[1].progress.Text != "approved" {
		t.Errorf("row 1 progress = %q, want approved", r.rows[1].progress.Text)
	}
}

func TestOnPeerStartChangedRefreshesFinishRow(t *testing.T) {
	r, _, _ := startedTimer(t, "pft")

	if r.rows[1].startTime.Text != "waiting for start…" {
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

func TestOnScheduleChangedRefreshesTitleInPlace(t *testing.T) {
	r, sch, _ := startedTimer(t, "pst")

	next := *sch
	next.Races = append([]store.ScheduleRace(nil), sch.Races...)
	next.Races[0].BoatClass = "JV8"

	r.onScheduleChanged(&next)

	if !strings.Contains(r.rows[1].title.Text, "JV8") {
		t.Errorf("row 1 title = %q, want it to reflect the new class", r.rows[1].title.Text)
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
