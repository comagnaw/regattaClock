package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/timesync"
)

func sessionFor(t *testing.T, id, root string) persona.Session {
	t.Helper()
	def, ok := persona.ByID(id)
	if !ok {
		t.Fatalf("unknown persona %q", id)
	}
	return persona.Session{Definition: def, Root: root}
}

func ptrTime(tm time.Time) *time.Time { return &tm }

func TestSaveAndLoadStart(t *testing.T) {
	root := t.TempDir()
	pst := sessionFor(t, "pst", root)

	at := time.Date(2026, 4, 12, 9, 30, 0, 0, time.UTC)
	in := &StartLog{
		Races: map[int]StartRecord{
			12: {
				RaceNumber: 12,
				StartedAt:  ptrTime(at),
				Display:    "09:30:00.0",
				Clock:      timesync.ClockRef{Offset: 120 * time.Millisecond, Source: "ntp:x", MeasuredAt: at},
			},
		},
	}

	if err := SaveStart(pst, in); err != nil {
		t.Fatalf("SaveStart: %v", err)
	}

	want := filepath.Join(root, "timing", "primary", "start.json")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("start.json not at %s: %v", want, err)
	}

	if in.Version != SchemaVersion || in.Role != persona.RoleStart || in.Team != persona.TeamPrimary {
		t.Fatalf("envelope not stamped: %+v", in.Envelope)
	}
	if in.Sequence != 1 || in.WrittenAt.IsZero() {
		t.Fatalf("sequence/writtenAt: seq=%d writtenAt=%v", in.Sequence, in.WrittenAt)
	}

	got, err := LoadStart(pst)
	if err != nil {
		t.Fatalf("LoadStart: %v", err)
	}
	rec := got.Races[12]
	if rec.Display != "09:30:00.0" || rec.StartedAt == nil || !rec.StartedAt.Equal(at) {
		t.Fatalf("record round-trip: %+v", rec)
	}
	if rec.Clock.Offset != 120*time.Millisecond || rec.Clock.Source != "ntp:x" {
		t.Fatalf("clock ref round-trip: %+v", rec.Clock)
	}
}

func TestSaveStartBumpsSequence(t *testing.T) {
	pst := sessionFor(t, "pst", t.TempDir())
	log := &StartLog{Races: map[int]StartRecord{1: {RaceNumber: 1}}}

	_ = SaveStart(pst, log)
	if log.Sequence != 1 {
		t.Fatalf("after first save Sequence = %d, want 1", log.Sequence)
	}
	_ = SaveStart(pst, log)
	if log.Sequence != 2 {
		t.Fatalf("after second save Sequence = %d, want 2", log.Sequence)
	}
}

// TestSaveStartKeepsWholeMap is persona-plan.md section 11's required test:
// saving after adding one race must not drop the races already collected.
func TestSaveStartKeepsWholeMap(t *testing.T) {
	pst := sessionFor(t, "pst", t.TempDir())

	log := &StartLog{Races: map[int]StartRecord{
		1: {RaceNumber: 1, Display: "a"},
		2: {RaceNumber: 2, Display: "b"},
	}}
	if err := SaveStart(pst, log); err != nil {
		t.Fatal(err)
	}

	log.Races[3] = StartRecord{RaceNumber: 3, Display: "c"}
	if err := SaveStart(pst, log); err != nil {
		t.Fatal(err)
	}

	got, err := LoadStart(pst)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{1, 2, 3} {
		if _, ok := got.Races[n]; !ok {
			t.Fatalf("race %d missing after incremental save; file has %d races", n, len(got.Races))
		}
	}
}

func TestSaveStartRejectsNonStartPersona(t *testing.T) {
	pft := sessionFor(t, "pft", t.TempDir())

	err := SaveStart(pft, &StartLog{Races: map[int]StartRecord{}})
	if !errors.Is(err, ErrWrongPersona) {
		t.Fatalf("err = %v, want ErrWrongPersona", err)
	}
	if _, statErr := os.Stat(pft.StartPath()); !os.IsNotExist(statErr) {
		t.Fatal("start.json should not have been written by a finish persona")
	}
}

