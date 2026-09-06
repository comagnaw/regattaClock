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
