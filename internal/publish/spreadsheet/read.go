package spreadsheet

import (
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/publish"
)

// ErrForeignWorkbook is returned by Read for an existing file that has no
// ledger sheet - a workbook RegattaClock did not write (say, a hand-kept
// results file with the same name in the publish folder). A caller must not
// overwrite it: regenerating would silently discard someone else's work.
var ErrForeignWorkbook = errors.New("not a RegattaClock results workbook")

// ErrOtherRegatta is returned when an existing results workbook belongs to a
// different regatta than the one being published - e.g. last year's regatta
// of the same name, in a results folder carried over from it. Merging into it
// would republish that regatta's races under this one's, so a caller must
// refuse rather than overwrite.
var ErrOtherRegatta = errors.New("results workbook belongs to a different regatta")

// Published is what an existing results workbook says has been published:
// its ledger, plus each ledger race's lane rows as they appear in the sheet.
// Rows lets a republish carry forward a race that is in the ledger but no
// longer approved, instead of blanking a result the public has already seen.
// Exists is false (and everything else empty) when there was no file.
type Published struct {
	Exists     bool
	RegattaKey string
	Ledger     Ledger
	Rows       map[int][]publish.Row
}

// CheckRegatta returns ErrOtherRegatta when an existing workbook is not
// regattaKey's. A workbook with no recorded key is treated as another
// regatta's: its provenance cannot be confirmed.
func (p Published) CheckRegatta(regattaKey string) error {
	if p.Exists && p.RegattaKey != regattaKey {
		return fmt.Errorf("%w (workbook %q, this regatta %q)", ErrOtherRegatta, p.RegattaKey, regattaKey)
	}
	return nil
}

// Read loads path's ledger and published rows. A missing file is not an
// error - nothing has been published yet - and returns an empty Published.
func Read(path string) (Published, error) {
	out := Published{Ledger: Ledger{}, Rows: map[int][]publish.Row{}}

	f, err := excelize.OpenFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
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