func TestSaveFinishRejectsNonFinishPersona(t *testing.T) {
	pst := sessionFor(t, "pst", t.TempDir())
	if err := SaveFinish(pst, &FinishLog{Races: map[int]RaceResult{}}); !errors.Is(err, ErrWrongPersona) {
		t.Fatalf("err = %v, want ErrWrongPersona", err)
	}
}

func TestSaveAndLoadFinish(t *testing.T) {
	root := t.TempDir()
	pft := sessionFor(t, "pft", root)

	now := time.Now().UTC().Round(time.Millisecond)
	in := &FinishLog{Races: map[int]RaceResult{
		12: {
			RaceNumber:  12,
			WinningTime: "06:12.3",
			Rows: []LapRow{
				{Lane: 3, Place: "1", Split: "00:00.0", Time: "06:12.3"},
				{Lane: 1, Place: "2", Split: "00:02.1", Time: "06:14.4"},
			},
			Approved:   true,
			ApprovedAt: ptrTime(now),
			UpdatedAt:  now,
		},
	}}

	if err := SaveFinish(pft, in); err != nil {
		t.Fatalf("SaveFinish: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "timing", "primary", "finish.json")); err != nil {
		t.Fatalf("finish.json missing: %v", err)
	}

	got, err := LoadFinish(pft)
	if err != nil {
		t.Fatalf("LoadFinish: %v", err)
	}
	res := got.Races[12]
	if res.WinningTime != "06:12.3" || !res.Approved || len(res.Rows) != 2 || res.Rows[0].Lane != 3 {
		t.Fatalf("result round-trip: %+v", res)
	}
}

func TestFinishTimerReadsPeerStartFile(t *testing.T) {
	root := t.TempDir()
	pst := sessionFor(t, "pst", root)
	pft := sessionFor(t, "pft", root)

	if err := SaveStart(pst, &StartLog{Races: map[int]StartRecord{7: {RaceNumber: 7, Display: "peer"}}}); err != nil {
		t.Fatal(err)
	}

	// The FT reads the same team's start.json even though it did not write it.
	got, err := LoadStart(pft)
	if err != nil {
		t.Fatalf("FT LoadStart: %v", err)
	}
	if got.Races[7].Display != "peer" {
		t.Fatalf("FT did not see the ST's record: %+v", got.Races)
	}
}

func TestLoadStartMissingAndCorrupt(t *testing.T) {
	pst := sessionFor(t, "pst", t.TempDir())

	if _, err := LoadStart(pst); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing: err = %v, want fs.ErrNotExist", err)
	}

	if err := os.MkdirAll(filepath.Dir(pst.StartPath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pst.StartPath(), []byte("nope"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadStart(pst); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("corrupt: err = %v, want ErrCorrupt", err)
	}
}

func TestTrimCleared(t *testing.T) {
	rec := StartRecord{}
	for i := 0; i < MaxClearedPerRace+2; i++ {
		rec.Cleared = append(rec.Cleared, ClearedStart{Display: string(rune('a' + i))})
	}
	rec.TrimCleared()

	if len(rec.Cleared) != MaxClearedPerRace {
		t.Fatalf("len = %d, want %d", len(rec.Cleared), MaxClearedPerRace)
	}
	if rec.Cleared[0].Display != "c" { // "a" and "b" dropped as oldest
		t.Fatalf("oldest not dropped: first = %q", rec.Cleared[0].Display)
	}
}

func TestRegattaKey(t *testing.T) {
	k := RegattaKey("Spring Sprints", "2026-04-12")
	if len(k) != 12 {
		t.Fatalf("key length = %d, want 12", len(k))
	}
	if RegattaKey("  Spring Sprints ", " 2026-04-12 ") != k {
		t.Error("RegattaKey should be whitespace-insensitive")
	}
	if RegattaKey("Other", "2026-04-12") == k {
		t.Error("different name should give a different key")
	}
}
