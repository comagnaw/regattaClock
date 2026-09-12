package main

import (
	"strings"
	"testing"
	"time"

	"github.com/comagnaw/regattaClock/internal/reader"
	"github.com/comagnaw/regattaClock/internal/regattacentral"
)

func TestClassifyForPublish(t *testing.T) {
	matches := []laneMatch{
		{RaceNumber: 1, Lane: 1, SchoolName: "Springfield HS", Status: statusMatched, Candidates: []rcEntry{{ID: "4821"}}},
		{RaceNumber: 1, Lane: 2, SchoolName: "Justice High", Status: statusGuessed, Candidates: []rcEntry{{ID: "65"}, {ID: "64"}}},
		{RaceNumber: 1, Lane: 3, SchoolName: "North Haverbrook", Status: statusUnmatched},
		{RaceNumber: 1, Lane: 4, SchoolName: "Ambiguous Twins", Status: statusAmbiguous, Candidates: []rcEntry{{ID: "1"}, {ID: "2"}}},
		{RaceNumber: 1, Lane: 5, SchoolName: "Non Numeric", Status: statusMatched, Candidates: []rcEntry{{ID: "not-a-number"}}},
	}

	publishable, guessed, excluded := classifyForPublish(matches)

	if len(publishable) != 2 {
		t.Fatalf("publishable = %d, want 2 (lanes 1 and 2): %+v", len(publishable), publishable)
	}
	if publishable[0].Lane != 1 || publishable[1].Lane != 2 {
		t.Errorf("publishable lanes = [%d, %d], want [1, 2]", publishable[0].Lane, publishable[1].Lane)
	}
	if len(guessed) != 1 || guessed[0].Lane != 2 {
		t.Errorf("guessed = %+v, want just lane 2", guessed)
	}
	if len(excluded) != 3 {
		t.Fatalf("excluded = %d, want 3 (unmatched, ambiguous, non-numeric id): %+v", len(excluded), excluded)
	}
	excludedLanes := map[int]bool{}
	for _, m := range excluded {
		excludedLanes[m.Lane] = true
	}
	if !excludedLanes[3] || !excludedLanes[4] || !excludedLanes[5] {
		t.Errorf("excluded lanes = %v, want {3, 4, 5}", excludedLanes)
	}
}

func TestBuildScheduleRequestOneLaneRecordPerPublishableLane(t *testing.T) {
	publishable := []laneMatch{
		{RaceNumber: 1, Lane: 1, AdditionalInfo: "A", Place: "1", Candidates: []rcEntry{{ID: "4821"}}},
		{RaceNumber: 1, Lane: 2, Place: "SCR", Candidates: []rcEntry{{ID: "4822"}}},
		{RaceNumber: 2, Lane: 1, Candidates: []rcEntry{{ID: "9001"}}},
	}

	req := buildScheduleRequest(publishable)

	if len(req.Lanes) != 3 {
		t.Fatalf("lanes = %d, want 3: %+v", len(req.Lanes), req.Lanes)
	}
	if len(req.Results) != 0 {
		t.Errorf("results = %+v, want none - schedule stage never sends results", req.Results)
	}
	if req.Lanes[0].EntryID != 4821 || req.Lanes[0].DisplayNumber != "A" {
		t.Errorf("lane 0 = %+v, want EntryID 4821 and DisplayNumber \"A\"", req.Lanes[0])
	}
	if req.Lanes[1].Status != regattacentral.LaneScratched {
		t.Errorf("lane 1 status = %q, want LaneScratched from Place=SCR", req.Lanes[1].Status)
	}
	if len(req.Races) != 2 {
		t.Fatalf("races = %d, want 2 (one per distinct race number): %+v", len(req.Races), req.Races)
	}
	for _, r := range req.Races {
		if r.Status != regattacentral.StatusDraw {
			t.Errorf("race %+v status = %q, want StatusDraw", r, r.Status)
		}
	}
}

