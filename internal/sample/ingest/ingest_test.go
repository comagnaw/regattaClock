package ingest

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/publish/spreadsheet"
	"github.com/comagnaw/regattaClock/internal/reader"
	"github.com/comagnaw/regattaClock/internal/sample"
)

// realNames are the invented "real" names planted in the test workbook; none
// may survive an ingest.
var realNames = []string{"Chesapeake Bay", "Occoquan", "Potomac", "Lee", "Abernathy-Real", "Quimby", "Real Test Classic"}

// realDay is a hand-kept-style race day: two crews of one school, a double, a
// single's rower in additional info, vocabulary that must be kept, a break
// after race 2, and an empty trailing race.
func realDay() *sample.Fixture {
	return &sample.Fixture{
		Name:       "Real Test Classic",
		BreakAfter: []int{2},
		Races: []sample.Race{
			{RaceNumber: 1, ScheduledTime: "09:00 AM", BoatClass: "W-1x", FlightInfo: "Final", Lanes: map[int]sample.Lane{
				3: {School: "Occoquan", AdditionalInfo: "2", Rowers: "Lee"},
				4: {School: "Potomac A", AdditionalInfo: "Exhibition", Rowers: "Quimby"},
				5: {School: "Potomac B", AdditionalInfo: "W-Jr-2x"},
			}},
			{RaceNumber: 2, ScheduledTime: "09:07 AM", BoatClass: "M-2x", FlightInfo: "Heat 1", Note: "3 to Advance", Lanes: map[int]sample.Lane{
				1: {School: "Chesapeake Bay Rowing", Rowers: "Lee/Abernathy-Real"},
				2: {School: "Potomac", AdditionalInfo: "Heat 1"},
				6: {School: "Occoquan", AdditionalInfo: "SCRATCHED"},
			}},
			{RaceNumber: 3, ScheduledTime: "10:15 AM", BoatClass: "W-4+", FlightInfo: "Final", Lanes: map[int]sample.Lane{
				1: {School: "Occoquan"},
				2: {School: "Chesapeake Bay Rowing"},
			}},
			{RaceNumber: 4, ScheduledTime: "10:22 AM"},
		},
	}
}

// result is one Results-sheet lane: place, and split/time either as Excel
// day fractions (float64) or text.
type result struct {
	place       string
	split, time any
}

// writeWorkbook writes fx as a Heat Sheet plus a Results sheet holding
// results, block per race in race-number order with no break block - the
// hand-kept workbook's shape.
func writeWorkbook(t *testing.T, fx *sample.Fixture, results map[int]map[int]result) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "day.source.xlsm")
	if err := sample.WriteHeatSheet(path, fx, "Saturday, April 18, 2026"); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sheet := common.ResultsSheetName
	if _, err := f.NewSheet(sheet); err != nil {
		t.Fatal(err)
	}
	set := func(col, row int, v any) {
		name, _ := excelize.CoordinatesToCellName(col, row)
		if err := f.SetCellValue(sheet, name, v); err != nil {
			t.Fatal(err)
		}
	}
	set(1, spreadsheet.TitleRow, fx.Name+" Regatta Results")
	for i, race := range fx.Races {
		origin := spreadsheet.BlockOrigin(i)
		set(spreadsheet.RaceNumCol, origin, race.RaceNumber)
		for lane, r := range results[race.RaceNumber] {
			col := spreadsheet.LaneCol(lane)
			set(col, origin+spreadsheet.RowPlace, r.place)
			if r.split != nil {
				set(col, origin+spreadsheet.RowSplit, r.split)
			}
			if r.time != nil {
				set(col, origin+spreadsheet.RowTime, r.time)
			}
		}
	}
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	return path
}

func realResults() map[int]map[int]result {
	return map[int]map[int]result{
		1: {
			3: {"1", 0.0, 0.00490277777777778},                  // 07:03.6
			4: {"3", 0.000195601851851852, 0.00509837962962963}, // 00:16.9, 07:20.5
			5: {"2", "00:08.6", "07:12.2"},                      // text cells
		},
		2: {
			1: {"1", 0.0, 0.00425}, // 06:07.2
			2: {"DNS", nil, nil},
		},
		3: {
			1: {"2", 0.000157407407407407, 0.00440740740740741}, // 00:13.6, 06:20.8
			2: {"1", 0.0, 0.0042488425925925925},                // rounds to 06:07.1
		},
	}
}

