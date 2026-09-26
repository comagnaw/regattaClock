package sample

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/comagnaw/regattaClock/internal/persona"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/publish"
	"github.com/comagnaw/regattaClock/internal/publish/spreadsheet"
	"github.com/comagnaw/regattaClock/internal/reader"
)

var testDay = time.Date(2026, time.September, 26, 0, 0, 0, 0, time.Local)

func generate(t *testing.T, unraced int) (string, Report) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "sample")
	rep, err := Generate(Options{Dir: dir, Date: testDay, Unraced: unraced, Now: testDay.Add(8 * time.Hour)})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return dir, rep
}

func sessionIn(dir string, role persona.Role, team persona.Team) persona.Session {
	root := filepath.Join(dir, RegattaDataDirName)
	if role == persona.RoleDirector {
		return persona.Session{Definition: persona.DirectorDefinition, Root: root}
	}
	return persona.Session{Definition: definitionFor(role, team), Root: root}
}

func TestEmbeddedFixtureParses(t *testing.T) {
	fx, err := Embedded()
	if err != nil {
		t.Fatalf("Embedded: %v", err)
	}
	if fx.Name == "" || len(fx.Races) == 0 {
		t.Fatalf("embedded fixture is empty: %+v", fx)
	}
}

func TestGenerate_WritesOnlyUnderDir(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "sample")
	rep, err := Generate(Options{Dir: dir, Date: testDay, Unraced: 2})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	top, _ := os.ReadDir(parent)
	if len(top) != 1 || top[0].Name() != "sample" {
		t.Fatalf("parent holds %v, want only sample/", top)
	}

	var got []string
	err = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(dir, p)
			got = append(got, filepath.ToSlash(rel))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)
	want := []string{
		"Sample Heat Sheet.xlsx",
		"regattaData/director/regattaSchedule.json",
		"regattaData/timing/primary/finish.json",
		"regattaData/timing/primary/start.json",
		"regattaData/timing/secondary/finish.json",
		"regattaData/timing/secondary/start.json",
		"results/" + spreadsheet.FileName("Sample Regatta Day", "Saturday, September 26, 2026"),
	}
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("files = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("files = %v, want %v", got, want)
		}
	}
	if len(rep.Artifacts) != len(want) {
		t.Errorf("report lists %d artifacts, want %d", len(rep.Artifacts), len(want))
	}
}

func TestGenerate_RefusesNonEmptyDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "stale.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(Options{Dir: dir, Date: testDay}); !errors.Is(err, ErrDirNotEmpty) {
		t.Fatalf("err = %v, want ErrDirNotEmpty", err)
	}
}

func TestGenerate_RefusesNegativeUnraced(t *testing.T) {
	if _, err := Generate(Options{Dir: t.TempDir(), Unraced: -1}); err == nil {
		t.Fatal("negative Unraced accepted")
	}
}

