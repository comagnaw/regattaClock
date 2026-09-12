package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReport(t *testing.T) {
	matches := []laneMatch{
		{RaceNumber: 1, Lane: 1, SchoolName: "Springfield HS", AdditionalInfo: "A",
			Status: statusMatched, Candidates: []rcEntry{{ID: "1", OrgName: "Springfield High School"}}},
		{RaceNumber: 1, Lane: 2, SchoolName: "Springfield HS", AdditionalInfo: "B",
			Status: statusAmbiguous, Candidates: []rcEntry{{ID: "1", OrgName: "Springfield A"}, {ID: "2", OrgName: "Springfield B"}}},
		{RaceNumber: 1, Lane: 3, SchoolName: "North Haverbrook", Status: statusUnmatched},
	}
	unused := []rcEntry{{ID: "9", OrgName: "Ogdenville Composite"}}

	data := buildReport("  Test Regatta  ", matches, unused)

	if data.RegattaName != "Test Regatta" {
		t.Errorf("RegattaName = %q, want trimmed", data.RegattaName)
	}
	if data.Total != 3 || data.MatchedCount != 1 {
		t.Errorf("Total=%d MatchedCount=%d, want 3 and 1", data.Total, data.MatchedCount)
	}
	if len(data.Races) != 1 || len(data.Races[0].Lanes) != 3 {
		t.Fatalf("Races = %+v", data.Races)
	}
	if got := data.Races[0].Lanes[0]; got.StatusClass != "ok" || got.Boat != "Springfield HS (A)" {
		t.Errorf("matched lane = %+v", got)
	}
	if got := data.Races[0].Lanes[1]; got.StatusClass != "warn" || !strings.Contains(got.MatchName, "/") {
		t.Errorf("ambiguous lane = %+v, want both candidate names joined", got)
	}
	if got := data.Races[0].Lanes[2]; got.StatusClass != "bad" {
		t.Errorf("unmatched lane = %+v", got)
	}
	if len(data.Unused) != 1 || data.Unused[0] != "Ogdenville Composite" {
		t.Errorf("Unused = %v", data.Unused)
	}
}

func TestBuildReportGuessedShowsPickAndAlternative(t *testing.T) {
	matches := []laneMatch{
		{RaceNumber: 8, Lane: 3, SchoolName: "Justice High", Status: statusGuessed,
			Candidates: []rcEntry{{ID: "64", OrgName: "Justice High"}, {ID: "65", OrgName: "Justice High"}}},
	}

	data := buildReport("Test Regatta", matches, nil)

	if data.MatchedCount != 0 {
		t.Errorf("MatchedCount = %d, want 0 - a guess is not a confident match", data.MatchedCount)
	}
	lane := data.Races[0].Lanes[0]
	if lane.StatusClass != "warn" {
		t.Errorf("guessed lane StatusClass = %q, want \"warn\"", lane.StatusClass)
	}
	if !strings.Contains(lane.MatchName, "Justice High") || !strings.Contains(lane.StatusLabel, "Best guess") {
		t.Errorf("guessed lane = %+v, want the pick's name and a \"Best guess\" label", lane)
	}
}

func TestBuildReportUnresolvedOrgIDGetsAFriendlyLabel(t *testing.T) {
	unused := []rcEntry{
		{ID: "9", OrgID: "77"},    // never resolved against organizations.json
		{ID: "10"},                // no org info at all
		{ID: "11", OrgName: "Ok"}, // resolved fine
	}
	data := buildReport("Test", nil, unused)
	want := []string{"Unknown organization (RegattaCentral id 77)", "Unknown organization", "Ok"}
	if len(data.Unused) != len(want) {
		t.Fatalf("Unused = %v, want %v", data.Unused, want)
	}
	for i, w := range want {
		if data.Unused[i] != w {
			t.Errorf("Unused[%d] = %q, want %q", i, data.Unused[i], w)
		}
	}
}

func TestBuildReportBlankRegattaName(t *testing.T) {
	if got := buildReport("   ", nil, nil).RegattaName; got != "This regatta" {
		t.Errorf("RegattaName = %q, want a friendly fallback", got)
	}
}

func TestWriteReportCreatesDirAndFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "report.html")
	data := buildReport("Test Regatta", []laneMatch{
		{RaceNumber: 1, Lane: 1, SchoolName: "Springfield HS", Status: statusMatched,
			Candidates: []rcEntry{{ID: "1", OrgName: "Springfield High School"}}},
	}, nil)

	if err := writeReport(path, data); err != nil {
		t.Fatalf("writeReport: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("report not written: %v", err)
	}
	html := string(b)
	for _, want := range []string{"Test Regatta", "Springfield HS", "Springfield High School", "1 of 1 boats matched"} {
		if !strings.Contains(html, want) {
			t.Errorf("report missing %q", want)
		}
	}
	// No script tags, no external assets - a self-contained file.
	if strings.Contains(html, "<script") || strings.Contains(html, "http://") || strings.Contains(html, "https://") {
		t.Error("report should not reference scripts or external URLs")
	}
}