func lane(t *testing.T, fx *sample.Fixture, race, n int) sample.Lane {
	t.Helper()
	for _, r := range fx.Races {
		if r.RaceNumber == race {
			l, ok := r.Lanes[n]
			if !ok {
				t.Fatalf("race %d has no lane %d", race, n)
			}
			return l
		}
	}
	t.Fatalf("no race %d", race)
	return sample.Lane{}
}

func TestIngest_ReadsBothSheets(t *testing.T) {
	fx, rep, err := Ingest(writeWorkbook(t, realDay(), realResults()), DefaultSeed)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	if fx.Name != SampleName {
		t.Errorf("name = %q, want %q", fx.Name, SampleName)
	}
	if len(fx.Races) != 4 || rep.Races != 4 || rep.Scheduled != 3 || rep.WithResults != 3 {
		t.Errorf("races = %d, report %+v", len(fx.Races), rep)
	}
	if len(fx.BreakAfter) != 1 || fx.BreakAfter[0] != 2 {
		t.Errorf("BreakAfter = %v, want [2]", fx.BreakAfter)
	}

	for _, c := range []struct {
		race, lane         int
		place, split, time string
	}{
		{1, 3, "1", "00:00.0", "07:03.6"},
		{1, 4, "3", "00:16.9", "07:20.5"},
		{1, 5, "2", "00:08.6", "07:12.2"},
		{2, 2, "DNS", "", ""},
		{3, 1, "2", "00:13.6", "06:20.8"},
		{3, 2, "1", "00:00.0", "06:07.1"},
	} {
		l := lane(t, fx, c.race, c.lane)
		if l.Place != c.place || l.Split != c.split || l.Time != c.time {
			t.Errorf("race %d lane %d = %s/%s/%s, want %s/%s/%s",
				c.race, c.lane, l.Place, l.Split, l.Time, c.place, c.split, c.time)
		}
	}

	// Results are joined by race number: the Heat Sheet's break block shifts
	// race 3 down a block there but not on the Results sheet.
	if l := lane(t, fx, 3, 2); l.Place != "1" {
		t.Errorf("race 3 lane 2 place = %q - results joined by block position?", l.Place)
	}

	if s := lane(t, fx, 2, 6); s.Status != "SCR" || s.AdditionalInfo != "SCRATCHED" {
		t.Errorf("scratched lane = %+v", s)
	}
	if r := fx.Races[3]; r.HasBoats() || r.ScheduledTime != "10:22 AM" {
		t.Errorf("empty trailing race = %+v", r)
	}
}

func TestIngest_ObfuscatesNamesKeepsVocabulary(t *testing.T) {
	fx, _, err := Ingest(writeWorkbook(t, realDay(), realResults()), DefaultSeed)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	out, _ := fx.Marshal()
	for _, name := range realNames {
		if regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(name) + `\b`).Match(out) {
			t.Errorf("real name %q survives in the fixture", name)
		}
	}

	// One school, one fake - its crews keep their designators.
	a, b, plain := lane(t, fx, 1, 4).School, lane(t, fx, 1, 5).School, lane(t, fx, 2, 2).School
	if a != plain+" A" || b != plain+" B" {
		t.Errorf("Potomac A/B/plain = %q / %q / %q, want one fake with A and B", a, b, plain)
	}
	if lane(t, fx, 1, 3).School != lane(t, fx, 3, 1).School {
		t.Error("the same school got two different fakes")
	}

	// Rowers map per name, the same name to the same fake.
	single := lane(t, fx, 1, 3).Rowers
	double := strings.Split(lane(t, fx, 2, 1).Rowers, "/")
	if len(double) != 2 || double[0] != single || double[1] == "" {
		t.Errorf("rowers: single %q, double %v - want Lee mapped once, pair kept", single, double)
	}

	for _, c := range []struct {
		race, lane int
		want       string
	}{
		{1, 3, "2"}, {1, 4, "Exhibition"}, {1, 5, "W-Jr-2x"}, {2, 2, "Heat 1"},
	} {
		if got := lane(t, fx, c.race, c.lane).AdditionalInfo; got != c.want {
			t.Errorf("race %d lane %d additional info = %q, want %q kept", c.race, c.lane, got, c.want)
		}
	}
	if fx.Races[1].Note != "3 to Advance" {
		t.Errorf("note = %q, want kept", fx.Races[1].Note)
	}
}

