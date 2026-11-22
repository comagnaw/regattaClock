package webjson

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/publish"
)

var (
	publishedAt = time.Date(2026, 10, 3, 9, 30, 0, 0, time.UTC)
	approvedAt  = time.Date(2026, 10, 3, 9, 28, 0, 0, time.UTC)
	meta        = publish.Meta{Name: "Charlie Brown Classic", Date: "Saturday, May 30, 2027", RegattaKey: "key-2027"}
)

func fixtureRaces() []publish.PublishableRace {
	return []publish.PublishableRace{
		{
			RaceNumber:    1,
			Title:         "Race 1 - M-1x - Heat 1",
			ScheduledTime: "09:00 AM",
			BoatClass:     "M-1x",
			FlightInfo:    "Heat 1",
			WinningTime:   "06:00.0",
			ApprovedAt:    approvedAt,
			Lanes: map[int]store.ScheduleEntry{
				1: {SchoolName: "Snoopy", AdditionalInfo: "A"},
				2: {SchoolName: "Linus"},
				6: {SchoolName: "Pig Pen", Status: store.StatusScratched},
			},
			Rows: []publish.Row{
				{Place: "1", Lane: "2", School: "Linus", Split: "00:00.0", Time: "06:00.0"},
				{Place: "2", Lane: "1", School: "Snoopy", AdditionalInfo: "A", Split: "00:05.0", Time: "06:05.0"},
				{Place: "DQ", Lane: "", School: "Woodstock"},
			},
		},
		// Scheduled but not published: must not appear in the file.
		{RaceNumber: 2, Title: "Race 2 - W-2x", Lanes: map[int]store.ScheduleEntry{3: {SchoolName: "Lucy"}}},
	}
}

func writeFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), FileName(meta.Name, meta.Date))
	if err := Write(path, meta, fixtureRaces(), publish.Ledger{1: {Revision: "abc123", PublishedAt: publishedAt}}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	return path
}

func TestWrite_Format(t *testing.T) {
	data, err := os.ReadFile(writeFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	var got file
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}

	want := regatta{Name: meta.Name, Date: "2027-05-30", Season: "2027", RegattaKey: "key-2027"}
	if got.SchemaVersion != SchemaVersion || got.Generator != Generator || got.Regatta != want {
		t.Errorf("header = %d %q %+v, want %d %q %+v", got.SchemaVersion, got.Generator, got.Regatta, SchemaVersion, Generator, want)
	}
	if !got.PublishedAt.Equal(publishedAt) {
		t.Errorf("publishedAt = %v, want %v", got.PublishedAt, publishedAt)
	}
	if len(got.Races) != 1 {
		t.Fatalf("races = %d, want only the published race", len(got.Races))
	}
	r := got.Races[0]
	if r.Title != "Race 1 - M-1x - Heat 1" || r.WinningTime != "06:00.0" || r.Revision != "abc123" ||
		r.ApprovedAt == nil || !r.ApprovedAt.Equal(approvedAt) || !r.PublishedAt.Equal(publishedAt) {
		t.Errorf("race = %+v", r)
	}
	if l := r.Lanes["6"]; l.School != "Pig Pen" || l.Status != "SCR" {
		t.Errorf("lane 6 = %+v, want the scratch", l)
	}
	if len(r.Results) != 3 || r.Results[1].AdditionalInfo != "A" || r.Results[2].Place != "DQ" {
		t.Errorf("results = %+v", r.Results)
	}
}

