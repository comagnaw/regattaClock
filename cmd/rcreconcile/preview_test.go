package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/regattacentral"
)

func TestParseRaceTime(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"6:12.5", 6*time.Minute + 12*time.Second + 500*time.Millisecond, true},
		{"0:00.0", 0, true},
		{" 6:12.5 ", 6*time.Minute + 12*time.Second + 500*time.Millisecond, true},
		{"", 0, false},
		{"DNF", 0, false},
		{"6.5", 0, false},
		{"a:12.5", 0, false},
	}
	for _, tt := range tests {
		got, ok := parseRaceTime(tt.in)
		if ok != tt.ok || (ok && got != tt.want) {
			t.Errorf("parseRaceTime(%q) = %v, %v; want %v, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestLaneStatusForPlace(t *testing.T) {
	tests := map[string]regattacentral.LaneStatus{
		common.RaceDisqualification: regattacentral.LaneDisqualified,
		common.RaceDidNotFinish:     regattacentral.LaneDidNotFinish,
		common.RaceDidNotStart:      regattacentral.LaneDidNotStart,
		"1":                         regattacentral.LaneOK,
		"":                          regattacentral.LaneOK,
	}
	for place, want := range tests {
		if got := laneStatusForPlace(place); got != want {
			t.Errorf("laneStatusForPlace(%q) = %q, want %q", place, got, want)
		}
	}
}

func TestLaneStatusCombinesPlaceAndExhibition(t *testing.T) {
	tests := []struct {
		place, laneClass string
		want             regattacentral.LaneStatus
	}{
		{"", "Exhibition M-1-4x", regattacentral.LaneExhibition},
		{"", "M-1-4x", regattacentral.LaneOK}, // a real class, no "Exhibition" decorator
		{"", "", regattacentral.LaneOK},
		// A real outcome takes priority over a pre-race Exhibition designation.
		{common.RaceDidNotFinish, "Exhibition M-1-4x", regattacentral.LaneDidNotFinish},
	}
	for _, tt := range tests {
		if got := laneStatus(tt.place, tt.laneClass); got != tt.want {
			t.Errorf("laneStatus(%q, %q) = %q, want %q", tt.place, tt.laneClass, got, tt.want)
		}
	}
}

func TestIsExhibitionLane(t *testing.T) {
	tests := map[string]bool{
		"Exhibition M-1-4x": true,
		"exhibition":        true,
		"M-1-4x":            false,
		"":                  false,
		"A":                 false,
	}
	for laneClass, want := range tests {
		if got := isExhibitionLane(laneClass); got != want {
			t.Errorf("isExhibitionLane(%q) = %v, want %v", laneClass, got, want)
		}
	}
}

func TestNewPlaceholderUUIDLooksLikeAUUIDAndVaries(t *testing.T) {
	a := newPlaceholderUUID()
	b := newPlaceholderUUID()
	if a == b {
		t.Fatalf("two placeholder UUIDs were identical: %q", a)
	}
	for _, u := range []string{a, b} {
		parts := strings.Split(u, "-")
		if len(parts) != 5 {
			t.Errorf("newPlaceholderUUID() = %q, want 5 hyphen-separated groups", u)
		}
	}
}

// allLanes flattens a preview's nested events -> races -> lanes tree for
// assertions that don't care about the event/race grouping itself.
func allLanes(req *regattacentral.UploadRequest) []regattacentral.LaneRecord {
	var out []regattacentral.LaneRecord
	for _, ev := range req.Events {
		for _, race := range ev.Races {
			out = append(out, race.Lanes...)
		}
	}
	return out
}

func TestBuildUploadPreviewMatchedLaneGetsRealEntryID(t *testing.T) {
	matches := []laneMatch{
		{
			RaceNumber: 1, Lane: 1, SchoolName: "Springfield High School",
			Place: "1", Time: "6:12.5",
			Status:     statusMatched,
			Candidates: []rcEntry{{ID: "4821", EventID: "100"}},
		},
	}
	req, warnings := buildUploadPreview(matches)
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none for a confidently matched lane", warnings)
	}
	lanes := allLanes(req)
	if len(lanes) != 1 || lanes[0].EntryID != 4821 || lanes[0].UUID != "" {
		t.Errorf("lane = %+v, want EntryID 4821 and no placeholder UUID", lanes[0])
	}
	if len(lanes[0].Results) != 1 || lanes[0].Results[0].Time != (6*time.Minute+12*time.Second+500*time.Millisecond).Milliseconds() {
		t.Errorf("results = %+v, want one finish result matching the parsed time", lanes[0].Results)
	}
	if len(req.Events) != 1 || len(req.Events[0].Races) != 1 || req.Events[0].Races[0].Status != regattacentral.StatusOfficial {
		t.Errorf("events = %+v, want race 1 marked Official since it has a result", req.Events)
	}
}

func TestBuildUploadPreviewCarriesExhibitionFromHeatSheet(t *testing.T) {
	matches := []laneMatch{
		{
			RaceNumber: 6, Lane: 6, SchoolName: "Justice High",
			LaneClass:  "Exhibition M-1-4x",
			Status:     statusMatched,
			Candidates: []rcEntry{{ID: "122", EventID: "100"}},
		},
	}
	req, _ := buildUploadPreview(matches)
	lanes := allLanes(req)
	if len(lanes) != 1 || lanes[0].Status != regattacentral.LaneExhibition {
		t.Errorf("lane = %+v, want Status LaneExhibition", lanes[0])
	}
}

func TestBuildUploadPreviewGuessedLaneGetsRealEntryIDAndAWarning(t *testing.T) {
	matches := []laneMatch{
		{
			RaceNumber: 8, Lane: 3, SchoolName: "Justice High",
			Status:     statusGuessed,
			Candidates: []rcEntry{{ID: "64", EventID: "100"}, {ID: "65", EventID: "100"}},
		},
	}
	req, warnings := buildUploadPreview(matches)
	lanes := allLanes(req)
	if len(lanes) != 1 || lanes[0].EntryID != 64 || lanes[0].UUID != "" {
		t.Errorf("lane = %+v, want the picked EntryID 64 and no placeholder UUID", lanes[0])
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "not a confident match") {
		t.Errorf("warnings = %v, want one flagging this as a guess, not a confident match", warnings)
	}
}

func TestBuildUploadPreviewAmbiguousAndUnmatchedGetPlaceholders(t *testing.T) {
	matches := []laneMatch{
		{
			RaceNumber: 2, Lane: 3, SchoolName: "Shelbyville Rowing Club",
			Status:     statusAmbiguous,
			Candidates: []rcEntry{{ID: "1", EventID: "100"}, {ID: "2", EventID: "100"}},
		},
		{
			RaceNumber: 2, Lane: 4, SchoolName: "Ogdenville Composite",
			Status: statusUnmatched,
		},
	}
	req, warnings := buildUploadPreview(matches)
	if len(warnings) != 2 {
		t.Fatalf("warnings = %v, want one per unresolved lane", warnings)
	}
	lanes := allLanes(req)
	for _, l := range lanes {
		if l.EntryID != 0 || l.UUID == "" {
			t.Errorf("lane %+v: want a placeholder UUID and no real EntryID", l)
		}
		if len(l.Results) != 0 {
			t.Errorf("lane %+v: results = %v, want none - neither lane had a finish time", l, l.Results)
		}
	}
}

func TestBuildUploadPreviewNonNumericEntryIDFallsBackToPlaceholder(t *testing.T) {
	matches := []laneMatch{
		{
			RaceNumber: 1, Lane: 1, SchoolName: "Springfield High School",
			Status:     statusMatched,
			Candidates: []rcEntry{{ID: "not-a-number", EventID: "100"}},
		},
	}
	req, warnings := buildUploadPreview(matches)
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want one flagging the non-numeric id", warnings)
	}
	lanes := allLanes(req)
	if len(lanes) != 1 || lanes[0].EntryID != 0 || lanes[0].UUID == "" {
		t.Errorf("lane = %+v, want a placeholder UUID since the real id isn't numeric", lanes[0])
	}
}

func TestWriteUploadPreviewProducesHumanReadableAndJSON(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/preview.txt"
	matches := []laneMatch{
		{
			RaceNumber: 1, Lane: 1, SchoolName: "Springfield High School",
			Time:       "6:12.5",
			Status:     statusMatched,
			Candidates: []rcEntry{{ID: "4821"}},
		},
	}
	req, warnings := buildUploadPreview(matches)
	if err := writeUploadPreview(path, req, warnings); err != nil {
		t.Fatalf("writeUploadPreview: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read preview: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, "Race 1 Lane 1 - RC entry #4821") {
		t.Errorf("preview missing human-readable line: %s", body)
	}
	if !strings.Contains(body, `"raceId"`) && !strings.Contains(body, `"races"`) {
		t.Errorf("preview missing raw JSON payload: %s", body)
	}
	if !strings.Contains(body, "NOT sent, local dry-run only") {
		t.Errorf("preview missing the never-uploaded disclaimer: %s", body)
	}
}
