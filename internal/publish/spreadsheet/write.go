package spreadsheet

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/comagnaw/regattaClock/internal/common"
	"github.com/comagnaw/regattaClock/internal/filesystem"
	"github.com/comagnaw/regattaClock/internal/publish"
)

// ErrLocked is returned (wrapped) by Write when another program - typically
// Excel on Windows - holds the workbook open, so the caller can ask the user
// to close it and retry. The previous workbook is left intact.
var ErrLocked = errors.New("results workbook is open in another program")

// Meta is the regatta-level text at the top of the sheet, plus the
// store.RegattaKey the workbook belongs to - recorded in the ledger sheet so
// a later publish can refuse to merge into another regatta's workbook.
type Meta struct {
	Name, Date, RegattaKey string
}

// LedgerEntry records one published race: the publish.Revision the public
// copy was rendered from, and when.
type LedgerEntry struct {
	Revision    string
	PublishedAt time.Time
}

// Ledger is the published-race record, keyed by race number, kept in the
// workbook's very-hidden LedgerSheetName sheet.
type Ledger map[int]LedgerEntry

const (
	borderThin   = 1
	borderMedium = 2
	fontName     = "Aptos Narrow"
	black        = "000000"
	white        = "FFFFFF"
)

// Write renders a complete results workbook to path, replacing whatever was
// there atomically (filesystem.SaveBytesFileAtomic) - the file is always
// regenerated whole, never patched. races is every block to lay out, in
// order; a race's Place/Split/Time cells are filled from its Rows, so a race
// with no Rows is an empty block. A Row with no lane, or a lane beyond
// MaxLanes, has no cell to go in and is skipped. ledger is written as given.
func Write(path string, meta Meta, races []publish.PublishableRace, ledger Ledger) error {
	f, err := render(meta, races, ledger)
	if err != nil {
		return err
	}
	defer f.Close()

	buf, err := f.WriteToBuffer()
	if err != nil {
		return fmt.Errorf("results workbook could not be rendered: %w", err)
	}
	if err := filesystem.SaveBytesFileAtomic(buf.Bytes(), path); err != nil {
		if filesystem.IsFileLocked(err) {
			return fmt.Errorf("%w: %w", ErrLocked, err)
		}
		return err
	}
	return nil
}

// sheetWriter wraps the excelize calls a render makes, keeping the first
// error and turning every later call into a no-op, so render reads as a
// layout rather than a wall of error checks.
type sheetWriter struct {
	f      *excelize.File
	sheet  string
	err    error
	styles map[styleKey]int
}

type styleKey struct {
	kind   string
	col    int
	offset int
}

func (w *sheetWriter) do(err error) {
	if w.err == nil && err != nil {
		w.err = err
	}
}

func (w *sheetWriter) str(col, row int, v string) {
	if w.err == nil && v != common.EmptyString {
		w.do(w.f.SetCellStr(w.sheet, cell(col, row), v))
	}
}

// value writes v as a number when it is a plain integer (a place, a race
// number) so Excel does not flag "number stored as text", otherwise as text.
func (w *sheetWriter) value(col, row int, v string) {
	if n, err := strconv.Atoi(v); err == nil {
		if w.err == nil {
			w.do(w.f.SetCellInt(w.sheet, cell(col, row), int64(n)))
		}
		return
	}
	w.str(col, row, v)
}

func (w *sheetWriter) merge(fromCol, fromRow, toCol, toRow int) {
	if w.err == nil {
		w.do(w.f.MergeCell(w.sheet, cell(fromCol, fromRow), cell(toCol, toRow)))
	}
}

func (w *sheetWriter) height(row int, h float64) {
	if w.err == nil {
		w.do(w.f.SetRowHeight(w.sheet, row, h))
	}
}

// style applies the cached style for key (built by build on first use) to
// (col, row).
func (w *sheetWriter) style(key styleKey, col, row int, build func() *excelize.Style) {
	if w.err != nil {
		return
	}
	id, ok := w.styles[key]
	if !ok {
		var err error
		if id, err = w.f.NewStyle(build()); err != nil {
			w.do(err)
			return
		}
		w.styles[key] = id
	}
	w.do(w.f.SetCellStyle(w.sheet, cell(col, row), cell(col, row), id))
}

func render(meta Meta, races []publish.PublishableRace, ledger Ledger) (*excelize.File, error) {
	f := excelize.NewFile()
	sheet := common.ResultsSheetName
	w := &sheetWriter{f: f, sheet: sheet, styles: map[styleKey]int{}}
	w.do(f.SetSheetName(f.GetSheetName(0), sheet))

	for i, width := range ColumnWidths {
		name, _ := excelize.ColumnNumberToName(i + 1)
		if w.err == nil {
			w.do(f.SetColWidth(sheet, name, name, width))
		}
	}

	w.merge(RaceNumCol, TitleRow, LastCol, TitleRow)
	w.str(RaceNumCol, TitleRow, meta.Name+common.ResultsTitleSuffix)
	w.merge(RaceNumCol, DateRow, LastCol, DateRow)
	w.str(RaceNumCol, DateRow, meta.Date)
	for _, row := range []int{TitleRow, DateRow} {
		w.style(styleKey{kind: "title"}, RaceNumCol, row, titleStyle)
	}
	w.height(TitleRow, 26.25)
	w.height(DateRow, 30)
	w.height(HeaderRow, 34)

	for i, label := range HeaderLabels() {
		w.str(i+1, HeaderRow, label)
		w.style(styleKey{kind: "header"}, i+1, HeaderRow, headerStyle)
	}

	for i, race := range races {
		writeBlock(w, BlockOrigin(i), race)
	}

	writeLedger(w, meta.RegattaKey, ledger)
	if w.err != nil {
		f.Close()
		return nil, fmt.Errorf("results workbook could not be built: %w", w.err)
	}
	return f, nil
}

