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

	matches, unused := matchRaces(races, entries, nil, nil)
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
	matches, _ := matchRaces(races, entries, nil, nil)
	if matches[0].Status != statusAmbiguous || len(matches[0].Candidates) != 2 {
		t.Fatalf("got %+v, want ambiguous with 2 candidates", matches[0])
	}
}

// TestMatchRacesWidensPoolAndDisambiguatesByRowerName reproduces the real
// scenario that motivated heatsheet.go: the RD combines a small class into a
// race's open lanes for lack of entries (e.g. "event 6, Lane 6 is a M-Jr-1x
// but racing in a M-2x"), so that lane's real RegattaCentral entry belongs to
// a completely different event than the one entriesForRace resolves for the
// rest of the race - the race-scoped pool correctly excludes it. Without any
// heat sheet data this lane would be unmatched; with the rower's last name
// from the Heat Sheet tab, it should resolve to exactly one entry even though
// that school has entries in two other events.
func TestMatchRacesWidensPoolAndDisambiguatesByRowerName(t *testing.T) {
	entries := []rcEntry{
		// Team A and Team C anchor roster overlap cleanly to event "10" for
		// this race (2 schools vs. 1 for any other event, so no tie).
		{ID: "1", EventID: "10", OrgName: "Team A"},
		{ID: "4", EventID: "10", OrgName: "Team C"},
		// Team B's real entry is NOT in event 10 at all - roster overlap
		// correctly excludes it from the race-scoped pool, and it only shows
		// up once that pool is widened back to every entry (see
		// matchRaces's len(cands) == 0 fallback).
		{ID: "2", EventID: "20", OrgName: "Team B", ParticipantNames: []string{"Alex Mihalovich"}},
		{ID: "3", EventID: "30", OrgName: "Team B", ParticipantNames: []string{"Jamie Smith"}},
	}
	races := []reader.RaceData{
		raceWithLanes(6, "M-2x", map[int]reader.RaceEntry{
			1: {SchoolName: "Team A"},
			2: {SchoolName: "Team C"},
			6: {SchoolName: "Team B"}, // combined in on an open lane
		}),
	}
	heatSheet := map[[2]int]heatSheetLane{{6, 6}: {RowerLastName: "Mihalovich"}}

	matches, _ := matchRaces(races, entries, nil, heatSheet)

	byLane := map[int]laneMatch{}
	for _, m := range matches {
		byLane[m.Lane] = m
	}
	if got := byLane[1]; got.Status != statusMatched || got.Candidates[0].ID != "1" {
		t.Errorf("lane 1 = %+v, want matched to entry 1 (event 10)", got)
	}
	if got := byLane[2]; got.Status != statusMatched || got.Candidates[0].ID != "4" {
		t.Errorf("lane 2 = %+v, want matched to entry 4 (event 10)", got)
	}
	if got := byLane[6]; got.Status != statusMatched || got.Candidates[0].ID != "2" {
		t.Errorf("lane 6 = %+v, want matched to entry 2 (Team B / Mihalovich) after widening + rower-name disambiguation", got)
	}
}

func TestDisambiguateByRowerLastName(t *testing.T) {
	cands := []rcEntry{
		{ID: "1", ParticipantNames: []string{"Alex Mihalovich"}},
		{ID: "2", ParticipantNames: []string{"Jamie Smith"}},
	}

	if got := disambiguateByRowerLastName(cands, "Mihalovich"); len(got) != 1 || got[0].ID != "1" {
		t.Errorf("Mihalovich: got %+v, want just entry 1", got)
	}
	if got := disambiguateByRowerLastName(cands, ""); len(got) != 2 {
		t.Errorf("blank last name: got %+v, want cands unchanged", got)
	}
	if got := disambiguateByRowerLastName(cands, "Exhibition"); len(got) != 2 {
		t.Errorf("no participant named Exhibition: got %+v, want cands unchanged", got)
	}
	if got := disambiguateByRowerLastName([]rcEntry{cands[0]}, "Mihalovich"); len(got) != 1 {
		t.Errorf("single candidate: got %+v, want it returned untouched regardless of name", got)
	}
}

