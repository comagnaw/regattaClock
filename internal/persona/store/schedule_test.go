package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/comagnaw/regattaClock/internal/persona"
)

func directorSession(t *testing.T) persona.Session {
	t.Helper()
	return persona.Session{Definition: persona.DirectorDefinition, Root: t.TempDir()}
}

func sampleSchedule() *Schedule {
	return &Schedule{
		Name:   "Spring Sprints",
		Date:   "2026-04-12",
		Origin: Origin{Type: "excel", URI: "C:\\Regatta\\SpringSprints.xlsx", Hash: "abc123"},
		Races: []ScheduleRace{
			{
				RaceNumber: 12,
				BoatClass:  "Varsity 8",
				FlightInfo: "Heat 1",
				BoatCount:  3,
				Lanes: map[int]ScheduleEntry{
					1: {SchoolName: "School A"},
					2: {SchoolName: "School B", AdditionalInfo: "A"},
					3: {SchoolName: ""},
				},
			},
		},
	}
}

func TestSaveAndLoadSchedule(t *testing.T) {
	s := directorSession(t)
	want := sampleSchedule()

	if err := SaveSchedule(s, want); err != nil {
		t.Fatalf("SaveSchedule: %v", err)
	}

	if _, err := os.Stat(s.SchedulePath()); err != nil {
		t.Fatalf("schedule file not at %s: %v", s.SchedulePath(), err)
	}
	if filepath.Base(filepath.Dir(s.SchedulePath())) != "director" {
		t.Fatalf("schedule not under director/: %s", s.SchedulePath())
	}

	got, err := LoadSchedule(s)
	if err != nil {
		t.Fatalf("LoadSchedule: %v", err)
	}
	if got.Name != want.Name || got.Date != want.Date || got.Origin != want.Origin {
		t.Fatalf("metadata round-trip: got %+v", got)
	}
	if len(got.Races) != 1 || got.Races[0].RaceNumber != 12 || got.Races[0].BoatClass != "Varsity 8" {
		t.Fatalf("race round-trip: got %+v", got.Races)
	}
	if got.Races[0].Lanes[2] != (ScheduleEntry{SchoolName: "School B", AdditionalInfo: "A"}) {
		t.Fatalf("lane round-trip: got %+v", got.Races[0].Lanes)
	}
}

func TestLoadScheduleMissingIsNotExist(t *testing.T) {
	s := directorSession(t)

	got, err := LoadSchedule(s)
	if got != nil {
		t.Fatalf("expected nil schedule, got %+v", got)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v, want fs.ErrNotExist", err)
	}
}

