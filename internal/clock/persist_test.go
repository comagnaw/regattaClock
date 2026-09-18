package clock

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/reader"
)

func pftSession(t *testing.T) persona.Session {
	t.Helper()
	def, ok := persona.ByID("pft")
	if !ok {
		t.Fatal("no pft persona")
	}
	return persona.Session{Definition: def, Root: filepath.Join(t.TempDir(), common.RegattaDataDir)}
}

func openBoundClock(t *testing.T, s persona.Session, log *store.FinishLog) *Clock {
	t.Helper()
	app := test.NewTempApp(t)
	clk := NewClock(app, createTestRegattaData(), createTestRaceData()).WithFinishLog(s, log)
	clk.OpenRaceClock()
	t.Cleanup(clk.closeWindow)
	return clk
}

func TestClockStartWritesInProgressResult(t *testing.T) {
	s := pftSession(t)
	log := &store.FinishLog{Races: map[int]store.RaceResult{}}
	clk := openBoundClock(t, s, log)

	clk.buttons.start.OnTapped()

	res, ok := log.Races[1]
	if !ok || res.FirstFinishAt == nil {
		t.Fatalf("Start did not stamp FirstFinishAt: %+v", res)
	}
	if res.WinningTime != "" || res.Approved {
		t.Errorf("in-progress result should have no winning time / not be approved: %+v", res)
	}

	onDisk, err := store.LoadFinish(s)
	if err != nil {
		t.Fatalf("LoadFinish: %v", err)
	}
	if onDisk.Races[1].FirstFinishAt == nil {
		t.Error("finish.json on disk has no FirstFinishAt for race 1")
	}
}

func TestClockApprovalWritesFullResult(t *testing.T) {
	s := pftSession(t)
	log := &store.FinishLog{Races: map[int]store.RaceResult{}}
	clk := openBoundClock(t, s, log)

	clk.buttons.start.OnTapped()
	clk.laps.setOOFLaneNum(0, "3")
	clk.laps.setPlace(0, "1")
	clk.laps.setSplit(0, "00:05.0")
	clk.laps.setCalculatedTime(0, "01:05.0")
	clk.winningTime.SetText("01:00.0")

	clk.refereeApprovalFunc(1)(true)

	res := log.Races[1]
	if !res.Approved || res.WinningTime != "01:00.0" {
		t.Fatalf("approval result: %+v", res)
	}
	if len(res.Rows) != 6 {
		t.Fatalf("want 6 lap rows, got %d", len(res.Rows))
	}
	if res.Rows[0] != (store.LapRow{Lane: 3, Place: "1", Split: "00:05.0", Time: "01:05.0"}) {
		t.Errorf("row 0 = %+v", res.Rows[0])
	}
	if res.FirstFinishAt == nil {
		t.Error("FirstFinishAt should carry over from Start")
	}

	onDisk, _ := store.LoadFinish(s)
	if onDisk.Races[1].WinningTime != "01:00.0" || !onDisk.Races[1].Approved {
		t.Errorf("disk result = %+v", onDisk.Races[1])
	}
}

// approveRace1 times and approves race 1 with a known winning time, the
// shared setup for the Clear-after-approval tests below.
func approveRace1(clk *Clock) {
	clk.buttons.start.OnTapped()
	clk.laps.setOOFLaneNum(0, "3")
	clk.laps.setPlace(0, "1")
	clk.laps.setSplit(0, "00:05.0")
	clk.laps.setCalculatedTime(0, "01:05.0")
	clk.buttons.stop.OnTapped()
	clk.winningTime.SetText("01:00.0")
	clk.refereeApprovalFunc(1)(true)
}

// TestClearOnApprovedRace_RequiresConfirmation - docs/features/PRE-RELEASE-BUGS.md
// corner case: Clear must not silently discard an approved result. Tapping
// it should raise a confirm dialog and change nothing until answered.
func TestClearOnApprovedRace_RequiresConfirmation(t *testing.T) {
	s := pftSession(t)
	log := &store.FinishLog{Races: map[int]store.RaceResult{}}
	clk := openBoundClock(t, s, log)
	approveRace1(clk)
	before := log.Races[1]

	clk.buttons.clear.OnTapped()

	got := log.Races[1]
	if got.Approved != before.Approved || got.WinningTime != before.WinningTime ||
		got.FirstFinishAt != before.FirstFinishAt || len(got.Rows) != len(before.Rows) {
		t.Errorf("Clear on an approved race must not mutate anything before confirmation: got %+v, want %+v", got, before)
	}
	if len(clk.window.Canvas().Overlays().List()) == 0 {
		t.Error("Clear on an approved race should raise a confirm dialog")
	}
}