func TestDisambiguateByBoatClass(t *testing.T) {
	events := map[string]string{"20": "Junior Men's 1x", "30": "Men's 4+"}
	cands := []rcEntry{
		{ID: "1", EventID: "20", BoatClass: "M-Jr-1x"},
		{ID: "2", EventID: "30"}, // no inline BoatClass - falls back to its event's label
	}

	if got := disambiguateByBoatClass(cands, "M-Jr-1x", events); len(got) != 1 || got[0].ID != "1" {
		t.Errorf("exact BoatClass match: got %+v, want just entry 1", got)
	}
	if got := disambiguateByBoatClass(cands, "Men's 4+", events); len(got) != 1 || got[0].ID != "2" {
		t.Errorf("exact event-label match: got %+v, want just entry 2", got)
	}
	if got := disambiguateByBoatClass(cands, "", events); len(got) != 2 {
		t.Errorf("blank lane class: got %+v, want cands unchanged", got)
	}
	if got := disambiguateByBoatClass(cands, "A", events); len(got) != 2 {
		t.Errorf("an A/B label, not a class: got %+v, want cands unchanged (no coincidental match)", got)
	}
	if got := disambiguateByBoatClass([]rcEntry{cands[0]}, "M-Jr-1x", events); len(got) != 1 {
		t.Errorf("single candidate: got %+v, want it returned untouched", got)
	}

	// Real example: a Heat Sheet cell read "Exhibition M-1-4x" - a local RD
	// decorator sharing the cell with the real class, which RegattaCentral's
	// own text never carries.
	decorated := []rcEntry{
		{ID: "3", BoatClass: "M-1-4x"},
		{ID: "4", BoatClass: "W-1-4x"},
	}
	if got := disambiguateByBoatClass(decorated, "Exhibition M-1-4x", nil); len(got) != 1 || got[0].ID != "3" {
		t.Errorf("decorated lane class: got %+v, want just entry 3 (Exhibition stripped)", got)
	}
}

