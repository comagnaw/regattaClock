package sample

import (
	"fmt"
	"maps"
	"slices"

	"github.com/xuri/excelize/v2"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/filesystem"
	"github.com/comagnaw/regattaClock/internal/publish/spreadsheet"
)

// The Heat Sheet worksheet's geometry, as reader.ReadExcelFile reads it: a
// merged A1:I1 title ("<name> Heat Sheet"), a merged A2:I2 date, a header on
// row 4, then one 3-row block per race from row 5 - race number and scheduled
// time merged down columns A and B, and C-I holding, per block row:
//
//	row 1: boat class, then each lane's school
//	row 2: flight, then each lane's additional info
//	row 3: note, then each lane's rower names
const (
	heatTitleSuffix = " Heat Sheet"
	heatFirstRow    = 5
	heatBlockRows   = 3
	heatLastCol     = 9 // I
)

// WriteHeatSheet writes fx as a Heat Sheet workbook at path, dated date (the
// A2 text). A break block follows each race in fx.BreakAfter, as a real
// workbook's lunch break does: no race number, BreakMarker in column B.
func WriteHeatSheet(path string, fx *Fixture, date string) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := common.HeatSheetName
	w := &heatWriter{f: f, sheet: sheet}
	w.do(f.SetSheetName(f.GetSheetName(0), sheet))

	for i, width := range spreadsheet.ColumnWidths {
		name, _ := excelize.ColumnNumberToName(i + 1)
		w.do(f.SetColWidth(sheet, name, name, width))
	}

	w.merge(1, 1, heatLastCol, 1)
	w.set(1, 1, fx.Name+heatTitleSuffix)
	w.merge(1, 2, heatLastCol, 2)
	w.set(1, 2, date)
	for i, label := range spreadsheet.HeaderLabels() {
		w.set(i+1, spreadsheet.HeaderRow, label)
	}

	breaks := make(map[int]bool, len(fx.BreakAfter))
	for _, n := range fx.BreakAfter {
		breaks[n] = true
	}

	row := heatFirstRow
	for _, race := range fx.Races {
		w.block(row, race)
		row += heatBlockRows
		if breaks[race.RaceNumber] {
			w.merge(1, row, 1, row+heatBlockRows-1)
			w.merge(2, row, 2, row+heatBlockRows-1)
			w.set(2, row, BreakMarker)
			row += heatBlockRows
		}
	}

	if w.err != nil {
		return fmt.Errorf("sample heat sheet could not be built: %w", w.err)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return fmt.Errorf("sample heat sheet could not be rendered: %w", err)
	}
	return filesystem.SaveBytesFileAtomic(buf.Bytes(), path)
}

// heatWriter keeps the first excelize error and turns later calls into
// no-ops, like spreadsheet's own sheetWriter.
type heatWriter struct {
	f     *excelize.File
	sheet string
	err   error
}

func (w *heatWriter) do(err error) {
	if w.err == nil {
		w.err = err
	}
}

func (w *heatWriter) set(col, row int, v any) {
	if w.err != nil {
		return
	}
	name, err := excelize.CoordinatesToCellName(col, row)
	if err != nil {
		w.do(err)
		return
	}
	w.do(w.f.SetCellValue(w.sheet, name, v))
}

func (w *heatWriter) merge(fromCol, fromRow, toCol, toRow int) {
	if w.err != nil {
		return
	}
	from, _ := excelize.CoordinatesToCellName(fromCol, fromRow)
	to, _ := excelize.CoordinatesToCellName(toCol, toRow)
	w.do(w.f.MergeCell(w.sheet, from, to))
}

// block writes one race's 3-row block starting at row. The race number is
// written as a number, as the real workbook's formula leaves it.
func (w *heatWriter) block(row int, race Race) {
	last := row + heatBlockRows - 1
	w.merge(1, row, 1, last)
	w.merge(2, row, 2, last)
	w.set(1, row, race.RaceNumber)
	w.set(2, row, race.ScheduledTime)

	w.set(3, row, race.BoatClass)
	w.set(3, row+1, race.FlightInfo)
	w.set(3, row+2, race.Note)
	// Lane order, not map order: excelize numbers shared strings in the order
	// cells are written, so map order would change the file's bytes (and the
	// schedule's Origin.Hash) from run to run.
	for _, lane := range slices.Sorted(maps.Keys(race.Lanes)) {
		l := race.Lanes[lane]
		col := spreadsheet.LaneCol(lane)
		w.set(col, row, l.School)
		w.set(col, row+1, l.AdditionalInfo)
		w.set(col, row+2, l.Rowers)
	}
}