// TestClearOnApprovedRace_ConfirmedResetFixesStaleFirstFinish - the actual
// bug: after a confirmed re-time, the previous approved FirstFinishAt must
// not survive to corrupt the next winning-time calculation, and the winning
// time must be left for manual entry (the ST's old start time is not a
// meaningful reference point for a race that already happened).
func TestClearOnApprovedRace_ConfirmedResetFixesStaleFirstFinish(t *testing.T) {
	s := pftSession(t)
	log := &store.FinishLog{Races: map[int]store.RaceResult{}}
	clk := openBoundClock(t, s, log)
	approveRace1(clk)
	staleFinish := log.Races[1].FirstFinishAt

	// What initClear's confirm callback does once the operator answers Yes.
	clk.clockState.skipAutoWinningTime = true
	clk.performClear()

	if res := log.Races[1]; res.FirstFinishAt != nil || res.WinningTime != "" || res.Approved {
		t.Fatalf("performClear should fully reset the race, got %+v", res)
	}

	clk.buttons.start.OnTapped()

	res := log.Races[1]
	if res.FirstFinishAt == nil {
		t.Fatal("a fresh Start click should record a new FirstFinishAt")
	}
	if staleFinish != nil && res.FirstFinishAt.Equal(*staleFinish) {
		t.Error("FirstFinishAt still matches the pre-Clear approved value - the stale-data bug is back")
	}
	if res.WinningTime != "" || clk.winningTime.Text != "" {
		t.Errorf("winning time should stay manual-entry only after a confirmed re-time, got record=%q field=%q",
			res.WinningTime, clk.winningTime.Text)
	}
}

// TestClearOnUnapprovedRace_NoConfirmationNeeded - the ordinary "I
// mis-clicked Start" case stays exactly as before: Clear resets instantly,
// no dialog, and now also fully resets the in-memory RaceResult (not just
// UI state) so a stale FirstFinishAt can never survive a Clear regardless
// of approval state.
func TestClearOnUnapprovedRace_NoConfirmationNeeded(t *testing.T) {
	s := pftSession(t)
	log := &store.FinishLog{Races: map[int]store.RaceResult{}}
	clk := openBoundClock(t, s, log)

	clk.buttons.start.OnTapped()
	if log.Races[1].FirstFinishAt == nil {
		t.Fatal("precondition: Start should have recorded FirstFinishAt")
	}
	clk.buttons.stop.OnTapped()

	clk.buttons.clear.OnTapped()

	if len(clk.window.Canvas().Overlays().List()) != 0 {
		t.Error("Clear on a not-yet-approved race should not raise a confirm dialog")
	}
	if res := log.Races[1]; res.FirstFinishAt != nil || res.WinningTime != "" {
		t.Errorf("Clear should fully reset the race even before approval, got %+v", res)
	}
	if !clk.clockState.isCleared {
		t.Error("clockState.isCleared should be true after Clear")
	}
}

func TestClockStampsLaneMapHash(t *testing.T) {
	s := pftSession(t)
	log := &store.FinishLog{Races: map[int]store.RaceResult{}}
	clk := openBoundClock(t, s, log)

	// The hash of the lane map the clock is showing.
	race := createTestRaceData()
	sr := store.ScheduleRace{RaceNumber: race.RaceNumber, Lanes: map[int]store.ScheduleEntry{}}
	for lane, e := range race.Lanes {
		sr.Lanes[lane] = store.ScheduleEntry{SchoolName: e.SchoolName, AdditionalInfo: e.AdditionalInfo}
	}
	want := sr.LaneMapHash()
	if want == "" {
		t.Fatal("expected a non-empty lane-map hash for the test race")
	}

	// In-progress result on Start carries it.
	clk.buttons.start.OnTapped()
	if got := log.Races[1].LaneMapHash; got != want {
		t.Errorf("in-progress LaneMapHash = %q, want %q", got, want)
	}

	// Approval carries it, and it round-trips through finish.json.
	clk.winningTime.SetText("01:00.0")
	clk.refereeApprovalFunc(1)(true)
	if log.Races[1].LaneMapHash != want {
		t.Errorf("approved LaneMapHash = %q, want %q", log.Races[1].LaneMapHash, want)
	}
	onDisk, err := store.LoadFinish(s)
	if err != nil {
		t.Fatalf("LoadFinish: %v", err)
	}
	if onDisk.Races[1].LaneMapHash != want {
		t.Errorf("disk LaneMapHash = %q, want %q", onDisk.Races[1].LaneMapHash, want)
	}

	// A schedule change to the open clock re-stamps on the next write.
	moved := createTestRaceData()
	moved.Lanes[1] = reader.RaceEntry{SchoolName: "Moved Crew"}
	clk.UpdateSchedule(moved, []int{1})
	clk.refereeApprovalFunc(1)(true)
	if log.Races[1].LaneMapHash == want {
		t.Error("LaneMapHash should change after the lane map changed and the result was re-saved")
	}
}

