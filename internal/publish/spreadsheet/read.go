package spreadsheet

import (
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/filesystem"
	"github.com/comagnaw/regattaClock/internal/publish"
)

// ErrForeignWorkbook is returned by Read for an existing file that has no
// ledger sheet - a workbook RegattaClock did not write. It is
// publish.ErrForeign.
var ErrForeignWorkbook = publish.ErrForeign

// ErrOtherRegatta is publish.ErrOtherRegatta, returned by
// Published.CheckRegatta for another regatta's workbook.
var ErrOtherRegatta = publish.ErrOtherRegatta

// Read loads path's ledger and published rows. A missing file is not an
// error - nothing has been published yet - and returns an empty Published.
func Read(path string) (Published, error) {
	out := Published{Ledger: Ledger{}, Rows: map[int][]publish.Row{}}

	f, err := excelize.OpenFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		if filesystem.IsFileLocked(err) {
			return out, fmt.Errorf("%w: %w", ErrLocked, err)
		}
		return out, fmt.Errorf("results workbook %s could not be opened: %w", path, err)
	}
	defer f.Close()

	if idx, _ := f.GetSheetIndex(LedgerSheetName); idx < 0 {
		return out, fmt.Errorf("%s: %w", path, ErrForeignWorkbook)
	}
	ledgerRows, err := f.GetRows(LedgerSheetName)
	if err != nil {
		return out, fmt.Errorf("results workbook %s ledger could not be read: %w", path, err)
	}
	out.Exists = true
	if len(ledgerRows) > 0 && len(ledgerRows[0]) > ledgerKeyCol {
		out.RegattaKey = ledgerRows[0][ledgerKeyCol] // F1, beside the ledgerKeyHead label
	}
	for i, row := range ledgerRows {
		if i == 0 || len(row) < 2 {
			continue // header, or a row too short to mean anything
		}
		n, err := strconv.Atoi(row[0])
		if err != nil {
			continue
		}
		entry := LedgerEntry{Revision: row[1]}
		if len(row) > 2 {
			entry.PublishedAt, _ = time.Parse(time.RFC3339, row[2])
		}
		out.Ledger[n] = entry
	}

	sheet := common.ResultsSheetName
	for i := 0; ; i++ {
		origin := BlockOrigin(i)
		v, err := f.GetCellValue(sheet, cell(RaceNumCol, origin))
		if err != nil || v == common.EmptyString {
			break // past the last block
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			continue
		}
		if _, published := out.Ledger[n]; published {
			out.Rows[n] = readBlockRows(f, sheet, origin)
		}
	}
	return out, nil
}

// readBlockRows returns one Row per lane with any place, split, or time.
func readBlockRows(f *excelize.File, sheet string, origin int) []publish.Row {
	get := func(col, row int) string {
		v, _ := f.GetCellValue(sheet, cell(col, row))
		return v
	}
	var rows []publish.Row
	for lane := 1; lane <= MaxLanes; lane++ {
		col := LaneCol(lane)
		row := publish.Row{
			Lane:           strconv.Itoa(lane),
			School:         get(col, origin+RowSchool),
			AdditionalInfo: get(col, origin+RowInfo),
			Place:          get(col, origin+RowPlace),
			Split:          get(col, origin+RowSplit),
			Time:           get(col, origin+RowTime),
		}
		if row.Place == common.EmptyString && row.Split == common.EmptyString && row.Time == common.EmptyString {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}