func TestGenerate_TreeLoadsAndAgrees(t *testing.T) {
	dir, rep := generate(t, 2)

	sch, err := store.LoadSchedule(sessionIn(dir, persona.RoleDirector, ""))
	if err != nil {
		t.Fatalf("LoadSchedule: %v", err)
	}
	key := sch.Key()
	if sch.Date != "Saturday, September 26, 2026" {
		t.Errorf("schedule date = %q", sch.Date)
	}

	heat := filepath.Join(dir, HeatSheetFile)
	if sch.Origin.Type != "excel" || sch.Origin.URI != heat {
		t.Errorf("origin = %+v, want excel at %s", sch.Origin, heat)
	}

	fx, _ := Embedded()
	if len(sch.Races) != len(fx.Races) {
		t.Errorf("schedule has %d races, fixture %d", len(sch.Races), len(fx.Races))
	}

	var raced, unraced []int
	for _, r := range fx.Races {
		if r.HasBoats() {
			raced = append(raced, r.RaceNumber)
		}
	}
	raced, unraced = raced[:len(raced)-2], raced[len(raced)-2:]
	if rep.Raced != len(raced) || rep.Unraced != 2 || rep.FirstUnraced != unraced[0] {
		t.Errorf("report = %+v, want raced %d, un-raced 2 from race %d", rep, len(raced), unraced[0])
	}

	laneHash := map[int]string{}
	for _, r := range sch.Races {
		laneHash[r.RaceNumber] = r.LaneMapHash()
	}

	for _, team := range []persona.Team{persona.TeamPrimary, persona.TeamSecondary} {
		start, err := store.LoadStart(sessionIn(dir, persona.RoleStart, team))
		if err != nil {
			t.Fatalf("LoadStart(%s): %v", team, err)
		}
		fin, err := store.LoadFinish(sessionIn(dir, persona.RoleFinish, team))
		if err != nil {
			t.Fatalf("LoadFinish(%s): %v", team, err)
		}
		for _, env := range []store.Envelope{start.Envelope, fin.Envelope} {
			if env.RegattaKey != key || env.Team != team || env.Version != store.SchemaVersion {
				t.Errorf("%s envelope = %+v, want key %s", team, env, key)
			}
		}

		want := store.StateApproved
		if team == persona.TeamSecondary {
			want = store.StateSaved
		}
		for _, n := range raced {
			res := fin.Races[n]
			if got := store.DeriveTeamState(start.Races[n], res); got != want {
				t.Errorf("%s race %d state = %v, want %v", team, n, got, want)
			}
			if res.LaneMapHash != laneHash[n] {
				t.Errorf("%s race %d lane-map hash %q, schedule %q (stale dagger)", team, n, res.LaneMapHash, laneHash[n])
			}
		}
		for _, n := range unraced {
			if got := store.DeriveTeamState(start.Races[n], fin.Races[n]); got != store.StateNotStarted {
				t.Errorf("%s race %d state = %v, want NotStarted", team, n, got)
			}
		}
	}
}

func TestGenerate_HeatSheetRoundTrips(t *testing.T) {
	dir, _ := generate(t, DefaultUnraced)

	sch, err := store.LoadSchedule(sessionIn(dir, persona.RoleDirector, ""))
	if err != nil {
		t.Fatal(err)
	}
	rd, err := reader.ReadExcelFile(filepath.Join(dir, HeatSheetFile))
	if err != nil {
		t.Fatalf("ReadExcelFile: %v", err)
	}
	if got, want := store.ScheduleFromRegattaData(rd).ContentHash(), sch.ContentHash(); got != want {
		t.Errorf("heat sheet content hash %s, schedule %s", got, want)
	}

	// The fixture's break must not swallow or renumber a race.
	fx, _ := Embedded()
	for _, r := range fx.Races {
		found := false
		for _, got := range rd.Races {
			if got.RaceNumber == r.RaceNumber {
				found = true
				if got.ScheduledTime != r.ScheduledTime || got.BoatClass != r.BoatClass || len(got.Lanes) != len(r.Lanes) {
					t.Errorf("race %d read back as %+v", r.RaceNumber, got)
				}
			}
		}
		if !found {
			t.Errorf("race %d missing from the heat sheet", r.RaceNumber)
		}
	}
}

