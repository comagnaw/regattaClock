package main

import (
	"testing"

	"github.com/comagnaw/regattaClock/internal/reader"
)

// raceWithLanes builds a synthetic reader.RaceData the way the Excel loader
// would, without touching a real workbook.
func raceWithLanes(num int, boatClass string, lanes map[int]reader.RaceEntry) reader.RaceData {
	rd := reader.RaceData{RaceNumber: num, BoatClass: boatClass, Lanes: map[int]reader.RaceEntry{}}
	for lane, e := range lanes {
		rd.Lanes[lane] = e
		rd.BoatCount++
	}
	return rd
}

func TestMatchRaces(t *testing.T) {
	entries := []rcEntry{
		{ID: "1", OrgName: "Springfield High School", OrgAbbrev: "SHS", BoatClass: "Varsity 8", Label: "A"},
		{ID: "2", OrgName: "Springfield High School", OrgAbbrev: "SHS", BoatClass: "Varsity 8", Label: "B"},
		{ID: "3", OrgName: "Shelbyville Rowing Club", OrgShortName: "Shelbyville", BoatClass: "Varsity 8"},
		{ID: "4", OrgName: "Ogdenville Composite", BoatClass: "Junior Varsity 8"}, // never used by any lane
	}

	races := []reader.RaceData{
		raceWithLanes(1, "Varsity 8", map[int]reader.RaceEntry{
			1: {SchoolName: "Springfield HS", AdditionalInfo: "A"},  // exact-ish + label -> matched to 1
			2: {SchoolName: "Springfield HS", AdditionalInfo: "B"},  // matched to 2
			3: {SchoolName: "Shelbyville", AdditionalInfo: ""},      // partial match -> matched to 3
			4: {SchoolName: "North Haverbrook", AdditionalInfo: ""}, // no RC entry -> unmatched
		}),
	}

	matches, unused := matchRaces(races, entries)
	if len(matches) != 4 {
		t.Fatalf("matches = %d, want 4", len(matches))
	}

	byLane := map[int]laneMatch{}
	for _, m := range matches {
		byLane[m.Lane] = m
	}

	if got := byLane[1]; got.Status != statusMatched || got.Candidates[0].ID != "1" {
		t.Errorf("lane 1 = %+v, want matched to entry 1", got)
	}
	if got := byLane[2]; got.Status != statusMatched || got.Candidates[0].ID != "2" {
		t.Errorf("lane 2 = %+v, want matched to entry 2", got)
	}
	if got := byLane[3]; got.Status != statusMatched || got.Candidates[0].ID != "3" {
		t.Errorf("lane 3 = %+v, want matched to entry 3", got)
	}
	if got := byLane[4]; got.Status != statusUnmatched {
		t.Errorf("lane 4 = %+v, want unmatched", got)
	}

	if len(unused) != 1 || unused[0].ID != "4" {
		t.Errorf("unused = %+v, want just entry 4 (Ogdenville, never referenced)", unused)
	}
}

func TestMatchRacesAmbiguousWithoutLabel(t *testing.T) {
	entries := []rcEntry{
		{ID: "1", OrgName: "Springfield High School", BoatClass: "Varsity 8", Label: "A"},
		{ID: "2", OrgName: "Springfield High School", BoatClass: "Varsity 8", Label: "B"},
	}
	races := []reader.RaceData{
		raceWithLanes(1, "Varsity 8", map[int]reader.RaceEntry{
			1: {SchoolName: "Springfield HS", AdditionalInfo: ""}, // two candidates, no label to disambiguate
		}),
	}
	matches, _ := matchRaces(races, entries)
	if matches[0].Status != statusAmbiguous || len(matches[0].Candidates) != 2 {
		t.Fatalf("got %+v, want ambiguous with 2 candidates", matches[0])
	}
}

func TestCandidatesForNormalization(t *testing.T) {
	pool := []rcEntry{{ID: "1", OrgName: "Saint Mary High School", OrgAbbrev: "SMHS"}}

	tests := []struct {
		school string
		want   int
	}{
		{"Saint Mary HS", 1},          // "HS" expands to match the spelled-out org name
		{"saint mary high school", 1}, // exact, case-insensitive
		{"SMHS", 1},                   // matches the abbreviation field directly
		{"Totally Different School", 0},
		{"", 0},
	}
	for _, tc := range tests {
		got := candidatesFor(tc.school, pool)
		if len(got) != tc.want {
			t.Errorf("candidatesFor(%q) = %d candidates, want %d", tc.school, len(got), tc.want)
		}
	}
}

func TestExtractBoatLabel(t *testing.T) {
	tests := map[string]string{
		"A":        "A",
		"  b  ":    "B",
		"Boat A":   "A",
		"Coxswain": "", // no isolated single letter
		"":         "",
		"Jane Doe": "",
	}
	for in, want := range tests {
		if got := extractBoatLabel(in); got != want {
			t.Errorf("extractBoatLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalize(t *testing.T) {
	if got := normalize("St. Mary's H.S.!"); got != "stmaryshs" {
		t.Errorf("normalize = %q, want stmaryshs", got)
	}
}