func TestBuildResultsRequestOnlyLanesWithAParseableTime(t *testing.T) {
	publishable := []laneMatch{
		{RaceNumber: 1, Lane: 1, Time: "6:12.5", Candidates: []rcEntry{{ID: "4821"}}},
		{RaceNumber: 1, Lane: 2, Time: "", Candidates: []rcEntry{{ID: "4822"}}}, // no time yet - excluded from results
	}

	req := buildResultsRequest(publishable)

	if len(req.Lanes) != 0 {
		t.Errorf("lanes = %+v, want none - results stage never sends lanes (already sent by publish-schedule)", req.Lanes)
	}
	if len(req.Results) != 1 || req.Results[0].Lane != 1 {
		t.Fatalf("results = %+v, want exactly one result for lane 1", req.Results)
	}
	want := (6*time.Minute + 12*time.Second + 500*time.Millisecond).Milliseconds()
	if req.Results[0].Time != want {
		t.Errorf("result time = %d, want %d", req.Results[0].Time, want)
	}
	if len(req.Races) != 1 || req.Races[0].Status != regattacentral.StatusOfficial {
		t.Errorf("races = %+v, want race 1 marked StatusOfficial", req.Races)
	}
}

func TestPrintSummaryAndConfirmWithoutConfirmNeverPrompts(t *testing.T) {
	rd := &reader.RegattaData{Races: []reader.RaceData{raceWithLanes(1, "Varsity 8", map[int]reader.RaceEntry{
		1: {SchoolName: "Springfield HS"},
	})}}
	publishable := []laneMatch{{RaceNumber: 1, Lane: 1, SchoolName: "Springfield HS", Candidates: []rcEntry{{ID: "4821"}}}}

	var out strings.Builder
	// A reader that errors on any read - proves the non-confirm path never
	// touches stdin at all.
	proceed := printSummaryAndConfirm(&out, failingReader{}, "schedule", publishOptions{regattaID: "999", confirm: false}, rd, publishable, nil, nil)

	if proceed {
		t.Error("proceed = true, want false when --confirm is not set")
	}
	if !strings.Contains(out.String(), "--confirm not set") {
		t.Errorf("summary = %q, want it to say --confirm was not set", out.String())
	}
}

func TestPrintSummaryAndConfirmRequiresExactRegattaID(t *testing.T) {
	rd := &reader.RegattaData{Races: []reader.RaceData{raceWithLanes(1, "Varsity 8", map[int]reader.RaceEntry{
		1: {SchoolName: "Springfield HS"},
	})}}
	publishable := []laneMatch{{RaceNumber: 1, Lane: 1, SchoolName: "Springfield HS", Candidates: []rcEntry{{ID: "4821"}}}}
	guessed := []laneMatch{publishable[0]}
	excluded := []laneMatch{{RaceNumber: 2, Lane: 1, SchoolName: "North Haverbrook"}}

	var out strings.Builder
	proceed := printSummaryAndConfirm(&out, strings.NewReader("wrong-id\n"), "schedule", publishOptions{regattaID: "999", confirm: true}, rd, publishable, guessed, excluded)
	if proceed {
		t.Error("proceed = true, want false for a mismatched confirmation")
	}
	if !strings.Contains(out.String(), "North Haverbrook") {
		t.Errorf("summary = %q, want the excluded lane listed by school name", out.String())
	}
	if !strings.Contains(out.String(), "RC entry 4821") {
		t.Errorf("summary = %q, want the guessed lane's picked entry id shown", out.String())
	}

	out.Reset()
	proceed = printSummaryAndConfirm(&out, strings.NewReader("999\n"), "schedule", publishOptions{regattaID: "999", confirm: true}, rd, publishable, guessed, excluded)
	if !proceed {
		t.Error("proceed = false, want true when the typed id matches exactly")
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	panic("stdin must not be read when --confirm is not set")
}
