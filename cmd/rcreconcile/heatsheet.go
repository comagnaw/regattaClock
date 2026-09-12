package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// heatSheetLane is what the xlsm's "Heat Sheet" tab (a different tab and row
// shape from "Results" - internal/reader never reads it, see its
// findRaceSheet comment) can tell rcreconcile about one lane that the
// Results tab alone cannot.
type heatSheetLane struct {
	// LaneClass is row 2's per-lane text: an alternate boat class when the RD
	// combines a small class into a race's open lanes for lack of entries
	// (e.g. race's nominal class "M-2x", this lane's actual class
	// "M-Jr-1x"), but that same cell can just as easily hold "A"/"B",
	// "SCRATCHED", or an advancement note - see disambiguateByBoatClass,
	// which only acts on it when it looks like a real signal.
	LaneClass string
	// RowerLastName is row 3's per-lane text: a stroke/rower's last name,
	// listed only for 1x/2x boats per a real RD-authored template
	// ("Heat Sheet Input Examples.xlsx"). Anything else landing there
	// (blank, an advancement note, "Exhibition") simply won't match any real
	// participant name later - see disambiguateByRowerLastName.
	RowerLastName string
}

// heatSheetStats reports what readHeatSheet actually saw, so reconcile.go can
// tell the difference between "this regatta simply has no rower-name signal"
// and "the parser's assumptions about this workbook's layout are wrong" -
// the same distinguish-the-real-cause approach as reconcile's "found 0
// RegattaCentral entries" troubleshooting. All PII-safe: counts only, never
// names.
type heatSheetStats struct {
	SheetFound       bool // a worksheet named exactly "Heat Sheet" exists
	RaceBlocksSeen   int  // any race-number merge in column A, any row count
	ThreeRowBlocks   int  // blocks matching the expected 3-row shape
	RowerNamesFound  int  // non-blank cells found in row 3 of a 3-row block
	LaneClassesFound int  // non-blank cells found in row 2 of a 3-row block
}

// readHeatSheet opens xlsmPath a second time - internal/reader only parses
// the Results tab - and returns, for every lane that has one, its
// heatSheetLane. Confirmed against a real heat-sheet template ("Heat Sheet
// Input Examples.xlsx", its Instructions tab): every race is exactly 3 rows
// - row 1 is the nominal boat class (column C) and per-lane school names
// (D-I); row 2 and row 3 are described on heatSheetLane's fields above.
//
// This is a small, standalone parser scoped to cmd/rcreconcile on purpose,
// not internal/reader - this investigation isn't meant to grow the shipped
// app's Excel-parsing surface for a one-off disambiguation signal. A
// workbook with no sheet named exactly "Heat Sheet" (so "Referee Heat Sheet"
// is never mistaken for it) returns an empty map, not an error - this signal
// is optional and best-effort, and reconcile must still work without it.
func readHeatSheet(xlsmPath string) (map[[2]int]heatSheetLane, heatSheetStats, error) {
	var stats heatSheetStats

	f, err := excelize.OpenFile(xlsmPath)
	if err != nil {
		return nil, stats, fmt.Errorf("open %q: %w", xlsmPath, err)
	}
	defer f.Close()

	var sheetName string
	for _, s := range f.GetSheetList() {
		if strings.EqualFold(strings.TrimSpace(s), "Heat Sheet") {
			sheetName = s
			break
		}
	}
	if sheetName == "" {
		return map[[2]int]heatSheetLane{}, stats, nil
	}
	stats.SheetFound = true

	merges, err := f.GetMergeCells(sheetName)
	if err != nil {
		return nil, stats, fmt.Errorf("read %q merged cells: %w", sheetName, err)
	}

	out := map[[2]int]heatSheetLane{}
	laneCols := []string{"D", "E", "F", "G", "H", "I"}
	for _, mc := range merges {
		start, end := mc.GetStartAxis(), mc.GetEndAxis()
		if !strings.HasPrefix(start, "A") || !strings.HasPrefix(end, "A") {
			continue
		}
		startRow, endRow := heatSheetRowNumber(start), heatSheetRowNumber(end)
		if _, err := strconv.Atoi(strings.TrimSpace(mc.GetCellValue())); err != nil {
			continue
		}
		stats.RaceBlocksSeen++
		if endRow-startRow != 2 { // 3 rows (inclusive) - a Heat Sheet lineup block
			continue
		}
		stats.ThreeRowBlocks++

		raceNum, _ := strconv.Atoi(strings.TrimSpace(mc.GetCellValue()))
		classRow, rowerRow := startRow+1, startRow+2
		for lane, col := range laneCols {
			class, _ := f.GetCellValue(sheetName, fmt.Sprintf("%s%d", col, classRow))
			rower, _ := f.GetCellValue(sheetName, fmt.Sprintf("%s%d", col, rowerRow))
			class, rower = strings.TrimSpace(class), strings.TrimSpace(rower)
			if class == "" && rower == "" {
				continue
			}
			if class != "" {
				stats.LaneClassesFound++
			}
			if rower != "" {
				stats.RowerNamesFound++
			}
			out[[2]int{raceNum, lane + 1}] = heatSheetLane{LaneClass: class, RowerLastName: rower}
		}
	}
	return out, stats, nil
}

func heatSheetRowNumber(cellRef string) int {
	row := 0
	for _, c := range cellRef {
		if c >= '0' && c <= '9' {
			row = row*10 + int(c-'0')
		}
	}
	return row
}
