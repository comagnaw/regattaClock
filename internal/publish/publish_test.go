package publish

import (
	"testing"
	"time"

	"github.com/comagnaw/regattaClock/internal/persona/store"
)

func sampleSchedule() *store.Schedule {
	return &store.Schedule{
		Name: "Fall Classic",
		Date: "2026-10-03",
		Races: []store.ScheduleRace{
			{
				RaceNumber: 1,
				BoatClass:  "Varsity 8",
				FlightInfo: "Heat 1",
				Lanes: map[int]store.ScheduleEntry{
					1: {SchoolName: "Robinson"},
					2: {SchoolName: "Gloucester", AdditionalInfo: "Combined"},
				},
			},
			{RaceNumber: 2, BoatClass: "Novice 4"},
		},
	}
}

func approvedResult() store.RaceResult {
	approvedAt := time.Date(2026, 10, 3, 9, 30, 0, 0, time.UTC)
	return store.RaceResult{
		RaceNumber:  1,
		WinningTime: "06:00.0",
		Approved:    true,
		ApprovedAt:  &approvedAt,
		UpdatedAt:   approvedAt,
		Rows: []store.LapRow{
			{Lane: 1, Place: "1", Time: "06:00.0"},
			{Lane: 2, Place: "2", Time: "06:05.0"},
		},
	}
}

func TestBuildView_ApprovedFilter(t *testing.T) {
	fin := &store.FinishLog{Races: map[int]store.RaceResult{
		1: approvedResult(),
		2: {RaceNumber: 2, WinningTime: "05:00.0", Approved: false},
	}}

	got := BuildView(sampleSchedule(), fin)
	if len(got) != 1 || got[0].RaceNumber != 1 {
		t.Fatalf("BuildView() = %+v, want only the approved race", got)
	}
}

func TestBuildView_RaceMissingFromSchedule(t *testing.T) {
	fin := &store.FinishLog{Races: map[int]store.RaceResult{
		99: {RaceNumber: 99, WinningTime: "05:00.0", Approved: true},
	}}

	if got := BuildView(sampleSchedule(), fin); len(got) != 0 {
		t.Errorf("BuildView() = %+v, want nothing (race 99 is not on the schedule)", got)
	}
}

func TestBuildView_RowJoinHitAndMiss(t *testing.T) {
	fin := &store.FinishLog{Races: map[int]store.RaceResult{1: approvedResult()}}

	got := BuildView(sampleSchedule(), fin)
	if len(got) != 1 || len(got[0].Rows) != 2 {
		t.Fatalf("BuildView() = %+v", got)
	}

	byLane := map[string]Row{}
	for _, row := range got[0].Rows {
		byLane[row.Lane] = row
	}
	if got := byLane["1"].School; got != "Robinson" {
		t.Errorf("lane 1 school = %q, want Robinson (join hit)", got)
	}
	if got := byLane["2"].AdditionalInfo; got != "Combined" {
		t.Errorf("lane 2 additional info = %q, want Combined (join hit)", got)
	}
}

func TestBuildView_UnassignedLaneAndStaleLaneJoinMiss(t *testing.T) {
	fin := &store.FinishLog{Races: map[int]store.RaceResult{
		1: {
			RaceNumber:  1,
			WinningTime: "06:00.0",
			Approved:    true,
			Rows: []store.LapRow{
				{Lane: 0, Place: "DNS", Time: ""},       // never assigned a lane
				{Lane: 6, Place: "DQ", Time: "06:10.0"}, // lane not on the schedule (join miss)
			},
		},
	}}

	got := BuildView(sampleSchedule(), fin)
	if len(got) != 1 || len(got[0].Rows) != 2 {
		t.Fatalf("BuildView() = %+v", got)
	}

	var unassigned, staleLane Row
	for _, row := range got[0].Rows {
		switch row.Place {
		case "DNS":
			unassigned = row
		case "DQ":
			staleLane = row
		}
	}
	if unassigned.Lane != "" || unassigned.School != "" {
		t.Errorf("unassigned lane row = %+v, want empty Lane and School", unassigned)
	}
	if staleLane.Lane != "6" || staleLane.School != "" {
		t.Errorf("stale-lane row = %+v, want Lane=6, School empty (no schedule entry for lane 6)", staleLane)
	}
}