func writeBlock(w *sheetWriter, origin int, race publish.PublishableRace) {
	last := origin + BlockRows - 1
	w.merge(RaceNumCol, origin, RaceNumCol, last)
	w.merge(TimeCol, origin, TimeCol, last)
	w.value(RaceNumCol, origin, strconv.Itoa(race.RaceNumber))
	w.str(TimeCol, origin, race.ScheduledTime)

	w.str(LabelCol, origin+RowSchool, race.BoatClass)
	w.str(LabelCol, origin+RowInfo, race.FlightInfo)
	for off := RowPlace; off <= RowTime; off++ {
		w.str(LabelCol, origin+off, BlockLabels[off])
	}

	for lane := 1; lane <= MaxLanes; lane++ {
		entry := race.Lanes[lane]
		w.str(LaneCol(lane), origin+RowSchool, entry.SchoolName)
		w.str(LaneCol(lane), origin+RowInfo, entry.AdditionalInfo)
	}
	for _, row := range race.Rows {
		lane, err := strconv.Atoi(row.Lane)
		if err != nil || lane < 1 || lane > MaxLanes {
			continue
		}
		w.value(LaneCol(lane), origin+RowPlace, row.Place)
		w.str(LaneCol(lane), origin+RowSplit, row.Split)
		w.str(LaneCol(lane), origin+RowTime, row.Time)
	}

	for off := range BlockRows {
		for col := RaceNumCol; col <= LastCol; col++ {
			key := styleKey{kind: "block", col: col, offset: off}
			w.style(key, col, origin+off, func() *excelize.Style { return blockStyle(col, off) })
		}
	}
}

// writeLedger adds the very-hidden ledger sheet: a header row, then one
// "race | revision | publishedAt" row per published race in race order, with
// the workbook's regatta key beside the header (ledgerKeyHead / its value).
func writeLedger(w *sheetWriter, regattaKey string, ledger Ledger) {
	if w.err != nil {
		return
	}
	if _, err := w.f.NewSheet(LedgerSheetName); err != nil {
		w.do(err)
		return
	}
	lw := &sheetWriter{f: w.f, sheet: LedgerSheetName, styles: w.styles}
	lw.str(1, 1, ledgerRaceHead)
	lw.str(2, 1, ledgerRevisionHead)
	lw.str(3, 1, ledgerPublishedHead)
	lw.str(ledgerKeyCol, 1, ledgerKeyHead)
	lw.str(ledgerKeyCol+1, 1, regattaKey)

	races := make([]int, 0, len(ledger))
	for n := range ledger {
		races = append(races, n)
	}
	sort.Ints(races)
	for i, n := range races {
		row := i + 2
		lw.value(1, row, strconv.Itoa(n))
		lw.str(2, row, ledger[n].Revision)
		lw.str(3, row, ledger[n].PublishedAt.UTC().Format(time.RFC3339))
	}
	w.do(lw.err)
	if w.err == nil {
		w.do(w.f.SetSheetVisible(LedgerSheetName, false, true))
	}
}

// The ledger sheet's column heads - machine-read only (never shown), so they
// live here rather than in internal/common.
const (
	ledgerRaceHead      = "race"
	ledgerRevisionHead  = "revision"
	ledgerPublishedHead = "publishedAt"
	ledgerKeyHead       = "regattaKey"
	ledgerKeyCol        = 5 // E - key label, its value in F
)

func titleStyle() *excelize.Style {
	return &excelize.Style{
		Font:      &excelize.Font{Family: fontName, Size: 18},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	}
}

func headerStyle() *excelize.Style {
	return &excelize.Style{
		Font: &excelize.Font{Family: fontName, Size: 14, Color: white},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{black}},
		Border: []excelize.Border{
			{Type: "left", Color: black, Style: borderThin},
			{Type: "right", Color: black, Style: borderThin},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	}
}

// blockStyle is the manual worksheet's block look: thin borders inside, a
// medium rule above each race and below its Time row, and medium outer edges
// on columns A and I - so each race reads as one boxed unit.
func blockStyle(col, offset int) *excelize.Style {
	left, right, top, bottom := borderThin, borderThin, borderThin, borderThin
	if col == RaceNumCol {
		left = borderMedium
	}
	if col == LastCol {
		right = borderMedium
	}
	if offset == RowSchool {
		top = borderMedium
	}
	if offset == RowTime {
		bottom = borderMedium
	}
	size := 12.0
	if col >= FirstLaneCol {
		size = 11
	}
	return &excelize.Style{
		Font: &excelize.Font{Family: fontName, Size: size},
		Border: []excelize.Border{
			{Type: "left", Color: black, Style: left},
			{Type: "right", Color: black, Style: right},
			{Type: "top", Color: black, Style: top},
			{Type: "bottom", Color: black, Style: bottom},
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: col == LabelCol},
	}
}