// TestMatchRacesWidensPoolAndDisambiguatesByBoatClass covers the mash-up case
// via the Heat Sheet's row-2 lane class alone (e.g. a bigger boat with no
// rower name available) - the RD combines a different class into this race's
// open lanes for lack of entries, and the lane's real entry only turns up
// once the pool widens past the race-scoped one.
func TestMatchRacesWidensPoolAndDisambiguatesByBoatClass(t *testing.T) {
	events := map[string]string{"20": "Junior Men's 1x", "30": "Men's 4+"}
	entries := []rcEntry{
		{ID: "1", EventID: "10", OrgName: "Team A"},
		{ID: "4", EventID: "10", OrgName: "Team C"},
		{ID: "2", EventID: "20", OrgName: "Team B", BoatClass: "M-Jr-1x"},
		{ID: "3", EventID: "30", OrgName: "Team B"}, // same school, a different boat entirely
	}
	races := []reader.RaceData{
		raceWithLanes(6, "M-2x", map[int]reader.RaceEntry{
			1: {SchoolName: "Team A"},
			2: {SchoolName: "Team C"},
			6: {SchoolName: "Team B"}, // combined in on an open lane
		}),
	}
	heatSheet := map[[2]int]heatSheetLane{{6, 6}: {LaneClass: "M-Jr-1x"}}

	matches, _ := matchRaces(races, entries, events, heatSheet)

	byLane := map[int]laneMatch{}
	for _, m := range matches {
		byLane[m.Lane] = m
	}
	if got := byLane[6]; got.Status != statusMatched || got.Candidates[0].ID != "2" {
		t.Errorf("lane 6 = %+v, want matched to entry 2 (Team B / M-Jr-1x) after widening + boat-class disambiguation", got)
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

// TestBestMatchingEventScopesByRosterOverlap is a regression test for the
// real failure mode: an xlsm using short boat-class codes ("M-2-8+") that
// share no text with RegattaCentral's event labels, so label matching (the
// events map here is deliberately unhelpful) resolves nothing and every
// school with more than one boat in the whole regatta showed up as
// "ambiguous" in every race it raced in - not just its own. Roster overlap
// must resolve each race to its own event using only which schools raced
// together, with no event-label text involved at all.
func TestBestMatchingEventScopesByRosterOverlap(t *testing.T) {
	entries := []rcEntry{
		{ID: "1", OrgName: "McLean", EventID: "10"},
		{ID: "2", OrgName: "Woodson", EventID: "10"},
		{ID: "3", OrgName: "Colgan", EventID: "10"},
		{ID: "4", OrgName: "Langley", EventID: "10"},
		{ID: "5", OrgName: "McLean", EventID: "20"}, // McLean's other boat, different event
		{ID: "6", OrgName: "Independence", EventID: "20"},
		{ID: "7", OrgName: "Battlefield", EventID: "20"},
	}
	// Labels deliberately don't match either race's short xlsm code, forcing
	// bestMatchingEvent (not matchingEventIDs) to be what resolves this.
	events := map[string]string{"10": "Men's Second Eight", "20": "Women's First Four"}

	race1 := raceWithLanes(1, "M-2-8+", map[int]reader.RaceEntry{
		1: {SchoolName: "McLean"},
		2: {SchoolName: "Woodson"},
		3: {SchoolName: "Colgan"},
		4: {SchoolName: "Langley"},
	})
	pool1 := entriesForRace(entries, race1, events)
	if len(pool1) != 4 {
		t.Fatalf("race1 pool = %+v, want exactly the 4 event-10 entries", pool1)
	}
	for _, e := range pool1 {
		if e.EventID != "10" {
			t.Errorf("race1 pool contains a non-event-10 entry: %+v", e)
		}
	}
	// The real bug: McLean's event-20 boat must not appear as a second
	// "McLean" candidate for a race that is actually event 10's.
	mcLean1 := candidatesFor("McLean", pool1)
	if len(mcLean1) != 1 || mcLean1[0].ID != "1" {
		t.Errorf("race1 McLean candidates = %+v, want exactly entry 1", mcLean1)
	}

	race2 := raceWithLanes(2, "W-1-4+", map[int]reader.RaceEntry{
		1: {SchoolName: "McLean"},
		2: {SchoolName: "Independence"},
		3: {SchoolName: "Battlefield"},
	})
	pool2 := entriesForRace(entries, race2, events)
	if len(pool2) != 3 {
		t.Fatalf("race2 pool = %+v, want exactly the 3 event-20 entries", pool2)
	}
	mcLean2 := candidatesFor("McLean", pool2)
	if len(mcLean2) != 1 || mcLean2[0].ID != "5" {
		t.Errorf("race2 McLean candidates = %+v, want exactly entry 5", mcLean2)
	}
}

func TestBestMatchingEventReturnsBlankOnATieOrNoSignal(t *testing.T) {
	entries := []rcEntry{
		{ID: "1", OrgName: "Alpha", EventID: "10"},
		{ID: "2", OrgName: "Alpha", EventID: "20"},
	}
	// "Alpha" alone matches both events equally - a coin flip, not a
	// confident scoping decision, so this must return "" rather than guess.
	tie := raceWithLanes(1, "X", map[int]reader.RaceEntry{1: {SchoolName: "Alpha"}})
	if id := bestMatchingEvent(entries, tie); id != "" {
		t.Errorf("bestMatchingEvent on a tie = %q, want blank", id)
	}

	// No lanes at all -> no signal.
	if id := bestMatchingEvent(entries, reader.RaceData{}); id != "" {
		t.Errorf("bestMatchingEvent with no lanes = %q, want blank", id)
	}

	// No entries match any of the race's schools -> no signal.
	noMatch := raceWithLanes(1, "X", map[int]reader.RaceEntry{1: {SchoolName: "Nobody Here"}})
	if id := bestMatchingEvent(entries, noMatch); id != "" {
		t.Errorf("bestMatchingEvent with no matching entries = %q, want blank", id)
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