func TestBuildView_DQDNFDNSRowsPassThrough(t *testing.T) {
	fin := &store.FinishLog{Races: map[int]store.RaceResult{
		1: {
			RaceNumber:  1,
			WinningTime: "06:00.0",
			Approved:    true,
			Rows: []store.LapRow{
				{Lane: 1, Place: "DQ"},
				{Lane: 2, Place: "DNF"},
			},
		},
	}}

	got := BuildView(sampleSchedule(), fin)
	places := map[string]bool{}
	for _, row := range got[0].Rows {
		places[row.Place] = true
	}
	if !places["DQ"] || !places["DNF"] {
		t.Errorf("rows = %+v, want DQ and DNF to pass through unchanged", got[0].Rows)
	}
}

func TestRevision_StableAcrossUpdatedAtOnlyRewrite(t *testing.T) {
	base := BuildView(sampleSchedule(), &store.FinishLog{Races: map[int]store.RaceResult{1: approvedResult()}})[0]

	touched := approvedResult()
	touched.UpdatedAt = touched.UpdatedAt.Add(time.Hour)
	after := BuildView(sampleSchedule(), &store.FinishLog{Races: map[int]store.RaceResult{1: touched}})[0]

	if base.Revision != after.Revision {
		t.Errorf("Revision changed on an UpdatedAt-only rewrite: %q -> %q", base.Revision, after.Revision)
	}
}

func TestRevision_ChangesOnVisibleEdit(t *testing.T) {
	base := BuildView(sampleSchedule(), &store.FinishLog{Races: map[int]store.RaceResult{1: approvedResult()}})[0]

	edited := approvedResult()
	edited.Rows[0].Place = "2"
	edited.Rows[1].Place = "1"
	after := BuildView(sampleSchedule(), &store.FinishLog{Races: map[int]store.RaceResult{1: edited}})[0]

	if base.Revision == after.Revision {
		t.Error("Revision did not change after a place edit")
	}
}

func TestRevision_OrderIndependent(t *testing.T) {
	forward := PublishableRace{
		Title:       "Race 1",
		WinningTime: "06:00.0",
		Rows: []Row{
			{Place: "1", Lane: "1", School: "Robinson", Time: "06:00.0"},
			{Place: "2", Lane: "2", School: "Gloucester", Time: "06:05.0"},
		},
	}
	reversed := forward
	reversed.Rows = []Row{forward.Rows[1], forward.Rows[0]}

	if Revision(forward) != Revision(reversed) {
		t.Error("Revision depends on row order, want it order-independent")
	}
}

func TestRenderText(t *testing.T) {
	pr := PublishableRace{
		Title: "Race 1: W-N-4+ Heat 1",
		Rows: []Row{
			{Place: "1", Lane: "1", School: "Robinson", Time: "07:26.8"},
			{Place: "2", Lane: "2", School: "", Time: "07:29.9"},
		},
	}
	want := "Race 1: W-N-4+ Heat 1 Results\n1 - Robinson 07:26.8\n2 - — 07:29.9\n"
	if got := RenderText(pr); got != want {
		t.Errorf("RenderText() = %q, want %q", got, want)
	}
}

func TestIsStale(t *testing.T) {
	// IsStale recomputes Revision(pr) itself rather than trusting pr.Revision
	// (race-state-machine.md's own sketch), so the "matches" case needs the
	// actual current hash, not an arbitrary placeholder.
	pr := PublishableRace{RaceNumber: 1, Title: "Race 1", WinningTime: "06:00.0"}
	current := Revision(pr)

	if !IsStale(map[int]string{}, pr) {
		t.Error("IsStale() = false with nothing published yet, want true")
	}
	if IsStale(map[int]string{1: current}, pr) {
		t.Error("IsStale() = true when the published revision matches, want false")
	}
	if !IsStale(map[int]string{1: "old"}, pr) {
		t.Error("IsStale() = false when the published revision is stale, want true")
	}
}
