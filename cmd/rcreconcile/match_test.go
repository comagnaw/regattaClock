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

	matches, unused := matchRaces(races, entries, nil)
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
	matches, _ := matchRaces(races, entries, nil)
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
	if got := normalize("St. Mary's H.S.!"); got != "saintmaryshs" {
		t.Errorf("normalize = %q, want saintmaryshs", got)
	}
}

// TestStVsSaintRealSchools is a regression test for two real schools at one
// regatta: RegattaCentral spells the org "St.", the xlsm spells it "Saint".
// The fix must resolve both without confusing them with each other - "St.
// Johns College HS" and "St. John Paul" stay distinguishable after "st"
// expands to "saint", even though both start "Saint John".
func TestStVsSaintRealSchools(t *testing.T) {
	pool := []rcEntry{
		{ID: "1", OrgName: "St. Johns College HS"},
		{ID: "2", OrgName: "St. John Paul"},
	}

	tests := []struct {
		school   string
		wantID   string
		wantOnly bool // true: must match wantID and nothing else
	}{
		{"Saint John's", "1", true},
		{"Saint John Paul", "2", true},
	}
	for _, tc := range tests {
		got := candidatesFor(tc.school, pool)
		if len(got) != 1 || got[0].ID != tc.wantID {
			t.Errorf("candidatesFor(%q) = %+v, want exactly entry %s", tc.school, got, tc.wantID)
		}
	}
}

// TestShortAbbreviationDoesNotFalseMatch is a regression test for a real
// mismatch: "Bishop Ireton" (xlsm) was showing "Osbourn Park" as a candidate.
// normalize("Bishop Ireton") contains "op" (from "bish-OP"), and "OP" is a
// plausible abbreviation for "Osbourn Park" - a coincidental substring match,
// not a real one. Below minSubstringMatchLen, only an exact match counts.
func TestShortAbbreviationDoesNotFalseMatch(t *testing.T) {
	pool := []rcEntry{
		{ID: "1", OrgName: "Bishop Ireton HS"},
		{ID: "2", OrgName: "Osbourn Park", OrgAbbrev: "OP"},
	}
	got := candidatesFor("Bishop Ireton", pool)
	if len(got) != 1 || got[0].ID != "1" {
		t.Errorf("candidatesFor(%q) = %+v, want exactly entry 1 (Osbourn Park's \"OP\" must not coincidentally match)", "Bishop Ireton", got)
	}

	// The short form still matches when it is genuinely an exact query.
	got = candidatesFor("OP", pool)
	if len(got) != 1 || got[0].ID != "2" {
		t.Errorf("candidatesFor(%q) = %+v, want exactly entry 2 (exact match still works)", "OP", got)
	}
}

func TestMatchingEventIDs(t *testing.T) {
	events := map[string]string{
		"10": "Varsity 8",
		"11": "Junior Varsity 8",
	}
	race := reader.RaceData{BoatClass: "Varsity 8"}
	ids := matchingEventIDs(race, events)
	if len(ids) != 1 || !ids["10"] {
		t.Errorf("matchingEventIDs = %v, want just {10}", ids)
	}

	if got := matchingEventIDs(reader.RaceData{BoatClass: "Novice 4"}, events); len(got) != 0 {
		t.Errorf("matchingEventIDs for an unresolvable class = %v, want none", got)
	}
}

func TestEntriesForRaceScopesByEventAndFallsBack(t *testing.T) {
	entries := []rcEntry{
		{ID: "1", OrgName: "Springfield High School", EventID: "10"}, // Varsity 8
		{ID: "2", OrgName: "Springfield High School", EventID: "11"}, // JV 8 - different event
		{ID: "3", OrgName: "Shelbyville", EventID: "10"},
	}
	events := map[string]string{"10": "Varsity 8", "11": "Junior Varsity 8"}

	// Without event-scoping (Milestone 1.6 behavior), entry 2 leaks into every
	// race regardless of its own event - this is exactly the bug Issue B
	// described. With events supplied, it must not.
	pool := entriesForRace(entries, reader.RaceData{BoatClass: "Varsity 8"}, events)
	for _, e := range pool {
		if e.ID == "2" {
			t.Errorf("entry 2 (JV 8) leaked into the Varsity 8 pool: %+v", pool)
		}
	}
	if len(pool) != 2 {
		t.Errorf("pool = %+v, want entries 1 and 3 only", pool)
	}

	// A race whose class matches no known event falls back to the full pool
	// (entriesForBoatClass's own "keep everything, blank BoatClass" behavior)
	// rather than returning zero candidates.
	fallback := entriesForRace(entries, reader.RaceData{BoatClass: "Novice 4"}, events)
	if len(fallback) != len(entries) {
		t.Errorf("fallback pool = %+v, want all %d entries", fallback, len(entries))
	}

	// No events index at all (e.g. events.json was never captured) must also
	// fall back rather than erroring or returning nothing.
	if got := entriesForRace(entries, reader.RaceData{BoatClass: "Varsity 8"}, nil); len(got) != len(entries) {
		t.Errorf("nil events pool = %+v, want all %d entries", got, len(entries))
	}
}
