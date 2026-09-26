// Package spreadsheet writes and reads the published results workbook - the
// RegattaClock-owned .xlsx the Primary Finish Timer's Publish button
// regenerates (results-publisher.md). Its layout reproduces the manual
// "Results" worksheet regatta officials already recognize: a title, date, and
// header row, then one fixed-size block of rows per race. A leaf package: it
// renders publish.PublishableRace values and never reads finish.json, imports
// Fyne, or imports internal/regatta.
package spreadsheet

import (
	"fmt"

	"github.com/xuri/excelize/v2"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/filesystem"
)

// The Results sheet's fixed geometry, exported as a reusable definition so a
// second producer of this layout (the sample-regatta generator,
// docs/features/testing/sample-regatta.md) builds on the same numbers rather
// than its own copy of the cell offsets. Rows and columns are 1-based, as
// excelize and Excel number them.
const (
	TitleRow   = 1
	DateRow    = 2
	HeaderRow  = 4
	HeaderRows = 4 // title, date, blank spacer, header
	BlockRows  = 5 // school, info, place, split, time
	MaxLanes   = 6

	RaceNumCol   = 1 // A - race number, merged down the block
	TimeCol      = 2 // B - scheduled time, merged down the block
	LabelCol     = 3 // C - boat class, flight, then the Place/Split/Time labels
	FirstLaneCol = 4 // D - lane 1
	LastCol      = FirstLaneCol + MaxLanes - 1

	// LedgerSheetName is the very-hidden sheet recording which races have been
	// published and at what publish.Revision, so the workbook itself - not a
	// preference or a JSON copy - is the record of what the public has seen.
	LedgerSheetName = "_regattaClock"
)

// Offsets of each row within a race block, from BlockOrigin.
const (
	RowSchool = iota
	RowInfo
	RowPlace
	RowSplit
	RowTime
)

// ColumnWidths are the manual Results worksheet's widths, A through I.
var ColumnWidths = [LastCol]float64{4.83, 10.83, 13.83, 13.83, 13.83, 13.83, 14, 13.83, 14.16}

// BlockLabels are column C's fixed text per block row; the school and info
// rows carry the race's boat class and flight there instead.
var BlockLabels = [BlockRows]string{
	RowPlace: common.ResultsPlaceLabel,
	RowSplit: common.ResultsSplitLabel,
	RowTime:  common.ResultsTimeLabel,
}

// BlockOrigin is the first (school) row of the i-th race block, 0-based i.
func BlockOrigin(i int) int {
	return HeaderRows + 1 + i*BlockRows
}

// LaneCol is lane's column (lane 1 -> D).
func LaneCol(lane int) int {
	return FirstLaneCol + lane - 1
}

// HeaderLabels is the header row's text, A through I.
func HeaderLabels() []string {
	labels := []string{common.ResultsRaceNumHead, common.ResultsTimeHead, common.ResultsRaceHead}
	for lane := 1; lane <= MaxLanes; lane++ {
		labels = append(labels, fmt.Sprintf(common.ResultsLaneHead, lane))
	}
	return labels
}

// FileName is the results workbook's file name for a regatta - one workbook
// per regatta, safe to create on Windows and POSIX. The date is part of it
// because regatta names repeat year to year ("Charlie Brown Classic"); the
// name alone would point this year's publish at last year's file. The
// ledger's regatta key (Published.CheckRegatta) is the hard guard; this just
// keeps the two from colliding in the first place.
func FileName(regattaName, regattaDate string) string {
	base := regattaName
	if regattaDate != common.EmptyString {
		base += " - " + regattaDate
	}
	return filesystem.SanitizeForFilename(base + common.ResultsFileSuffix)
}

// cell names a 1-based (col, row); the geometry above is always in range.
func cell(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}
