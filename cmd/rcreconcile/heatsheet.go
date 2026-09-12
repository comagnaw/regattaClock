package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// heatSheetStats reports what readHeatSheet actually saw, so reconcile.go can
// tell the difference between "this regatta simply has no rower-name signal"
// and "the parser's assumptions about this workbook's layout are wrong" -
// the same distinguish-the-real-cause approach as reconcile's "found 0
// RegattaCentral entries" troubleshooting. All PII-safe: counts only, never
// names.
type heatSheetStats struct {
	SheetFound      bool // a worksheet named exactly "Heat Sheet" exists
	RaceBlocksSeen  int  // any race-number merge in column A, any row count
	ThreeRowBlocks  int  // blocks matching the expected 3-row shape
	RowerNamesFound int  // non-blank cells found in row 3 of a 3-row block
}

// readHeatSheet opens xlsmPath a second time - internal/reader only parses
// the Results tab (see its findRaceSheet comment: the Heat Sheet tab's 3-row
// blocks are a different shape) - and returns, for every lane that has one,
// the rower's last name from that tab's third block row: the one signal it
// carries that the Results tab does not. Confirmed against a real heat-sheet
// template ("Heat Sheet Input Examples.xlsx", its Instructions tab): every
// race is exactly 3 rows - row 1 is the nominal boat class (column C) and
// per-lane school names (D-I); row 2 is a free-text per-lane annotation (an
// alternate boat class when the RD combines a small class into open lanes
// for lack of entries, "A"/"B", "SCRATCHED", ...) that this parser
// deliberately does not interpret, matching too many different real-world
// meanings to guess reliably; row 3 is the rower's last name, listed only
// for 1x/2x boats per that template. Anything else landing in row 3 (blank,
// an advancement note, "Exhibition") simply won't match any real participant
// name later and is a harmless no-op - see disambiguateByRowerLastName.
//
// This is a small, standalone parser scoped to cmd/rcreconcile on purpose,
// not internal/reader - this investigation isn't meant to grow the shipped
// app's Excel-parsing surface for a one-off disambiguation signal. A
// workbook with no sheet named exactly "Heat Sheet" (so "Referee Heat Sheet"
// is never mistaken for it) returns an empty map, not an error - this signal
// is optional and best-effort, and reconcile must still work without it.
func readHeatSheet(xlsmPath string) (map[[2]int]string, heatSheetStats, error) {
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
		return map[[2]int]string{}, stats, nil
	}
	stats.SheetFound = true

	merges, err := f.GetMergeCells(sheetName)
	if err != nil {
		return nil, stats, fmt.Errorf("read %q merged cells: %w", sheetName, err)
	}

	out := map[[2]int]string{}
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
		rowerRow := startRow + 2
		for lane, col := range laneCols {
			name, _ := f.GetCellValue(sheetName, fmt.Sprintf("%s%d", col, rowerRow))
			if name = strings.TrimSpace(name); name != "" {
				out[[2]int{raceNum, lane + 1}] = name
				stats.RowerNamesFound++
			}
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
