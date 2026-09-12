package main

import (
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

// writeHeatSheetFixture builds a minimal xlsm-shaped workbook with a "Heat
// Sheet" tab: race 1 is a 3-row block (rows 1-3) with a rower's last name in
// row 3 for lane 2 only, matching the real layout confirmed against an RD's
// own template (see heatsheet.go's doc comment).
func writeHeatSheetFixture(t *testing.T, sheetName string) string {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	f.SetSheetName("Sheet1", sheetName)

	f.SetCellValue(sheetName, "A1", "1")
	f.SetCellValue(sheetName, "C1", "M-1x")
	f.SetCellValue(sheetName, "D1", "Team A")
	f.SetCellValue(sheetName, "E1", "Team B")
	f.MergeCell(sheetName, "A1", "A3")

	f.SetCellValue(sheetName, "C2", "Heat 1")
	f.SetCellValue(sheetName, "E3", "Mihalovich")

	path := filepath.Join(t.TempDir(), "fixture.xlsm")
	if err := f.SaveAs(path); err != nil {
		t.Fatalf("save fixture: %v", err)
	}
	return path
}

func TestReadHeatSheetExtractsRowerLastName(t *testing.T) {
	path := writeHeatSheetFixture(t, "Heat Sheet")

	got, err := readHeatSheet(path)
	if err != nil {
		t.Fatalf("readHeatSheet: %v", err)
	}
	if want := "Mihalovich"; got[[2]int{1, 2}] != want {
		t.Errorf("lane 2 rower = %q, want %q", got[[2]int{1, 2}], want)
	}
	if _, ok := got[[2]int{1, 1}]; ok {
		t.Errorf("lane 1 has no rower name in the fixture, want no entry: %v", got)
	}
	if len(got) != 1 {
		t.Errorf("got %d entries, want exactly 1: %v", len(got), got)
	}
}

func TestReadHeatSheetIgnoresRefereeHeatSheet(t *testing.T) {
	// "Referee Heat Sheet" contains "Heat Sheet" as a substring but must not
	// be mistaken for the real tab (isEventsFile/isOrganizationsFile in
	// rcmodel.go guard against the analogous mistake for captured JSON).
	path := writeHeatSheetFixture(t, "Referee Heat Sheet")

	got, err := readHeatSheet(path)
	if err != nil {
		t.Fatalf("readHeatSheet: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want an empty map - no sheet named exactly \"Heat Sheet\"", got)
	}
}

func TestReadHeatSheetNoSheetReturnsEmptyMapNotError(t *testing.T) {
	f := excelize.NewFile()
	path := filepath.Join(t.TempDir(), "no-heat-sheet.xlsm")
	if err := f.SaveAs(path); err != nil {
		t.Fatalf("save fixture: %v", err)
	}
	f.Close()

	got, err := readHeatSheet(path)
	if err != nil {
		t.Fatalf("readHeatSheet: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want an empty map", got)
	}
}