func TestGenerate_ResultsWorkbookPublished(t *testing.T) {
	dir, rep := generate(t, 2)

	sch, _ := store.LoadSchedule(sessionIn(dir, persona.RoleDirector, ""))
	fin, err := store.LoadFinish(sessionIn(dir, persona.RoleFinish, persona.TeamPrimary))
	if err != nil {
		t.Fatal(err)
	}

	resultsDir := filepath.Join(dir, ResultsDirName)
	if fin.ResultsDir != resultsDir || !filepath.IsAbs(fin.ResultsDir) {
		t.Errorf("ResultsDir = %q, want absolute %q", fin.ResultsDir, resultsDir)
	}

	pub, err := spreadsheet.Read(filepath.Join(resultsDir, spreadsheet.FileName(sch.Name, sch.Date)))
	if err != nil {
		t.Fatalf("spreadsheet.Read: %v", err)
	}
	if err := pub.CheckRegatta(sch.Key()); err != nil {
		t.Fatalf("CheckRegatta: %v", err)
	}
	if len(pub.Ledger) != rep.Raced {
		t.Errorf("ledger holds %d races, want %d", len(pub.Ledger), rep.Raced)
	}
	for _, pr := range publish.BuildView(sch, fin) {
		entry, ok := pub.Ledger[pr.RaceNumber]
		if !ok || entry.Revision != pr.Revision {
			t.Errorf("race %d ledger = %+v, want revision %s", pr.RaceNumber, entry, pr.Revision)
		}
	}
	if _, ok := pub.Ledger[rep.FirstUnraced]; ok {
		t.Errorf("un-raced race %d is in the ledger", rep.FirstUnraced)
	}
}

func TestGenerate_Deterministic(t *testing.T) {
	a, _ := generate(t, DefaultUnraced)
	b, _ := generate(t, DefaultUnraced)
	for _, rel := range []string{
		"regattaData/director/regattaSchedule.json",
		"regattaData/timing/primary/start.json",
		"regattaData/timing/primary/finish.json",
		"regattaData/timing/secondary/start.json",
		"regattaData/timing/secondary/finish.json",
	} {
		// Each run's own directory is in the absolute paths it records
		// (Origin.URI, ResultsDir); everything else must match byte for byte.
		x, _ := os.ReadFile(filepath.Join(a, rel))
		y, _ := os.ReadFile(filepath.Join(b, rel))
		if withoutDir(x, a) != withoutDir(y, b) {
			t.Errorf("%s differs between runs", rel)
		}
	}
}

// withoutDir is a JSON artifact's text with dir (as JSON-escaped) replaced
// by a placeholder.
func withoutDir(b []byte, dir string) string {
	esc, _ := json.Marshal(dir)
	return strings.ReplaceAll(string(b), strings.Trim(string(esc), `"`), "<dir>")
}

func TestFinishRows_Order(t *testing.T) {
	race := Race{Lanes: map[int]Lane{
		1: {Place: "DNS"},
		2: {Place: "3", Split: "00:05.0", Time: "06:05.0"},
		3: {Place: "1", Split: "00:00.0", Time: "06:00.0"},
		4: {Place: "Excluded", Split: "00:09.0", Time: "06:09.0"},
		5: {Place: "3", Split: "00:05.0", Time: "06:05.0"}, // dead heat with lane 2
		6: {Place: "2", Split: "00:02.0", Time: "06:02.0"},
	}}
	want := []int{3, 6, 2, 5, 1, 4}
	for range 20 { // map iteration order varies run to run
		rows, err := finishRows(race)
		if err != nil {
			t.Fatal(err)
		}
		for i, r := range rows {
			if r.Lane != want[i] {
				t.Fatalf("lane order = %v, want %v", lanesOf(rows), want)
			}
		}
	}
}

func lanesOf(rows []store.LapRow) []int {
	out := make([]int, len(rows))
	for i, r := range rows {
		out[i] = r.Lane
	}
	return out
}

func TestParseClock(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"00:00.0": 0,
		"07:03.6": 7*time.Minute + 3600*time.Millisecond,
		"12:59.9": 12*time.Minute + 59900*time.Millisecond,
	} {
		got, err := ParseClock(in)
		if err != nil || got != want {
			t.Errorf("ParseClock(%q) = %v, %v; want %v", in, got, err, want)
		}
		if FormatClock(got) != in {
			t.Errorf("FormatClock(%v) = %q, want %q", got, FormatClock(got), in)
		}
	}
	for _, bad := range []string{"", "7:3", "07:03", "07:63.0", "aa:bb.c", "07:03.65"} {
		if _, err := ParseClock(bad); err == nil {
			t.Errorf("ParseClock(%q) accepted", bad)
		}
	}
}