func TestIngest_Deterministic(t *testing.T) {
	path := writeWorkbook(t, realDay(), realResults())
	marshal := func(seed int64) string {
		fx, _, err := Ingest(path, seed)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := fx.Marshal()
		return string(b)
	}
	if marshal(1) != marshal(1) {
		t.Error("same seed gave different fixtures")
	}
	if marshal(1) == marshal(2) {
		t.Error("different seeds gave the same fixture")
	}
}

func TestIngest_LeakCheckTrips(t *testing.T) {
	day := realDay()
	day.Races[0].Note = "Potomac advances" // a real name in a kept-verbatim field
	_, _, err := Ingest(writeWorkbook(t, day, realResults()), DefaultSeed)
	if err == nil || !strings.Contains(err.Error(), "leak check") {
		t.Fatalf("err = %v, want a leak-check failure", err)
	}
}

func TestIngest_FixtureGenerates(t *testing.T) {
	fx, _, err := Ingest(writeWorkbook(t, realDay(), realResults()), DefaultSeed)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := fx.Marshal()
	parsed, err := sample.Parse(b)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	rep, err := sample.Generate(sample.Options{Dir: filepath.Join(t.TempDir(), "s"), Unraced: 1, Fixture: parsed})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if rep.Raced != 2 || rep.FirstUnraced != 3 {
		t.Errorf("report = %+v, want races 1-2 raced, 3 un-raced", rep)
	}
}

func TestIngest_ExampleWorkbook(t *testing.T) {
	path := filepath.Join("..", "..", "..", "examples", "Example Heat Sheets and Results With Macros.xlsm")
	rd, err := reader.ReadExcelFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fx, rep, err := Ingest(path, DefaultSeed)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	// The public example has no results filled in.
	if len(fx.Races) != len(rd.Races) || rep.WithResults != 0 || len(rep.NoResults) != rd.ScheduledRaces() {
		t.Errorf("races %d (reader %d), report %+v", len(fx.Races), len(rd.Races), rep)
	}
}

func TestIsNonName(t *testing.T) {
	for s, want := range map[string]bool{
		"": true, "3": true, "2V": true, "Heat 3": true, "Exhibition": true, "SCRATCHED": true,
		"W-Jr-2x": true, "M-2x Exhibition": true, "Exhibition  W-Jr-2x": true, "M-1-8+": true,
		"Smith": false, "Lee": false, "Heat Smith": false,
	} {
		if got := isNonName(s); got != want {
			t.Errorf("isNonName(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestSplitSchool(t *testing.T) {
	for in, want := range map[string][2]string{
		"Potomac A":      {"Potomac", "A"},
		"Fairfax 2V":     {"Fairfax", "2V"},
		"Briar Woods":    {"Briar Woods", ""},
		"A":              {"A", ""},
		"South County B": {"South County", "B"},
	} {
		base, des := splitSchool(in)
		if base != want[0] || des != want[1] {
			t.Errorf("splitSchool(%q) = %q, %q; want %q, %q", in, base, des, want[0], want[1])
		}
	}
}

func TestLeakSet(t *testing.T) {
	l := newLeakSet()
	l.add("Lee")
	l.add("West Springfield")
	for s, want := range map[string]bool{
		"Lee": true, "Lee/Smith": true, "Leesburg Academy": false,
		"springfield prep": true, "West Haven": false, "Heat 1": false,
	} {
		if got := l.contains(s); got != want {
			t.Errorf("contains(%q) = %v, want %v", s, got, want)
		}
	}
}
