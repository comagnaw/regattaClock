package clock

import (
	"strings"
	"testing"

	"github.com/comagnaw/regattaClock/internal/persona/store"
)

// TestApprovalGrid_RendersRaceResultRows - ApprovalGrid must render the same
// OOF/Place/Split/Time/School content asApprovals would, but from a plain
// store.RaceResult's Rows (no live laps/results Clock state), so a read-only
// viewer with no live clock (the Director/Awards results window) sees the
// identical grid a Referee saw and approved.
func TestApprovalGrid_RendersRaceResultRows(t *testing.T) {
	race := createTestRaceData() // lanes 1-4: School A-D

	rows := []store.LapRow{
		{Lane: 2, Place: "1", Split: "00:00.0", Time: "06:00.0"},
		{Lane: 1, Place: "2", Split: "00:02.5", Time: "06:02.5"},
	}

	grid := ApprovalGrid(race, rows)
	if grid == nil {
		t.Fatal("ApprovalGrid returned nil")
	}

	wantCells := (1 + len(rows)) * refereeCols // header row + one row per result
	if len(grid.Objects) != wantCells {
		t.Fatalf("grid has %d cells, want %d (%d cols x %d rows)",
			len(grid.Objects), wantCells, refereeCols, 1+len(rows))
	}

	var texts []string
	for _, o := range grid.Objects {
		if txt := cellText(o); txt != nil {
			texts = append(texts, txt.Text)
		}
	}
	joined := strings.Join(texts, " ")
	for _, want := range []string{
		"OOF", "Place", "Split", "Time", "School", // header
		"06:00.0", "School B", "06:02.5", "School A", // the two rows
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("approval grid missing %q; got %q", want, joined)
		}
	}
}

// TestApprovalGrid_UnassignedLaneHasNoOOFOrSchool - a LapRow with no lane
// (a DQ/DNF/DNS with Lane == 0, or an out-of-range value) renders blank OOF
// and School cells rather than "0" or an index-out-of-range panic.
func TestApprovalGrid_UnassignedLaneHasNoOOFOrSchool(t *testing.T) {
	race := createTestRaceData()

	grid := ApprovalGrid(race, []store.LapRow{{Lane: 0, Place: "DQ"}})
	if grid == nil {
		t.Fatal("ApprovalGrid returned nil")
	}

	var texts []string
	for _, o := range grid.Objects {
		if txt := cellText(o); txt != nil {
			texts = append(texts, txt.Text)
		}
	}
	joined := strings.Join(texts, " ")
	if !strings.Contains(joined, "DQ") {
		t.Errorf("approval grid missing %q; got %q", "DQ", joined)
	}
	if strings.Contains(joined, "0") {
		t.Errorf("approval grid should not render a lane number for Lane 0; got %q", joined)
	}
}
