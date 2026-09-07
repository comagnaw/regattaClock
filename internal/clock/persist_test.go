package clock

import (
	"path/filepath"
	"testing"

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
	t.Cleanup(func() { clk.window.Close() })
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
	if clk.buttons.save.Disabled() {
		t.Error("save button should be enabled for an approved race")
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
