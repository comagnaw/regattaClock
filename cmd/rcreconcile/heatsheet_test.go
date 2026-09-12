package main

import (
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

// writeHeatSheetFixture builds a minimal xlsm-shaped workbook with a "Heat
// Sheet" tab: race 1 is a 3-row block (rows 1-3) with a per-lane class
// override in row 2 and a rower's last name in row 3, for lane 2 only,
// matching the real layout confirmed against an RD's own template (see
// heatsheet.go's doc comment).
func writeHeatSheetFixture(t *testing.T, sheetName string) string {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	f.SetSheetName("Sheet1", sheetName)

	f.SetCellValue(sheetName, "A1", "1")
	f.SetCellValue(sheetName, "C1", "M-2x")
	f.SetCellValue(sheetName, "D1", "Team A")
	f.SetCellValue(sheetName, "E1", "Team B")
	f.MergeCell(sheetName, "A1", "A3")

	f.SetCellValue(sheetName, "E2", "M-Jr-1x")
	f.SetCellValue(sheetName, "E3", "Mihalovich")

	path := filepath.Join(t.TempDir(), "fixture.xlsm")
	if err := f.SaveAs(path); err != nil {
		t.Fatalf("save fixture: %v", err)
	}
	return path
}

func TestReadHeatSheetExtractsLaneClassAndRowerLastName(t *testing.T) {
	path := writeHeatSheetFixture(t, "Heat Sheet")

	got, stats, err := readHeatSheet(path)
	if err != nil {
		t.Fatalf("readHeatSheet: %v", err)
	}
	want := heatSheetLane{LaneClass: "M-Jr-1x", RowerLastName: "Mihalovich"}
	if got[[2]int{1, 2}] != want {
		t.Errorf("lane 2 = %+v, want %+v", got[[2]int{1, 2}], want)
	}
	if _, ok := got[[2]int{1, 1}]; ok {
		t.Errorf("lane 1 has no heat sheet data in the fixture, want no entry: %v", got)
	}
	if len(got) != 1 {
		t.Errorf("got %d entries, want exactly 1: %v", len(got), got)
	}
	wantStats := heatSheetStats{SheetFound: true, RaceBlocksSeen: 1, ThreeRowBlocks: 1, RowerNamesFound: 1, LaneClassesFound: 1}
	if stats != wantStats {
		t.Errorf("stats = %+v, want %+v", stats, wantStats)
	}
}

func TestReadHeatSheetIgnoresRefereeHeatSheet(t *testing.T) {
	// "Referee Heat Sheet" contains "Heat Sheet" as a substring but must not
	// be mistaken for the real tab (isEventsFile/isOrganizationsFile in
	// rcmodel.go guard against the analogous mistake for captured JSON).
	path := writeHeatSheetFixture(t, "Referee Heat Sheet")

	got, stats, err := readHeatSheet(path)
	if err != nil {
		t.Fatalf("readHeatSheet: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want an empty map - no sheet named exactly \"Heat Sheet\"", got)
	}
	if stats.SheetFound {
		t.Errorf("stats.SheetFound = true, want false")
	}
}

func TestReadHeatSheetNoSheetReturnsEmptyMapNotError(t *testing.T) {
	f := excelize.NewFile()
	path := filepath.Join(t.TempDir(), "no-heat-sheet.xlsm")
	if err := f.SaveAs(path); err != nil {
		t.Fatalf("save fixture: %v", err)
	}
	f.Close()

	got, stats, err := readHeatSheet(path)
	if err != nil {
		t.Fatalf("readHeatSheet: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want an empty map", got)
	}
	if stats.SheetFound {
		t.Errorf("stats.SheetFound = true, want false")
	}
}

// TestReadHeatSheetCountsBlocksOfTheWrongShape is a diagnostic test: a
// workbook whose "Heat Sheet" tab exists but whose race blocks aren't the
// expected 3 rows (e.g. a different RD's layout) should still report
// SheetFound and RaceBlocksSeen, so reconcile.go's troubleshooting message
// can tell "no such tab" apart from "tab exists, layout assumption is wrong."
func TestReadHeatSheetCountsBlocksOfTheWrongShape(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	sheetName := "Heat Sheet"
	f.SetSheetName("Sheet1", sheetName)
	f.SetCellValue(sheetName, "A1", "1")
	f.MergeCell(sheetName, "A1", "A5") // 5 rows, not the expected 3

	path := filepath.Join(t.TempDir(), "wrong-shape.xlsm")
	if err := f.SaveAs(path); err != nil {
		t.Fatalf("save fixture: %v", err)
	}

	got, stats, err := readHeatSheet(path)
	if err != nil {
		t.Fatalf("readHeatSheet: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want no heat sheet data from a mis-shaped block", got)
	}
	want := heatSheetStats{SheetFound: true, RaceBlocksSeen: 1, ThreeRowBlocks: 0, RowerNamesFound: 0, LaneClassesFound: 0}
	if stats != want {
		t.Errorf("stats = %+v, want %+v", stats, want)
	}
}