func TestRead_RoundTrip(t *testing.T) {
	got, err := Read(writeFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Exists || got.RegattaKey != "key-2027" {
		t.Errorf("Exists=%v RegattaKey=%q", got.Exists, got.RegattaKey)
	}
	if e := got.Ledger[1]; e.Revision != "abc123" || !e.PublishedAt.Equal(publishedAt) || len(got.Ledger) != 1 {
		t.Errorf("ledger = %+v", got.Ledger)
	}
	if rows := got.Rows[1]; len(rows) != 3 || rows[0] != fixtureRaces()[0].Rows[0] {
		t.Errorf("rows = %+v", rows)
	}
	if err := got.CheckRegatta("key-2028"); !errors.Is(err, publish.ErrOtherRegatta) {
		t.Errorf("CheckRegatta(other) = %v, want ErrOtherRegatta", err)
	}
}

func TestRead_MissingFileIsEmpty(t *testing.T) {
	got, err := Read(filepath.Join(t.TempDir(), "none.json"))
	if err != nil || got.Exists || got.Ledger == nil || got.Rows == nil {
		t.Errorf("Read(missing) = %+v, %v; want empty, non-nil maps", got, err)
	}
}

func TestRead_ForeignFileRefused(t *testing.T) {
	for name, body := range map[string]string{
		"not json":      "<html>",
		"other json":    `{"hello":"world"}`,
		"PoC sample":    `{"schemaVersion":1,"regatta":{"regattaKey":"k"},"races":[]}`,
		"other program": `{"schemaVersion":1,"generator":"someone-else"}`,
	} {
		path := filepath.Join(t.TempDir(), "r.json")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Read(path); !errors.Is(err, publish.ErrForeign) {
			t.Errorf("%s: Read() error = %v, want ErrForeign", name, err)
		}
	}
}

func TestRead_NewerSchemaRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "r.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":2,"generator":"regattaClock"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path); err == nil {
		t.Error("Read() accepted a newer schema version")
	}
}

func TestWrite_CarriedForwardRaceKeepsWinningTime(t *testing.T) {
	// An un-approved race carried forward has its published rows but none of
	// its current result fields.
	carried := publish.PublishableRace{RaceNumber: 3, Rows: []publish.Row{{Place: "2", Time: "07:00.0"}, {Place: "1", Time: "06:50.0"}}}
	path := filepath.Join(t.TempDir(), "r.json")
	if err := Write(path, meta, []publish.PublishableRace{carried}, publish.Ledger{3: {Revision: "r3", PublishedAt: publishedAt}}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	var got file
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if r := got.Races[0]; r.WinningTime != "06:50.0" || r.ApprovedAt != nil {
		t.Errorf("race = %+v, want the first-place time and no approvedAt", r)
	}
}

func TestWrite_RepublishKeepsFileIdentity(t *testing.T) {
	path := writeFixture(t)
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(path, meta, fixtureRaces()[:1], publish.Ledger{}); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Error("republish replaced the file instead of rewriting it in place (a synced file would get a new Drive ID)")
	}
}

func TestWrite_ReadOnlyFolderIsLocked(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions do not block writes here")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	if err := Write(filepath.Join(dir, "r.json"), meta, nil, nil); !errors.Is(err, publish.ErrLocked) {
		t.Errorf("Write(read-only dir) error = %v, want ErrLocked", err)
	}
}

func TestFileName(t *testing.T) {
	tests := []struct{ name, date, want string }{
		{"Charlie Brown Classic", "Saturday, May 30, 2027", "results-2027-charlie-brown-classic.json"},
		{"Fall: \"Head\" Race!", "2026-10-03", "results-2026-fall-head-race.json"},
		{"Spring Sprints", "Spring 2025", "results-2025-spring-sprints.json"},
		{"No Date", "", "results-no-date.json"},
		{"???", "2026-01-01", "results-2026-regatta.json"},
	}
	for _, tt := range tests {
		if got := FileName(tt.name, tt.date); got != tt.want {
			t.Errorf("FileName(%q, %q) = %q, want %q", tt.name, tt.date, got, tt.want)
		}
	}
}

func TestDestination_IsAPublishDestination(t *testing.T) {
	var d publish.Destination = Destination{}
	if d.FileName(meta) != FileName(meta.Name, meta.Date) {
		t.Error("Destination.FileName differs from FileName")
	}
}