func TestClockRehydratesSavedRace(t *testing.T) {
	s := pftSession(t)

	seed := &store.FinishLog{Races: map[int]store.RaceResult{
		1: {
			RaceNumber:  1,
			WinningTime: "02:00.0",
			Approved:    true,
			Rows: []store.LapRow{
				{Lane: 2, Place: "1", Split: "00:00.0", Time: "02:00.0"},
				{Lane: 4, Place: "2", Split: "00:03.0", Time: "02:03.0"},
				{}, {}, {}, {},
			},
		},
	}}
	if err := store.SaveFinish(s, seed); err != nil {
		t.Fatal(err)
	}

	restored, err := store.LoadFinish(s)
	if err != nil {
		t.Fatal(err)
	}
	clk := openBoundClock(t, s, restored)

	if clk.winningTime.Text != "02:00.0" {
		t.Errorf("winning time = %q, want 02:00.0", clk.winningTime.Text)
	}
	if clk.laps.oofLaneNum(0) != "2" || clk.laps.place(0) != "1" || clk.laps.split(0) != "00:00.0" {
		t.Errorf("row 0 not restored: oof=%q place=%q split=%q",
			clk.laps.oofLaneNum(0), clk.laps.place(0), clk.laps.split(0))
	}
	if clk.buttons.referee.Disabled() {
		t.Error("referee button should be enabled for a race with a winning time")
	}
	if clk.buttons.close.Disabled() {
		t.Error("close button should be enabled for an approved race")
	}
	if !strings.HasPrefix(clk.commitStatus.Text, "Approved on ") {
		t.Errorf("commit status = %q, want an \"Approved on …\" line", clk.commitStatus.Text)
	}
}

func TestCommitStatusLineHasDateAndHost(t *testing.T) {
	s := pftSession(t)
	approvedAt := time.Date(2026, time.October, 3, 18, 15, 47, 0, time.UTC)
	seed := &store.FinishLog{
		Races: map[int]store.RaceResult{
			1: {RaceNumber: 1, WinningTime: "02:00.0", Approved: true, ApprovedAt: &approvedAt},
		},
	}
	seed.Machine = "bow-line-02"
	if err := store.SaveFinish(s, seed); err != nil {
		t.Fatal(err)
	}
	restored, _ := store.LoadFinish(s)
	clk := openBoundClock(t, s, restored)

	got := clk.commitStatus.Text
	want := "Approved on " + approvedAt.Local().Format(common.CommitStatusTimeFormat) + " by bow-line-02"
	if got != want {
		t.Errorf("commit status = %q, want %q", got, want)
	}
	for _, part := range []string{"2026", "Oct", " by bow-line-02"} {
		if !strings.Contains(got, part) {
			t.Errorf("commit status %q is missing %q", got, part)
		}
	}
}

func TestClockWithoutFinishLogDoesNotPersist(t *testing.T) {
	app := test.NewTempApp(t)
	clk := NewClock(app, createTestRegattaData(), createTestRaceData()) // no WithFinishLog
	clk.OpenRaceClock()
	t.Cleanup(func() { clk.window.Close() })

	if clk.canPersist() {
		t.Fatal("a clock with no finish log must not persist")
	}
	// Must not panic.
	clk.buttons.start.OnTapped()
	clk.winningTime.SetText("01:00.0")
	clk.refereeApprovalFunc(1)(true)
}

func TestClockWithNonFinishSessionDoesNotPersist(t *testing.T) {
	rd, _ := persona.ByID("rd")
	s := persona.Session{Definition: rd, Root: t.TempDir()}
	clk := openBoundClock(t, s, &store.FinishLog{Races: map[int]store.RaceResult{}})

	if clk.canPersist() {
		t.Fatal("a director session must not persist from the clock")
	}
	clk.buttons.start.OnTapped()
	if _, err := store.LoadFinish(s); err == nil {
		t.Error("no finish.json should have been written for a director session")
	}
}