func TestLoadScheduleCorruptIsErrCorrupt(t *testing.T) {
	s := directorSession(t)
	if err := os.MkdirAll(filepath.Dir(s.SchedulePath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.SchedulePath(), []byte("{ not json"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadSchedule(s); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("err = %v, want ErrCorrupt", err)
	}
}

func TestScheduleKey(t *testing.T) {
	sch := sampleSchedule()
	if sch.Key() != RegattaKey(sch.Name, sch.Date) {
		t.Fatalf("Key() = %q, want %q", sch.Key(), RegattaKey(sch.Name, sch.Date))
	}
}

func TestLaneMapHash(t *testing.T) {
	base := func() ScheduleRace {
		return ScheduleRace{
			RaceNumber: 12,
			BoatClass:  "Varsity 8",
			FlightInfo: "Heat 1",
			Lanes: map[int]ScheduleEntry{
				1: {SchoolName: "School A"},
				2: {SchoolName: "School B", AdditionalInfo: "A"},
				3: {SchoolName: ""}, // a scratch
			},
		}
	}
	want := base().LaneMapHash()

	if len(want) != 12 {
		t.Fatalf("LaneMapHash length = %d, want 12", len(want))
	}

	t.Run("stable across lane insertion order", func(t *testing.T) {
		shuffled := ScheduleRace{RaceNumber: 12, Lanes: map[int]ScheduleEntry{}}
		shuffled.Lanes[3] = ScheduleEntry{SchoolName: ""}
		shuffled.Lanes[1] = ScheduleEntry{SchoolName: "School A"}
		shuffled.Lanes[2] = ScheduleEntry{SchoolName: "School B", AdditionalInfo: "A"}
		if got := shuffled.LaneMapHash(); got != want {
			t.Errorf("hash = %q, want %q - map order must not matter", got, want)
		}
	})

	t.Run("unchanged when only class or flight differ", func(t *testing.T) {
		r := base()
		r.BoatClass = "Junior 8"
		r.FlightInfo = "Final"
		r.BoatCount = 6
		if got := r.LaneMapHash(); got != want {
			t.Errorf("hash = %q, want %q - class/flight are out of scope", got, want)
		}
	})

	for _, tc := range []struct {
		name   string
		mutate func(*ScheduleRace)
	}{
		{"school moves lane", func(r *ScheduleRace) {
			r.Lanes[1] = ScheduleEntry{SchoolName: "School B", AdditionalInfo: "A"}
			r.Lanes[2] = ScheduleEntry{SchoolName: "School A"}
		}},
		{"additional info changes", func(r *ScheduleRace) {
			r.Lanes[2] = ScheduleEntry{SchoolName: "School B", AdditionalInfo: "B"}
		}},
		{"lane scratched", func(r *ScheduleRace) {
			r.Lanes[1] = ScheduleEntry{SchoolName: ""}
		}},
		{"scratch filled", func(r *ScheduleRace) {
			r.Lanes[3] = ScheduleEntry{SchoolName: "School C"}
		}},
		{"race number changes", func(r *ScheduleRace) {
			r.RaceNumber = 13
		}},
	} {
		t.Run(tc.name+" changes the hash", func(t *testing.T) {
			r := base()
			tc.mutate(&r)
			if got := r.LaneMapHash(); got == want {
				t.Errorf("hash unchanged (%q) after %s", got, tc.name)
			}
		})
	}
}

func TestContentHash(t *testing.T) {
	base := sampleSchedule().ContentHash()
	if len(base) != 16 {
		t.Fatalf("ContentHash length = %d, want 16", len(base))
	}

	t.Run("ignores Origin metadata", func(t *testing.T) {
		s := sampleSchedule()
		s.Origin = Origin{Type: "api", URI: "https://example.test/x", Hash: "different"}
		if s.ContentHash() != base {
			t.Error("ContentHash must not depend on Origin")
		}
	})

	t.Run("stable across race slice order", func(t *testing.T) {
		s := sampleSchedule()
		s.Races = append(s.Races, ScheduleRace{RaceNumber: 5, BoatClass: "V4", Lanes: map[int]ScheduleEntry{1: {SchoolName: "X"}}})
		want := s.ContentHash()

		s.Races[0], s.Races[1] = s.Races[1], s.Races[0]
		if s.ContentHash() != want {
			t.Error("ContentHash must not depend on Races slice order")
		}
	})

	for _, tc := range []struct {
		name   string
		mutate func(*Schedule)
	}{
		{"lane move", func(s *Schedule) { s.Races[0].Lanes[1] = ScheduleEntry{SchoolName: "Moved"} }},
		{"scratch", func(s *Schedule) { s.Races[0].Lanes[2] = ScheduleEntry{SchoolName: ""} }},
		{"class", func(s *Schedule) { s.Races[0].BoatClass = "Junior 8" }},
		{"flight", func(s *Schedule) { s.Races[0].FlightInfo = "Final" }},
		{"boat count", func(s *Schedule) { s.Races[0].BoatCount = 6 }},
		{"race added", func(s *Schedule) { s.Races = append(s.Races, ScheduleRace{RaceNumber: 99}) }},
		{"name", func(s *Schedule) { s.Name = "Autumn Sprints" }},
		{"date", func(s *Schedule) { s.Date = "2026-04-13" }},
	} {
		t.Run(tc.name+" changes the hash", func(t *testing.T) {
			s := sampleSchedule()
			tc.mutate(s)
			if s.ContentHash() == base {
				t.Errorf("ContentHash unchanged after %s", tc.name)
			}
		})
	}
}
